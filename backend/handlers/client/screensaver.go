package client

import (
	"net/http"

	"appdeck/config"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type ScreensaverHandler struct {
	cfg                *config.Config
	screensaverService *services.ScreensaverService
}

func NewScreensaverHandler(cfg *config.Config) *ScreensaverHandler {
	return &ScreensaverHandler{
		cfg:                cfg,
		screensaverService: services.GetScreensaverService(),
	}
}

// GetSettings handles GET /screensaver/settings (CLT_SS_SETTINGS)
func (h *ScreensaverHandler) GetSettings(c *gin.Context) {
	machineCode := c.Query("machine_code")
	if machineCode != "" {
		config := h.screensaverService.GetClientScreensaverConfig(machineCode)
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    config,
		})
		return
	}

	// Get current settings (returns defaults if not set)
	enabled := h.screensaverService.GetEnabled()
	timeout := h.screensaverService.GetTimeout()

	// Ensure defaults are persisted in DB if they don't exist
	// This prevents "record not found" errors on first access
	h.screensaverService.SetEnabled(enabled)
	h.screensaverService.SetTimeout(timeout)

	videoHash, err := h.screensaverService.GetVideoHash()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve settings",
			"data":    nil,
		})
		return
	}

	uploadDir := h.cfg.Data.IconsDir
	hasVideo := h.screensaverService.HasVideo(uploadDir)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled":    enabled,
			"timeout":    timeout,
			"video_hash": videoHash,
			"has_video":  hasVideo,
		},
	})
}

// GetVideo handles GET /screensaver/video (CLT_SS_VIDEO)
func (h *ScreensaverHandler) GetVideo(c *gin.Context) {
	uploadDir := h.cfg.Data.IconsDir

	// Check if video exists
	if !h.screensaverService.HasVideo(uploadDir) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No screensaver video available",
			"data":    nil,
		})
		return
	}

	videoPath, contentType := h.screensaverService.GetVideoPath(uploadDir)

	// Enable Range request support for video streaming
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "inline")
	c.File(videoPath)
}

// GetVideoByID handles GET /screensaver/videos/:id
func (h *ScreensaverHandler) GetVideoByID(c *gin.Context) {
	id := c.Param("id")

	video, err := h.screensaverService.GetVideoByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve video",
			"data":    nil,
		})
		return
	}
	if video == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Video not found",
			"data":    nil,
		})
		return
	}

	filePath := h.screensaverService.GetVideoFilePath(video, h.cfg.Data.IconsDir)

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", video.ContentType)
	c.Header("Content-Disposition", "inline")
	c.File(filePath)
}
