package models

import "time"

type TenantScreensaver struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID  string    `gorm:"size:36;not null;uniqueIndex" json:"tenant_id"`
	VideoID   string    `gorm:"size:36;not null" json:"video_id"`
	StartDate string    `gorm:"size:10" json:"start_date"`
	EndDate   string    `gorm:"size:10" json:"end_date"`
	StartTime string    `gorm:"size:5" json:"start_time"`
	EndTime   string    `gorm:"size:5" json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Tenant *Tenant           `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Video  *ScreensaverVideo `gorm:"foreignKey:VideoID" json:"video,omitempty"`
}

func (TenantScreensaver) TableName() string { return "tenant_screensavers" }
