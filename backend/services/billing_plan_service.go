package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"
	"time"

	"github.com/google/uuid"
)

type BillingPlanService struct{}

func NewBillingPlanService() *BillingPlanService {
	return &BillingPlanService{}
}

func (s *BillingPlanService) GetAll() ([]models.BillingPlan, error) {
	var plans []models.BillingPlan
	err := database.DB.Order("is_default DESC, created_at DESC").Find(&plans).Error
	return plans, err
}

func (s *BillingPlanService) GetByID(id string) (*models.BillingPlan, error) {
	var plan models.BillingPlan
	err := database.DB.Where("id = ?", id).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *BillingPlanService) Create(name string, rate int, unit string, description string, isDefault, status int) (string, error) {
	if isDefault == 1 {
		database.DB.Model(&models.BillingPlan{}).Where("is_default = 1").Update("is_default", 0)
	}

	plan := models.BillingPlan{
		ID:          uuid.New().String(),
		Name:        name,
		Rate:        rate,
		Unit:        unit,
		Description: description,
		IsDefault:   isDefault,
		Status:      status,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := database.DB.Create(&plan).Error
	if err != nil {
		return "", err
	}

	return plan.ID, nil
}

func (s *BillingPlanService) Update(id string, updates map[string]interface{}) error {
	if isDefault, ok := updates["is_default"]; ok && isDefault == 1 {
		database.DB.Model(&models.BillingPlan{}).Where("id != ? AND is_default = 1", id).Update("is_default", 0)
	}

	updates["updated_at"] = time.Now()
	return database.DB.Model(&models.BillingPlan{}).Where("id = ?", id).Updates(updates).Error
}

func (s *BillingPlanService) Delete(id string) error {
	var count int64
	database.DB.Model(&models.BillingSession{}).Where("plan_id = ? AND ended_at IS NULL", id).Count(&count)
	if count > 0 {
		return errors.New("无法删除：仍有活跃会话使用此计费方案")
	}

	return database.DB.Where("id = ?", id).Delete(&models.BillingPlan{}).Error
}

func (s *BillingPlanService) GetDefaultPlan() (*models.BillingPlan, error) {
	var plan models.BillingPlan
	err := database.DB.Where("is_default = 1 AND status = 1").First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *BillingPlanService) GetPromoPlan() (*models.BillingPlan, error) {
	var plan models.BillingPlan
	err := database.DB.Where("status = 1").Order("rate ASC").First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}
