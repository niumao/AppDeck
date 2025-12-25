package database

import (
	"fmt"
	"os"

	"appdeck/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GenerateSQLiteDB 从 MySQL 生成客户端 SQLite 数据库
func GenerateSQLiteDB(outputPath string) error {
	// 删除旧的数据库文件
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old database: %w", err)
	}

	// 创建 SQLite 数据库
	sqliteDB, err := gorm.Open(sqlite.Open(outputPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to create SQLite database: %w", err)
	}

	// 创建表结构
	if err := sqliteDB.AutoMigrate(
		&models.App{},
		&models.Category{},
		&models.Version{},
	); err != nil {
		return fmt.Errorf("failed to migrate SQLite schema: %w", err)
	}

	// 复制应用数据
	var apps []models.App
	if err := DB.Find(&apps).Error; err != nil {
		return fmt.Errorf("failed to fetch apps: %w", err)
	}
	if len(apps) > 0 {
		if err := sqliteDB.Create(&apps).Error; err != nil {
			return fmt.Errorf("failed to insert apps: %w", err)
		}
	}

	// 复制分类数据
	var categories []models.Category
	if err := DB.Find(&categories).Error; err != nil {
		return fmt.Errorf("failed to fetch categories: %w", err)
	}
	if len(categories) > 0 {
		if err := sqliteDB.Create(&categories).Error; err != nil {
			return fmt.Errorf("failed to insert categories: %w", err)
		}
	}

	// 复制应用分类关联数据
	type AppCategory struct {
		AppID      string `gorm:"column:app_id"`
		CategoryID string `gorm:"column:category_id"`
	}
	var appCategories []AppCategory
	if err := DB.Table("app_categories").Find(&appCategories).Error; err != nil {
		return fmt.Errorf("failed to fetch app_categories: %w", err)
	}
	if len(appCategories) > 0 {
		if err := sqliteDB.Table("app_categories").Create(&appCategories).Error; err != nil {
			return fmt.Errorf("failed to insert app_categories: %w", err)
		}
	}

	// 复制版本数据
	var version models.Version
	if err := DB.First(&version, 1).Error; err != nil {
		return fmt.Errorf("failed to fetch version: %w", err)
	}
	if err := sqliteDB.Create(&version).Error; err != nil {
		return fmt.Errorf("failed to insert version: %w", err)
	}

	// 关闭 SQLite 连接
	sqlDB, err := sqliteDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQLite DB instance: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close SQLite database: %w", err)
	}

	return nil
}


