package services

import (
	"errors"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BalanceService 余额服务
type BalanceService struct{}

// NewBalanceService 创建余额服务
func NewBalanceService() *BalanceService {
	return &BalanceService{}
}

// GetBalance 获取用户余额信息
func (s *BalanceService) GetBalance(userID string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// HasValidBalance 检查用户是否有有效余额
func (s *BalanceService) HasValidBalance(userID string) (bool, error) {
	user, err := s.GetBalance(userID)
	if err != nil {
		return false, err
	}
	return user.HasValidBalance(), nil
}

// ExtendBalance 延长用户余额时间
func (s *BalanceService) ExtendBalance(userID string, minutes int, orderID *string, logType string, remark string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前用户
	var user models.User
	if err := tx.First(&user, "id = ?", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 计算新的过期时间
	var balanceBefore *time.Time
	var newExpiry time.Time

	if user.BalanceExpiry != nil {
		balanceBefore = user.BalanceExpiry
		// 如果还有余额，在现有基础上延长
		if user.BalanceExpiry.After(time.Now()) {
			newExpiry = user.BalanceExpiry.Add(time.Duration(minutes) * time.Minute)
		} else {
			// 已过期，从现在开始计算
			newExpiry = time.Now().Add(time.Duration(minutes) * time.Minute)
		}
	} else {
		// 首次充值
		newExpiry = time.Now().Add(time.Duration(minutes) * time.Minute)
	}

	// 更新用户余额
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"balance_expiry": newExpiry,
		"updated_at":     time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 记录余额日志
	log := &models.BalanceLog{
		ID:            uuid.New().String(),
		UserID:        userID,
		Type:          logType,
		DeltaMinutes:  minutes,
		BalanceBefore: balanceBefore,
		BalanceAfter:  &newExpiry,
		OrderID:       orderID,
		Remark:        remark,
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(log).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// AdminAdjustBalance 管理员调整余额
func (s *BalanceService) AdminAdjustBalance(userID string, minutes int, adminID string, remark string) error {
	return s.ExtendBalance(userID, minutes, nil, models.BalanceLogTypeAdminAdjust, remark)
}

// AdminAdjustBalanceCents 管理员调整余额（按分计，默认调整实际余额）
func (s *BalanceService) AdminAdjustBalanceCents(userID string, amountCents int, adminID string, remark string) error {
	return s.AdminAdjustBalanceCentsWithType(userID, amountCents, "real", adminID, remark)
}

// AdminAdjustBalanceCentsWithType 管理员调整余额（按分计，指定余额类型）
func (s *BalanceService) AdminAdjustBalanceCentsWithType(userID string, amountCents int, balanceType string, adminID string, remark string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前用户（加行锁防止并发问题）
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	balanceBefore := user.BalanceCents
	bonusBalanceBefore := user.BonusBalanceCents
	newBalance := balanceBefore
	newBonusBalance := bonusBalanceBefore

	// Determine which balance to adjust
	if balanceType == "bonus" {
		newBonusBalance += amountCents
		if newBonusBalance < 0 {
			tx.Rollback()
			return errors.New("余额不足")
		}
	} else {
		// Default to real balance for backward compatibility
		newBalance += amountCents
		if newBalance < 0 {
			tx.Rollback()
			return errors.New("余额不足")
		}
	}

	// 更新用户余额
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"balance_cents":       newBalance,
		"bonus_balance_cents": newBonusBalance,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 记录余额日志
	log := &models.BalanceLog{
		ID:                 uuid.New().String(),
		UserID:             userID,
		Type:               models.BalanceLogTypeAdminAdjust,
		AmountCents:        amountCents,
		BalanceBeforeCents: balanceBefore,
		BalanceAfterCents:  newBalance,
		AdminID:            &adminID,
		Remark:             remark,
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(log).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GetBalanceLogs 获取用户余额日志
func (s *BalanceService) GetBalanceLogs(userID string, page, pageSize int) ([]models.BalanceLog, int64, error) {
	var logs []models.BalanceLog
	var total int64

	query := database.DB.Model(&models.BalanceLog{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// AddBalance 增加用户余额（充值使用）
func (s *BalanceService) AddBalance(userID string, amountCents int, orderID *string, logType string, remark string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前用户（加行锁防止并发问题）
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	balanceBefore := user.BalanceCents
	newBalance := balanceBefore + amountCents

	// 更新用户余额
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"balance_cents": newBalance,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 记录余额日志
	log := &models.BalanceLog{
		ID:                 uuid.New().String(),
		UserID:             userID,
		Type:               logType,
		AmountCents:        amountCents,
		BalanceBeforeCents: balanceBefore,
		BalanceAfterCents:  newBalance,
		OrderID:            orderID,
		Remark:             remark,
		CreatedAt:          time.Now(),
	}

	if err := tx.Create(log).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// AddBalanceSplit 增加用户余额（分别记录实际金额和赠送金额）
func (s *BalanceService) AddBalanceSplit(userID string, realAmountCents int, bonusAmountCents int, orderID *string, logType string, remark string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前用户（加行锁防止并发问题）
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	balanceBefore := user.BalanceCents
	bonusBalanceBefore := user.BonusBalanceCents
	newBalance := balanceBefore + realAmountCents
	newBonusBalance := bonusBalanceBefore + bonusAmountCents

	// 更新用户余额（实际余额和赠送余额）
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"balance_cents":       newBalance,
		"bonus_balance_cents": newBonusBalance,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 记录余额日志 - real balance
	if realAmountCents > 0 {
		logReal := &models.BalanceLog{
			ID:                 uuid.New().String(),
			UserID:             userID,
			Type:               logType,
			Scope:              "global",
			BalanceType:        "real",
			AmountCents:        realAmountCents,
			BalanceBeforeCents: balanceBefore,
			BalanceAfterCents:  newBalance,
			OrderID:            orderID,
			Remark:             remark,
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(logReal).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 记录余额日志 - bonus balance
	if bonusAmountCents > 0 {
		logBonus := &models.BalanceLog{
			ID:                 uuid.New().String(),
			UserID:             userID,
			Type:               logType,
			Scope:              "global",
			BalanceType:        "bonus",
			AmountCents:        bonusAmountCents,
			BalanceBeforeCents: bonusBalanceBefore,
			BalanceAfterCents:  newBonusBalance,
			OrderID:            orderID,
			Remark:             remark,
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(logBonus).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// DeductBalance 扣减用户余额（优先扣除实际余额，不足时再扣赠送余额）
func (s *BalanceService) DeductBalance(userID string, amountCents int, logType string, orderID string, adminID *string, remark string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取当前用户（加行锁防止并发问题）
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	balanceBefore := user.BalanceCents
	bonusBalanceBefore := user.BonusBalanceCents
	totalBefore := balanceBefore + bonusBalanceBefore

	// Check if total balance is sufficient
	if totalBefore < amountCents {
		tx.Rollback()
		return errors.New("余额不足")
	}

	// Two-tier deduction: real balance first, then bonus
	newBalance := balanceBefore
	newBonusBalance := bonusBalanceBefore
	remaining := amountCents

	// Deduct from real balance first
	if newBalance > 0 {
		if newBalance >= remaining {
			newBalance -= remaining
			remaining = 0
		} else {
			remaining -= newBalance
			newBalance = 0
		}
	}

	// Deduct from bonus balance if needed
	if remaining > 0 {
		newBonusBalance -= remaining
		remaining = 0
	}

	// Calculate how much real balance was deducted
	realDeducted := balanceBefore - newBalance

	// 更新用户余额
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"balance_cents":       newBalance,
		"bonus_balance_cents": newBonusBalance,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// FIFO: decrement real_cents_remaining on oldest global orders
	if realDeducted > 0 {
		remainingReal := realDeducted
		var orders []models.RechargeOrder
		tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("scope = 'global' AND user_id = ? AND status = ? AND real_cents_remaining > 0", userID, models.RechargeOrderStatusPaid).
			Order("created_at ASC").
			Find(&orders)
		for i := range orders {
			if remainingReal <= 0 {
				break
			}
			deduct := remainingReal
			if deduct > orders[i].RealCentsRemaining {
				deduct = orders[i].RealCentsRemaining
			}
			tx.Model(&orders[i]).Update("real_cents_remaining", gorm.Expr("real_cents_remaining - ?", deduct))
			remainingReal -= deduct
		}
	}

	// 记录余额日志
	orderIDPtr := &orderID
	if orderID == "" {
		orderIDPtr = nil
	}

	// If we deducted real balance, log it
	if realDeducted > 0 {
		logReal := &models.BalanceLog{
			ID:                 uuid.New().String(),
			UserID:             userID,
			Type:               logType,
			Scope:              "global",
			BalanceType:        "real",
			AmountCents:        -realDeducted, // 负数表示扣减
			BalanceBeforeCents: balanceBefore,
			BalanceAfterCents:  newBalance,
			OrderID:            orderIDPtr,
			AdminID:            adminID,
			Remark:             remark,
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(logReal).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// If we deducted bonus balance, log it separately
	bonusDeducted := bonusBalanceBefore - newBonusBalance
	if bonusDeducted > 0 {
		logBonus := &models.BalanceLog{
			ID:                 uuid.New().String(),
			UserID:             userID,
			Type:               logType,
			Scope:              "global",
			BalanceType:        "bonus",
			AmountCents:        -bonusDeducted, // 负数表示扣减
			BalanceBeforeCents: bonusBalanceBefore,
			BalanceAfterCents:  newBonusBalance,
			OrderID:            orderIDPtr,
			AdminID:            adminID,
			Remark:             remark,
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(logBonus).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
