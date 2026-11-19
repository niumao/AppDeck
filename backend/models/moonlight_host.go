package models

import (
	"time"
)

// MoonlightHost represents a Moonlight streaming host in the pool
type MoonlightHost struct {
	ID                 string     `gorm:"type:char(36);primaryKey" json:"id"`
	Host               string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_host_port" json:"host"`
	Port               int        `gorm:"not null;default:0;uniqueIndex:idx_host_port" json:"port"`
	Enabled            bool       `gorm:"not null;default:true;index:idx_enabled_occupied" json:"enabled"`
	Paired             bool       `gorm:"not null;default:false" json:"paired"`
	SunshineUser       string     `gorm:"type:varchar(100);not null;default:''" json:"-"`
	SunshinePass       string     `gorm:"type:varchar(255);not null;default:''" json:"-"`
	OccupiedByDeviceID *string    `gorm:"type:varchar(100);index;index:idx_enabled_occupied" json:"occupied_by_device_id"`
	OccupiedAt         *time.Time `json:"occupied_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// TableName specifies the table name for MoonlightHost
func (MoonlightHost) TableName() string {
	return "moonlight_hosts"
}
