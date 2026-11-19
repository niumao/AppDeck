package models

import (
	"time"
)

// Setting is a key-value store for application settings
type Setting struct {
	ID        string     `gorm:"primaryKey;size:36" json:"id"`
	Key       string     `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value     string     `gorm:"type:text" json:"value"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}

// TableName specifies the table name for Setting
func (Setting) TableName() string {
	return "settings"
}
