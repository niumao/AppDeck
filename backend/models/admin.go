package models

import (
	"time"
)

// Admin 管理员模型
type Admin struct {
	ID          string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	Username    string    `gorm:"column:username;size:50;uniqueIndex;not null" json:"username"`
	Password    string    `gorm:"column:password;size:255;not null" json:"-"`
	DisplayName string    `gorm:"column:display_name;size:100" json:"display_name"`
	Status      int       `gorm:"column:status;default:1" json:"status"` // 1=active, 0=disabled
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (Admin) TableName() string {
	return "admins"
}

// IsActive 检查管理员是否激活
func (a *Admin) IsActive() bool {
	return a.Status == 1
}
