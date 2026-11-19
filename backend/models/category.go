package models

import (
	"time"
)

// Category 分类模型
type Category struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(100);not null;uniqueIndex" json:"name"`
	OrderNum  int       `gorm:"column:order_num;default:0" json:"order"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
	Apps      []App     `gorm:"many2many:app_categories;" json:"-"`
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}

// CategoryWithAppCount 分类及其应用数量（用于管理API）
type CategoryWithAppCount struct {
	Category
	AppCount int `json:"appCount"`
}


