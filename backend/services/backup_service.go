package services

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"
)

type BackupInfo struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type BackupService struct {
	dbConfig  *config.DatabaseConfig
	backupDir string
	mu        sync.Mutex
	ticker    *time.Ticker
	stopCh    chan struct{}
}

var (
	backupServiceInstance *BackupService
	backupServiceOnce     sync.Once
)

func InitBackupService(dbConfig *config.DatabaseConfig, backupDir string) *BackupService {
	backupServiceOnce.Do(func() {
		backupServiceInstance = &BackupService{
			dbConfig:  dbConfig,
			backupDir: backupDir,
			stopCh:    make(chan struct{}),
		}
	})
	return backupServiceInstance
}

func GetBackupService() *BackupService {
	return backupServiceInstance
}

func (s *BackupService) CreateBackup() (*BackupInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("backup-%s.sql", timestamp)
	filePath := filepath.Join(s.backupDir, filename)

	cmd := exec.Command("mysqldump",
		"-h", s.dbConfig.Host,
		"-P", s.dbConfig.Port,
		"-u", s.dbConfig.User,
		"--protocol=tcp",
		"--single-transaction",
		"--routines",
		"--triggers",
		s.dbConfig.DBName,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", s.dbConfig.Password))

	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("mysqldump failed: %s", stderr.String())
	}

	now := time.Now()
	database.DB.Model(&models.BackupSchedule{}).Where("1=1").Update("last_backup_at", &now)

	s.cleanOldBackups()

	stat, _ := os.Stat(filePath)
	return &BackupInfo{
		Filename:  filename,
		Size:      stat.Size(),
		CreatedAt: now,
	}, nil
}

func (s *BackupService) UploadBackup(reader io.Reader, originalFilename string) (*BackupInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate filename ends with .sql
	if !strings.HasSuffix(originalFilename, ".sql") {
		return nil, fmt.Errorf("invalid file type: must be a .sql file")
	}

	// Generate new filename with upload- prefix
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("upload-%s.sql", timestamp)
	filePath := filepath.Join(s.backupDir, filename)

	// Create the file
	outFile, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload file: %w", err)
	}
	defer outFile.Close()

	// Copy content from reader to file
	size, err := io.Copy(outFile, reader)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save upload file: %w", err)
	}

	now := time.Now()
	return &BackupInfo{
		Filename:  filename,
		Size:      size,
		CreatedAt: now,
	}, nil
}

func (s *BackupService) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		createdAt := info.ModTime()
		name := strings.TrimSuffix(entry.Name(), ".sql")
		name = strings.TrimPrefix(name, "backup-")
		name = strings.TrimPrefix(name, "upload-")
		if t, err := time.ParseInLocation("20060102-150405", name, time.Local); err == nil {
			createdAt = t
		}
		backups = append(backups, BackupInfo{
			Filename:  entry.Name(),
			Size:      info.Size(),
			CreatedAt: createdAt,
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

func (s *BackupService) RestoreBackup(filename string) error {
	if err := s.validateFilename(filename); err != nil {
		return err
	}

	filePath := filepath.Join(s.backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filename)
	}

	inFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer inFile.Close()

	cmd := exec.Command("mysql",
		"-h", s.dbConfig.Host,
		"-P", s.dbConfig.Port,
		"-u", s.dbConfig.User,
		"--protocol=tcp",
		s.dbConfig.DBName,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", s.dbConfig.Password))
	cmd.Stdin = inFile

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %s", stderr.String())
	}

	return nil
}

func (s *BackupService) DeleteBackup(filename string) error {
	if err := s.validateFilename(filename); err != nil {
		return err
	}

	filePath := filepath.Join(s.backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filename)
	}

	return os.Remove(filePath)
}

func (s *BackupService) DownloadBackupPath(filename string) (string, error) {
	if err := s.validateFilename(filename); err != nil {
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

func (s *BackupService) validateFilename(filename string) error {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	if !strings.HasSuffix(filename, ".sql") {
		return fmt.Errorf("invalid filename: must end with .sql")
	}
	return nil
}

func (s *BackupService) cleanOldBackups() {
	var schedule models.BackupSchedule
	if err := database.DB.First(&schedule).Error; err != nil {
		return
	}

	backups, err := s.ListBackups()
	if err != nil {
		return
	}

	maxBackups := schedule.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 20
	}

	if len(backups) <= maxBackups {
		return
	}

	for i := maxBackups; i < len(backups); i++ {
		path := filepath.Join(s.backupDir, backups[i].Filename)
		if err := os.Remove(path); err != nil {
			log.Printf("Failed to delete old backup %s: %v", backups[i].Filename, err)
		} else {
			log.Printf("Deleted old backup: %s", backups[i].Filename)
		}
	}
}

func (s *BackupService) GetSchedule() (*models.BackupSchedule, error) {
	var schedule models.BackupSchedule
	result := database.DB.First(&schedule)
	if result.Error != nil {
		schedule = models.BackupSchedule{
			Enabled:       false,
			IntervalHours: 24,
			MaxBackups:    20,
		}
		if err := database.DB.Create(&schedule).Error; err != nil {
			return nil, fmt.Errorf("failed to create default schedule: %w", err)
		}
	}
	return &schedule, nil
}

func (s *BackupService) UpdateSchedule(enabled bool, intervalHours int, maxBackups int) (*models.BackupSchedule, error) {
	schedule, err := s.GetSchedule()
	if err != nil {
		return nil, err
	}

	if intervalHours < 1 {
		intervalHours = 1
	}
	if maxBackups < 1 {
		maxBackups = 1
	}
	if maxBackups > 100 {
		maxBackups = 100
	}

	schedule.Enabled = enabled
	schedule.IntervalHours = intervalHours
	schedule.MaxBackups = maxBackups

	if err := database.DB.Save(schedule).Error; err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	return schedule, nil
}

func (s *BackupService) StartScheduler() {
	s.ticker = time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.checkAndRunScheduledBackup()
			case <-s.stopCh:
				s.ticker.Stop()
				return
			}
		}
	}()
	log.Println("Backup scheduler started")
}

func (s *BackupService) StopScheduler() {
	select {
	case s.stopCh <- struct{}{}:
	default:
	}
}

func (s *BackupService) checkAndRunScheduledBackup() {
	schedule, err := s.GetSchedule()
	if err != nil || !schedule.Enabled {
		return
	}

	if schedule.LastBackupAt != nil {
		elapsed := time.Since(*schedule.LastBackupAt)
		if elapsed < time.Duration(schedule.IntervalHours)*time.Hour {
			return
		}
	}

	log.Println("Running scheduled backup...")
	info, err := s.CreateBackup()
	if err != nil {
		log.Printf("Scheduled backup failed: %v", err)
		return
	}
	log.Printf("Scheduled backup created: %s (%d bytes)", info.Filename, info.Size)
}
