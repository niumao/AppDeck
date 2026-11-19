package services

import (
	"appdeck/database"
	"appdeck/models"
	"time"
)

// RevenueService 收入服务
type RevenueService struct{}

// NewRevenueService 创建收入服务实例
func NewRevenueService() *RevenueService {
	return &RevenueService{}
}

// RevenueSummaryParams 收入汇总查询参数
type RevenueSummaryParams struct {
	Period   string    // daily|weekly|monthly
	DateFrom time.Time // 开始日期（包含）
	DateTo   time.Time // 结束日期（包含）
	Page     int       // 当前页码（从1开始）
	Limit    int       // 每页数量
}

// BreakdownItem 收入分解数据项
type BreakdownItem struct {
	PeriodStart     string `json:"period_start"`      // ISO 8601
	PeriodEnd       string `json:"period_end"`        // ISO 8601
	RevenueCents    int    `json:"revenue_cents"`     // 实际收入（分）
	SessionCount    int    `json:"session_count"`     // 完成会话数
	DurationMinutes int    `json:"duration_minutes"`  // 总会话时长（分钟）
	UserCount       int    `json:"user_count"`        // 活跃用户数
}

// BreakdownPagination 分解数据分页响应
type BreakdownPagination struct {
	Items []BreakdownItem `json:"items"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

// RevenueSummary 收入汇总响应
type RevenueSummary struct {
	TotalRevenueCents    int                 `json:"total_revenue_cents"`
	TotalSessions        int                 `json:"total_sessions"`
	TotalDurationMinutes int                 `json:"total_duration_minutes"`
	ActiveUsersCount     int                 `json:"active_users_count"`
	GlobalPackageSales   int                 `json:"global_package_sales"`
	TenantPackageSales   int                 `json:"tenant_package_sales"`
	TotalRefunds         int                 `json:"total_refunds"`
	TotalBonusIssued     int                 `json:"total_bonus_issued"`
	Breakdown            BreakdownPagination `json:"breakdown"`
}

// GetRevenueSummary 获取收入汇总数据
func (s *RevenueService) GetRevenueSummary(params RevenueSummaryParams) (*RevenueSummary, error) {
	db := database.DB

	// 查询总体统计（完成的会话：ended_at IS NOT NULL）
	var totalRevenueCents int64
	var totalSessions int64
	var totalDurationSec int64
	var activeUsersCount int64

	query := db.Model(&models.BillingSession{}).
		Where("ended_at IS NOT NULL").
		Where("started_at >= ? AND started_at <= ?", params.DateFrom, params.DateTo)

	if err := query.Select("COALESCE(SUM(total_cost), 0)").Scan(&totalRevenueCents).Error; err != nil {
		return nil, err
	}

	if err := query.Count(&totalSessions).Error; err != nil {
		return nil, err
	}

	if err := query.Select("COALESCE(SUM(duration_sec), 0)").Scan(&totalDurationSec).Error; err != nil {
		return nil, err
	}

	if err := query.Select("COUNT(DISTINCT user_id)").Scan(&activeUsersCount).Error; err != nil {
		return nil, err
	}

	// Query recharge order stats
	var globalPackageSales int64
	var tenantPackageSales int64
	var totalRefunds int64
	var totalBonusIssued int64

	// Global package sales: SUM(amount_cents) WHERE scope='global' AND status=1
	if err := db.Model(&models.RechargeOrder{}).
		Where("scope = ? AND status = ?", "global", 1).
		Select("COALESCE(SUM(amount_cents), 0)").
		Scan(&globalPackageSales).Error; err != nil {
		return nil, err
	}

	// Tenant package sales: SUM(amount_cents) WHERE scope='tenant' AND status=1
	if err := db.Model(&models.RechargeOrder{}).
		Where("scope = ? AND status = ?", "tenant", 1).
		Select("COALESCE(SUM(amount_cents), 0)").
		Scan(&tenantPackageSales).Error; err != nil {
		return nil, err
	}

	// Total refunds: SUM(refunded_cents) WHERE status IN (1,3)
	if err := db.Model(&models.RechargeOrder{}).
		Where("status IN ?", []int{1, 3}).
		Select("COALESCE(SUM(refunded_cents), 0)").
		Scan(&totalRefunds).Error; err != nil {
		return nil, err
	}

	// Total bonus issued: SUM(bonus_cents) WHERE scope='global' AND status=1
	if err := db.Model(&models.RechargeOrder{}).
		Where("scope = ? AND status = ?", "global", 1).
		Select("COALESCE(SUM(bonus_cents), 0)").
		Scan(&totalBonusIssued).Error; err != nil {
		return nil, err
	}

	// 生成分解数据
	breakdown, err := s.generateBreakdown(params)
	if err != nil {
		return nil, err
	}

	return &RevenueSummary{
		TotalRevenueCents:    int(totalRevenueCents),
		TotalSessions:        int(totalSessions),
		TotalDurationMinutes: int(totalDurationSec / 60),
		ActiveUsersCount:     int(activeUsersCount),
		GlobalPackageSales:   int(globalPackageSales),
		TenantPackageSales:   int(tenantPackageSales),
		TotalRefunds:         int(totalRefunds),
		TotalBonusIssued:     int(totalBonusIssued),
		Breakdown:            breakdown,
	}, nil
}

// generateBreakdown 生成按时间段分解的数据（分页）
func (s *RevenueService) generateBreakdown(params RevenueSummaryParams) (BreakdownPagination, error) {
	db := database.DB

	// 计算所有日期段（降序，最新的在前）
	periods := s.calculatePeriods(params)
	totalPeriods := len(periods)

	// 计算分页范围
	startIdx := (params.Page - 1) * params.Limit
	endIdx := startIdx + params.Limit

	// 边界检查
	if startIdx >= totalPeriods {
		// 超出范围，返回空结果
		return BreakdownPagination{
			Items: []BreakdownItem{},
			Total: totalPeriods,
			Page:  params.Page,
			Limit: params.Limit,
		}, nil
	}

	if endIdx > totalPeriods {
		endIdx = totalPeriods
	}

	// 只查询当前页的时间段
	pagePeriods := periods[startIdx:endIdx]
	var results []BreakdownItem

	for _, period := range pagePeriods {
		var revenueCents int64
		var sessionCount int64
		var durationSec int64
		var userCount int64

		query := db.Model(&models.BillingSession{}).
			Where("ended_at IS NOT NULL").
			Where("started_at >= ? AND started_at < ?", period.Start, period.End)

		// 收入
		if err := query.Select("COALESCE(SUM(total_cost), 0)").Scan(&revenueCents).Error; err != nil {
			return BreakdownPagination{}, err
		}

		// 会话数
		if err := query.Count(&sessionCount).Error; err != nil {
			return BreakdownPagination{}, err
		}

		// 时长
		if err := query.Select("COALESCE(SUM(duration_sec), 0)").Scan(&durationSec).Error; err != nil {
			return BreakdownPagination{}, err
		}

		// 用户数
		if err := query.Select("COUNT(DISTINCT user_id)").Scan(&userCount).Error; err != nil {
			return BreakdownPagination{}, err
		}

		results = append(results, BreakdownItem{
			PeriodStart:     period.Start.Format(time.RFC3339),
			PeriodEnd:       period.End.Format(time.RFC3339),
			RevenueCents:    int(revenueCents),
			SessionCount:    int(sessionCount),
			DurationMinutes: int(durationSec / 60),
			UserCount:       int(userCount),
		})
	}

	return BreakdownPagination{
		Items: results,
		Total: totalPeriods,
		Page:  params.Page,
		Limit: params.Limit,
	}, nil
}

// Period 时间段
type Period struct {
	Start time.Time
	End   time.Time
}

// calculatePeriods 计算时间段列表（降序：最新的在前）
func (s *RevenueService) calculatePeriods(params RevenueSummaryParams) []Period {
	// First generate periods in ASC order, then reverse for DESC
	var periods []Period

	dateFromNormalized := time.Date(params.DateFrom.Year(), params.DateFrom.Month(), params.DateFrom.Day(), 0, 0, 0, 0, params.DateFrom.Location())
	dateToNormalized := time.Date(params.DateTo.Year(), params.DateTo.Month(), params.DateTo.Day(), 23, 59, 59, 999999999, params.DateTo.Location())

	current := dateFromNormalized

	for current.Before(dateToNormalized) {
		var periodEnd time.Time

		switch params.Period {
		case "daily":
			periodEnd = current.AddDate(0, 0, 1)
		case "weekly":
			periodEnd = current.AddDate(0, 0, 7)
		case "monthly":
			year, month, _ := current.Date()
			firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, current.Location())
			periodEnd = firstOfMonth.AddDate(0, 1, 0)
		default:
			periodEnd = current.AddDate(0, 0, 1)
		}

		// Don't exceed date range
		if periodEnd.After(dateToNormalized.AddDate(0, 0, 1)) {
			periodEnd = time.Date(dateToNormalized.Year(), dateToNormalized.Month(), dateToNormalized.Day()+1, 0, 0, 0, 0, dateToNormalized.Location())
		}

		periods = append(periods, Period{
			Start: current,
			End:   periodEnd,
		})

		current = periodEnd

		if len(periods) > 10000 {
			break
		}
	}

	// Reverse for DESC order (newest first)
	for i, j := 0, len(periods)-1; i < j; i, j = i+1, j-1 {
		periods[i], periods[j] = periods[j], periods[i]
	}

	return periods
}
