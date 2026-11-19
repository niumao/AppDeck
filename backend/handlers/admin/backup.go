package admin

import (
	"net/http"
	"strings"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	service *services.BackupService
}

func NewBackupHandler() *BackupHandler {
	return &BackupHandler{
		service: services.GetBackupService(),
	}
}

func (h *BackupHandler) ListBackups(c *gin.Context) {
	backups, err := h.service.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list backups: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    backups,
	})
}

func (h *BackupHandler) CreateBackup(c *gin.Context) {
	info, err := h.service.CreateBackup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Backup failed: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

func (h *BackupHandler) UploadBackup(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "File is required",
			"data":    nil,
		})
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(header.Filename, ".sql") {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid file type: must be a .sql file",
			"data":    nil,
		})
		return
	}

	info, err := h.service.UploadBackup(file, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Upload failed: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	var req struct {
		Filename string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "filename is required",
			"data":    nil,
		})
		return
	}

	if err := h.service.RestoreBackup(req.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Restore failed: " + err.Error(),
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

func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "filename is required",
			"data":    nil,
		})
		return
	}

	if err := h.service.DeleteBackup(filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Delete failed: " + err.Error(),
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

func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "filename is required",
			"data":    nil,
		})
		return
	}

	absPath, err := h.service.DownloadBackupPath(filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.File(absPath)
}

func (h *BackupHandler) GetSchedule(c *gin.Context) {
	schedule, err := h.service.GetSchedule()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get schedule: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    schedule,
	})
}

func (h *BackupHandler) UpdateSchedule(c *gin.Context) {
	var req struct {
		Enabled       bool `json:"enabled"`
		IntervalHours int  `json:"interval_hours"`
		MaxBackups    int  `json:"max_backups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request body",
			"data":    nil,
		})
		return
	}

	schedule, err := h.service.UpdateSchedule(req.Enabled, req.IntervalHours, req.MaxBackups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update schedule: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    schedule,
	})
}
