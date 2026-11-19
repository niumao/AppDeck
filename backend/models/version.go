package models

import (
	"time"
)

// Version 服务器数据版本模型
type Version struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	Version   string    `gorm:"column:version;not null;type:varchar(20)" json:"version"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名
func (Version) TableName() string {
	return "version"
}


