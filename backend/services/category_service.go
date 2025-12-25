package services

import (
	"fmt"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
)

type CategoryService struct {
	versionService *VersionService
}

func NewCategoryService() *CategoryService {
	return &CategoryService{
		versionService: NewVersionService(),
	}
}

// GetAll 获取所有分类
func (s *CategoryService) GetAll() ([]models.Category, error) {
	var categories []models.Category
	if err := database.DB.Order("order_num ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// GetAllWithAppCount 获取所有分类（带应用数量）
func (s *CategoryService) GetAllWithAppCount() ([]models.CategoryWithAppCount, error) {
	var categories []models.Category
	if err := database.DB.Order("order_num ASC").Find(&categories).Error; err != nil {
		return nil, err
	}

	result := make([]models.CategoryWithAppCount, len(categories))
	for i, cat := range categories {
		var count int64
		database.DB.Table("app_categories").Where("category_id = ?", cat.ID).Count(&count)
		result[i] = models.CategoryWithAppCount{
			Category: cat,
			AppCount: int(count),
		}
	}
	return result, nil
}

// GetByID 根据ID获取分类
func (s *CategoryService) GetByID(id string) (*models.Category, error) {
	var category models.Category
	if err := database.DB.First(&category, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// Create 创建分类
func (s *CategoryService) Create(category *models.Category) (*models.Category, error) {
	// 生成 UUID
	category.ID = uuid.New().String()

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(category).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return category, nil
}

// Update 更新分类
func (s *CategoryService) Update(id string, updates map[string]interface{}) (*models.Category, error) {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&models.Category{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

// Delete 删除分类
func (s *CategoryService) Delete(id string) error {
	// 检查是否有应用使用该分类
	var count int64
	if err := database.DB.Table("app_categories").Where("category_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该分类下还有 %d 个应用，无法删除", count)
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Delete(&models.Category{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// UpdateOrder 批量更新分类顺序
func (s *CategoryService) UpdateOrder(orders map[string]int) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for id, order := range orders {
		if err := tx.Model(&models.Category{}).Where("id = ?", id).Update("order_num", order).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}


