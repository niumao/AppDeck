package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type DataMigrationHandler struct {
	service *services.DataMigrationService
}

func NewDataMigrationHandler() *DataMigrationHandler {
	return &DataMigrationHandler{
		service: services.NewDataMigrationService(),
	}
}

func (h *DataMigrationHandler) ExportData(c *gin.Context) {
	var req struct {
		Tables []string `json:"tables" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "tables is required and must be a non-empty array",
			"data":    nil,
		})
		return
	}

	if err := h.service.ValidateTableKeys(req.Tables); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	data, err := h.service.Export(req.Tables)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Export failed: " + err.Error(),
			"data":    nil,
		})
		return
	}

	filename := fmt.Sprintf("appdeck-export-%s.json", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, data)
}

func (h *DataMigrationHandler) ImportData(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "file is required",
			"data":    nil,
		})
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Failed to read file",
			"data":    nil,
		})
		return
	}

	var data services.ExportData
	if err := json.Unmarshal(body, &data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid JSON format: " + err.Error(),
			"data":    nil,
		})
		return
	}

	results, err := h.service.Import(&data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Import failed: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"results": results,
		},
	})
}
