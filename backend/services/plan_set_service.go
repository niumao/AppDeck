package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlanSetService struct{}

func NewPlanSetService() *PlanSetService {
	return &PlanSetService{}
}

// GetAll returns all plan sets with pagination and search
func (s *PlanSetService) GetAll(page, limit int, search string) ([]models.PlanSet, int64, error) {
	var planSets []models.PlanSet
	var total int64

	query := database.DB.Model(&models.PlanSet{})
	if search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").
		Preload("StandardPlan").
		Preload("GlobalRatePlan").
		Preload("Plans").
		Offset(offset).
		Limit(limit).
		Find(&planSets).Error; err != nil {
		return nil, 0, err
	}

	return planSets, total, nil
}

// GetByID returns a single plan set by ID with preloads
func (s *PlanSetService) GetByID(id string) (*models.PlanSet, error) {
	var planSet models.PlanSet
	if err := database.DB.
		Preload("StandardPlan").
		Preload("GlobalRatePlan").
		Preload("Plans").
		First(&planSet, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &planSet, nil
}

// Create creates a new plan set
func (s *PlanSetService) Create(name, description string, isDefault int) (string, error) {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// If this is set as default, clear other defaults first
	if isDefault == 1 {
		if err := tx.Model(&models.PlanSet{}).Where("is_default = 1").Update("is_default", 0).Error; err != nil {
			tx.Rollback()
			return "", err
		}
	}

	planSet := &models.PlanSet{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		IsDefault:   isDefault,
		Status:      1,
	}

	if err := tx.Create(planSet).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return planSet.ID, nil
}

// Update updates plan set fields
func (s *PlanSetService) Update(id string, updates map[string]interface{}) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// If setting is_default to 1, clear other defaults first
	if val, ok := updates["is_default"]; ok && val == 1 {
		if err := tx.Model(&models.PlanSet{}).
			Where("is_default = 1 AND id != ?", id).
			Update("is_default", 0).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Validate standard_plan_id and global_rate_plan_id if provided
	if standardPlanID, ok := updates["standard_plan_id"]; ok && standardPlanID != nil {
		var plan models.BillingPlan
		planIDStr := standardPlanID.(string)
		if err := tx.First(&plan, "id = ? AND plan_set_id = ?", planIDStr, id).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("standard plan must belong to this plan set")
			}
			return err
		}
	}

	if globalRatePlanID, ok := updates["global_rate_plan_id"]; ok && globalRatePlanID != nil {
		var plan models.BillingPlan
		planIDStr := globalRatePlanID.(string)
		if err := tx.First(&plan, "id = ? AND plan_set_id = ?", planIDStr, id).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("global rate plan must belong to this plan set")
			}
			return err
		}
	}

	result := tx.Model(&models.PlanSet{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return errors.New("PLAN_SET_NOT_FOUND")
	}

	return tx.Commit().Error
}

// UpdateStatus updates plan set status
func (s *PlanSetService) UpdateStatus(id string, status int) error {
	// Cannot disable the default plan set
	if status == 0 {
		var planSet models.PlanSet
		if err := database.DB.First(&planSet, "id = ?", id).Error; err != nil {
			return err
		}
		if planSet.IsDefault == 1 {
			return errors.New("cannot disable default plan set")
		}
	}

	result := database.DB.Model(&models.PlanSet{}).Where("id = ?", id).Update("status", status)
	if result.RowsAffected == 0 {
		return errors.New("PLAN_SET_NOT_FOUND")
	}
	return result.Error
}

// GetDefaultPlanSet returns the default plan set with plans preloaded
func (s *PlanSetService) GetDefaultPlanSet() (*models.PlanSet, error) {
	var planSet models.PlanSet
	if err := database.DB.
		Preload("StandardPlan").
		Preload("GlobalRatePlan").
		Preload("Plans").
		Where("is_default = 1").
		First(&planSet).Error; err != nil {
		return nil, err
	}
	return &planSet, nil
}

// GetActivePlanSets returns all active plan sets for dropdowns
func (s *PlanSetService) GetActivePlanSets() ([]models.PlanSet, error) {
	var planSets []models.PlanSet
	if err := database.DB.Where("status = 1").Order("name ASC").Find(&planSets).Error; err != nil {
		return nil, err
	}
	return planSets, nil
}

