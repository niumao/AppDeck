package services

import (
	"appdeck/database"
	"appdeck/models"
	"log"
	"time"

	"github.com/google/uuid"
)

type AppLogService struct{}

func NewAppLogService() *AppLogService {
	return &AppLogService{}
}

// CreateAppLog creates a new app log entry. Auto-closes any prior open log with reason "user_switch".
func (s *AppLogService) CreateAppLog(sessionID, appID, appName string) error {
	// Close any open app log for this session first
	s.CloseOpenAppLogs(sessionID, "user_switch")

	now := time.Now()
	appLog := models.SessionAppLog{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		AppID:      appID,
		AppName:    appName,
		LaunchedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return database.DB.Create(&appLog).Error
}

// EndAppLog closes the open app log for this session with the given reason.
func (s *AppLogService) EndAppLog(sessionID, reason string) error {
	var appLog models.SessionAppLog
	err := database.DB.Where("session_id = ? AND quit_at IS NULL", sessionID).
		Order("launched_at DESC").
		First(&appLog).Error
	if err != nil {
		return err // No open log found
	}

	now := time.Now()
	durationSec := int(now.Sub(appLog.LaunchedAt).Seconds())

	return database.DB.Model(&appLog).Updates(map[string]interface{}{
		"quit_at":      now,
		"quit_reason":  reason,
		"duration_sec": durationSec,
		"updated_at":   now,
	}).Error
}

// CloseOpenAppLogs closes all open app logs for a session (used on logout/session end).
func (s *AppLogService) CloseOpenAppLogs(sessionID, reason string) {
	now := time.Now()
	database.DB.Model(&models.SessionAppLog{}).
		Where("session_id = ? AND quit_at IS NULL", sessionID).
		Updates(map[string]interface{}{
			"quit_at":     now,
			"quit_reason": reason,
			"updated_at":  now,
		})
}

// CloseOrphanedAppLogs closes all app logs left open from a previous server run.
func (s *AppLogService) CloseOrphanedAppLogs() {
	now := time.Now()
	result := database.DB.Model(&models.SessionAppLog{}).
		Where("quit_at IS NULL").
		Updates(map[string]interface{}{
			"quit_at":     now,
			"quit_reason": "server_restart",
			"updated_at":  now,
		})
	if result.RowsAffected > 0 {
		log.Printf("Closed %d orphaned app logs on startup", result.RowsAffected)
	}
}
