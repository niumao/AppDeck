package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type TenantService struct{}

var tenantServiceInstance *TenantService

func GetTenantService() *TenantService {
	if tenantServiceInstance == nil {
		tenantServiceInstance = &TenantService{}
	}
	return tenantServiceInstance
}

func (s *TenantService) Create(username, password, displayName, phone string) (*models.Tenant, error) {
	var count int64
	database.DB.Model(&models.Tenant{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, errors.New("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	tenant := &models.Tenant{
		ID:          uuid.New().String(),
		Username:    username,
		Password:    string(hashedPassword),
		DisplayName: displayName,
		Phone:       phone,
		Status:      1,
	}

	if err := database.DB.Create(tenant).Error; err != nil {
		return nil, err
	}
	return tenant, nil
}

func (s *TenantService) FindByUsername(username string) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := database.DB.Where("username = ?", username).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (s *TenantService) FindByID(id string) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := database.DB.Where("id = ?", id).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (s *TenantService) VerifyPassword(tenant *models.Tenant, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(tenant.Password), []byte(password)) == nil
}

func (s *TenantService) GetAll(page, limit int, search string) ([]models.Tenant, int64, error) {
	var tenants []models.Tenant
	var total int64

	query := database.DB.Model(&models.Tenant{})
	if search != "" {
		query = query.Where("username LIKE ? OR display_name LIKE ? OR phone LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tenants).Error; err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

func (s *TenantService) GetAllActive() ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := database.DB.Where("status = 1").Order("display_name ASC").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

func (s *TenantService) Update(id, displayName, phone string) error {
	result := database.DB.Model(&models.Tenant{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"display_name": displayName,
			"phone":        phone,
		})
	if result.RowsAffected == 0 {
		return errors.New("TENANT_NOT_FOUND")
	}
	return result.Error
}

func (s *TenantService) UpdatePassword(id, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	result := database.DB.Model(&models.Tenant{}).Where("id = ?", id).
		Update("password", string(hashedPassword))
	if result.RowsAffected == 0 {
		return errors.New("TENANT_NOT_FOUND")
	}
	return result.Error
}

func (s *TenantService) UpdateStatus(id string, status int) error {
	result := database.DB.Model(&models.Tenant{}).Where("id = ?", id).
		Update("status", status)
	if result.RowsAffected == 0 {
		return errors.New("TENANT_NOT_FOUND")
	}
	return result.Error
}

func (s *TenantService) Delete(id string) error {
	database.DB.Model(&models.RegisteredDevice{}).Where("tenant_id = ?", id).
		Update("tenant_id", nil)

	result := database.DB.Where("id = ?", id).Delete(&models.Tenant{})
	if result.RowsAffected == 0 {
		return errors.New("TENANT_NOT_FOUND")
	}
	return result.Error
}

func (s *TenantService) UpdateBillingConfig(id string, planSetID *string, globalEnabled *int, freePlay *int) error {
	updates := map[string]interface{}{}
	if planSetID != nil {
		if *planSetID == "" {
			updates["plan_set_id"] = nil
		} else {
			updates["plan_set_id"] = *planSetID
		}
	}
	if globalEnabled != nil {
		updates["global_enabled"] = *globalEnabled
	}
	if freePlay != nil {
		updates["free_play"] = *freePlay
	}
	if len(updates) == 0 {
		return nil
	}
	result := database.DB.Model(&models.Tenant{}).Where("id = ?", id).Updates(updates)
	if result.RowsAffected == 0 {
		return errors.New("TENANT_NOT_FOUND")
	}
	return result.Error
}
