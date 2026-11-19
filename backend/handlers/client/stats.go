package client

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	appService              *services.AppService
	registeredDeviceService *services.RegisteredDeviceService
}

func NewStatsHandler() *StatsHandler {
	return &StatsHandler{
		appService:              services.NewAppService(),
		registeredDeviceService: services.GetRegisteredDeviceService(),
	}
}

// ClickData 点击统计数据
type ClickData struct {
	AppID string `json:"appId" binding:"required"`
	Count int    `json:"count" binding:"required"`
}

// UploadClicks 批量上报点击统计
// POST /api/v1/stats/clicks
func (h *StatsHandler) UploadClicks(c *gin.Context) {
	var req struct {
		MachineCode string      `json:"machineCode"`
		Clicks      []ClickData `json:"clicks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	tenantID := ""
	if req.MachineCode != "" {
		tenantID = h.registeredDeviceService.GetTenantIDByMachineCode(req.MachineCode)
	}

	// 批量更新点击统计
	for _, click := range req.Clicks {
		if err := h.appService.UpdateClickCount(click.AppID, click.Count); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "更新点击统计失败",
				"error":   err.Error(),
			})
			return
		}

		if tenantID != "" {
			if err := h.appService.UpdateTenantClickCount(tenantID, click.AppID, click.Count); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "更新地区点击统计失败",
					"error":   err.Error(),
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上报成功",
	})
}
