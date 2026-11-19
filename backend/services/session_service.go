package services

import (
	"appdeck/database"
	"appdeck/logger"
	"appdeck/models"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionService struct{}

func NewSessionService() *SessionService {
	return &SessionService{}
}

func (s *SessionService) GetActiveSessions() ([]models.BillingSession, error) {
	var sessions []models.BillingSession
	err := database.DB.
		Preload("User").
		Preload("Client").
		Preload("App").
		Preload("Plan").
		Where("ended_at IS NULL").
		Order("started_at DESC").
		Find(&sessions).Error
	return sessions, err
}

func (s *SessionService) GetSessionByID(id string) (*models.BillingSession, error) {
	var session models.BillingSession
	err := database.DB.
		Preload("User").
		Preload("Client").
		Preload("App").
		Preload("Plan").
		Where("id = ?", id).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *SessionService) CreateSession(userID, clientID string, appID *string, planID string) (string, error) {
	session := models.BillingSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		ClientID:  clientID,
		AppID:     appID,
		PlanID:    planID,
		StartedAt: time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := database.DB.Create(&session).Error
	if err != nil {
		return "", err
	}

	return session.ID, nil
}

func (s *SessionService) EndSession(id string) error {
	var session models.BillingSession
	err := database.DB.Where("id = ?", id).First(&session).Error
	if err != nil {
		return err
	}

	if session.EndedAt != nil {
		return errors.New("会话已结束")
	}

	now := time.Now()
	durationSec := int(now.Sub(session.StartedAt).Seconds())

	var plan models.BillingPlan
	err = database.DB.Where("id = ?", session.PlanID).First(&plan).Error
	if err != nil {
		return err
	}

	totalCost := s.calculateCost(&plan, durationSec)

	return database.DB.Model(&session).Updates(map[string]interface{}{
		"ended_at":     now,
		"duration_sec": durationSec,
		"total_cost":   totalCost,
		"updated_at":   now,
	}).Error
}

// SessionStartResult represents the result of starting a session
type SessionStartResult struct {
	ID                   string   `json:"id"`
	PlanID               string   `json:"plan_id"`
	PlanRateCents        int      `json:"plan_rate_cents"`
	PlanUnit             string   `json:"plan_unit"`
	BalanceCents         int      `json:"balance_cents"`
	RemainingSeconds     int      `json:"remaining_seconds"`
	BalanceSource        string   `json:"balance_source"`
	TenantBalanceMinutes int      `json:"tenant_balance_minutes"`
	GlobalBalanceCents   int      `json:"global_balance_cents"`
	OtherSessionCount    int      `json:"other_session_count"`
	OtherDeviceNames     []string `json:"other_device_names"`
}

// SessionHeartbeatResult represents the result of a session heartbeat
type SessionHeartbeatResult struct {
	BalanceCents         int    `json:"balance_cents"`
	RemainingSeconds     int    `json:"remaining_seconds"`
	DurationSeconds      int    `json:"duration_seconds"`
	CostCents            int    `json:"cost_cents"`
	BalanceSource        string `json:"balance_source"`
	TenantBalanceMinutes int    `json:"tenant_balance_minutes"`
	GlobalBalanceCents   int    `json:"global_balance_cents"`
}

// StartSessionOnBind starts a billing session when device is bound (no specific app)
func (s *SessionService) StartSessionOnBind(userID, clientID string) (*SessionStartResult, error) {
	return s.startSessionInternal(userID, clientID, nil)
}

// startSessionInternal is the internal implementation with nullable appID
func (s *SessionService) startSessionInternal(userID, clientID string, appID *string) (*SessionStartResult, error) {
	// 1. Get user
	var user models.User
	err := database.DB.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 2. Get UserClient by clientID to get machine_code
	var client models.UserClient
	err = database.DB.Where("id = ?", clientID).First(&client).Error
	if err != nil {
		return nil, errors.New("客户端不存在")
	}

	// 3. Find RegisteredDevice by machine_code to get tenant_id
	var device models.RegisteredDevice
	err = database.DB.Where("machine_code = ?", client.MachineCode).First(&device).Error
	if err != nil {
		return nil, errors.New("设备未注册")
	}

	if device.TenantID == nil {
		return nil, errors.New("设备未分配租户")
	}
	tenantID := *device.TenantID

	// 4. Resolve PlanSet: if tenant has plan_set_id, use it (if active); else use default
	planSetService := NewPlanSetService()
	planSet, err := planSetService.ResolvePlanSetForTenant(tenantID)
	if err != nil {
		return nil, errors.New("系统计费配置错误，请联系管理员")
	}

	// Verify plan set has both standard and global rate plans set
	if planSet.StandardPlanID == nil || planSet.GlobalRatePlanID == nil {
		return nil, errors.New("系统计费配置错误，请联系管理员")
	}

	// 5. Check tenant time balance
	tenantBalanceService := NewTenantBalanceService()
	tenantBalance, err := tenantBalanceService.GetOrCreateBalance(userID, tenantID)
	if err != nil {
		return nil, err
	}

	// 6. Get tenant to check global_enabled
	var tenant models.Tenant
	err = database.DB.Where("id = ?", tenantID).First(&tenant).Error
	if err != nil {
		return nil, errors.New("租户不存在")
	}

	var balanceSource string
	var planID string
	var rateSnapshot int
	var rateUnitSnapshot string

	if tenantBalance.BalanceMinutes > 0 {
		// Use tenant balance
		balanceSource = "tenant"
		planID = *planSet.StandardPlanID
		rateSnapshot = 0
		rateUnitSnapshot = "hour"
	} else if tenant.GlobalEnabled == 1 && user.HasValidBalance() {
		// Use global balance
		balanceSource = "global"
		planID = *planSet.GlobalRatePlanID
		var globalRatePlan models.BillingPlan
		err = database.DB.Where("id = ?", *planSet.GlobalRatePlanID).First(&globalRatePlan).Error
		if err != nil {
			return nil, errors.New("全局计费方案不存在")
		}
		rateSnapshot = globalRatePlan.Rate
		rateUnitSnapshot = globalRatePlan.Unit
	} else if tenantBalance.BalanceMinutes <= 0 && tenant.GlobalEnabled == 0 {
		return nil, errors.New("请先购买本店套餐")
	} else {
		return nil, errors.New("余额不足")
	}

	// 7. Create BillingSession with tenant_id, plan_set_id, plan_id
	now := time.Now()
	session := models.BillingSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		ClientID:  clientID,
		AppID:     appID,
		PlanID:    planID,
		TenantID:  tenantID,
		PlanSetID: planSet.ID,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = database.DB.Create(&session).Error
	if err != nil {
		return nil, err
	}

	// 8. Create first BillingSessionSegment
	segment := models.BillingSessionSegment{
		ID:               uuid.New().String(),
		SessionID:        session.ID,
		BalanceSource:    balanceSource,
		RateSnapshot:     rateSnapshot,
		RateUnitSnapshot: rateUnitSnapshot,
		StartedAt:        now,
		CreatedAt:        now,
	}

	err = database.DB.Create(&segment).Error
	if err != nil {
		return nil, err
	}

	// 9. Query other active sessions on different devices (before remaining_seconds calc)
	var otherSessions []models.BillingSession
	database.DB.Where("user_id = ? AND client_id != ? AND ended_at IS NULL", userID, clientID).
		Preload("Client").Find(&otherSessions)
	var otherDeviceNames []string
	for _, s := range otherSessions {
		name := "未知设备"
		if s.Client != nil && s.Client.Name != "" {
			name = s.Client.Name
		}
		otherDeviceNames = append(otherDeviceNames, name)
	}
	totalSessions := 1 + len(otherSessions)

	// 10. Combined remaining seconds: tenant time + global time, divided by total sessions
	ratePerSecond := s.getRatePerSecond(rateSnapshot, rateUnitSnapshot)
	remainingSeconds := combinedRemainingSeconds(tenantBalance.BalanceMinutes, totalSessions, user.GetTotalBalance(), ratePerSecond)

	s.scheduleSessionExpiry(clientID, session.ID, userID, remainingSeconds)

	return &SessionStartResult{
		ID:                   session.ID,
		PlanID:               planID,
		PlanRateCents:        rateSnapshot,
		PlanUnit:             rateUnitSnapshot,
		BalanceCents:         user.GetTotalBalance(),
		RemainingSeconds:     remainingSeconds,
		BalanceSource:        balanceSource,
		TenantBalanceMinutes: tenantBalance.BalanceMinutes,
		GlobalBalanceCents:   user.GetTotalBalance(),
		OtherSessionCount:    len(otherSessions),
		OtherDeviceNames:     otherDeviceNames,
	}, nil
}

// Heartbeat processes a session heartbeat and deducts cost
func (s *SessionService) Heartbeat(sessionID, userID string) (*SessionHeartbeatResult, error) {
	var session models.BillingSession
	err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error
	if err != nil {
		return nil, errors.New("会话不存在或无权限")
	}

	if session.EndedAt != nil {
		return nil, errors.New("会话已结束")
	}

	// Get active segment
	segment, err := s.getActiveSegment(sessionID)
	if err != nil {
		return nil, errors.New("会话段不存在")
	}

	// Get user
	var user models.User
	err = database.DB.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}

	// Get tenant balance
	tenantBalanceService := NewTenantBalanceService()
	tenantBalance, err := tenantBalanceService.GetBalance(userID, session.TenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	elapsed := 60 // heartbeat interval in seconds

	if segment.BalanceSource == "tenant" {
		// Tenant balance flow
		err = database.DB.Transaction(func(tx *gorm.DB) error {
			// Lock session row to prevent race with EndSessionsByClientID
			var lockedSession models.BillingSession
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND ended_at IS NULL", sessionID).
				First(&lockedSession).Error; err != nil {
				return errors.New("会话已结束")
			}

			// Deduct 1 minute from tenant balance (with FIFO tracking)
			err := tenantBalanceService.DeductMinute(tx, userID, session.TenantID)
			if err != nil {
				// Balance exhausted - check if can switch to global
				var tenant models.Tenant
				tx.Where("id = ?", session.TenantID).First(&tenant)

				if tenant.GlobalEnabled == 1 && user.HasValidBalance() {
					// Close current segment
					tx.Model(segment).Updates(map[string]interface{}{
						"ended_at":     now,
						"duration_sec": segment.DurationSec + elapsed,
					})

					// Get global rate plan
					var planSet models.PlanSet
					tx.Preload("GlobalRatePlan").Where("id = ?", session.PlanSetID).First(&planSet)
					if planSet.GlobalRatePlan == nil {
						return errors.New("全局计费方案不存在")
					}

					// Create new segment with global balance
					newSegment := models.BillingSessionSegment{
						ID:               uuid.New().String(),
						SessionID:        sessionID,
						BalanceSource:    "global",
						RateSnapshot:     planSet.GlobalRatePlan.Rate,
						RateUnitSnapshot: planSet.GlobalRatePlan.Unit,
						StartedAt:        now,
						CreatedAt:        now,
					}
					tx.Create(&newSegment)

					// Update session plan_id to global rate plan
					tx.Model(&lockedSession).Update("plan_id", planSet.GlobalRatePlan.ID)

					return nil
				}

				// Cannot switch to global - end session
				return errors.New("tenant_balance_exhausted")
			}

			// Update session
			tx.Model(&lockedSession).Updates(map[string]interface{}{
				"tenant_minutes_consumed": gorm.Expr("tenant_minutes_consumed + 1"),
				"duration_sec":            gorm.Expr("duration_sec + ?", elapsed),
			})

			// Update segment
			tx.Model(segment).Updates(map[string]interface{}{
				"minutes_consumed": gorm.Expr("minutes_consumed + 1"),
				"duration_sec":     gorm.Expr("duration_sec + ?", elapsed),
			})

			return nil
		})

		if err != nil {
			if err.Error() == "tenant_balance_exhausted" {
				// Force end session
				s.forceEndSessionDualBalance(sessionID, userID, &session)
				return nil, errors.New("余额不足，会话已自动结束")
			}
			return nil, err
		}

		// Refresh balances
		tenantBalance, _ = tenantBalanceService.GetBalance(userID, session.TenantID)
		database.DB.Where("id = ?", userID).First(&user)

		// Balance update is delivered via the HTTP heartbeat response;
		// the Wails BillingService emits ws:balance-update locally.

		// Count other active sessions to divide remaining time
		hbTotalSessions := 1 + s.countOtherActiveSessions(userID, sessionID)
		hbRate := s.getRatePerSecond(segment.RateSnapshot, segment.RateUnitSnapshot)
		remainingSeconds := combinedRemainingSeconds(tenantBalance, hbTotalSessions, user.GetTotalBalance(), hbRate)

		if remainingSeconds <= 0 {
			// Tenant balance landed exactly on a whole-minute boundary with no global
			// fallback left -- end now instead of leaving the session to hang until the
			// next tick would find DeductMinute failing.
			s.forceEndSessionDualBalance(sessionID, userID, &session)
			return nil, errors.New("余额不足，会话已自动结束")
		}

		durationSec := int(now.Sub(session.StartedAt).Seconds())

		return &SessionHeartbeatResult{
			BalanceCents:         user.GetTotalBalance(),
			RemainingSeconds:     remainingSeconds,
			DurationSeconds:      durationSec,
			CostCents:            0,
			BalanceSource:        "tenant",
			TenantBalanceMinutes: tenantBalance,
			GlobalBalanceCents:   user.GetTotalBalance(),
		}, nil

	} else {
		// Global balance flow
		ratePerSecond := s.getRatePerSecond(segment.RateSnapshot, segment.RateUnitSnapshot)
		costCents := int(float64(elapsed) * ratePerSecond)
		if costCents < 1 {
			costCents = 1 // Minimum 1 cent per heartbeat
		}

		var realDeducted, bonusDeducted int
		var shouldForceEnd bool

		err = database.DB.Transaction(func(tx *gorm.DB) error {
			// Lock session row to prevent race with EndSessionsByClientID
			var lockedSession models.BillingSession
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND ended_at IS NULL", sessionID).
				First(&lockedSession).Error; err != nil {
				return errors.New("会话已结束")
			}

			// Lock user row
			var lockedUser models.User
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&lockedUser).Error
			if err != nil {
				return err
			}

			totalBalance := lockedUser.BalanceCents + lockedUser.BonusBalanceCents
			if totalBalance <= 0 {
				// Balance fully exhausted - end session (nothing to deduct, safe to rollback)
				return errors.New("global_balance_exhausted")
			}

			// If remaining balance is less than heartbeat cost, deduct whatever is left
			// and end the session after the transaction commits.
			// NOTE: We must NOT return an error here because GORM rolls back on error,
			// which would undo the balance deduction and cause an infinite login loop.
			if totalBalance < costCents {
				costCents = totalBalance
				shouldForceEnd = true
			}

			// Two-tier deduction: real first, then bonus
			newBalance := lockedUser.BalanceCents
			newBonusBalance := lockedUser.BonusBalanceCents
			remaining := costCents

			if newBalance > 0 {
				if newBalance >= remaining {
					realDeducted = remaining
					newBalance -= remaining
					remaining = 0
				} else {
					realDeducted = newBalance
					remaining -= newBalance
					newBalance = 0
				}
			}

			if remaining > 0 {
				bonusDeducted = remaining
				newBonusBalance -= remaining
			}

			// Update user balance
			tx.Model(&lockedUser).Updates(map[string]interface{}{
				"balance_cents":       newBalance,
				"bonus_balance_cents": newBonusBalance,
			})

			// FIFO: decrement real_cents_remaining on oldest global order
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

			// Update session
			tx.Model(&lockedSession).Updates(map[string]interface{}{
				"global_real_cents_consumed":  gorm.Expr("global_real_cents_consumed + ?", realDeducted),
				"global_bonus_cents_consumed": gorm.Expr("global_bonus_cents_consumed + ?", bonusDeducted),
				"duration_sec":                gorm.Expr("duration_sec + ?", elapsed),
			})

			// Update segment
			tx.Model(segment).Updates(map[string]interface{}{
				"real_cents_consumed":  gorm.Expr("real_cents_consumed + ?", realDeducted),
				"bonus_cents_consumed": gorm.Expr("bonus_cents_consumed + ?", bonusDeducted),
				"duration_sec":         gorm.Expr("duration_sec + ?", elapsed),
			})

			// Return nil so the transaction COMMITS (deduction persists)
			return nil
		})

		if err != nil {
			if err.Error() == "global_balance_exhausted" {
				s.forceEndSessionDualBalance(sessionID, userID, &session)
				return nil, errors.New("余额不足，会话已自动结束")
			}
			return nil, err
		}

		// Force-end AFTER transaction committed so the final deduction is persisted
		if shouldForceEnd {
			s.forceEndSessionDualBalance(sessionID, userID, &session)
			return nil, errors.New("余额不足，会话已自动结束")
		}

		// Refresh balances
		database.DB.Where("id = ?", userID).First(&user)

		// Balance update is delivered via the HTTP heartbeat response;
		// the Wails BillingService emits ws:balance-update locally.

		ratePerSecond = s.getRatePerSecond(segment.RateSnapshot, segment.RateUnitSnapshot)

		// Divide by total active sessions for this user
		hbTotalGlobal := 1 + s.countOtherActiveSessions(userID, sessionID)
		remainingSeconds := combinedRemainingSeconds(tenantBalance, hbTotalGlobal, user.GetTotalBalance(), ratePerSecond)

		if remainingSeconds <= 0 {
			s.forceEndSessionDualBalance(sessionID, userID, &session)
			return nil, errors.New("余额不足，会话已自动结束")
		}

		durationSec := int(now.Sub(session.StartedAt).Seconds())

		return &SessionHeartbeatResult{
			BalanceCents:         user.GetTotalBalance(),
			RemainingSeconds:     remainingSeconds,
			DurationSeconds:      durationSec,
			CostCents:            costCents,
			BalanceSource:        "global",
			TenantBalanceMinutes: tenantBalance,
			GlobalBalanceCents:   user.GetTotalBalance(),
		}, nil
	}
}

