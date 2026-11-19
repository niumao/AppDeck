package admin

import (
	"appdeck/config"
	"appdeck/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type GlobalPackagesHandler struct {
	globalPackageService *services.GlobalPackageService
	rechargeService      *services.RechargeService
}

func NewGlobalPackagesHandler(cfg *config.Config) *GlobalPackagesHandler {
	return &GlobalPackagesHandler{
		globalPackageService: services.NewGlobalPackageService(),
		rechargeService:      services.NewRechargeService(cfg),
	}
}

// GetAll handles GET /admin/global-packages
func (h *GlobalPackagesHandler) GetAll(c *gin.Context) {
	page := 1
	limit := 20

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	packages, total, err := h.globalPackageService.GetAll(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve global packages",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"packages": packages,
			"total":    total,
			"page":     page,
			"limit":    limit,
		},
	})
}

// Create handles POST /admin/global-packages
func (h *GlobalPackagesHandler) Create(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		PriceCents int    `json:"price_cents" binding:"required"`
		RealCents  int    `json:"real_cents" binding:"required"`
		BonusCents int    `json:"bonus_cents"`
		SortOrder  int    `json:"sort_order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
			"data":    nil,
		})
		return
	}

	id, err := h.globalPackageService.Create(req.Name, req.PriceCents, req.RealCents, req.BonusCents, req.SortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create global package",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}

// Update handles PUT /admin/global-packages/:id
func (h *GlobalPackagesHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	updates := make(map[string]interface{})
	allowedFields := []string{"name", "price_cents", "real_cents", "bonus_cents", "sort_order"}
	for _, field := range allowedFields {
		if val, ok := req[field]; ok {
			updates[field] = val
		}
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "No fields to update",
			"data":    nil,
		})
		return
	}

	if err := h.globalPackageService.Update(id, updates); err != nil {
		if err.Error() == "PACKAGE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Package not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update package",
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

// UpdateStatus handles PUT /admin/global-packages/:id/status
func (h *GlobalPackagesHandler) UpdateStatus(c *gin.Context) {
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

	if err := h.globalPackageService.UpdateStatus(id, req.Status); err != nil {
		if err.Error() == "PACKAGE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Package not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update package status",
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

// GetOrders handles GET /admin/recharge-orders
func (h *GlobalPackagesHandler) GetOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}

	search := c.Query("search")
	userID := c.Query("user_id")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	var status *int
	if statusStr := c.Query("status"); statusStr != "" {
		s, _ := strconv.Atoi(statusStr)
		status = &s
	}

	orders, total, err := h.rechargeService.GetAllOrdersV2(page, limit, search, userID, status, dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": orders,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

// GetOrderByID handles GET /admin/recharge-orders/:id
func (h *GlobalPackagesHandler) GetOrderByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少订单ID",
		})
		return
	}

	order, err := h.rechargeService.GetOrderByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "订单不存在",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    order,
	})
}

// Delete handles DELETE /admin/global-packages/:id
func (h *GlobalPackagesHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.globalPackageService.Delete(id); err != nil {
		if err.Error() == "PACKAGE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Package not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete package",
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

// GetDeleted handles GET /admin/global-packages/deleted
func (h *GlobalPackagesHandler) GetDeleted(c *gin.Context) {
	packages, err := h.globalPackageService.GetDeleted()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve deleted packages",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    packages,
	})
}

// Recover handles POST /admin/global-packages/:id/recover
func (h *GlobalPackagesHandler) Recover(c *gin.Context) {
	id := c.Param("id")

	if err := h.globalPackageService.Recover(id); err != nil {
		if err.Error() == "PACKAGE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Package not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to recover package",
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

// PermanentDelete handles DELETE /admin/global-packages/:id/permanent
func (h *GlobalPackagesHandler) PermanentDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.globalPackageService.PermanentDelete(id); err != nil {
		if err.Error() == "PACKAGE_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Package not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to permanently delete package",
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
