package admin

import (
	"net/http"

	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type CategoriesHandler struct {
	categoryService *services.CategoryService
}

func NewCategoriesHandler() *CategoriesHandler {
	return &CategoriesHandler{
		categoryService: services.NewCategoryService(),
	}
}

// GetAll 获取所有分类
// GET /api/v1/admin/categories
func (h *CategoriesHandler) GetAll(c *gin.Context) {
	categories, err := h.categoryService.GetAllWithAppCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    categories,
	})
}

// GetByID 获取单个分类
// GET /api/v1/admin/categories/:id
func (h *CategoriesHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	
	category, err := h.categoryService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "分类不存在",
			"error":   err.Error(),
		})
		return
	}

	// 获取应用数量
	var count int64
	// TODO: 实现获取应用数量的逻辑

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":        category.ID,
			"name":      category.Name,
			"order":     category.OrderNum,
			"appCount":  count,
			"createdAt": category.CreatedAt,
			"updatedAt": category.UpdatedAt,
		},
	})
}

// Create 创建分类
// POST /api/v1/admin/categories
func (h *CategoriesHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	category := &models.Category{
		Name: req.Name,
	}

	created, err := h.categoryService.Create(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    created,
	})
}

// Update 更新分类
// PUT /api/v1/admin/categories/:id
func (h *CategoriesHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name *string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}

	updated, err := h.categoryService.Update(id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新分类失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    updated,
	})
}

// Delete 删除分类
// DELETE /api/v1/admin/categories/:id
func (h *CategoriesHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.categoryService.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "删除失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// Reorder 批量调整分类顺序
// PUT /api/v1/admin/categories/reorder
func (h *CategoriesHandler) Reorder(c *gin.Context) {
	var req struct {
		Orders []struct {
			ID    string `json:"id" binding:"required"`
			Order int    `json:"order" binding:"required"`
		} `json:"orders" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	orders := make(map[string]int)
	for _, item := range req.Orders {
		orders[item.ID] = item.Order
	}

	if err := h.categoryService.UpdateOrder(orders); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新排序失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "排序更新成功",
	})
}