// runSessionTick runs one billing heartbeat for a session and, if the session is still active
// afterward, reschedules itself for the next tick via scheduleSessionExpiryForDevice. Because
// each session's chain of ticks is anchored to its own start time (not a shared global clock),
// every interval is a real ~60s (or less, for the final partial stretch), so Heartbeat's
// elapsed-per-tick assumption holds and the chain naturally lands on the exact moment balance
// reaches zero instead of overshooting or cutting off early.
func (s *SessionService) runSessionTick(hostID, machineCode, sessionID, userID string) {
	result, err := s.Heartbeat(sessionID, userID)
	if err != nil {
		// Heartbeat already handles force-end + session_end WS push for exhausted balance.
		// Any other error (session already ended by logout/quit/admin action, etc.) also
		// means there's nothing left to reschedule.
		logger.Error("billing", "Tick error for session %s: %v", sessionID, err)
		return
	}

	s.scheduleSessionExpiryForDevice(hostID, machineCode, sessionID, userID, result.RemainingSeconds)
}

// getActiveSegment returns the active segment for a session
func (s *SessionService) getActiveSegment(sessionID string) (*models.BillingSessionSegment, error) {
	var segment models.BillingSessionSegment
	err := database.DB.Where("session_id = ? AND ended_at IS NULL", sessionID).First(&segment).Error
	return &segment, err
}

