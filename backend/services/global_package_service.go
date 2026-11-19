package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"

	"github.com/google/uuid"
)

type GlobalPackageService struct{}

func NewGlobalPackageService() *GlobalPackageService {
	return &GlobalPackageService{}
}

// GetAll returns all global recharge packages with pagination
func (s *GlobalPackageService) GetAll(page, limit int) ([]models.GlobalRechargePackage, int64, error) {
	var packages []models.GlobalRechargePackage
	var total int64

	query := database.DB.Model(&models.GlobalRechargePackage{})

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("status DESC, sort_order ASC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&packages).Error; err != nil {
		return nil, 0, err
	}

	return packages, total, nil
}

// GetActivePackages returns all active packages for client display
func (s *GlobalPackageService) GetActivePackages() ([]models.GlobalRechargePackage, error) {
	var packages []models.GlobalRechargePackage
	if err := database.DB.
		Where("status = 1").
		Order("sort_order ASC").
		Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

// GetByID returns a single package by ID
func (s *GlobalPackageService) GetByID(id string) (*models.GlobalRechargePackage, error) {
	var pkg models.GlobalRechargePackage
	if err := database.DB.First(&pkg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

// Create creates a new global recharge package
func (s *GlobalPackageService) Create(name string, priceCents, realCents, bonusCents, sortOrder int) (string, error) {
	pkg := &models.GlobalRechargePackage{
		ID:         uuid.New().String(),
		Name:       name,
		PriceCents: priceCents,
		RealCents:  realCents,
		BonusCents: bonusCents,
		SortOrder:  sortOrder,
		Status:     1,
	}

	if err := database.DB.Create(pkg).Error; err != nil {
		return "", err
	}

	return pkg.ID, nil
}

// Update updates package fields
func (s *GlobalPackageService) Update(id string, updates map[string]interface{}) error {
	result := database.DB.Model(&models.GlobalRechargePackage{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("PACKAGE_NOT_FOUND")
	}
	return nil
}

// UpdateStatus updates package status
func (s *GlobalPackageService) UpdateStatus(id string, status int) error {
	result := database.DB.Model(&models.GlobalRechargePackage{}).Where("id = ?", id).Update("status", status)
	if result.RowsAffected == 0 {
		return errors.New("PACKAGE_NOT_FOUND")
	}
	return result.Error
}

// Delete soft-deletes a global package
func (s *GlobalPackageService) Delete(id string) error {
	result := database.DB.Delete(&models.GlobalRechargePackage{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return errors.New("PACKAGE_NOT_FOUND")
	}
	return result.Error
}

// GetDeleted returns all soft-deleted global packages
func (s *GlobalPackageService) GetDeleted() ([]models.GlobalRechargePackage, error) {
	var packages []models.GlobalRechargePackage
	if err := database.DB.Unscoped().Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

// Recover restores a soft-deleted global package
func (s *GlobalPackageService) Recover(id string) error {
	result := database.DB.Unscoped().Model(&models.GlobalRechargePackage{}).Where("id = ? AND deleted_at IS NOT NULL", id).Update("deleted_at", nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("PACKAGE_NOT_FOUND")
	}
	return nil
}

// PermanentDelete permanently removes a soft-deleted global package from the database
func (s *GlobalPackageService) PermanentDelete(id string) error {
	result := database.DB.Unscoped().Where("id = ? AND deleted_at IS NOT NULL", id).Delete(&models.GlobalRechargePackage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("PACKAGE_NOT_FOUND")
	}
	return nil
}
