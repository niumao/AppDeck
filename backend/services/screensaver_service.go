package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ScreensaverService struct{}

var screensaverServiceInstance *ScreensaverService

// NewScreensaverService returns the singleton screensaver service instance
func NewScreensaverService() *ScreensaverService {
	if screensaverServiceInstance == nil {
		screensaverServiceInstance = &ScreensaverService{}
	}
	return screensaverServiceInstance
}

// GetScreensaverService returns the singleton instance
func GetScreensaverService() *ScreensaverService {
	return NewScreensaverService()
}

// InitializeDefaults ensures default screensaver settings exist in database
func (s *ScreensaverService) InitializeDefaults() error {
	// Check and create default enabled setting
	setting, err := s.GetSetting("screensaver_enabled")
	if err != nil {
		return err
	}
	if setting == nil {
		if err := s.SetSetting("screensaver_enabled", "false"); err != nil {
			return err
		}
	}

	// Check and create default timeout setting
	setting, err = s.GetSetting("screensaver_timeout")
	if err != nil {
		return err
	}
	if setting == nil {
		if err := s.SetSetting("screensaver_timeout", "5"); err != nil {
			return err
		}
	}

	return nil
}

// GetSetting retrieves a setting by key
func (s *ScreensaverService) GetSetting(key string) (*models.Setting, error) {
	var setting models.Setting
	err := database.DB.Where("`key` = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil for not found (not an error)
		}
		return nil, err
	}
	return &setting, nil
}

// SetSetting creates or updates a setting
func (s *ScreensaverService) SetSetting(key, value string) error {
	var setting models.Setting
	err := database.DB.Where("`key` = ?", key).First(&setting).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new setting
			setting = models.Setting{
				ID:    uuid.New().String(),
				Key:   key,
				Value: value,
			}
			return database.DB.Create(&setting).Error
		}
		return err
	}

	// Update existing setting
	setting.Value = value
	return database.DB.Save(&setting).Error
}

// GetEnabled retrieves the screensaver enabled status (default: false)
func (s *ScreensaverService) GetEnabled() bool {
	setting, err := s.GetSetting("screensaver_enabled")
	if err != nil || setting == nil {
		return false // Default: disabled
	}

	return setting.Value == "true" || setting.Value == "1"
}

// SetEnabled sets the screensaver enabled status
func (s *ScreensaverService) SetEnabled(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return s.SetSetting("screensaver_enabled", value)
}

// GetTimeout retrieves the screensaver timeout in minutes (default: 5)
func (s *ScreensaverService) GetTimeout() int {
	setting, err := s.GetSetting("screensaver_timeout")
	if err != nil || setting == nil {
		return 5 // Default: 5 minutes
	}

	timeout, err := strconv.Atoi(setting.Value)
	if err != nil || timeout < 1 || timeout > 60 {
		return 5 // Fallback to default if invalid
	}

	return timeout
}

// SetTimeout sets the screensaver timeout in minutes (1-60)
func (s *ScreensaverService) SetTimeout(timeout int) error {
	if timeout < 1 || timeout > 60 {
		return errors.New("timeout must be between 1 and 60 minutes")
	}
	return s.SetSetting("screensaver_timeout", strconv.Itoa(timeout))
}

// GetVideoHash retrieves the SHA256 hash of the current screensaver video
func (s *ScreensaverService) GetVideoHash() (string, error) {
	setting, err := s.GetSetting("screensaver_video_hash")
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil // No video uploaded yet
	}
	return setting.Value, nil
}

// SetVideoHash stores the SHA256 hash of the screensaver video
func (s *ScreensaverService) SetVideoHash(hash string) error {
	return s.SetSetting("screensaver_video_hash", hash)
}

// SaveVideo saves the uploaded video file and returns its SHA256 hash
func (s *ScreensaverService) SaveVideo(uploadDir string, fileData io.Reader, contentType string) (string, error) {
	// Create upload directory if it doesn't exist
	videoDir := filepath.Join(uploadDir, "screensaver")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return "", err
	}

	// Determine file extension from content type
	ext := ".mp4"
	if contentType == "video/x-matroska" {
		ext = ".mkv"
	}

	// Save file to disk
	videoPath := filepath.Join(videoDir, "video"+ext)

	// Remove old video files (both .mp4 and .mkv)
	os.Remove(filepath.Join(videoDir, "video.mp4"))
	os.Remove(filepath.Join(videoDir, "video.mkv"))

	outFile, err := os.Create(videoPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Copy data and compute SHA256 hash simultaneously
	hash := sha256.New()
	multiWriter := io.MultiWriter(outFile, hash)

	if _, err := io.Copy(multiWriter, fileData); err != nil {
		os.Remove(videoPath) // Cleanup on failure
		return "", err
	}

	// Get hash as hex string
	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	// Store hash and content type in database
	if err := s.SetVideoHash(hashString); err != nil {
		return "", err
	}
	if err := s.SetSetting("screensaver_video_type", contentType); err != nil {
		return "", err
	}

	return hashString, nil
}

