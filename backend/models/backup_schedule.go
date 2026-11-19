package models

import "time"

type BackupSchedule struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Enabled       bool       `json:"enabled" gorm:"default:false"`
	IntervalHours int        `json:"interval_hours" gorm:"default:24"`
	MaxBackups    int        `json:"max_backups" gorm:"default:20"`
	LastBackupAt  *time.Time `json:"last_backup_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
