package client

import (
	"net/http"
	"strconv"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type SystemConfigHandler struct {
	configService *services.SystemConfigService
}

func NewSystemConfigHandler() *SystemConfigHandler {
	return &SystemConfigHandler{
		configService: services.GetSystemConfigService(),
	}
}

// GetMoonlightHost handles GET /client/config/moonlight_host (CLT_CFG_ML_HOST)
func (h *SystemConfigHandler) GetMoonlightHost(c *gin.Context) {
	config, err := h.configService.GetMoonlightHost()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve configuration",
			"data":    nil,
		})
		return
	}

	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Configuration not found",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"host": config.Value,
		},
	})
}

// GetMoonlightPort handles GET /client/config/moonlight_port (CLT_CFG_ML_PORT)
func (h *SystemConfigHandler) GetMoonlightPort(c *gin.Context) {
	config, err := h.configService.GetMoonlightPort()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve configuration",
			"data":    nil,
		})
		return
	}

	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Configuration not found",
			"data":    nil,
		})
		return
	}

	// Convert value to integer (0 if empty/not configured)
	port := 0
	if config.Value != "" {
		// Parse port value (already validated by service layer)
		if p, err := strconv.Atoi(config.Value); err == nil {
			port = p
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"port": port,
		},
	})
}
