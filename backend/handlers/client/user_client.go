package client

import (
	"appdeck/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserClientHandler struct {
	service *services.UserClientService
}

func NewUserClientHandler() *UserClientHandler {
	return &UserClientHandler{
		service: services.NewUserClientService(),
	}
}

func (h *UserClientHandler) GetActiveClients(c *gin.Context) {
	userID, _ := c.Get("userID")

	clients, err := h.service.GetActiveClients(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取客户端列表失败",
			"data":    nil,
		})
		return
	}

	registeredDeviceService := services.GetRegisteredDeviceService()
	items := make([]gin.H, 0, len(clients))
	for _, client := range clients {
		name := client.Name
		if adminName := registeredDeviceService.GetDeviceName(client.MachineCode); adminName != "" {
			name = adminName
		}
		items = append(items, gin.H{
			"id":   client.ID,
			"name": name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

func (h *UserClientHandler) QuitClient(c *gin.Context) {
	userID, _ := c.Get("userID")
	clientID := c.Param("id")

	// Get client info before quitting to extract host_id and machine_code
	client, err := h.service.GetClientByID(clientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "客户端不存在",
			"data":    nil,
		})
		return
	}

	// Verify ownership
	if client.UserID != userID.(string) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无权限操作此客户端",
			"data":    nil,
		})
		return
	}

	err = h.service.QuitClient(clientID, userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	// Explicit quit from H5: end sessions immediately, then close WS
	if client.HostID != "" && client.MachineCode != "" {
		deviceStateService := services.GetDeviceStateService()
		sessionService := services.NewSessionService()

		// Get device and session info before ending
		device, devErr := deviceStateService.GetDevice(client.HostID, client.MachineCode)
		var sessionID string
		var durationSec int
		var totalCost int
		var balanceCents int

		if devErr == nil && device.ActiveSessionID != "" {
			// Get session details before ending
			session, err := sessionService.GetSessionByID(device.ActiveSessionID)
			if err == nil && session != nil {
				sessionID = session.ID
				balanceCents = session.User.BalanceCents
			}
		}

		// Clear ActiveSessionID first so the billing ticker skips this device
		deviceStateService.SetActiveSession(client.HostID, client.MachineCode, "")

		// End sessions immediately (no grace period) - do this FIRST so balance deduction happens
		sessionService.EndSessionsByDevice(client.HostID, client.MachineCode, "h5_quit")

		// Get updated session info after ending
		if sessionID != "" {
			session, err := sessionService.GetSessionByID(sessionID)
			if err == nil && session != nil {
				durationSec = session.DurationSec
				totalCost = session.TotalCost
				balanceCents = session.User.BalanceCents
			}
		}

		// Send session_end WS message with updated balance
		if sessionID != "" {
			deviceStateService.SendMessage(client.HostID, client.MachineCode, "session_end", gin.H{
				"session_id":        sessionID,
				"reason":            "h5_quit",
				"duration_seconds":  durationSec,
				"total_cost_cents":  totalCost,
				"balance_cents":     balanceCents,
			})
		}

		// Send client_quit WS message
		deviceStateService.SendMessage(client.HostID, client.MachineCode, "client_quit", gin.H{
			"reason": "h5_user",
		})

		// Clear in-memory state: unbind user and set device back to idle
		deviceStateService.UnbindUser(client.HostID, client.MachineCode)
		deviceStateService.UpdateDeviceState(client.HostID, client.MachineCode, services.DeviceStateIdle)
		deviceStateService.SendMessage(client.HostID, client.MachineCode, "device_state_change", gin.H{
			"state": "idle",
		})

		// Mark explicit so WS defer block skips grace period
		deviceStateService.MarkExplicitDisconnect(client.HostID, client.MachineCode)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"success": true},
	})
}

func (h *UserClientHandler) GetCurrentClient(c *gin.Context) {
	userID, _ := c.Get("userID")

	clients, err := h.service.GetActiveClients(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取客户端信息失败",
			"data":    nil,
		})
		return
	}

	if len(clients) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "未找到客户端",
			"data":    nil,
		})
		return
	}

	// Return the most recent active client
	client := clients[0]
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":         client.ID,
			"name":       client.Name,
			"machine_id": client.MachineID,
			"plan_id":    client.PlanID,
			"steam_id":   client.SteamID,
		},
	})
}
