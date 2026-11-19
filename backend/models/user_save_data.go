package models

import "time"

// UserSaveData tracks the latest uploaded game-save bundle for a user --
// one row per user, covering every Eden (Switch emulator) title they have
// progress in at once, since Eden shares a single on-device profile across
// all titles.
type UserSaveData struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"column:user_id;size:36;not null;uniqueIndex:idx_user_save" json:"userId"`
	StorageKey string    `gorm:"column:storage_key;size:255;not null" json:"storageKey"`
	SizeBytes  int64     `gorm:"column:size_bytes;not null" json:"sizeBytes"`
	Checksum   string    `gorm:"column:checksum;size:64;not null" json:"checksum"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名
func (UserSaveData) TableName() string {
	return "user_save_data"
}
