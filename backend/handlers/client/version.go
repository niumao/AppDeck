package client

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type VersionHandler struct {
	versionService *services.VersionService
}

func NewVersionHandler() *VersionHandler {
	return &VersionHandler{
		versionService: services.NewVersionService(),
	}
}

// GetVersion 获取服务器数据版本
// GET /api/v1/version
func (h *VersionHandler) GetVersion(c *gin.Context) {
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
		"data":    version,
	})
}


