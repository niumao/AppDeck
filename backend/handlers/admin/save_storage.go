package admin

import (
	"net/http"
	"strings"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type SaveStorageHandler struct {
	service *services.SaveDataService
}

func NewSaveStorageHandler() *SaveStorageHandler {
	return &SaveStorageHandler{
		service: services.GetSaveDataService(),
	}
}

func (h *SaveStorageHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetStorageConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get save storage config: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cfg,
	})
}

// ListSaves returns every uploaded save-data record for the admin table.
func (h *SaveStorageHandler) ListSaves(c *gin.Context) {
	saves, err := h.service.ListSaves()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list saves: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    saves,
	})
}

// CreateBackup bundles every user's save data for every game into one
// backup archive -- not scoped to a specific user or game.
func (h *SaveStorageHandler) CreateBackup(c *gin.Context) {
	info, err := h.service.CreateFullBackup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Backup failed: " + err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": info})
}

// ListBackups lists save-data backup archives (all users, all games each).
func (h *SaveStorageHandler) ListBackups(c *gin.Context) {
	backups, err := h.service.ListFullBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to list backups: " + err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": backups})
}

// UploadBackup lets an admin upload a previously-downloaded backup archive
// so it can be restored later. Does not apply it immediately.
func (h *SaveStorageHandler) UploadBackup(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "File is required", "data": nil})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid file type: must be a .zip file", "data": nil})
		return
	}

	info, err := h.service.UploadFullBackup(file, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Upload failed: " + err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": info})
}

// RestoreBackup applies a backup archive: every user's save for every game
// it contains is written back, overwriting current save data.
func (h *SaveStorageHandler) RestoreBackup(c *gin.Context) {
	var req struct {
		Filename string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request: " + err.Error(), "data": nil})
		return
	}

	if err := h.service.RestoreFullBackup(req.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Restore failed: " + err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": nil})
}

// DeleteBackup removes a backup archive.
func (h *SaveStorageHandler) DeleteBackup(c *gin.Context) {
	filename := c.Param("filename")
	if err := h.service.DeleteFullBackup(filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Delete failed: " + err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": nil})
}

// DownloadBackup streams a backup archive to the admin.
func (h *SaveStorageHandler) DownloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	absPath, err := h.service.DownloadFullBackupPath(filename)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error(), "data": nil})
		return
	}
	c.FileAttachment(absPath, filename)
}

func (h *SaveStorageHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Enabled     bool   `json:"enabled"`
		StorageType string `json:"storageType" binding:"required"`
		LocalPath   string `json:"localPath"`
		S3Bucket    string `json:"s3Bucket"`
		S3Region    string `json:"s3Region"`
		S3Prefix    string `json:"s3Prefix"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request: " + err.Error(),
			"data":    nil,
		})
		return
	}

	cfg, err := h.service.UpdateStorageConfig(req.Enabled, req.StorageType, req.LocalPath, req.S3Bucket, req.S3Region, req.S3Prefix)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Failed to update save storage config: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    cfg,
	})
}
