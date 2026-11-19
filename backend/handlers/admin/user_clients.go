package admin

import (
	"appdeck/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserClientsHandler struct {
	service      *services.UserClientService
	steamService *services.SteamService
}

func NewUserClientsHandler() *UserClientsHandler {
	return &UserClientsHandler{
		service:      services.NewUserClientService(),
		steamService: services.NewSteamService(),
	}
}

func (h *UserClientsHandler) GetAll(c *gin.Context) {
	clients, err := h.service.GetAllClientsWithDetails()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取客户端列表失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    clients,
	})
}

func (h *UserClientsHandler) Create(c *gin.Context) {
	var req struct {
		UserID    string  `json:"user_id" binding:"required"`
		MachineID string  `json:"machine_id" binding:"required"`
		PlanID    *string `json:"plan_id"`
		SteamID   string  `json:"steam_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
			"data":    nil,
		})
		return
	}

	// Validate Steam ID if provided
	if req.SteamID != "" {
		if !h.steamService.ValidateSteamID(req.SteamID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Steam ID 格式无效",
				"data":    nil,
			})
			return
		}
	}

	id, err := h.service.CreateClientAssociation(req.UserID, req.MachineID, req.PlanID, req.SteamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建客户端关联失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}
