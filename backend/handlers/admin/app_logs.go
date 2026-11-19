package admin

import (
	"appdeck/database"
	"appdeck/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AppLogsHandler struct{}

func NewAppLogsHandler() *AppLogsHandler {
	return &AppLogsHandler{}
}

// GetSessionLogs returns all app logs for a given session, ordered by launched_at.
// GET /api/v1/admin/sessions/:id/logs
func (h *AppLogsHandler) GetSessionLogs(c *gin.Context) {
	sessionID := c.Param("id")

	var logs []models.SessionAppLog
	err := database.DB.Where("session_id = ?", sessionID).
		Order("launched_at ASC").
		Find(&logs).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to query app logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    logs,
	})
}
