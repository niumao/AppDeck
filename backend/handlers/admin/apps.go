package admin

import (
	"net/http"

	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type AppsHandler struct {
	appService *services.AppService
}

func NewAppsHandler() *AppsHandler {
	return &AppsHandler{
		appService: services.NewAppService(),
	}
}

// GetAll 获取所有应用
// GET /api/v1/admin/apps
func (h *AppsHandler) GetAll(c *gin.Context) {
	apps, err := h.appService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取应用列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// GetByID 获取单个应用
// GET /api/v1/admin/apps/:id
func (h *AppsHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	app, err := h.appService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "应用不存在",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    app,
	})
}

// Create 创建应用
// POST /api/v1/admin/apps
func (h *AppsHandler) Create(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		IconURL     string   `json:"iconUrl" binding:"required"`
		BigPicURL   string   `json:"bigPicUrl" binding:"required"`
		LaunchParam string   `json:"launchParam" binding:"required"`
		CategoryIDs []string `json:"categoryIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	app := &models.App{
		Name:        req.Name,
		Description: req.Description,
		IconURL:     req.IconURL,
		BigPicURL:   req.BigPicURL,
		LaunchParam: req.LaunchParam,
	}

	created, err := h.appService.Create(app, req.CategoryIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建应用失败",
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

// Update 更新应用
// PUT /api/v1/admin/apps/:id
func (h *AppsHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		IconURL     *string   `json:"iconUrl"`
		BigPicURL   *string   `json:"bigPicUrl"`
		LaunchParam *string   `json:"launchParam"`
		CategoryIDs *[]string `json:"categoryIds"`
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
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IconURL != nil {
		updates["icon_url"] = *req.IconURL
	}
	if req.BigPicURL != nil {
		updates["big_pic_url"] = *req.BigPicURL
	}
	if req.LaunchParam != nil {
		updates["launch_param"] = *req.LaunchParam
	}

	var categoryIDs []string
	if req.CategoryIDs != nil {
		categoryIDs = *req.CategoryIDs
	}

	updated, err := h.appService.Update(id, updates, categoryIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新应用失败",
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

// Delete 删除应用
// DELETE /api/v1/admin/apps/:id
func (h *AppsHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.appService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除应用失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// Reorder 批量调整应用顺序
// PUT /api/v1/admin/apps/reorder
func (h *AppsHandler) Reorder(c *gin.Context) {
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

	if err := h.appService.UpdateOrder(orders); err != nil {
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


