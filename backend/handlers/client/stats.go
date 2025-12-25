package client

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	appService *services.AppService
}

func NewStatsHandler() *StatsHandler {
	return &StatsHandler{
		appService: services.NewAppService(),
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
		Clicks []ClickData `json:"clicks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
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
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "上报成功",
	})
}


