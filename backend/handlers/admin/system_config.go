package admin

import (
	"net/http"
	"strings"

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

// GetMoonlightHost handles GET /admin/config/moonlight_host (ADM_CFG_ML_GET)
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
			"key":         config.Key,
			"value":       config.Value,
			"description": config.Description,
			"updated_at":  config.UpdatedAt,
		},
	})
}

// UpdateMoonlightHost handles PUT /admin/config/moonlight_host (ADM_CFG_ML_UPDATE)
func (h *SystemConfigHandler) UpdateMoonlightHost(c *gin.Context) {
	var req struct {
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: value is required",
			"data":    nil,
		})
		return
	}

	// Update the configuration (includes validation)
	config, err := h.configService.UpdateMoonlightHost(req.Value)
	if err != nil {
		// Check if it's a validation error
		if strings.Contains(err.Error(), "INVALID_HOSTNAME") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "INVALID_HOSTNAME",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update configuration",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"key":         config.Key,
			"value":       config.Value,
			"description": config.Description,
			"updated_at":  config.UpdatedAt,
		},
	})
}

// GetMoonlightPort handles GET /admin/config/moonlight_port (ADM_CFG_ML_PORT_GET)
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

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"key":         config.Key,
			"value":       config.Value,
			"description": config.Description,
			"updated_at":  config.UpdatedAt,
		},
	})
}

// UpdateMoonlightPort handles PUT /admin/config/moonlight_port (ADM_CFG_ML_PORT_UPDATE)
func (h *SystemConfigHandler) UpdateMoonlightPort(c *gin.Context) {
	var req struct {
		Value string `json:"value"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	// Update the configuration (includes validation)
	config, err := h.configService.UpdateMoonlightPort(req.Value)
	if err != nil {
		// Check if it's a validation error
		if strings.Contains(err.Error(), "INVALID_PORT") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "INVALID_PORT",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update configuration",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"key":         config.Key,
			"value":       config.Value,
			"description": config.Description,
			"updated_at":  config.UpdatedAt,
		},
	})
}
