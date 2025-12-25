package models

import (
	"time"
)

// App 应用模型
type App struct {
	ID          string     `gorm:"column:id;primaryKey" json:"id"`
	Name        string     `gorm:"column:name;not null" json:"name"`
	Description string     `gorm:"column:description;type:text" json:"description"`
	IconURL     string     `gorm:"column:icon_url" json:"iconUrl"`
	BigPicURL   string     `gorm:"column:big_pic_url" json:"bigPicUrl"`
	LaunchParam string     `gorm:"column:launch_param;not null" json:"launchParam"`
	OrderNum    int        `gorm:"column:order_num;default:0" json:"order"`
	ClickCount  int        `gorm:"column:click_count;default:0" json:"clickCount"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
	Categories  []Category `gorm:"many2many:app_categories;" json:"-"`
}

// TableName 指定表名
func (App) TableName() string {
	return "apps"
}

// AppWithCategories 应用及其分类ID（用于API响应）
type AppWithCategories struct {
	App
	CategoryIDs []string `json:"categoryIds"`
}


