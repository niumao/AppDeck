package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TenantBalanceService struct{}

func NewTenantBalanceService() *TenantBalanceService {
	return &TenantBalanceService{}
}

// GetOrCreateBalance gets or creates a user tenant balance record
func (s *TenantBalanceService) GetOrCreateBalance(userID, tenantID string) (*models.UserTenantBalance, error) {
	var balance models.UserTenantBalance
	err := database.DB.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&balance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new balance record
			balance = models.UserTenantBalance{
				ID:             uuid.New().String(),
				UserID:         userID,
				TenantID:       tenantID,
				BalanceMinutes: 0,
			}
			if err := database.DB.Create(&balance).Error; err != nil {
				return nil, err
			}
			return &balance, nil
		}
		return nil, err
	}

	return &balance, nil
}

// AddMinutes adds minutes to user tenant balance (within transaction)
func (s *TenantBalanceService) AddMinutes(tx *gorm.DB, userID, tenantID string, minutes int, orderID string) error {
	// Lock and get or create balance
	var balance models.UserTenantBalance
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&balance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new balance record
			balance = models.UserTenantBalance{
				ID:             uuid.New().String(),
				UserID:         userID,
				TenantID:       tenantID,
				BalanceMinutes: minutes,
			}
			if err := tx.Create(&balance).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// Update existing balance
		newBalance := balance.BalanceMinutes + minutes
		if err := tx.Model(&balance).Update("balance_minutes", newBalance).Error; err != nil {
			return err
		}
	}

	return nil
}

// DeductMinute deducts 1 minute from user tenant balance and updates FIFO order tracking (within transaction)
func (s *TenantBalanceService) DeductMinute(tx *gorm.DB, userID, tenantID string) error {
	// Lock balance row
	var balance models.UserTenantBalance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&balance).Error; err != nil {
		return err
	}

	// Check if balance is sufficient
	if balance.BalanceMinutes <= 0 {
		return errors.New("insufficient tenant balance")
	}

	// Deduct from balance
	newBalance := balance.BalanceMinutes - 1
	if err := tx.Model(&balance).Update("balance_minutes", newBalance).Error; err != nil {
		return err
	}

	// FIFO: find oldest tenant order with minutes_remaining > 0 and decrement
	var order models.RechargeOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("scope = ? AND tenant_id = ? AND user_id = ? AND status = ? AND minutes_remaining > 0", "tenant", tenantID, userID, models.RechargeOrderStatusPaid).
		Order("created_at ASC").
		First(&order).Error; err != nil {
		// If no order found, it's okay - balance may have been admin-adjusted
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	// Decrement minutes_remaining on this order
	if err := tx.Model(&order).Update("minutes_remaining", order.MinutesRemaining-1).Error; err != nil {
		return err
	}

	return nil
}

// GetBalance returns the current balance in minutes for a user-tenant pair
func (s *TenantBalanceService) GetBalance(userID, tenantID string) (int, error) {
	var balance models.UserTenantBalance
	err := database.DB.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&balance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}

	return balance.BalanceMinutes, nil
}

// RemoveMinutes removes minutes from balance (for refunds, within transaction)
func (s *TenantBalanceService) RemoveMinutes(tx *gorm.DB, userID, tenantID string, minutes int) error {
	// Lock balance row
	var balance models.UserTenantBalance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&balance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No balance to remove from, that's okay
			return nil
		}
		return err
	}

	// Subtract minutes, floor at 0
	newBalance := balance.BalanceMinutes - minutes
	if newBalance < 0 {
		newBalance = 0
	}

	if err := tx.Model(&balance).Update("balance_minutes", newBalance).Error; err != nil {
		return err
	}

	return nil
}
