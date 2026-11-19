package client

import (
	"net/http"
	"os"

	"appdeck/config"

	"github.com/gin-gonic/gin"
)

type DownloadHandler struct {
	cfg *config.Config
}

func NewDownloadHandler(cfg *config.Config) *DownloadHandler {
	return &DownloadHandler{
		cfg: cfg,
	}
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
