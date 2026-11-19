package client

import (
	"net/http"
	"strings"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type MoonlightHostsHandler struct {
	hostService *services.MoonlightHostService
}

func NewMoonlightHostsHandler() *MoonlightHostsHandler {
	return &MoonlightHostsHandler{
		hostService: services.GetMoonlightHostService(),
	}
}

// AcquireHost handles POST /client/moonlight/acquire (CLT_MH_ACQUIRE)
func (h *MoonlightHostsHandler) AcquireHost(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: device_id is required",
			"data":    nil,
		})
		return
	}

	host, err := h.hostService.AcquireHost(req.DeviceID)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "NO_AVAILABLE_HOSTS" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "NO_AVAILABLE_HOSTS",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to acquire host",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":   host.ID,
			"host": host.Host,
			"port": host.Port,
		},
	})
}

// ReleaseHost handles POST /client/moonlight/release (CLT_MH_RELEASE)
func (h *MoonlightHostsHandler) ReleaseHost(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: device_id is required",
			"data":    nil,
		})
		return
	}

	err := h.hostService.ReleaseHost(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to release host",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"success": true,
		},
	})
}

// GetAssignedHost handles GET /client/moonlight/assigned (CLT_MH_ASSIGNED)
func (h *MoonlightHostsHandler) GetAssignedHost(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "device_id query parameter is required",
			"data":    nil,
		})
		return
	}

	host, err := h.hostService.GetAssignedHost(deviceID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve assigned host",
			"data":    nil,
		})
		return
	}

	if host == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No host assigned to this device",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":   host.ID,
			"host": host.Host,
			"port": host.Port,
		},
	})
}
