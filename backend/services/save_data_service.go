package services

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
)

// SaveBlobStore stores and retrieves save-data blobs by key.
// LocalSaveStore is the only implementation today; an S3-backed
// implementation can satisfy this same interface later without any
// change to SaveDataService.
type SaveBlobStore interface {
	Put(key string, r io.Reader) error
	Get(key string) (io.ReadCloser, error)
	Delete(key string) error
}

// LocalSaveStore stores save blobs as files under a base directory.
type LocalSaveStore struct {
	baseDir string
}

func NewLocalSaveStore(baseDir string) *LocalSaveStore {
	return &LocalSaveStore{baseDir: baseDir}
}

func (l *LocalSaveStore) Put(key string, r io.Reader) error {
	path := filepath.Join(l.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create save dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create save file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to write save file: %w", err)
	}
	return nil
}

func (l *LocalSaveStore) Get(key string) (io.ReadCloser, error) {
	path := filepath.Join(l.baseDir, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("save not found: %w", err)
	}
	return f, nil
}

func (l *LocalSaveStore) Delete(key string) error {
	return os.Remove(filepath.Join(l.baseDir, key))
}

// buildSaveStore resolves a SaveStorageConfig into a SaveBlobStore.
// "s3" is accepted as a config value (so the admin UI can offer it) but
// isn't implemented yet, so it falls back to local storage.
func buildSaveStore(cfg *models.SaveStorageConfig) SaveBlobStore {
	return NewLocalSaveStore(cfg.LocalPath)
}

func computeChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SaveDataService stores per-(user, app) save-data blobs behind a SaveBlobStore.
type SaveDataService struct {
	mu        sync.RWMutex
	store     SaveBlobStore
	backupDir string
}

var (
	saveDataServiceInstance *SaveDataService
	saveDataServiceOnce     sync.Once
)

func InitSaveDataService(store SaveBlobStore, backupDir string) *SaveDataService {
	saveDataServiceOnce.Do(func() {
		saveDataServiceInstance = &SaveDataService{store: store, backupDir: backupDir}
	})
	return saveDataServiceInstance
}

func GetSaveDataService() *SaveDataService {
	return saveDataServiceInstance
}

// getStore returns the currently active blob store (read-locked so it's
// safe to read concurrently with an admin-triggered SetStore swap).
func (s *SaveDataService) getStore() SaveBlobStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store
}

// SetStore swaps the active blob store, e.g. after an admin updates the
// storage config. Takes effect immediately for subsequent uploads/downloads.
func (s *SaveDataService) SetStore(store SaveBlobStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
}

// GetStorageConfig returns the admin-configured save storage settings,
// creating a default (local, "./data/saves") row if none exists yet.
func (s *SaveDataService) GetStorageConfig() (*models.SaveStorageConfig, error) {
	var cfg models.SaveStorageConfig
	result := database.DB.First(&cfg)
	if result.Error != nil {
		cfg = models.SaveStorageConfig{
			Enabled:     true,
			StorageType: "local",
			LocalPath:   "./data/saves",
		}
		if err := database.DB.Create(&cfg).Error; err != nil {
			return nil, fmt.Errorf("failed to create default storage config: %w", err)
		}
	}
	return &cfg, nil
}

// IsEnabled reports whether game-save sync is currently turned on. Both the
// session-end upload trigger and the client-facing upload/download
// endpoints check this, so toggling it off stops the feature end-to-end.
// Defaults to enabled if the config can't be read for some reason -- a
// storage hiccup shouldn't silently disable a feature nobody turned off.
func (s *SaveDataService) IsEnabled() bool {
	cfg, err := s.GetStorageConfig()
	if err != nil {
		return true
	}
	return cfg.Enabled
}

// UpdateStorageConfig persists new storage settings and immediately swaps
// the active store to match. storageType "s3" is accepted but resolves to
// local storage until an S3-backed SaveBlobStore is implemented.
func (s *SaveDataService) UpdateStorageConfig(enabled bool, storageType, localPath, s3Bucket, s3Region, s3Prefix string) (*models.SaveStorageConfig, error) {
	if storageType != "local" && storageType != "s3" {
		return nil, fmt.Errorf("invalid storage type: %s", storageType)
	}
	if localPath == "" {
		localPath = "./data/saves"
	}

	cfg, err := s.GetStorageConfig()
	if err != nil {
		return nil, err
	}

	cfg.Enabled = enabled
	cfg.StorageType = storageType
	cfg.LocalPath = localPath
	cfg.S3Bucket = s3Bucket
	cfg.S3Region = s3Region
	cfg.S3Prefix = s3Prefix

	if err := database.DB.Save(cfg).Error; err != nil {
		return nil, fmt.Errorf("failed to update storage config: %w", err)
	}

	s.SetStore(buildSaveStore(cfg))
	return cfg, nil
}