// GetVideoContentType retrieves the content type of the uploaded video
func (s *ScreensaverService) GetVideoContentType() string {
	setting, err := s.GetSetting("screensaver_video_type")
	if err != nil || setting == nil {
		return "video/mp4" // Default
	}
	return setting.Value
}

// GetVideoPath returns the path to the screensaver video file
func (s *ScreensaverService) GetVideoPath(uploadDir string) (string, string) {
	videoDir := filepath.Join(uploadDir, "screensaver")

	// Check for .mp4 first
	mp4Path := filepath.Join(videoDir, "video.mp4")
	if _, err := os.Stat(mp4Path); err == nil {
		return mp4Path, "video/mp4"
	}

	// Check for .mkv
	mkvPath := filepath.Join(videoDir, "video.mkv")
	if _, err := os.Stat(mkvPath); err == nil {
		return mkvPath, "video/x-matroska"
	}

	// Return mp4 path as default (even if doesn't exist)
	return mp4Path, "video/mp4"
}

// HasVideo checks if a screensaver video exists
func (s *ScreensaverService) HasVideo(uploadDir string) bool {
	videoPath, _ := s.GetVideoPath(uploadDir)
	_, err := os.Stat(videoPath)
	return err == nil
}

// SaveVideoToLibrary saves an uploaded video to the library and returns the model
func (s *ScreensaverService) SaveVideoToLibrary(iconsDir string, fileData io.Reader, originalName, contentType string) (*models.ScreensaverVideo, error) {
	// Determine file extension from content type
	ext := ".mp4"
	if contentType == "video/x-matroska" {
		ext = ".mkv"
	}

	// Generate UUID for filename
	videoID := uuid.New().String()
	filename := videoID + ext

	// Create directory if needed
	videoDir := filepath.Join(iconsDir, "screensaver")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		return nil, err
	}

	// Save file to disk
	videoPath := filepath.Join(videoDir, filename)
	outFile, err := os.Create(videoPath)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	// Compute SHA256 hash while writing
	hash := sha256.New()
	multiWriter := io.MultiWriter(outFile, hash)

	written, err := io.Copy(multiWriter, fileData)
	if err != nil {
		os.Remove(videoPath) // Cleanup on failure
		return nil, err
	}

	// Get hash as hex string
	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	// Create ScreensaverVideo record
	video := &models.ScreensaverVideo{
		ID:           videoID,
		Filename:     filename,
		OriginalName: originalName,
		ContentType:  contentType,
		Size:         written,
		Hash:         hashString,
		CreatedAt:    time.Now(),
	}

	if err := database.DB.Create(video).Error; err != nil {
		os.Remove(videoPath) // Cleanup on failure
		return nil, err
	}

	return video, nil
}

// ListVideos returns all screensaver videos ordered by creation date
func (s *ScreensaverService) ListVideos() ([]models.ScreensaverVideo, error) {
	var videos []models.ScreensaverVideo
	err := database.DB.Order("created_at DESC").Find(&videos).Error
	return videos, err
}

// GetVideoByID retrieves a video by ID
func (s *ScreensaverService) GetVideoByID(id string) (*models.ScreensaverVideo, error) {
	var video models.ScreensaverVideo
	err := database.DB.Where("id = ?", id).First(&video).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &video, nil
}

// DeleteVideo deletes a video from the library
func (s *ScreensaverService) DeleteVideo(id string, iconsDir string) error {
	// Check if video is the global default
	defaultID, err := s.GetDefaultVideoID()
	if err != nil {
		return err
	}
	if defaultID == id {
		return fmt.Errorf("cannot delete: video is set as global default")
	}

	// Check if any TenantScreensaver references this video
	var count int64
	if err := database.DB.Model(&models.TenantScreensaver{}).Where("video_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete: video is assigned to %d tenant(s)", count)
	}

	// Get the video record to find the filename
	video, err := s.GetVideoByID(id)
	if err != nil {
		return err
	}
	if video == nil {
		return fmt.Errorf("video not found")
	}

	// Delete file from disk
	videoPath := filepath.Join(iconsDir, "screensaver", video.Filename)
	if err := os.Remove(videoPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Delete DB record
	return database.DB.Delete(&models.ScreensaverVideo{}, "id = ?", id).Error
}

