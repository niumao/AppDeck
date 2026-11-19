package services

import (
	"appdeck/database"
	"appdeck/models"
)

// secondsPerUnit returns how many seconds are in one billing unit ("minute",
// "hour", "day"). fallback is returned for any other unit string, so callers
// that need different fallback behavior for unrecognized units keep it.
func secondsPerUnit(unit string, fallback float64) float64 {
	switch unit {
	case "minute":
		return 60.0
	case "hour":
		return 3600.0
	case "day":
		return 86400.0
	default:
		return fallback
	}
}

// calculateCost computes the ceiling-rounded cost for a finished session,
// used for final settlement (a partial unit is billed as a full unit).
func (s *SessionService) calculateCost(plan *models.BillingPlan, durationSec int) int {
	if durationSec <= 0 {
		return 0
	}

	units := float64(durationSec) / secondsPerUnit(plan.Unit, 60.0)

	// Apply ceiling rounding for final settlement
	if units > float64(int(units)) {
		units = float64(int(units) + 1)
	}

	return int(units) * plan.Rate
}

// getRatePerSecond calculates rate per second from rate and unit
func (s *SessionService) getRatePerSecond(rate int, unit string) float64 {
	return float64(rate) / secondsPerUnit(unit, 3600.0)
}

// countOtherActiveSessions counts a user's other currently-active sessions,
// excluding the given session ID. Used to split remaining balance evenly
// across a user's concurrent sessions.
func (s *SessionService) countOtherActiveSessions(userID, excludeSessionID string) int {
	var count int64
	database.DB.Model(&models.BillingSession{}).
		Where("user_id = ? AND id != ? AND ended_at IS NULL", userID, excludeSessionID).
		Count(&count)
	return int(count)
}

// combinedRemainingSeconds converts a user's tenant-minute balance and
// per-second global rate into total remaining seconds, split evenly across
// totalSessions concurrent sessions for the same user.
func combinedRemainingSeconds(tenantBalanceMinutes, totalSessions int, globalBalanceCents int, ratePerSecond float64) int {
	tenantSeconds := tenantBalanceMinutes * 60 / totalSessions
	globalSeconds := 0
	if ratePerSecond > 0 {
		globalSeconds = int(float64(globalBalanceCents)/ratePerSecond) / totalSessions
	}
	return tenantSeconds + globalSeconds
}