// AddPlanToSet creates a new billing plan within this plan set
func (s *PlanSetService) AddPlanToSet(planSetID, name string, rate int, unit, planType string, priceCents, givesMinutes int, description string, status int) (string, error) {
	// Verify plan set exists
	var planSet models.PlanSet
	if err := database.DB.First(&planSet, "id = ?", planSetID).Error; err != nil {
		return "", err
	}

	planSetIDPtr := &planSetID
	plan := &models.BillingPlan{
		ID:           uuid.New().String(),
		Name:         name,
		Rate:         rate,
		Unit:         unit,
		PlanSetID:    planSetIDPtr,
		Type:         planType,
		PriceCents:   priceCents,
		GivesMinutes: givesMinutes,
		Description:  description,
		Status:       status,
	}

	if err := database.DB.Create(plan).Error; err != nil {
		return "", err
	}

	return plan.ID, nil
}

// DeletePlan removes a plan from the set (with validation)
func (s *PlanSetService) DeletePlan(planID string) error {
	// Check if plan is referenced as standard_plan_id in any plan set
	var countStandard int64
	if err := database.DB.Model(&models.PlanSet{}).
		Where("standard_plan_id = ?", planID).
		Count(&countStandard).Error; err != nil {
		return err
	}
	if countStandard > 0 {
		return errors.New("cannot delete plan: it is set as standard plan in a plan set")
	}

	// Check if plan is referenced as global_rate_plan_id in any plan set
	var countGlobal int64
	if err := database.DB.Model(&models.PlanSet{}).
		Where("global_rate_plan_id = ?", planID).
		Count(&countGlobal).Error; err != nil {
		return err
	}
	if countGlobal > 0 {
		return errors.New("cannot delete plan: it is set as global rate plan in a plan set")
	}

	// Check if plan has active sessions
	var countSessions int64
	if err := database.DB.Model(&models.BillingSession{}).
		Where("plan_id = ? AND ended_at IS NULL", planID).
		Count(&countSessions).Error; err != nil {
		return err
	}
	if countSessions > 0 {
		return errors.New("cannot delete plan: it has active sessions")
	}

	result := database.DB.Delete(&models.BillingPlan{}, "id = ?", planID)
	if result.RowsAffected == 0 {
		return errors.New("PLAN_NOT_FOUND")
	}
	return result.Error
}

// ResolvePlanSetForTenant returns the plan set for a tenant, fallback to default
func (s *PlanSetService) ResolvePlanSetForTenant(tenantID string) (*models.PlanSet, error) {
	// Get tenant
	var tenant models.Tenant
	if err := database.DB.First(&tenant, "id = ?", tenantID).Error; err != nil {
		return nil, err
	}

	// If tenant has a plan set assigned and it's active, use it
	if tenant.PlanSetID != nil {
		var planSet models.PlanSet
		if err := database.DB.
			Preload("StandardPlan").
			Preload("GlobalRatePlan").
			Preload("Plans").
			First(&planSet, "id = ? AND status = 1", *tenant.PlanSetID).Error; err == nil {
			return &planSet, nil
		}
		// If assigned plan set is not found or disabled, fall through to default
	}

	// Use default plan set
	defaultPlanSet, err := s.GetDefaultPlanSet()
	if err != nil {
		return nil, err
	}

	// Check if default is active
	if !defaultPlanSet.IsActive() {
		return nil, errors.New("default plan set is disabled")
	}

	return defaultPlanSet, nil
}

// Delete soft-deletes a plan set
func (s *PlanSetService) Delete(id string) error {
	// Cannot delete the default plan set
	var planSet models.PlanSet
	if err := database.DB.First(&planSet, "id = ?", id).Error; err != nil {
		return errors.New("PLAN_SET_NOT_FOUND")
	}
	if planSet.IsDefault == 1 {
		return errors.New("cannot delete default plan set")
	}

	return database.DB.Delete(&planSet).Error
}

// GetDeleted returns all soft-deleted plan sets
func (s *PlanSetService) GetDeleted() ([]models.PlanSet, error) {
	var planSets []models.PlanSet
	if err := database.DB.Unscoped().Where("deleted_at IS NOT NULL").
		Preload("StandardPlan").
		Preload("GlobalRatePlan").
		Preload("Plans").
		Order("deleted_at DESC").
		Find(&planSets).Error; err != nil {
		return nil, err
	}
	return planSets, nil
}

// Recover restores a soft-deleted plan set
func (s *PlanSetService) Recover(id string) error {
	result := database.DB.Unscoped().Model(&models.PlanSet{}).Where("id = ? AND deleted_at IS NOT NULL", id).Update("deleted_at", nil)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("PLAN_SET_NOT_FOUND")
	}
	return nil
}