// scheduleSessionExpiry (re)schedules a session's next billing tick so it fires at most 60s out,
// or sooner if less than 60s of balance remains. This is both how a session's tick chain gets
// started (from startSessionInternal, or a recharge pushing the cutoff further out) and how each
// tick reschedules itself (via runSessionTick) once Heartbeat reports the balance still active.
func (s *SessionService) scheduleSessionExpiry(clientID, sessionID, userID string, remainingSeconds int) {
	var client models.UserClient
	if err := database.DB.Select("host_id", "machine_code").Where("id = ?", clientID).First(&client).Error; err != nil {
		return
	}
	s.scheduleSessionExpiryForDevice(client.HostID, client.MachineCode, sessionID, userID, remainingSeconds)
}

// scheduleSessionExpiryForDevice is scheduleSessionExpiry's underlying logic, split out so
// callers that already have the device in hand (e.g. a recharge push, which walks
// DeviceStateService's in-memory device list) can reschedule without a redundant
// client_id -> host_id/machine_code DB round trip.
//
// The delay is capped at 60s: with more than a minute of balance left there's nothing to gain
// by waiting longer between billing deductions, and capping keeps every session's tick chain
// anchored to its own cadence rather than drifting. With less than a minute left, scheduling the
// next tick for exactly that remainder is what lets the session end within moments of the
// balance actually reaching zero instead of overshooting to the next round tick.
func (s *SessionService) scheduleSessionExpiryForDevice(hostID, machineCode, sessionID, userID string, remainingSeconds int) {
	if remainingSeconds <= 0 {
		// Nothing to arm a timer for -- e.g. a brand-new session whose starting balance
		// rounds down to under a second of runway (tiny balance vs. a high rate, or split
		// thin across several concurrent sessions). Heartbeat's own remainingSeconds<=0
		// checks only run on ticks that this same timer chain would have produced, so
		// without this the session would never get a first tick and would run un-billed
		// and un-timed-out indefinitely. End it now instead.
		s.endSessionIfStillActive(sessionID, userID)
		return
	}
	if hostID == "" || machineCode == "" {
		return
	}
	delay := remainingSeconds
	if delay > 60 {
		delay = 60
	}
	expiryTime := time.Now().Add(time.Duration(delay) * time.Second)
	GetDeviceStateService().SetSessionExpiry(hostID, machineCode, expiryTime, func() {
		s.runSessionTick(hostID, machineCode, sessionID, userID)
	})
}

