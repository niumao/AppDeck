package admin

import (
	"appdeck/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PlansHandler struct {
	service *services.BillingPlanService
}

func NewPlansHandler() *PlansHandler {
	return &PlansHandler{
		service: services.NewBillingPlanService(),
	}
}

func (h *PlansHandler) GetAll(c *gin.Context) {
	plans, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取计费方案列表失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    plans,
	})
}

func (h *PlansHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	plan, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "计费方案不存在",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    plan,
	})
}

func (h *PlansHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		RateCents   int    `json:"rate_cents" binding:"required"`
		Unit        string `json:"unit" binding:"required"`
		Description string `json:"description"`
		IsDefault   *int   `json:"is_default"`
		Status      *int   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
			"data":    nil,
		})
		return
	}

	isDefault := 0
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}
	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	id, err := h.service.Create(req.Name, req.RateCents, req.Unit, req.Description, isDefault, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建计费方案失败",
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

func (h *PlansHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
			"data":    nil,
		})
		return
	}

	updates := make(map[string]interface{})
	allowedFields := []string{"name", "rate_cents", "unit", "description", "is_default", "status"}
	fieldMapping := map[string]string{
		"rate_cents": "rate",
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
			"message": "没有可更新的字段",
			"data":    nil,
		})
		return
	}

	err := h.service.Update(id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新计费方案失败",
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

func (h *PlansHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	err := h.service.Delete(id)
	if err != nil {
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