// UploadUserSave stores a user's whole save bundle (every Eden title they
// have progress in, zipped together). If the content's checksum matches
// what's already stored, it skips the write entirely.
func (s *SaveDataService) UploadUserSave(userID string, data []byte) (*models.UserSaveData, error) {
	checksum := computeChecksum(data)

	var existing models.UserSaveData
	err := database.DB.Where("user_id = ?", userID).First(&existing).Error
	if err == nil && existing.Checksum == checksum {
		return &existing, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	key := fmt.Sprintf("%s.zip", userID)
	if err := s.getStore().Put(key, bytes.NewReader(data)); err != nil {
		return nil, err
	}

	now := time.Now()
	record := existing
	record.UserID = userID
	record.StorageKey = key
	record.SizeBytes = int64(len(data))
	record.Checksum = checksum
	record.UpdatedAt = now
	if record.ID == 0 {
		record.CreatedAt = now
		if err := database.DB.Create(&record).Error; err != nil {
			return nil, err
		}
	} else {
		if err := database.DB.Save(&record).Error; err != nil {
			return nil, err
		}
	}

	return &record, nil
}

// DownloadUserSave retrieves a user's stored save bundle.
// Returns (nil, nil, nil) if no save has been uploaded yet.
func (s *SaveDataService) DownloadUserSave(userID string) (io.ReadCloser, *models.UserSaveData, error) {
	var record models.UserSaveData
	err := database.DB.Where("user_id = ?", userID).First(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	rc, err := s.getStore().Get(record.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return rc, &record, nil
}

// SaveBackupInfo describes a full save-data backup archive, containing
// every user's save for every game in one file -- not scoped to a
// particular user or game, same shape as a database backup.
type SaveBackupInfo struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SaveDataService) validateBackupFilename(filename string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	if !strings.HasSuffix(filename, ".zip") {
		return fmt.Errorf("invalid filename: must end with .zip")
	}
	return nil
}

// CreateFullBackup bundles every stored save-data blob plus a manifest of
// their DB metadata into one zip archive -- all users, all games.
func (s *SaveDataService) CreateFullBackup() (*SaveBackupInfo, error) {
	var rows []models.UserSaveData
	if err := database.DB.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list save records: %w", err)
	}

	if err := os.MkdirAll(s.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup dir: %w", err)
	}
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("save-backup-%s.zip", timestamp)
	filePath := filepath.Join(s.backupDir, filename)

	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	zw := zip.NewWriter(outFile)

	manifestBytes, err := json.Marshal(rows)
	if err != nil {
		zw.Close()
		outFile.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to build manifest: %w", err)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		zw.Close()
		outFile.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}
	if _, err := mw.Write(manifestBytes); err != nil {
		zw.Close()
		outFile.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	store := s.getStore()
	for _, r := range rows {
		rc, err := store.Get(r.StorageKey)
		if err != nil {
			log.Printf("save backup: skipping missing blob %s: %v", r.StorageKey, err)
			continue
		}
		bw, err := zw.Create("blobs/" + r.StorageKey)
		if err == nil {
			io.Copy(bw, rc)
		}
		rc.Close()
	}

	if err := zw.Close(); err != nil {
		outFile.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to finalize backup: %w", err)
	}
	outFile.Close()

	stat, _ := os.Stat(filePath)
	return &SaveBackupInfo{Filename: filename, Size: stat.Size(), CreatedAt: time.Now()}, nil
}

// ListFullBackups lists save-data backup archives, newest first.
func (s *SaveDataService) ListFullBackups() ([]SaveBackupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SaveBackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []SaveBackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, SaveBackupInfo{Filename: entry.Name(), Size: info.Size(), CreatedAt: info.ModTime()})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})
	return backups, nil
}

