package client

import (
	"fmt"
	"io"
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type SaveDataHandler struct {
	service *services.SaveDataService
}

func NewSaveDataHandler() *SaveDataHandler {
	return &SaveDataHandler{
		service: services.GetSaveDataService(),
	}
}

// GetSyncEnabled tells the client whether game-save sync is currently
// turned on, so it can skip local restore/cleanup work (not just uploads)
// when an admin has disabled the feature.
func (h *SaveDataHandler) GetSyncEnabled(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"enabled": h.service.IsEnabled()}})
}

// UploadSave receives a zipped bundle of a user's whole Eden save profile
// (every title they have progress in) and stores it against that user.
// Device-authed rather than user-JWT-authed (see routes.go) -- userId
// travels as a path param rather than coming from a JWT.
func (h *SaveDataHandler) UploadSave(c *gin.Context) {
	if !h.service.IsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "Save sync is disabled", "data": nil})
		return
	}

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "userId is required", "data": nil})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "File is required", "data": nil})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to read upload", "data": nil})
		return
	}

	record, err := h.service.UploadUserSave(userID, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Upload failed: " + err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": record})
}

// DownloadSave returns a user's stored save bundle (see UploadSave for why
// this is device-authed, not user-JWT-authed). Returns 404 if nothing has
// been uploaded yet.
func (h *SaveDataHandler) DownloadSave(c *gin.Context) {
	if !h.service.IsEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "Save sync is disabled", "data": nil})
		return
	}

	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "userId is required", "data": nil})
		return
	}

	rc, record, err := h.service.DownloadUserSave(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Download failed: " + err.Error(), "data": nil})
		return
	}
	if rc == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "No save data found", "data": nil})
		return
	}
	defer rc.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", record.UserID))
	c.Header("X-Save-Checksum", record.Checksum)
	c.DataFromReader(http.StatusOK, record.SizeBytes, "application/zip", rc, nil)
}
