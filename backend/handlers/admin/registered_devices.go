package admin

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type RegisteredDevicesHandler struct {
	deviceService      *services.RegisteredDeviceService
	deviceStateService *services.DeviceStateService
}

func NewRegisteredDevicesHandler() *RegisteredDevicesHandler {
	return &RegisteredDevicesHandler{
		deviceService:      services.GetRegisteredDeviceService(),
		deviceStateService: services.GetDeviceStateService(),
	}
}

// GetAll handles GET /admin/registered-devices (ADM_RDEV_LIST)
func (h *RegisteredDevicesHandler) GetAll(c *gin.Context) {
	search := c.Query("search")
	status := c.Query("status")

	devices, err := h.deviceService.ListAll(search, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve devices",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    devices,
	})
}

// Create handles POST /admin/registered-devices (ADM_RDEV_CREATE)
func (h *RegisteredDevicesHandler) Create(c *gin.Context) {
	var req struct {
		MachineCode string  `json:"machine_code" binding:"required"`
		HostID      string  `json:"host_id" binding:"required"`
		DeviceName  *string `json:"device_name"`
		Remark      *string `json:"remark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: machine_code and host_id are required",
			"data":    nil,
		})
		return
	}

	deviceName := ""
	if req.DeviceName != nil {
		deviceName = *req.DeviceName
	}

	remark := ""
	if req.Remark != nil {
		remark = *req.Remark
	}

	device, err := h.deviceService.Create(req.MachineCode, req.HostID, deviceName, remark)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "INVALID_MACHINE_CODE") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "INVALID_MACHINE_CODE",
				"data":    nil,
			})
			return
		}
		if errMsg == "DUPLICATE_MACHINE_CODE" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "DUPLICATE_MACHINE_CODE",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create device",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id": device.ID,
		},
	})
}

// Approve handles POST /admin/registered-devices/:id/approve (ADM_RDEV_APPROVE)
func (h *RegisteredDevicesHandler) Approve(c *gin.Context) {
	id := c.Param("id")

	err := h.deviceService.Approve(id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to approve device",
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

// Reject handles POST /admin/registered-devices/:id/reject (ADM_RDEV_REJECT)
func (h *RegisteredDevicesHandler) Reject(c *gin.Context) {
	id := c.Param("id")

	err := h.deviceService.Reject(id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to reject device",
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

// Update handles PUT /admin/registered-devices/:id (ADM_RDEV_UPDATE)
func (h *RegisteredDevicesHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		DeviceName *string `json:"device_name"`
		Remark     *string `json:"remark"`
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
	if req.DeviceName == nil && req.Remark == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "At least one field must be provided",
			"data":    nil,
		})
		return
	}

	err := h.deviceService.Update(id, req.DeviceName, req.Remark)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update device",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id": id,
		},
	})
}

// Delete handles DELETE /admin/registered-devices/:id (ADM_RDEV_DELETE)
func (h *RegisteredDevicesHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	err := h.deviceService.Delete(id)
	if err != nil {
		errMsg := err.Error()
		if errMsg == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete device",
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

// AssignTenant handles PUT /admin/registered-devices/:id/assign-tenant
func (h *RegisteredDevicesHandler) AssignTenant(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		TenantID *string `json:"tenant_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	if err := h.deviceService.AssignTenant(id, req.TenantID); err != nil {
		if err.Error() == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to assign tenant",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}

var timeFormatRegex = regexp.MustCompile(`^\d{2}:\d{2}$`)

// UpdateSchedule handles PUT /admin/registered-devices/:id/schedule
func (h *RegisteredDevicesHandler) UpdateSchedule(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ShutdownEnabled bool   `json:"shutdown_enabled"`
		ShutdownTime    string `json:"shutdown_time"`
		PoweronEnabled  bool   `json:"poweron_enabled"`
		PoweronTime     string `json:"poweron_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	// Validate time formats
	if !isValidTime(req.ShutdownTime) || !isValidTime(req.PoweronTime) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid time format, expected HH:MM (00-23:00-59)",
			"data":    nil,
		})
		return
	}

	// Validate >= 10 minute gap between poweron and shutdown when both enabled
	if req.ShutdownEnabled && req.PoweronEnabled {
		if !hasMinGap(req.ShutdownTime, req.PoweronTime, 10) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Auto power-on and auto power-off times must be at least 10 minutes apart",
				"data":    nil,
			})
			return
		}
	}

	device, err := h.deviceService.UpdateSchedule(id, req.ShutdownEnabled, req.ShutdownTime, req.PoweronEnabled, req.PoweronTime)
	if err != nil {
		if err.Error() == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update schedule",
			"data":    nil,
		})
		return
	}

	// Push power_schedule to device if online
	scheduleData := gin.H{
		"shutdown_enabled": req.ShutdownEnabled,
		"shutdown_time":    req.ShutdownTime,
		"poweron_enabled":  req.PoweronEnabled,
		"poweron_time":     req.PoweronTime,
	}
	h.deviceStateService.SendToDevicesByMachineCodes([]string{device.MachineCode}, "power_schedule", scheduleData)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}

// UpdateBypassCharge handles PUT /admin/registered-devices/:id/bypass-charge
func (h *RegisteredDevicesHandler) UpdateBypassCharge(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		BypassChargeEnabled bool `json:"bypass_charge_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	device, err := h.deviceService.UpdateBypassCharge(id, req.BypassChargeEnabled)
	if err != nil {
		if err.Error() == "DEVICE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update bypass charge",
			"data":    nil,
		})
		return
	}

	// Push bypass_charge to device if online
	h.deviceStateService.SendToDevicesByMachineCodes([]string{device.MachineCode}, "bypass_charge", gin.H{
		"enabled": req.BypassChargeEnabled,
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}

func isValidTime(t string) bool {
	if !timeFormatRegex.MatchString(t) {
		return false
	}
	parts := strings.Split(t, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

// hasMinGap checks that two HH:MM times are at least minMinutes apart on a 24h clock.
func hasMinGap(t1, t2 string, minMinutes int) bool {
	p1 := strings.Split(t1, ":")
	p2 := strings.Split(t2, ":")
	h1, _ := strconv.Atoi(p1[0])
	m1, _ := strconv.Atoi(p1[1])
	h2, _ := strconv.Atoi(p2[0])
	m2, _ := strconv.Atoi(p2[1])
	total1 := h1*60 + m1
	total2 := h2*60 + m2
	diff := total1 - total2
	if diff < 0 {
		diff = -diff
	}
	// Shortest gap on circular 24h clock
	if diff > 720 {
		diff = 1440 - diff
	}
	return diff >= minMinutes
}
