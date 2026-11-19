package models

import (
	"time"
)

// AdminNotebook 管理员个人记事本（每个管理员一条记录）
type AdminNotebook struct {
	UserID    string    `gorm:"column:user_id;primaryKey;size:36" json:"user_id"`
	Content   string    `gorm:"column:content;type:text" json:"content"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (AdminNotebook) TableName() string {
	return "admin_notebooks"
}
