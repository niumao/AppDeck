package models

import (
	"time"
)

// SystemConfig is a key-value store for system-wide configuration
type SystemConfig struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Key         string     `gorm:"uniqueIndex;size:255;not null" json:"key"`
	Value       string     `gorm:"type:text;not null" json:"value"`
	Description string     `gorm:"type:text" json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName specifies the table name for SystemConfig
func (SystemConfig) TableName() string {
	return "system_config"
}