// endSessionIfStillActive re-checks the session is still active (it may have already ended
// via logout, a previous exhaustion, etc.) before force-ending it. Used where a caller needs
// to end a session by ID without already having it loaded.
func (s *SessionService) endSessionIfStillActive(sessionID, userID string) {
	var session models.BillingSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return
	}
	if session.EndedAt != nil {
		return
	}
	s.forceEndSessionDualBalance(sessionID, userID, &session)
}

// forceEndSessionDualBalance force-ends a session when balance is exhausted (dual-balance version)
func (s *SessionService) forceEndSessionDualBalance(sessionID, userID string, session *models.BillingSession) {
	now := time.Now()
	durationSec := int(now.Sub(session.StartedAt).Seconds())

	// Ask the device to upload its Eden save profile and close app logs before
	// the session is torn down -- otherwise a balance/time-exhausted logout
	// (unlike an explicit quit, which goes through EndSessionsByClientID)
	// never asks the device to save, and game progress is lost.
	requestSaveUploadIfNeeded(session.ClientID, sessionID)
	NewAppLogService().CloseOpenAppLogs(sessionID, "balance_zero")

	// Close active segment
	segment, err := s.getActiveSegment(sessionID)
	if err == nil {
		database.DB.Model(segment).Updates(map[string]interface{}{
			"ended_at":     now,
			"duration_sec": int(now.Sub(segment.StartedAt).Seconds()),
		})
	}

	// Calculate total cost from all segments
	var totalCost int
	database.DB.Model(&models.BillingSessionSegment{}).
		Where("session_id = ?", sessionID).
		Select("SUM(real_cents_consumed + bonus_cents_consumed)").
		Row().Scan(&totalCost)

	// Update session as ended
	database.DB.Model(&session).Updates(map[string]interface{}{
		"ended_at":     now,
		"duration_sec": durationSec,
		"total_cost":   totalCost,
		"updated_at":   now,
	})

	// Get client info for WebSocket push
	var client models.UserClient
	if database.DB.Where("id = ?", session.ClientID).First(&client).Error == nil {
		// Clear ActiveSessionID so the billing ticker skips this device
		deviceStateService := GetDeviceStateService()
		deviceStateService.SetActiveSession(client.HostID, client.MachineCode, "")
		deviceStateService.ClearSessionExpiry(client.HostID, client.MachineCode)

		// Deactivate the user_clients DB record (set status=0), same as an explicit
		// logout -- otherwise a balance-exhausted auto-logout leaves the device
		// showing up in the H5 active-client list even though it's back at idle.
		if err := NewUserClientService().DeactivateClientByHostInfo(client.HostID, client.MachineCode); err != nil {
			logger.Error("session", "Error deactivating client on balance-exhausted logout: hostID=%s, machineCode=%s, err=%v", client.HostID, client.MachineCode, err)
		}

		// Update device state to idle (session ended)
		deviceStateService.UpdateDeviceState(client.HostID, client.MachineCode, DeviceStateIdle)
		deviceStateService.UnbindUser(client.HostID, client.MachineCode)

		// Get fresh QR URL and free_play for device_state_change
		registeredDeviceService := GetRegisteredDeviceService()
		freePlay := registeredDeviceService.GetFreePlayByMachineCode(client.MachineCode)

		// Generate QR URL (need AuthService, so use GetWeChatOAService directly)
		qrURL := ""
		sceneStr := fmt.Sprintf("bind:%s:%s", client.HostID, client.MachineCode)
		oaService := GetWeChatOAService()
		if oaService != nil {
			if url, err := oaService.CreateParametricQR(sceneStr); err == nil {
				qrURL = url
			}
		}

		// Send single device_state_change with session_end event
		deviceStateService.SendMessage(client.HostID, client.MachineCode, "device_state_change", map[string]interface{}{
			"event":     "session_end",
			"state":     "idle",
			"qr_url":    qrURL,
			"free_play": freePlay,
			"session_summary": map[string]interface{}{
				"reason":           "balance_zero",
				"duration_seconds": durationSec,
				"total_cost_cents": totalCost,
				"balance_cents":    0,
			},
		})
	}
}

