package client

import (
	"log"
	"net/http"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type DeviceRegisterHandler struct {
	deviceService      *services.RegisteredDeviceService
	deviceStateService *services.DeviceStateService
	sessionService     *services.SessionService
	authService        *services.AuthService
}

func NewDeviceRegisterHandler(cfg *config.Config) *DeviceRegisterHandler {
	return &DeviceRegisterHandler{
		deviceService:      services.GetRegisteredDeviceService(),
		deviceStateService: services.GetDeviceStateService(),
		sessionService:     services.NewSessionService(),
		authService:        services.NewAuthService(cfg),
	}
}

// CheckRegistration handles GET /device/register (CLT_DEV_REGISTER)
func (h *DeviceRegisterHandler) CheckRegistration(c *gin.Context) {
	hostID := c.Query("host_id")
	machineCode := c.Query("machine_code")

	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Missing host_id or machine_code",
			"data":    nil,
		})
		return
	}

	// Get client IP
	ip := c.ClientIP()

	// Check or register device
	status, err := h.deviceService.CheckOrRegister(machineCode, hostID, ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to check device registration",
			"data":    nil,
		})
		return
	}

	// Check free play status from tenant
	freePlay := false
	if status == "approved" {
		freePlay = h.deviceService.GetFreePlayByMachineCode(machineCode)
	}

	// Check if device has active session (for page reload recovery)
	var activeUser gin.H
	if status == "approved" {
		device, devErr := h.deviceStateService.GetDevice(hostID, machineCode)
		if devErr == nil && device.BoundUserID != "" && device.ActiveSessionID != "" {
			// Look up user info
			var user models.User
			if database.DB.Where("id = ?", device.BoundUserID).First(&user).Error == nil {
				reconnData, reconErr := h.sessionService.GetSessionReconnectData(device.ActiveSessionID)
				if reconErr == nil {
					token, tokenErr := h.authService.GenerateToken(&user)
					if tokenErr != nil {
						log.Printf("Error generating token for active user: %v", tokenErr)
					}
					activeUser = gin.H{
						"user": gin.H{
							"id":         user.ID,
							"nickname":   user.Nickname,
							"avatar_url": user.AvatarURL,
						},
						"token":              token,
						"session_id":        reconnData.SessionID,
						"remaining_seconds": reconnData.RemainingSeconds,
						"balance_cents":     reconnData.BalanceCents,
					}
				}
			}
		}
	}

	responseData := gin.H{
		"status":      status,
		"free_play":   freePlay,
		"device_name": h.deviceService.GetDeviceName(machineCode),
	}
	if activeUser != nil {
		responseData["active_user"] = activeUser
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    responseData,
	})
}
