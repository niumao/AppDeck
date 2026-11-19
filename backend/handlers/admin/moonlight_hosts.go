package admin

import (
	"log"
	"net/http"
	"strings"

	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type MoonlightHostsHandler struct {
	hostService        *services.MoonlightHostService
	deviceStateService *services.DeviceStateService
}

func NewMoonlightHostsHandler() *MoonlightHostsHandler {
	return &MoonlightHostsHandler{
		hostService:        services.GetMoonlightHostService(),
		deviceStateService: services.GetDeviceStateService(),
	}
}

// GetAll handles GET /admin/moonlight/hosts (ADM_MH_LIST)
func (h *MoonlightHostsHandler) GetAll(c *gin.Context) {
	hosts, err := h.hostService.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve hosts",
			"data":    nil,
		})
		return
	}

	// Build response with sunshine credential indicators
	// (model has json:"-" on SunshineUser/SunshinePass for security)
	result := make([]gin.H, len(hosts))
	for i, host := range hosts {
		result[i] = gin.H{
			"id":                   host.ID,
			"host":                 host.Host,
			"port":                 host.Port,
			"enabled":              host.Enabled,
			"paired":               host.Paired,
			"sunshine_user":        host.SunshineUser,
			"has_sunshine_pass":    host.SunshinePass != "",
			"occupied_by_device_id": host.OccupiedByDeviceID,
			"occupied_at":          host.OccupiedAt,
			"created_at":           host.CreatedAt,
			"updated_at":           host.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// Create handles POST /admin/moonlight/hosts (ADM_MH_CREATE)
func (h *MoonlightHostsHandler) Create(c *gin.Context) {
	var req struct {
		Host         string `json:"host" binding:"required"`
		Port         *int   `json:"port"`
		SunshineUser string `json:"sunshine_user"`
		SunshinePass string `json:"sunshine_pass"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: host is required",
			"data":    nil,
		})
		return
	}

	port := 0
	if req.Port != nil {
		port = *req.Port
	}

	host, err := h.hostService.Create(req.Host, port, req.SunshineUser, req.SunshinePass)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "INVALID_HOST") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "INVALID_HOST", "data": nil})
			return
		}
		if strings.Contains(errMsg, "INVALID_PORT") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "INVALID_PORT", "data": nil})
			return
		}
		if errMsg == "DUPLICATE_HOST_PORT" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "DUPLICATE_HOST_PORT", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create host", "data": nil})
		return
	}

	// Auto-trigger pairing if sunshine credentials provided
	if req.SunshineUser != "" && req.SunshinePass != "" {
		go h.triggerPairForHost(host)
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":      host.ID,
			"host":    host.Host,
			"port":    host.Port,
			"enabled": host.Enabled,
			"paired":  host.Paired,
		},
	})
}

// Update handles PUT /admin/moonlight/hosts/:id (ADM_MH_UPDATE)
func (h *MoonlightHostsHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Host         *string `json:"host"`
		Port         *int    `json:"port"`
		Enabled      *bool   `json:"enabled"`
		SunshineUser *string `json:"sunshine_user"`
		SunshinePass *string `json:"sunshine_pass"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	// Check at least one field is provided
	if req.Host == nil && req.Port == nil && req.Enabled == nil && req.SunshineUser == nil && req.SunshinePass == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "At least one field must be provided",
			"data":    nil,
		})
		return
	}

	err := h.hostService.Update(id, req.Host, req.Port, req.Enabled, req.SunshineUser, req.SunshinePass)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "HOST_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Host not found",
				"data":    nil,
			})
			return
		}
		if errMsg == "HOST_OCCUPIED" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "HOST_OCCUPIED",
				"data":    nil,
			})
			return
		}
		if strings.Contains(errMsg, "INVALID_HOST") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "INVALID_HOST",
				"data":    nil,
			})
			return
		}
		if strings.Contains(errMsg, "INVALID_PORT") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "INVALID_PORT",
				"data":    nil,
			})
			return
		}
		if errMsg == "DUPLICATE_HOST_PORT" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "DUPLICATE_HOST_PORT",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update host",
			"data":    nil,
		})
		return
	}

	// Retrieve updated host to return
	host, err := h.hostService.GetByID(id)
	if err != nil || host == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve updated host",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":      host.ID,
			"host":    host.Host,
			"port":    host.Port,
			"enabled": host.Enabled,
			"paired":  host.Paired,
		},
	})
}

// Delete handles DELETE /admin/moonlight/hosts/:id (ADM_MH_DELETE)
func (h *MoonlightHostsHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	err := h.hostService.Delete(id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "HOST_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Host not found",
				"data":    nil,
			})
			return
		}
		if errMsg == "HOST_OCCUPIED" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "HOST_OCCUPIED",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete host",
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

// ForceRelease handles POST /admin/moonlight/hosts/:id/release (ADM_MH_FORCE_RELEASE)
func (h *MoonlightHostsHandler) ForceRelease(c *gin.Context) {
	id := c.Param("id")

	err := h.hostService.ForceRelease(id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "HOST_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Host not found",
				"data":    nil,
			})
			return
		}

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

// TriggerPair handles POST /admin/moonlight/hosts/:id/pair
func (h *MoonlightHostsHandler) TriggerPair(c *gin.Context) {
	id := c.Param("id")

	host, err := h.hostService.GetByIDWithCredentials(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to retrieve host", "data": nil})
		return
	}
	if host == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Host not found", "data": nil})
		return
	}

	if host.SunshineUser == "" || host.SunshinePass == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "MISSING_SUNSHINE_CREDENTIALS", "data": nil})
		return
	}

	go h.triggerPairForHost(host)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Pairing initiated",
		"data":    gin.H{"status": "pairing"},
	})
}

// triggerPairForHost broadcasts a pair_request to ALL connected clients
func (h *MoonlightHostsHandler) triggerPairForHost(host *models.MoonlightHost) {
	sunshinePort := 47990

	sent := h.deviceStateService.SendToAllDevices("pair_request", map[string]interface{}{
		"host_id":       host.ID,
		"host":          host.Host,
		"sunshine_port": sunshinePort,
		"sunshine_user": host.SunshineUser,
		"sunshine_pass": host.SunshinePass,
	})
	if sent == 0 {
		log.Printf("[pair] no connected devices to send pair_request for host %s", host.Host)
	} else {
		log.Printf("[pair] pair_request sent to %d device(s) for host %s", sent, host.Host)
	}
}
