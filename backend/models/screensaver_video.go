package models

import "time"

type ScreensaverVideo struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Filename     string    `gorm:"size:255;not null" json:"filename"`
	OriginalName string    `gorm:"size:255;not null" json:"original_name"`
	ContentType  string    `gorm:"size:50;not null" json:"content_type"`
	Size         int64     `gorm:"not null" json:"size"`
	Hash         string    `gorm:"size:64;not null" json:"hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ScreensaverVideo) TableName() string { return "screensaver_videos" }
