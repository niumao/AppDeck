package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

type RefundService struct{}

func NewRefundService() *RefundService {
	return &RefundService{}
}

type RefundPreview struct {
	OrderID            string `json:"order_id"`
	Scope              string `json:"scope"`
	OriginalPriceCents int    `json:"original_price_cents"`
	ConsumedValue      int    `json:"consumed_value"`
	RefundableCents    int    `json:"refundable_cents"`
	AlreadyRefunded    int    `json:"already_refunded"`
}

// unitToMinutes converts rate unit to minutes
func unitToMinutes(unit string) int {
	switch unit {
	case "minute":
		return 1
	case "hour":
		return 60
	case "day":
		return 1440
	default:
		return 60 // default to hour
	}
}

// PreviewRefund calculates refund amount without executing
func (s *RefundService) PreviewRefund(orderID string) (*RefundPreview, error) {
	var order models.RechargeOrder
	if err := database.DB.First(&order, "id = ?", orderID).Error; err != nil {
		return nil, err
	}

	// Check if order is paid
	if order.Status != models.RechargeOrderStatusPaid {
		return nil, errors.New("order is not paid")
	}

	preview := &RefundPreview{
		OrderID:            orderID,
		Scope:              order.Scope,
		OriginalPriceCents: order.AmountCents,
		AlreadyRefunded:    order.RefundedCents,
	}

	if order.Scope == "tenant" {
		// Tenant refund calculation
		consumedMinutes := order.MinutesCredited - order.MinutesRemaining
		ratePerMinute := float64(order.StandardRateSnapshot) / float64(unitToMinutes(order.StandardUnitSnapshot))
		consumedValue := int(math.Ceil(float64(consumedMinutes) * ratePerMinute))

		refund := order.AmountCents - consumedValue - order.RefundedCents
		if refund < 0 {
			refund = 0
		}

		preview.ConsumedValue = consumedValue
		preview.RefundableCents = refund
	} else {
		// Global refund calculation
		if order.RealCentsRemaining == 0 {
			preview.ConsumedValue = order.RealCents
			preview.RefundableCents = 0
		} else {
			consumedValue := order.RealCents - order.RealCentsRemaining
			refund := order.RealCentsRemaining
			maxRefund := order.AmountCents - order.RefundedCents
			if refund > maxRefund {
				refund = maxRefund
			}

			preview.ConsumedValue = consumedValue
			preview.RefundableCents = refund
		}
	}

	return preview, nil
}

// ProcessRefund executes the refund in a transaction
func (s *RefundService) ProcessRefund(orderID string) (*RefundPreview, error) {
	// First get preview to validate
	preview, err := s.PreviewRefund(orderID)
	if err != nil {
		return nil, err
	}

	if preview.RefundableCents <= 0 {
		return nil, errors.New("no refundable amount")
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Lock and reload order
	var order models.RechargeOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&order, "id = ?", orderID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	refundAmount := preview.RefundableCents

	if order.Scope == "tenant" {
		// Tenant refund: remove minutes_remaining from UserTenantBalance
		if order.TenantID != nil && order.MinutesRemaining > 0 {
			tenantBalanceService := NewTenantBalanceService()
			if err := tenantBalanceService.RemoveMinutes(tx, order.UserID, *order.TenantID, order.MinutesRemaining); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Update order: set minutes_remaining to 0, update refunded_cents and refund_status
		newRefundedCents := order.RefundedCents + refundAmount
		refundStatus := "full"
		if newRefundedCents < order.AmountCents {
			refundStatus = "partial"
		}

		if err := tx.Model(&order).Updates(map[string]interface{}{
			"minutes_remaining": 0,
			"refunded_cents":    newRefundedCents,
			"refund_status":     refundStatus,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Create balance log
		orderIDPtr := &orderID
		balanceLog := &models.BalanceLog{
			ID:          uuid.New().String(),
			UserID:      order.UserID,
			Type:        "refund",
			Scope:       "tenant",
			TenantID:    order.TenantID,
			AmountCents: refundAmount,
			OrderID:     orderIDPtr,
			Remark:      "Tenant package refund",
			CreatedAt:   time.Now(),
		}
		if err := tx.Create(balanceLog).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

	} else {
		// Global refund: deduct from user balance, remove proportional bonus
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, "id = ?", order.UserID).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Deduct refund from user.balance_cents
		newBalance := user.BalanceCents - refundAmount
		if newBalance < 0 {
			newBalance = 0
		}

		// Remove proportional bonus
		bonusToRemove := 0
		if order.RealCents > 0 {
			bonusRatio := float64(order.BonusCents) / float64(order.RealCents)
			bonusToRemove = int(math.Floor(float64(refundAmount) * bonusRatio))
			if bonusToRemove > user.BonusBalanceCents {
				bonusToRemove = user.BonusBalanceCents
			}
		}

		newBonusBalance := user.BonusBalanceCents - bonusToRemove
		if newBonusBalance < 0 {
			newBonusBalance = 0
		}

		if err := tx.Model(&user).Updates(map[string]interface{}{
			"balance_cents":       newBalance,
			"bonus_balance_cents": newBonusBalance,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Update order
		newRefundedCents := order.RefundedCents + refundAmount
		refundStatus := "full"
		if newRefundedCents < order.AmountCents {
			refundStatus = "partial"
		}

		if err := tx.Model(&order).Updates(map[string]interface{}{
			"real_cents_remaining": 0,
			"refunded_cents":       newRefundedCents,
			"refund_status":        refundStatus,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// Create balance log
		orderIDPtr := &orderID
		balanceLog := &models.BalanceLog{
			ID:                 uuid.New().String(),
			UserID:             order.UserID,
			Type:               "refund",
			Scope:              "global",
			BalanceType:        "real",
			AmountCents:        -refundAmount,
			BalanceBeforeCents: user.BalanceCents,
			BalanceAfterCents:  newBalance,
			OrderID:            orderIDPtr,
			Remark:             "Global package refund",
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(balanceLog).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return preview, nil
}
