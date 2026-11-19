package models

import (
	"time"

	"gorm.io/gorm"
)

// App 应用模型
type App struct {
	ID            string     `gorm:"column:id;primaryKey" json:"id"`
	Name          string     `gorm:"column:name;not null" json:"name"`
	Description   string     `gorm:"column:description;type:text" json:"description"`
	IconURL       string     `gorm:"column:icon_url" json:"iconUrl"`
	BigPicURL     string     `gorm:"column:big_pic_url" json:"bigPicUrl"`
	LaunchParam   string     `gorm:"column:launch_param;not null" json:"launchParam"`
	LaunchType    string     `gorm:"column:launch_type;default:'moonlight';not null" json:"launchType"`
	RetroarchCore string     `gorm:"column:retroarch_core" json:"retroarchCore"`
	RetroarchRom  string     `gorm:"column:retroarch_rom" json:"retroarchRom"`
	TitleID       string     `gorm:"column:title_id" json:"titleId"`
	OrderNum      int        `gorm:"column:order_num;default:0" json:"order"`
	ClickCount    int            `gorm:"column:click_count;default:0" json:"clickCount"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deletedAt,omitempty"`
	Categories    []Category     `gorm:"many2many:app_categories;" json:"-"`
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