// EndSessionsByDevice ends all active sessions for a device (triggered by WS disconnect)
func (s *SessionService) EndSessionsByDevice(hostID, machineCode, reason string) error {
	GetDeviceStateService().ClearSessionExpiry(hostID, machineCode)

	// Find UserClient by host_id and machine_code
	var client models.UserClient
	err := database.DB.Where("host_id = ? AND machine_code = ?", hostID, machineCode).First(&client).Error
	if err != nil {
		// No client found, nothing to clean up
		return nil
	}

	// Release any moonlight host occupied by this device
	deviceID := hostID + ":" + machineCode
	if err := GetMoonlightHostService().ReleaseHost(deviceID); err != nil {
		logger.Error("session", "Failed to release moonlight host for device %s: %v", deviceID, err)
	}

	return s.EndSessionsByClientID(client.ID, reason)
}

// requestSaveUploadIfNeeded checks whether ANY Eden (Switch emulator) title
// was launched during this session -- not just whichever one is still open --
// since Eden keeps every title's save under one shared on-device profile, so
// a user who played two games this session leaves both behind either way.
// If so, asks the device to upload its whole save profile for this user
// before the session's app logs are closed.
func requestSaveUploadIfNeeded(clientID, sessionID string) {
	if !GetSaveDataService().IsEnabled() {
		return
	}

	var edenLaunchCount int64
	if err := database.DB.Model(&models.SessionAppLog{}).
		Joins("JOIN apps ON apps.id = session_app_logs.app_id").
		Where("session_app_logs.session_id = ? AND apps.launch_type = ?", sessionID, "eden").
		Count(&edenLaunchCount).Error; err != nil || edenLaunchCount == 0 {
		return // no Eden title was played this session
	}

	var session models.BillingSession
	if err := database.DB.Select("user_id").Where("id = ?", sessionID).First(&session).Error; err != nil {
		return
	}

	var client models.UserClient
	if err := database.DB.Where("id = ?", clientID).First(&client).Error; err != nil {
		return
	}

	GetDeviceStateService().SendMessage(client.HostID, client.MachineCode, "save_upload_requested", map[string]string{
		"user_id": session.UserID,
	})
}

