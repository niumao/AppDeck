package admin

import (
	"log"
	"net/http"
	"strconv"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type TenantsHandler struct {
	tenantService           *services.TenantService
	deviceStateService      *services.DeviceStateService
	registeredDeviceService *services.RegisteredDeviceService
}

func NewTenantsHandler() *TenantsHandler {
	return &TenantsHandler{
		tenantService:           services.GetTenantService(),
		deviceStateService:      services.GetDeviceStateService(),
		registeredDeviceService: services.GetRegisteredDeviceService(),
	}
}

// GetAll handles GET /admin/tenants
func (h *TenantsHandler) GetAll(c *gin.Context) {
	page := 1
	limit := 20
	search := c.Query("search")

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	tenants, total, err := h.tenantService.GetAll(page, limit, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve tenants",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"tenants": tenants,
			"total":   total,
			"page":    page,
			"limit":   limit,
		},
	})
}

// GetAllActive handles GET /admin/tenants/active (for dropdowns)
func (h *TenantsHandler) GetAllActive(c *gin.Context) {
	tenants, err := h.tenantService.GetAllActive()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve tenants",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tenants,
	})
}

// Create handles POST /admin/tenants
func (h *TenantsHandler) Create(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Password    string `json:"password" binding:"required"`
		DisplayName string `json:"display_name"`
		Phone       string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "username and password are required",
			"data":    nil,
		})
		return
	}

	tenant, err := h.tenantService.Create(req.Username, req.Password, req.DisplayName, req.Phone)
	if err != nil {
		if err.Error() == "username already exists" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "username already exists",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create tenant",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    tenant,
	})
}

// Update handles PUT /admin/tenants/:id
func (h *TenantsHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		DisplayName string `json:"display_name"`
		Phone       string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	if err := h.tenantService.Update(id, req.DisplayName, req.Phone); err != nil {
		if err.Error() == "TENANT_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Tenant not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update tenant",
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

// UpdatePassword handles PUT /admin/tenants/:id/password
func (h *TenantsHandler) UpdatePassword(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "password is required",
			"data":    nil,
		})
		return
	}

	if err := h.tenantService.UpdatePassword(id, req.Password); err != nil {
		if err.Error() == "TENANT_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Tenant not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update password",
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

// UpdateStatus handles PUT /admin/tenants/:id/status
func (h *TenantsHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status int `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "status must be 0 or 1",
			"data":    nil,
		})
		return
	}

	if err := h.tenantService.UpdateStatus(id, req.Status); err != nil {
		if err.Error() == "TENANT_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Tenant not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update status",
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

// Delete handles DELETE /admin/tenants/:id
func (h *TenantsHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.tenantService.Delete(id); err != nil {
		if err.Error() == "TENANT_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Tenant not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete tenant",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"success": true},
	})
}

// UpdateBillingConfig handles PUT /admin/tenants/:id/billing
func (h *TenantsHandler) UpdateBillingConfig(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		PlanSetID     *string `json:"plan_set_id"`
		GlobalEnabled *int    `json:"global_enabled"`
		FreePlay      *int    `json:"free_play"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request", "data": nil})
		return
	}
	if err := h.tenantService.UpdateBillingConfig(id, req.PlanSetID, req.GlobalEnabled, req.FreePlay); err != nil {
		if err.Error() == "TENANT_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Tenant not found", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to update billing config", "data": nil})
		return
	}

	// Push free_play change to connected devices belonging to this tenant
	if req.FreePlay != nil {
		machineCodes := h.registeredDeviceService.GetMachineCodesByTenantID(id)
		log.Printf("[tenants] free_play changed to %d for tenant %s, found %d registered devices", *req.FreePlay, id, len(machineCodes))
		sent := h.deviceStateService.SendToDevicesByMachineCodes(machineCodes, "device_state_change", map[string]interface{}{
			"event":     "free_play_change",
			"free_play": *req.FreePlay == 1,
		})
		log.Printf("[tenants] free_play_change pushed to %d connected devices", sent)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"id": id}})
}
