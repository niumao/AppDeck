package models

import "time"

// SaveStorageConfig is the admin-configurable destination for uploaded
// game-save blobs. StorageType "s3" is a reserved option for a future
// S3-backed store; it is not implemented yet and resolves to local storage.
type SaveStorageConfig struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Enabled     bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	StorageType string    `gorm:"column:storage_type;size:20;not null;default:'local'" json:"storageType"`
	LocalPath   string    `gorm:"column:local_path;size:255;not null;default:'./data/saves'" json:"localPath"`
	S3Bucket    string    `gorm:"column:s3_bucket;size:255" json:"s3Bucket"`
	S3Region    string    `gorm:"column:s3_region;size:64" json:"s3Region"`
	S3Prefix    string    `gorm:"column:s3_prefix;size:255" json:"s3Prefix"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名
func (SaveStorageConfig) TableName() string {
	return "save_storage_config"
}
