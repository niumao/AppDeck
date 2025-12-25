package admin

import (
	"net/http"

	"appdeck/config"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type PackageHandler struct {
	packageService *services.PackageService
}

func NewPackageHandler(cfg *config.Config) *PackageHandler {
	return &PackageHandler{
		packageService: services.NewPackageService(cfg),
	}
}

// GetStatus 获取客户端包状态
// GET /api/v1/admin/package-status
func (h *PackageHandler) GetStatus(c *gin.Context) {
	pkg, err := h.packageService.GetPackageInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取包状态失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pkg,
	})
}

// Generate 生成客户端包
// POST /api/v1/admin/generate-client-package
func (h *PackageHandler) Generate(c *gin.Context) {
	// 异步生成（可选）
	go func() {
		if err := h.packageService.GenerateClientPackage(); err != nil {
			// 记录错误日志
			// log.Printf("生成客户端包失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "客户端包生成任务已启动",
	})
}


