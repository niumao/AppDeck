package admin

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

// broadcastScreensaverConfig pushes the current screensaver config to every connected
// device. The client only otherwise receives "screensaver_config" once, at WS connect
// time, so without this an already-connected device never learns about admin changes.
func (h *ScreensaverHandler) broadcastScreensaverConfig() {
	deviceStateService := services.GetDeviceStateService()
	for _, device := range deviceStateService.GetAllDevices() {
		if device.DisconnectedAt != nil {
			continue
		}
		cfg := h.screensaverService.GetClientScreensaverConfig(device.MachineCode)
		deviceStateService.SendMessage(device.HostID, device.MachineCode, "screensaver_config", cfg)
	}
}

// UploadVideo handles POST /admin/screensaver/video (ADM_SS_UPLOAD)
func (h *ScreensaverHandler) UploadVideo(c *gin.Context) {
	// Get uploaded file
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "No video file provided",
			"data":    nil,
		})
		return
	}

	// Validate file size (max 200MB)
	maxSize := int64(200 * 1024 * 1024) // 200MB in bytes
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "VIDEO_TOO_LARGE",
			"data":    nil,
		})
		return
	}

	// Validate content type (mp4 or mkv)
	contentType := file.Header.Get("Content-Type")
	if contentType != "video/mp4" && contentType != "video/x-matroska" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "INVALID_VIDEO_FORMAT",
			"data":    nil,
		})
		return
	}

	// Open file
	fileData, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to open uploaded file",
			"data":    nil,
		})
		return
	}
	defer fileData.Close()

	// Save video and get hash
	uploadDir := h.cfg.Data.IconsDir // Reuse existing upload directory structure
	videoHash, err := h.screensaverService.SaveVideo(uploadDir, fileData, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save video file",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"video_hash": videoHash,
		},
	})
}

// GetVideo handles GET /admin/screensaver/video (ADM_SS_GET_VIDEO)
func (h *ScreensaverHandler) GetVideo(c *gin.Context) {
	uploadDir := h.cfg.Data.IconsDir

	// Check if video exists
	if !h.screensaverService.HasVideo(uploadDir) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "No screensaver video uploaded",
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

// GetSettings handles GET /admin/screensaver/settings (ADM_SS_GET_SETTINGS)
func (h *ScreensaverHandler) GetSettings(c *gin.Context) {
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

// SetSettings handles PUT /admin/screensaver/settings (ADM_SS_SET_SETTINGS)
func (h *ScreensaverHandler) SetSettings(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
		Timeout *int  `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	// Validate that at least one field is provided
	if req.Enabled == nil && req.Timeout == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "At least one field (enabled or timeout) must be provided",
			"data":    nil,
		})
		return
	}

	// Update enabled if provided
	if req.Enabled != nil {
		if err := h.screensaverService.SetEnabled(*req.Enabled); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to update enabled setting",
				"data":    nil,
			})
			return
		}
	}

	// Update timeout if provided
	if req.Timeout != nil {
		if *req.Timeout < 1 || *req.Timeout > 60 {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Timeout must be between 1 and 60 minutes",
				"data":    nil,
			})
			return
		}
		if err := h.screensaverService.SetTimeout(*req.Timeout); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Failed to update timeout setting",
				"data":    nil,
			})
			return
		}
	}

	// Return current values
	enabled := h.screensaverService.GetEnabled()
	timeout := h.screensaverService.GetTimeout()

	h.broadcastScreensaverConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled": enabled,
			"timeout": timeout,
		},
	})
}

// UploadVideoToLibrary handles POST /admin/screensaver/videos
func (h *ScreensaverHandler) UploadVideoToLibrary(c *gin.Context) {
	// Get uploaded file
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "No video file provided",
			"data":    nil,
		})
		return
	}

	// Validate file size (max 200MB)
	maxSize := int64(200 * 1024 * 1024)
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "VIDEO_TOO_LARGE",
			"data":    nil,
		})
		return
	}

	// Validate content type
	contentType := file.Header.Get("Content-Type")
	if contentType != "video/mp4" && contentType != "video/x-matroska" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "INVALID_VIDEO_FORMAT",
			"data":    nil,
		})
		return
	}

	// Open file
	fileData, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to open uploaded file",
			"data":    nil,
		})
		return
	}
	defer fileData.Close()

	// Save to library
	video, err := h.screensaverService.SaveVideoToLibrary(h.cfg.Data.IconsDir, fileData, file.Filename, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to save video file",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    video,
	})
}

// ListVideos handles GET /admin/screensaver/videos
func (h *ScreensaverHandler) ListVideos(c *gin.Context) {
	videos, err := h.screensaverService.ListVideos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list videos",
			"data":    nil,
		})
		return
	}

	defaultID, _ := h.screensaverService.GetDefaultVideoID()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"videos":           videos,
			"default_video_id": defaultID,
		},
	})
}

// DeleteVideoFromLibrary handles DELETE /admin/screensaver/videos/:id
func (h *ScreensaverHandler) DeleteVideoFromLibrary(c *gin.Context) {
	id := c.Param("id")

	err := h.screensaverService.DeleteVideo(id, h.cfg.Data.IconsDir)
	if err != nil {
		if err.Error() == "cannot delete: video is set as global default" || err.Error()[:len("cannot delete:")] == "cannot delete:" {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
				"data":    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete video",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}

// ServeVideoFile handles GET /admin/screensaver/videos/:id/file
func (h *ScreensaverHandler) ServeVideoFile(c *gin.Context) {
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

// GetDefaultVideo handles GET /admin/screensaver/default-video
func (h *ScreensaverHandler) GetDefaultVideo(c *gin.Context) {
	defaultID, err := h.screensaverService.GetDefaultVideoID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve default video",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"video_id": defaultID,
		},
	})
}

// SetDefaultVideo handles PUT /admin/screensaver/default-video
func (h *ScreensaverHandler) SetDefaultVideo(c *gin.Context) {
	var req struct {
		VideoID string `json:"video_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	if err := h.screensaverService.SetDefaultVideoID(req.VideoID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	h.broadcastScreensaverConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}

// ListTenantScreensavers handles GET /admin/screensaver/tenant-configs
func (h *ScreensaverHandler) ListTenantScreensavers(c *gin.Context) {
	configs, err := h.screensaverService.ListTenantScreensavers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list tenant configs",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

// SetTenantScreensaver handles PUT /admin/screensaver/tenant-configs/:tenant_id
func (h *ScreensaverHandler) SetTenantScreensaver(c *gin.Context) {
	tenantID := c.Param("tenant_id")

	var req struct {
		VideoID   string `json:"video_id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request",
			"data":    nil,
		})
		return
	}

	err := h.screensaverService.SetTenantScreensaver(tenantID, req.VideoID, req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	h.broadcastScreensaverConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}

// DeleteTenantScreensaver handles DELETE /admin/screensaver/tenant-configs/:tenant_id
func (h *ScreensaverHandler) DeleteTenantScreensaver(c *gin.Context) {
	tenantID := c.Param("tenant_id")

	err := h.screensaverService.DeleteTenantScreensaver(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete tenant config",
			"data":    nil,
		})
		return
	}

	h.broadcastScreensaverConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    nil,
	})
}
