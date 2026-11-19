package admin

import (
	"appdeck/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SteamHandler struct {
	service *services.SteamService
}

func NewSteamHandler() *SteamHandler {
	return &SteamHandler{
		service: services.NewSteamService(),
	}
}

func (h *SteamHandler) Search(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "搜索关键词不能为空",
			"data":    nil,
		})
		return
	}

	results, err := h.service.Search(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    results,
	})
}

func (h *SteamHandler) ValidateSteamID(c *gin.Context) {
	var req struct {
		SteamID string `json:"steam_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
			"data":    nil,
		})
		return
	}

	isValid := h.service.ValidateSteamID(req.SteamID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"valid":    isValid,
			"steam_id": req.SteamID,
		},
	})
}