// DownloadFullBackupPath resolves a backup archive's path for download.
func (s *SaveDataService) DownloadFullBackupPath(filename string) (string, error) {
	if err := s.validateBackupFilename(filename); err != nil {
		return "", err
	}
	filePath := filepath.Join(s.backupDir, filename)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("backup file not found: %s", filename)
	}
	return absPath, nil
}

// DeleteFullBackup removes a backup archive.
func (s *SaveDataService) DeleteFullBackup(filename string) error {
	if err := s.validateBackupFilename(filename); err != nil {
		return err
	}
	filePath := filepath.Join(s.backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filename)
	}
	return os.Remove(filePath)
}

// UploadFullBackup stores an admin-provided backup archive for later
// restore. Does not apply it -- same as database backup's upload.
func (s *SaveDataService) UploadFullBackup(reader io.Reader, originalFilename string) (*SaveBackupInfo, error) {
	if !strings.HasSuffix(originalFilename, ".zip") {
		return nil, fmt.Errorf("invalid file type: must be a .zip file")
	}
	if err := os.MkdirAll(s.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("upload-%s.zip", timestamp)
	filePath := filepath.Join(s.backupDir, filename)

	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload file: %w", err)
	}
	defer outFile.Close()

	size, err := io.Copy(outFile, reader)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save upload file: %w", err)
	}

	return &SaveBackupInfo{Filename: filename, Size: size, CreatedAt: time.Now()}, nil
}

// RestoreFullBackup applies a backup archive: every blob it contains is
// written back to the active store, and every manifest record is upserted
// into user_save_data by user_id -- splitting the one bundle archive back
// into individual per-user blobs so ordinary client downloads keep working.
func (s *SaveDataService) RestoreFullBackup(filename string) error {
	if err := s.validateBackupFilename(filename); err != nil {
		return err
	}
	filePath := filepath.Join(s.backupDir, filename)

	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer zr.Close()

	var manifest []models.UserSaveData
	blobs := make(map[string][]byte)

	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to read manifest: %w", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return fmt.Errorf("failed to read manifest: %w", err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return fmt.Errorf("failed to parse manifest: %w", err)
			}
		} else if strings.HasPrefix(f.Name, "blobs/") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			blobs[strings.TrimPrefix(f.Name, "blobs/")] = data
		}
	}

	store := s.getStore()
	for _, rec := range manifest {
		data, ok := blobs[rec.StorageKey]
		if !ok {
			log.Printf("save restore: missing blob for %s, skipping", rec.StorageKey)
			continue
		}
		if err := store.Put(rec.StorageKey, bytes.NewReader(data)); err != nil {
			log.Printf("save restore: failed to write blob %s: %v", rec.StorageKey, err)
			continue
		}

		var existing models.UserSaveData
		row := rec
		if database.DB.Where("user_id = ?", rec.UserID).First(&existing).Error == nil {
			row.ID = existing.ID
			row.CreatedAt = existing.CreatedAt
			database.DB.Save(&row)
		} else {
			row.ID = 0
			database.DB.Create(&row)
		}
	}

	return nil
}

// SaveDataWithDetails is a UserSaveData row enriched with display info for
// the admin save-storage listing.
type SaveDataWithDetails struct {
	models.UserSaveData
	UserNickname string `json:"userNickname"`
}

// ListSaves returns every stored save-data record (one per user), newest
// first, with the user nickname resolved for display.
func (s *SaveDataService) ListSaves() ([]SaveDataWithDetails, error) {
	var rows []models.UserSaveData
	if err := database.DB.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SaveDataWithDetails{}, nil
	}

	userIDs := make([]string, 0, len(rows))
	seenUser := map[string]bool{}
	for _, r := range rows {
		if !seenUser[r.UserID] {
			userIDs = append(userIDs, r.UserID)
			seenUser[r.UserID] = true
		}
	}

	var users []models.User
	database.DB.Select("id, nickname").Where("id IN ?", userIDs).Find(&users)
	userNicknames := make(map[string]string, len(users))
	for _, u := range users {
		userNicknames[u.ID] = u.Nickname
	}

	result := make([]SaveDataWithDetails, 0, len(rows))
	for _, r := range rows {
		result = append(result, SaveDataWithDetails{
			UserSaveData: r,
			UserNickname: userNicknames[r.UserID],
		})
	}
	return result, nil
}
