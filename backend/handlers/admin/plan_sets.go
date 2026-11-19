package admin

import (
	"appdeck/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PlanSetsHandler struct {
	service *services.PlanSetService
}

func NewPlanSetsHandler() *PlanSetsHandler {
	return &PlanSetsHandler{service: services.NewPlanSetService()}
}

// GetAll handles GET /admin/plan-sets
func (h *PlanSetsHandler) GetAll(c *gin.Context) {
	page := 1
	limit := 20
	search := c.Query("search")

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	planSets, total, err := h.service.GetAll(page, limit, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve plan sets",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"plan_sets": planSets,
			"total":     total,
			"page":      page,
			"limit":     limit,
		},
	})
}

// GetByID handles GET /admin/plan-sets/:id
func (h *PlanSetsHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	planSet, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Plan set not found",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    planSet,
	})
}

// Create handles POST /admin/plan-sets
func (h *PlanSetsHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsDefault   *int   `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	isDefault := 0
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	id, err := h.service.Create(req.Name, req.Description, isDefault)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create plan set",
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

// Update handles PUT /admin/plan-sets/:id
func (h *PlanSetsHandler) Update(c *gin.Context) {
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
	allowedFields := []string{"name", "description", "is_default", "standard_plan_id", "global_rate_plan_id"}
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

	if err := h.service.Update(id, updates); err != nil {
		if err.Error() == "PLAN_SET_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Plan set not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
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

// UpdateStatus handles PUT /admin/plan-sets/:id/status
func (h *PlanSetsHandler) UpdateStatus(c *gin.Context) {
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

	if err := h.service.UpdateStatus(id, req.Status); err != nil {
		if err.Error() == "PLAN_SET_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Plan set not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
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

// GetActivePlanSets handles GET /admin/plan-sets/active
func (h *PlanSetsHandler) GetActivePlanSets(c *gin.Context) {
	planSets, err := h.service.GetActivePlanSets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve active plan sets",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    planSets,
	})
}

// AddPlan handles POST /admin/plan-sets/:id/plans
func (h *PlanSetsHandler) AddPlan(c *gin.Context) {
	planSetID := c.Param("id")

	var req struct {
		Name         string `json:"name" binding:"required"`
		RateCents    *int   `json:"rate_cents"`
		Unit         string `json:"unit"`
		Type         string `json:"type" binding:"required"`
		PriceCents   *int   `json:"price_cents"`
		GivesMinutes *int   `json:"gives_minutes"`
		Description  string `json:"description"`
		Status       *int   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Set defaults
	rateCents := 0
	if req.RateCents != nil {
		rateCents = *req.RateCents
	}
	unit := "hour"
	if req.Unit != "" {
		unit = req.Unit
	}
	priceCents := 0
	if req.PriceCents != nil {
		priceCents = *req.PriceCents
	}
	givesMinutes := 0
	if req.GivesMinutes != nil {
		givesMinutes = *req.GivesMinutes
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	planID, err := h.service.AddPlanToSet(
		planSetID,
		req.Name,
		rateCents,
		unit,
		req.Type,
		priceCents,
		givesMinutes,
		req.Description,
		status,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to add plan: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": planID},
	})
}

// UpdatePlan handles PUT /admin/plan-sets/plans/:id
func (h *PlanSetsHandler) UpdatePlan(c *gin.Context) {
	planID := c.Param("id")

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
	allowedFields := []string{"name", "unit", "type", "price_cents", "gives_minutes", "description", "status"}
	fieldMapping := map[string]string{
		"rate_cents": "rate",
	}

	// Handle rate_cents mapping
	if val, ok := req["rate_cents"]; ok {
		updates["rate"] = val
	}

	for _, field := range allowedFields {
		if val, ok := req[field]; ok {
			dbField := field
			if mapped, exists := fieldMapping[field]; exists {
				dbField = mapped
			}
			updates[dbField] = val
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

	billingPlanService := services.NewBillingPlanService()
	if err := billingPlanService.Update(planID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update plan",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": planID},
	})
}

// UpdatePlanStatus handles PUT /admin/plan-sets/plans/:id/status
func (h *PlanSetsHandler) UpdatePlanStatus(c *gin.Context) {
	planID := c.Param("id")

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

	billingPlanService := services.NewBillingPlanService()
	updates := map[string]interface{}{"status": req.Status}
	if err := billingPlanService.Update(planID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update plan status",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": planID},
	})
}

// DeletePlan handles DELETE /admin/plan-sets/plans/:id
func (h *PlanSetsHandler) DeletePlan(c *gin.Context) {
	planID := c.Param("id")

	if err := h.service.DeletePlan(planID); err != nil {
		if err.Error() == "PLAN_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Plan not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
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

// Delete handles DELETE /admin/plan-sets/:id
func (h *PlanSetsHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(id); err != nil {
		if err.Error() == "PLAN_SET_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Plan set not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
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

// GetDeleted handles GET /admin/plan-sets/deleted
func (h *PlanSetsHandler) GetDeleted(c *gin.Context) {
	planSets, err := h.service.GetDeleted()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve deleted plan sets",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    planSets,
	})
}

// Recover handles POST /admin/plan-sets/:id/recover
func (h *PlanSetsHandler) Recover(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Recover(id); err != nil {
		if err.Error() == "PLAN_SET_NOT_FOUND" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Plan set not found",
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to recover plan set",
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