// EndSessionsByClientID ends all active sessions for a client (cleanup method)
func (s *SessionService) EndSessionsByClientID(clientID, reason string) error {
	// Find all active session IDs for this client (lightweight query)
	var sessionIDs []string
	err := database.DB.Model(&models.BillingSession{}).
		Where("client_id = ? AND ended_at IS NULL", clientID).
		Pluck("id", &sessionIDs).Error
	if err != nil {
		return err
	}

	if len(sessionIDs) == 0 {
		return nil
	}

	// Close app logs for all sessions being ended. For any Eden (Switch
	// emulator) app still open, ask the device to upload its save data
	// first -- the local save folder gets reused/overwritten as soon as
	// the next user's session starts, so this has to happen before close.
	appLogSvc := NewAppLogService()
	for _, sid := range sessionIDs {
		requestSaveUploadIfNeeded(clientID, sid)
		appLogSvc.CloseOpenAppLogs(sid, reason)
	}

	// End each session with proper locking to prevent race with heartbeat
	for _, sessionID := range sessionIDs {
		// Use a transaction with row-level locking to prevent heartbeat race condition
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			// Lock the session row with FOR UPDATE to block concurrent heartbeat updates
			var session models.BillingSession
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND ended_at IS NULL", sessionID).
				First(&session).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					// Session already ended by another thread (e.g., balance exhaustion)
					return nil
				}
				return err
			}

			now := time.Now()
			durationSec := int(now.Sub(session.StartedAt).Seconds())

			// Close active segment
			var segment models.BillingSessionSegment
			err = tx.Where("session_id = ? AND ended_at IS NULL", sessionID).First(&segment).Error
			if err == nil {
				tx.Model(&segment).Updates(map[string]interface{}{
					"ended_at":     now,
					"duration_sec": int(now.Sub(segment.StartedAt).Seconds()),
				})
			}

			// Calculate total cost from all segments
			var totalCost int
			tx.Model(&models.BillingSessionSegment{}).
				Where("session_id = ?", sessionID).
				Select("SUM(real_cents_consumed + bonus_cents_consumed)").
				Row().Scan(&totalCost)

			// Update session to mark as ended
			return tx.Model(&session).Updates(map[string]interface{}{
				"ended_at":     now,
				"duration_sec": durationSec,
				"total_cost":   totalCost,
				"updated_at":   now,
			}).Error
		})

		if err != nil {
			// Log error but continue with other sessions
			logger.Error("session", "Error ending session %s: %v", sessionID, err)
		}
	}

	return nil
}

