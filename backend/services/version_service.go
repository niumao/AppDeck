package services

import (
	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
)

type VersionService struct{}

func NewVersionService() *VersionService {
	return &VersionService{}
}

// GetCurrentVersion 获取当前数据版本
func (s *VersionService) GetCurrentVersion() (*models.Version, error) {
	var version models.Version
	err := database.DB.First(&version, 1).Error
	return &version, err
}

// IncrementVersion 版本号自增
func (s *VersionService) IncrementVersion() error {
	return database.DB.Model(&models.Version{}).
		Where("id = ?", 1).
		UpdateColumn("version", gorm.Expr("version + 1")).
		Error
}

