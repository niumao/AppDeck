package admin

import (
	"net/http"
	"strconv"
	"time"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// RevenueHandler 收入管理处理器
type RevenueHandler struct {
	revenueService *services.RevenueService
}

// NewRevenueHandler 创建收入管理处理器
func NewRevenueHandler() *RevenueHandler {
	return &RevenueHandler{
		revenueService: services.NewRevenueService(),
	}
}

// GetRevenueSummary 获取收入汇总
// GET /api/v1/admin/revenue/summary
func (h *RevenueHandler) GetRevenueSummary(c *gin.Context) {
	// 解析分页参数
	page := 1
	limit := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 解析 period 参数（默认 daily）
	period := c.Query("period")
	if period == "" {
		period = "daily"
	}

	// 验证 period 参数
	if period != "daily" && period != "weekly" && period != "monthly" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的period参数，必须是 daily、weekly 或 monthly",
			"data":    nil,
		})
		return
	}

	// 解析日期参数（可选，默认最近30天）
	dateFromStr := c.Query("date_from")
	dateToStr := c.Query("date_to")

	var dateFrom, dateTo time.Time
	var err error

	if dateFromStr != "" {
		dateFrom, err = time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			// 尝试简单日期格式 YYYY-MM-DD
			dateFrom, err = time.Parse("2006-01-02", dateFromStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "date_from格式错误，应为ISO 8601格式（如：2026-01-01 或 2026-01-01T00:00:00Z）",
					"data":    nil,
				})
				return
			}
		}
	} else {
		// 默认：30天前
		dateFrom = time.Now().AddDate(0, 0, -30)
	}

	if dateToStr != "" {
		dateTo, err = time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			// 尝试简单日期格式 YYYY-MM-DD
			dateTo, err = time.Parse("2006-01-02", dateToStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "date_to格式错误，应为ISO 8601格式（如：2026-01-01 或 2026-01-01T00:00:00Z）",
					"data":    nil,
				})
				return
			}
		}
	} else {
		// 默认：今天
		dateTo = time.Now()
	}

	// 规范化时间：去除时分秒，统一为日期开始
	dateFrom = time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, dateFrom.Location())
	dateTo = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, 999999999, dateTo.Location())

	// 调用服务获取数据
	summary, err := h.revenueService.GetRevenueSummary(services.RevenueSummaryParams{
		Period:   period,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Page:     page,
		Limit:    limit,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取收入汇总失败",
			"data":    nil,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}
