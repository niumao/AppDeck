package client

import (
	"net/http"
	"os"

	"appdeck/config"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type DownloadHandler struct {
	packageService *services.PackageService
	cfg            *config.Config
}

func NewDownloadHandler(cfg *config.Config) *DownloadHandler {
	return &DownloadHandler{
		packageService: services.NewPackageService(cfg),
		cfg:            cfg,
	}
}

// GetPackageInfo 获取客户端包信息
// GET /api/v1/client/package-info
func (h *DownloadHandler) GetPackageInfo(c *gin.Context) {
	pkg, err := h.packageService.GetPackageInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取包信息失败",
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

// DownloadDatabase 下载 SQLite 数据库
// GET /api/v1/client/database
func (h *DownloadHandler) DownloadDatabase(c *gin.Context) {
	dbPath := h.cfg.Data.ClientDBPath

	// 检查文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "数据库文件不存在，请先生成客户端包",
		})
		return
	}

	// 发送文件
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=client.db")
	c.File(dbPath)
}

// DownloadResources 下载资源包
// GET /api/v1/client/resources
func (h *DownloadHandler) DownloadResources(c *gin.Context) {
	resourcesPath := h.cfg.Data.ResourcesZip

	// 检查文件是否存在
	if _, err := os.Stat(resourcesPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "资源包文件不存在，请先生成客户端包",
		})
		return
	}

	// 发送文件
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=resources.zip")
	c.File(resourcesPath)
}

// ServeIcon 提供图标文件
// GET /api/v1/icons/:filename
func (h *DownloadHandler) ServeIcon(c *gin.Context) {
	filename := c.Param("filename")
	filePath := h.cfg.Data.IconsDir + "/" + filename

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "图标文件不存在",
		})
		return
	}

	c.File(filePath)
}

// ServeBigPic 提供大图文件
// GET /api/v1/bigpics/:filename
func (h *DownloadHandler) ServeBigPic(c *gin.Context) {
	filename := c.Param("filename")
	filePath := h.cfg.Data.BigPicsDir + "/" + filename

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "大图文件不存在",
		})
		return
	}

	c.File(filePath)
}


