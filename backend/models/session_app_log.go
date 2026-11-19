package models

import "time"

type SessionAppLog struct {
	ID          string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SessionID   string     `gorm:"column:session_id;size:36;not null;index" json:"session_id"`
	AppID       string     `gorm:"column:app_id;size:36;not null;index" json:"app_id"`
	AppName     string     `gorm:"column:app_name;size:100;not null" json:"app_name"`
	LaunchedAt  time.Time  `gorm:"column:launched_at;not null" json:"launched_at"`
	QuitAt      *time.Time `gorm:"column:quit_at" json:"quit_at"`
	QuitReason  string     `gorm:"column:quit_reason;size:20" json:"quit_reason"`
	DurationSec int        `gorm:"column:duration_sec;default:0" json:"duration_sec"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (SessionAppLog) TableName() string {
	return "session_app_logs"
}