// GetActiveSessionByClientID returns active session for a client (if any)
func (s *SessionService) GetActiveSessionByClientID(clientID string) (*models.BillingSession, error) {
	var session models.BillingSession
	err := database.DB.
		Preload("User").
		Preload("Client").
		Preload("App").
		Preload("Plan").
		Where("client_id = ? AND ended_at IS NULL", clientID).
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetActiveSessionByUserID returns active session for a user (if any)
func (s *SessionService) GetActiveSessionByUserID(userID string) (*models.BillingSession, error) {
	var session models.BillingSession
	err := database.DB.
		Preload("User").
		Preload("Client").
		Preload("App").
		Preload("Plan").
		Where("user_id = ? AND ended_at IS NULL", userID).
		First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// CloseOrphanedSessions closes all sessions and segments left open from a previous server run.
// Should be called on startup before the billing ticker begins.
func (s *SessionService) CloseOrphanedSessions() error {
	now := time.Now()

	// Close orphaned segments first
	if err := database.DB.Model(&models.BillingSessionSegment{}).
		Where("ended_at IS NULL").
		Updates(map[string]interface{}{
			"ended_at": now,
		}).Error; err != nil {
		return err
	}

	// Close orphaned sessions, computing total_cost from segments
	var sessions []models.BillingSession
	if err := database.DB.Where("ended_at IS NULL").Find(&sessions).Error; err != nil {
		return err
	}

	for _, session := range sessions {
		durationSec := int(now.Sub(session.StartedAt).Seconds())

		var totalCost int
		row := database.DB.Model(&models.BillingSessionSegment{}).
			Where("session_id = ?", session.ID).
			Select("COALESCE(SUM(real_cents_consumed + bonus_cents_consumed), 0)").
			Row()
		if err := row.Scan(&totalCost); err != nil {
			totalCost = 0
		}

		database.DB.Model(&session).Updates(map[string]interface{}{
			"ended_at":     now,
			"duration_sec": durationSec,
			"total_cost":   totalCost,
			"updated_at":   now,
		})
	}

	if len(sessions) > 0 {
		logger.Info("session", "Closed %d orphaned sessions on startup", len(sessions))
	}

	return nil
}

// SessionReconnectData contains all state the client needs after a WS reconnect
type SessionReconnectData struct {
	SessionID            string                 `json:"session_id"`
	BalanceSource        string                 `json:"balance_source"`
	PlanRateCents        int                    `json:"plan_rate_cents"`
	PlanUnit             string                 `json:"plan_unit"`
	RemainingSeconds     int                    `json:"remaining_seconds"`
	BalanceCents         int                    `json:"balance_cents"`
	TenantBalanceMinutes int                    `json:"tenant_balance_minutes"`
	GlobalBalanceCents   int                    `json:"global_balance_cents"`
	User                 map[string]interface{} `json:"user"`
	OtherSessionCount    int                    `json:"other_session_count"`
}

// GetSessionReconnectData fetches everything the client needs to resume a session
func (s *SessionService) GetSessionReconnectData(sessionID string) (*SessionReconnectData, error) {
	var session models.BillingSession
	err := database.DB.Preload("User").Where("id = ? AND ended_at IS NULL", sessionID).First(&session).Error
	if err != nil {
		return nil, err
	}

	// Get active segment for balance source and rate
	segment, err := s.getActiveSegment(sessionID)
	if err != nil {
		return nil, err
	}

	// Count other active sessions for this user
	otherSessionCount := s.countOtherActiveSessions(session.UserID, sessionID)
	reconTotalSessions := 1 + otherSessionCount

	// Fetch both balance pools
	var tenantMinutes int
	tenantBalanceService := NewTenantBalanceService()
	if session.TenantID != "" {
		tb, err := tenantBalanceService.GetBalance(session.UserID, session.TenantID)
		if err == nil {
			tenantMinutes = tb
		}
	}

	var globalCents int
	var balanceCents int
	var userMap map[string]interface{}
	if session.User != nil {
		globalCents = session.User.GetTotalBalance()
		balanceCents = globalCents
		userMap = map[string]interface{}{
			"id":         session.User.ID,
			"nickname":   session.User.Nickname,
			"avatar_url": session.User.AvatarURL,
		}
	}

	// Combined remaining: tenant time + global time
	ratePerSecond := s.getRatePerSecond(segment.RateSnapshot, segment.RateUnitSnapshot)
	remainingSeconds := combinedRemainingSeconds(tenantMinutes, reconTotalSessions, globalCents, ratePerSecond)

	return &SessionReconnectData{
		SessionID:            sessionID,
		BalanceSource:        segment.BalanceSource,
		PlanRateCents:        segment.RateSnapshot,
		PlanUnit:             segment.RateUnitSnapshot,
		RemainingSeconds:     remainingSeconds,
		BalanceCents:         balanceCents,
		TenantBalanceMinutes: tenantMinutes,
		GlobalBalanceCents:   globalCents,
		User:                 userMap,
		OtherSessionCount:    otherSessionCount,
	}, nil
}

// CalculateSessionCost calculates the current cost of an active session
func (s *SessionService) CalculateSessionCost(session *models.BillingSession) int {
	if session.EndedAt != nil {
		// Session already ended, return stored cost
		return session.TotalCost
	}

	// Calculate duration from start to now
	durationSec := int(time.Since(session.StartedAt).Seconds())

	// Get the plan (already preloaded)
	if session.Plan == nil {
		return 0
	}

	return s.calculateCost(session.Plan, durationSec)
}
