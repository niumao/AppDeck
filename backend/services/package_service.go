package services

import (
	"fmt"
	"log"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
)

type PackageService struct {
	resourceService *ResourceService
	versionService  *VersionService
	cfg             *config.Config
}

func NewPackageService(cfg *config.Config) *PackageService {
	return &PackageService{
		resourceService: NewResourceService(),
		versionService:  NewVersionService(),
		cfg:             cfg,
	}
}

// GetPackageInfo 获取客户端包信息
func (s *PackageService) GetPackageInfo() (*models.ClientPackage, error) {
	var pkg models.ClientPackage
	if err := database.DB.First(&pkg, 1).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

// GenerateClientPackage 生成客户端数据包
func (s *PackageService) GenerateClientPackage() error {
	log.Println("开始生成客户端数据包...")

	// 1. 更新状态为 generating
	if err := s.updateStatus(models.PackageStatusGenerating); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// 2. 获取当前数据版本
	currentVersion, err := s.versionService.GetCurrentVersion()
	if err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to get version: %w", err)
	}

	// 3. 生成 SQLite 数据库
	log.Println("生成 SQLite 数据库...")
	dbPath := s.cfg.Data.ClientDBPath
	if err := database.GenerateSQLiteDB(dbPath); err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to generate SQLite database: %w", err)
	}

	// 4. 获取数据库文件大小
	dbSize, err := s.resourceService.GetFileSize(dbPath)
	if err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to get database size: %w", err)
	}
	log.Printf("数据库文件大小: %d bytes\n", dbSize)

	// 5. 打包图片资源
	log.Println("打包图片资源...")
	resourcesPath := s.cfg.Data.ResourcesZip
	if err := s.resourceService.PackResources(
		s.cfg.Data.IconsDir,
		s.cfg.Data.BigPicsDir,
		resourcesPath,
	); err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to pack resources: %w", err)
	}

	// 6. 获取资源包文件大小
	resourcesSize, err := s.resourceService.GetFileSize(resourcesPath)
	if err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to get resources size: %w", err)
	}
	log.Printf("资源包文件大小: %d bytes\n", resourcesSize)

	// 7. 更新客户端包信息
	now := time.Now()
	updates := map[string]interface{}{
		"package_version": gorm.Expr("package_version + 1"),
		"db_version":      currentVersion.Version,
		"generated_at":    now,
		"db_size":         dbSize,
		"resources_size":  resourcesSize,
		"status":          models.PackageStatusReady,
	}

	if err := database.DB.Model(&models.ClientPackage{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		s.updateStatus(models.PackageStatusError)
		return fmt.Errorf("failed to update package info: %w", err)
	}

	log.Println("客户端数据包生成成功！")
	return nil
}

// updateStatus 更新客户端包状态
func (s *PackageService) updateStatus(status models.PackageStatus) error {
	return database.DB.Model(&models.ClientPackage{}).
		Where("id = ?", 1).
		Update("status", status).
		Error
}

