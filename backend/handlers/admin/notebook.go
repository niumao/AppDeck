package admin

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type NotebookHandler struct {
	notebookService *services.NotebookService
}

func NewNotebookHandler() *NotebookHandler {
	return &NotebookHandler{
		notebookService: services.NewNotebookService(),
	}
}

// GetNote 获取当前管理员的记事本内容
// GET /api/v1/admin/notebook
func (h *NotebookHandler) GetNote(c *gin.Context) {
	adminID := c.GetString("admin_id")

	content, err := h.notebookService.GetNote(adminID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取记事本失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"content": content},
	})
}

// SaveNote 保存当前管理员的记事本内容
// PUT /api/v1/admin/notebook
func (h *NotebookHandler) SaveNote(c *gin.Context) {
	adminID := c.GetString("admin_id")

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	note, err := h.notebookService.SaveNote(adminID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存记事本失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"content": note.Content, "updated_at": note.UpdatedAt},
	})
}