// GetVideoFilePath returns the full path to a video file
func (s *ScreensaverService) GetVideoFilePath(video *models.ScreensaverVideo, iconsDir string) string {
	return filepath.Join(iconsDir, "screensaver", video.Filename)
}

// GetDefaultVideoID returns the ID of the global default video
func (s *ScreensaverService) GetDefaultVideoID() (string, error) {
	setting, err := s.GetSetting("screensaver_default_video_id")
	if err != nil {
		return "", err
	}
	if setting == nil {
		return "", nil
	}
	return setting.Value, nil
}

// SetDefaultVideoID sets the global default video
func (s *ScreensaverService) SetDefaultVideoID(videoID string) error {
	// Verify video exists
	video, err := s.GetVideoByID(videoID)
	if err != nil {
		return err
	}
	if video == nil {
		return fmt.Errorf("video not found")
	}

	return s.SetSetting("screensaver_default_video_id", videoID)
}

// ListTenantScreensavers returns all tenant screensaver configs
func (s *ScreensaverService) ListTenantScreensavers() ([]models.TenantScreensaver, error) {
	var configs []models.TenantScreensaver
	err := database.DB.Preload("Tenant").Preload("Video").Order("created_at DESC").Find(&configs).Error
	return configs, err
}

// GetTenantScreensaver retrieves a tenant's screensaver config
func (s *ScreensaverService) GetTenantScreensaver(tenantID string) (*models.TenantScreensaver, error) {
	var config models.TenantScreensaver
	err := database.DB.Where("tenant_id = ?", tenantID).Preload("Tenant").Preload("Video").First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// SetTenantScreensaver creates or updates a tenant screensaver config
func (s *ScreensaverService) SetTenantScreensaver(tenantID, videoID, startDate, endDate, startTime, endTime string) error {
	// Verify video exists
	video, err := s.GetVideoByID(videoID)
	if err != nil {
		return err
	}
	if video == nil {
		return fmt.Errorf("video not found")
	}

	// Try to find existing config
	existing, err := s.GetTenantScreensaver(tenantID)
	if err != nil {
		return err
	}

	if existing != nil {
		// Update existing
		existing.VideoID = videoID
		existing.StartDate = startDate
		existing.EndDate = endDate
		existing.StartTime = startTime
		existing.EndTime = endTime
		return database.DB.Save(existing).Error
	}

	// Create new
	config := &models.TenantScreensaver{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		VideoID:   videoID,
		StartDate: startDate,
		EndDate:   endDate,
		StartTime: startTime,
		EndTime:   endTime,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return database.DB.Create(config).Error
}

// DeleteTenantScreensaver deletes a tenant screensaver config
func (s *ScreensaverService) DeleteTenantScreensaver(tenantID string) error {
	return database.DB.Where("tenant_id = ?", tenantID).Delete(&models.TenantScreensaver{}).Error
}

// GetClientScreensaverConfig builds the screensaver config for a specific client
func (s *ScreensaverService) GetClientScreensaverConfig(machineCode string) map[string]interface{} {
	result := make(map[string]interface{})

	// Get global settings
	result["enabled"] = s.GetEnabled()
	result["timeout"] = s.GetTimeout()
	result["has_video"] = false // backward compat

	// Get default video
	defaultVideoID, _ := s.GetDefaultVideoID()
	var defaultVideoHash string
	if defaultVideoID != "" {
		if video, _ := s.GetVideoByID(defaultVideoID); video != nil {
			defaultVideoHash = video.Hash
			result["has_video"] = true
		}
	}

	result["default_video_id"] = defaultVideoID
	result["default_video_hash"] = defaultVideoHash
	result["video_hash"] = defaultVideoHash // backward compat

	// Look up device and tenant
	var device models.RegisteredDevice
	err := database.DB.Where("machine_code = ? AND status = 'approved'", machineCode).First(&device).Error
	if err == nil && device.TenantID != nil {
		// Get tenant screensaver config
		tenantConfig, _ := s.GetTenantScreensaver(*device.TenantID)
		if tenantConfig != nil {
			result["tenant_video_id"] = tenantConfig.VideoID
			if tenantConfig.Video != nil {
				result["tenant_video_hash"] = tenantConfig.Video.Hash
			}
			result["start_date"] = tenantConfig.StartDate
			result["end_date"] = tenantConfig.EndDate
			result["start_time"] = tenantConfig.StartTime
			result["end_time"] = tenantConfig.EndTime
		}
	}

	return result
}
