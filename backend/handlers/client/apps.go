package client

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type AppsHandler struct {
	appService      *services.AppService
	categoryService *services.CategoryService
	versionService  *services.VersionService
}

func NewAppsHandler() *AppsHandler {
	return &AppsHandler{
		appService:      services.NewAppService(),
		categoryService: services.NewCategoryService(),
		versionService:  services.NewVersionService(),
	}
}

// GetFullData 获取完整数据（首次安装使用）
// GET /api/v1/apps/full
func (h *AppsHandler) GetFullData(c *gin.Context) {
	// 获取应用列表
	apps, err := h.appService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取应用列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取分类列表
	categories, err := h.categoryService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取分类列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取版本信息
	version, err := h.versionService.GetCurrentVersion()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取版本失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"version":    version.Version,
			"apps":       apps,
			"categories": categories,
		},
	})
}


