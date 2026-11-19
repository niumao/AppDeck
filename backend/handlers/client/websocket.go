package client

import (
	"appdeck/config"
	"appdeck/database"
	"appdeck/logger"
	"appdeck/models"
	"appdeck/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境需要限制
	},
}

type WebSocketHandler struct {
	deviceStateService      *services.DeviceStateService
	sessionService          *services.SessionService
	userClientService       *services.UserClientService
	registeredDeviceService *services.RegisteredDeviceService
	moonlightHostService    *services.MoonlightHostService
	authService             *services.AuthService
	appLogService           *services.AppLogService
}

func NewWebSocketHandler(cfg *config.Config) *WebSocketHandler {
	return &WebSocketHandler{
		deviceStateService:      services.GetDeviceStateService(),
		sessionService:          services.NewSessionService(),
		userClientService:       services.NewUserClientService(),
		registeredDeviceService: services.GetRegisteredDeviceService(),
		moonlightHostService:    services.GetMoonlightHostService(),
		authService:             services.NewAuthService(cfg),
		appLogService:           services.NewAppLogService(),
	}
}

// HandleDeviceConnection WebSocket 设备连接处理 (CLT_WS)
// GET /api/v1/ws/device?host_id={hostId}&machine_code={machineCode}
func (h *WebSocketHandler) HandleDeviceConnection(c *gin.Context) {
	hostID := c.Query("host_id")
	machineCode := c.Query("machine_code")

	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Missing host_id or machine_code",
		})
		return
	}

	// Security backstop: check if device is approved
	if !h.registeredDeviceService.IsDeviceAllowed(machineCode) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "DEVICE_NOT_REGISTERED",
		})
		return
	}

	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("ws", "WebSocket upgrade error: %v", err)
		return
	}
	conn.SetReadLimit(512 * 1024) // 512KB for base64 screenshot payloads
	defer conn.Close()

	// 获取客户端 IP 地址
	clientIP := c.ClientIP()

	// 注册设备（临时使用 hostID 作为设备名）
	h.deviceStateService.RegisterDevice(hostID, machineCode, hostID, clientIP, conn)

	// Check if this is a reconnect within grace period
	if h.deviceStateService.CancelGraceTimer(hostID, machineCode) {
		logger.Info("ws", "Device reconnected within grace period: hostID=%s, machineCode=%s", hostID, machineCode)
	}

	// Cleanup on disconnect
	defer func() {
		// If this connection was replaced by a newer one (RegisterDevice closed us),
		// skip all cleanup — the new goroutine owns the device now.
		if !h.deviceStateService.IsCurrentConn(hostID, machineCode, conn) {
			logger.Debug("ws", "Connection superseded, skipping cleanup: hostID=%s, machineCode=%s", hostID, machineCode)
			return
		}

		device, err := h.deviceStateService.GetDevice(hostID, machineCode)
		if err != nil {
			h.deviceStateService.RemoveDevice(hostID, machineCode)
			return
		}

		// Explicit disconnect (admin/H5 quit) — sessions already ended, just clean up
		if device.ExplicitDisconnect {
			logger.Info("ws", "Explicit disconnect, cleaning up device=%s:%s", hostID, machineCode)
			h.deviceStateService.RemoveDevice(hostID, machineCode)
			return
		}

		if device.BoundUserID != "" {
			// Unexpected disconnect — start 90s grace period
			logger.Info("ws", "WebSocket disconnected, starting 90s grace period for user=%s, device=%s:%s",
				device.BoundUserID, hostID, machineCode)

			h.deviceStateService.MarkDisconnected(hostID, machineCode, 90*time.Second, func() {
				// This runs after 90s if client doesn't reconnect
				logger.Info("ws", "Grace period expired, ending sessions for device=%s:%s", hostID, machineCode)
				err := h.sessionService.EndSessionsByDevice(hostID, machineCode, "ws_disconnect")
				if err != nil {
					logger.Error("ws", "Error auto-ending sessions after grace period: %v", err)
				}
				h.deviceStateService.RemoveDevice(hostID, machineCode)
			})
		} else {
			// No bound user, just remove device immediately
			h.deviceStateService.RemoveDevice(hostID, machineCode)
		}
	}()

	logger.Info("ws", "Device connected: hostID=%s, machineCode=%s", hostID, machineCode)

	// 启动心跳检测
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// 读取消息的 goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var msg map[string]interface{}
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Error("ws", "WebSocket read error: %v", err)
				}
				return
			}

			// 处理客户端消息
			h.handleClientMessage(hostID, machineCode, msg)
		}
	}()

	// 发送初始状态
	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	freePlay := h.registeredDeviceService.GetFreePlayByMachineCode(machineCode)
	qrURL := h.authService.GenerateQRURLForDevice(hostID, machineCode)

	if err == nil && device.BoundUserID != "" && device.ActiveSessionID != "" {
		// Reconnecting device with active session — send full reconnect state
		reconnData, reconErr := h.sessionService.GetSessionReconnectData(device.ActiveSessionID)
		if reconErr == nil {
			msg := gin.H{
				"event":                  "state_sync",
				"state":                  string(device.State),
				"device_name":            device.DeviceName,
				"session_id":             reconnData.SessionID,
				"remaining_seconds":      reconnData.RemainingSeconds,
				"balance_cents":          reconnData.BalanceCents,
				"balance_source":         reconnData.BalanceSource,
				"plan_rate_cents":        reconnData.PlanRateCents,
				"plan_unit":              reconnData.PlanUnit,
				"tenant_balance_minutes": reconnData.TenantBalanceMinutes,
				"global_balance_cents":   reconnData.GlobalBalanceCents,
				"user":                   reconnData.User,
				"free_play":              freePlay,
				"qr_url":                 qrURL,
				"other_session_count":    reconnData.OtherSessionCount,
				"server_ts":              time.Now().Unix(),
			}
			// Generate fresh token for reconnecting client
			var user models.User
			if database.DB.Where("id = ?", device.BoundUserID).First(&user).Error == nil {
				if token, err := h.authService.GenerateToken(&user); err == nil {
					msg["token"] = token
				}
			}
			// Convert plan_unit to plan_unit_seconds
			if reconnData.PlanUnit == "hour" {
				msg["plan_unit_seconds"] = 3600
			} else if reconnData.PlanUnit == "minute" {
				msg["plan_unit_seconds"] = 60
			} else {
				msg["plan_unit_seconds"] = 60
			}
			h.deviceStateService.SendMessage(hostID, machineCode, "device_state_change", msg)
		} else {
			// Session lookup failed — fall back to basic state
			logger.Warn("ws", "Failed to get reconnect data for session %s: %v", device.ActiveSessionID, reconErr)
			h.deviceStateService.SendMessage(hostID, machineCode, "device_state_change", gin.H{
				"event":       "state_sync",
				"state":       string(device.State),
				"device_name": device.DeviceName,
				"session_id":  device.ActiveSessionID,
				"user_id":     device.BoundUserID,
				"free_play":   freePlay,
				"qr_url":      qrURL,
			})
		}
	} else {
		// New connection or no active session - send idle state
		h.deviceStateService.SendMessage(hostID, machineCode, "device_state_change", gin.H{
			"event":       "connect",
			"state":       "idle",
			"device_name": hostID,
			"free_play":   freePlay,
			"qr_url":      qrURL,
		})
	}

	// Push screensaver config
	screensaverService := services.GetScreensaverService()
	ssConfig := screensaverService.GetClientScreensaverConfig(machineCode)
	h.deviceStateService.SendMessage(hostID, machineCode, "screensaver_config", ssConfig)

	// Push power schedule config
	if schedule := h.registeredDeviceService.GetScheduleByMachineCode(machineCode); schedule != nil {
		h.deviceStateService.SendMessage(hostID, machineCode, "power_schedule", gin.H{
			"shutdown_enabled": schedule.ShutdownEnabled,
			"shutdown_time":    schedule.ShutdownTime,
			"poweron_enabled":  schedule.PoweronEnabled,
			"poweron_time":     schedule.PoweronTime,
		})

		// Push bypass charge config
		h.deviceStateService.SendMessage(hostID, machineCode, "bypass_charge", gin.H{
			"enabled": schedule.BypassChargeEnabled,
		})
	}

	// 主循环：心跳检测
	for {
		select {
		case <-done:
			// 连接已关闭
			logger.Info("ws", "Device disconnected: hostID=%s, machineCode=%s", hostID, machineCode)
			return
		case <-pingTicker.C:
			// If this connection was replaced, stop pinging — the new goroutine handles it
			if !h.deviceStateService.IsCurrentConn(hostID, machineCode, conn) {
				logger.Debug("ws", "Connection superseded during ping, exiting: hostID=%s, machineCode=%s", hostID, machineCode)
				return
			}
			// Server sends ping to client (reversed from old model)
			if err := h.deviceStateService.SendMessage(hostID, machineCode, "ping", gin.H{}); err != nil {
				logger.Warn("ws", "Failed to send ping to device %s:%s: %v", hostID, machineCode, err)
				return
			}
			// Increment missed pongs (reset to 0 when pong received)
			missed := h.deviceStateService.IncrementMissedPongs(hostID, machineCode)
			if missed >= 3 {
				logger.Warn("ws", "Device missed 3 pongs, ending session: hostID=%s, machineCode=%s", hostID, machineCode)
				// Mark explicit disconnect first so defer block skips grace period
				h.deviceStateService.MarkExplicitDisconnect(hostID, machineCode)
				h.deviceStateService.SetActiveSession(hostID, machineCode, "")
				if err := h.sessionService.EndSessionsByDevice(hostID, machineCode, "connection_lost"); err != nil {
					logger.Error("ws", "Error ending sessions for unresponsive device: %v", err)
				}
				h.deviceStateService.UnbindUser(hostID, machineCode)
				return
			}
		}
	}
}

// handleClientMessage 处理客户端消息
func (h *WebSocketHandler) handleClientMessage(hostID, machineCode string, msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	data, _ := msg["data"].(map[string]interface{})

	switch msgType {
	case "pong":
		// Client responded to our ping
		h.deviceStateService.ResetMissedPongs(hostID, machineCode)
		if data != nil {
			if bl, ok := data["battery_level"].(float64); ok {
				chargingState := "unknown"
				if cs, ok := data["charging_state"].(string); ok {
					chargingState = cs
				}
				h.deviceStateService.UpdateBatteryInfo(hostID, machineCode, int(bl), chargingState)
			}
			if dt, ok := data["device_temp"].(float64); ok {
				h.deviceStateService.UpdateDeviceTemp(hostID, machineCode, dt)
			}
		}

	case "register":
		// 注册设备（客户端发送设备名称和版本）
		deviceName, _ := data["device_name"].(string)
		var innerIPs []string
		if raw, ok := data["inner_ips"].([]interface{}); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					innerIPs = append(innerIPs, s)
				}
			}
		}
		if deviceName != "" || len(innerIPs) > 0 {
			h.deviceStateService.UpdateDeviceRegistration(hostID, machineCode, deviceName, innerIPs)
		}
		// Note: initial state was already sent on connection (lines above).
		// Do NOT re-send device_state_change here to avoid duplicate messages
		// that cause the client to start the heartbeat loop twice.

	case "logout":
		// 用户登出
		// Get device info to find active session
		device, err := h.deviceStateService.GetDevice(hostID, machineCode)
		var sessionID string
		var durationSec int
		var totalCost int
		var balanceCents int

		if err == nil && device.ActiveSessionID != "" {
			// Get session details before ending
			session, err := h.sessionService.GetSessionByID(device.ActiveSessionID)
			if err == nil && session != nil {
				sessionID = session.ID
				// Get user balance before ending session
				balanceCents = session.User.BalanceCents
			}
		}

		// Close any open app logs before ending session
		if device.ActiveSessionID != "" {
			h.appLogService.CloseOpenAppLogs(device.ActiveSessionID, "logout")
		}

		// Clear ActiveSessionID first so the billing ticker skips this device
		h.deviceStateService.SetActiveSession(hostID, machineCode, "")

		// End active billing sessions for this device (this also closes app logs)
		if err := h.sessionService.EndSessionsByDevice(hostID, machineCode, "logout"); err != nil {
			logger.Error("ws", "Error ending sessions on logout: hostID=%s, machineCode=%s, err=%v", hostID, machineCode, err)
		}

		// Get updated session info after ending
		if sessionID != "" {
			session, err := h.sessionService.GetSessionByID(sessionID)
			if err == nil && session != nil {
				durationSec = session.DurationSec
				totalCost = session.TotalCost
				balanceCents = session.User.BalanceCents
			}
		}

		// Deactivate the user_clients DB record (set status=0)
		if err := h.userClientService.DeactivateClientByHostInfo(hostID, machineCode); err != nil {
			logger.Error("ws", "Error deactivating client on logout: hostID=%s, machineCode=%s, err=%v", hostID, machineCode, err)
		}

		// Get fresh QR URL and free_play for idle state
		freePlay := h.registeredDeviceService.GetFreePlayByMachineCode(machineCode)
		qrURL := h.authService.GenerateQRURLForDevice(hostID, machineCode)

		// Clear in-memory state
		h.deviceStateService.UnbindUser(hostID, machineCode)
		h.deviceStateService.UpdateDeviceState(hostID, machineCode, services.DeviceStateIdle)

		// Send single device_state_change with session_end event
		sessionSummary := gin.H{
			"reason": "logout",
		}
		if sessionID != "" {
			sessionSummary["duration_seconds"] = durationSec
			sessionSummary["total_cost_cents"] = totalCost
			sessionSummary["balance_cents"] = balanceCents
		}

		h.deviceStateService.SendMessage(hostID, machineCode, "device_state_change", gin.H{
			"event":           "session_end",
			"state":           "idle",
			"qr_url":          qrURL,
			"free_play":       freePlay,
			"session_summary": sessionSummary,
		})

	case "app_launch":
		appID, _ := data["app_id"].(string)
		appName, _ := data["app_name"].(string)
		if appID == "" || appName == "" {
			logger.Warn("ws", "app_launch missing app_id or app_name")
			return
		}
		device, err := h.deviceStateService.GetDevice(hostID, machineCode)
		if err != nil || device.ActiveSessionID == "" {
			logger.Warn("ws", "app_launch: no active session for device %s:%s", hostID, machineCode)
			return
		}
		if err := h.appLogService.CreateAppLog(device.ActiveSessionID, appID, appName); err != nil {
			logger.Error("ws", "app_launch: failed to create app log: %v", err)
		}
		h.deviceStateService.SetCurrentApp(hostID, machineCode, appName)

	case "app_quit":
		quitReason, _ := data["quit_reason"].(string)
		if quitReason == "" {
			quitReason = "user_quit"
		}
		device, err := h.deviceStateService.GetDevice(hostID, machineCode)
		if err != nil || device.ActiveSessionID == "" {
			logger.Warn("ws", "app_quit: no active session for device %s:%s", hostID, machineCode)
			return
		}
		if err := h.appLogService.EndAppLog(device.ActiveSessionID, quitReason); err != nil {
			logger.Error("ws", "app_quit: failed to end app log: %v", err)
		}
		h.deviceStateService.ClearCurrentApp(hostID, machineCode)

	case "pair_result":
		hostID, _ := data["host_id"].(string)
		success, _ := data["success"].(bool)
		errorMsg, _ := data["error"].(string)

		if hostID == "" {
			logger.Warn("ws", "pair_result missing host_id")
			return
		}

		if success {
			if err := h.moonlightHostService.MarkPaired(hostID, true); err != nil {
				logger.Error("ws", "failed to mark host %s as paired: %v", hostID, err)
			} else {
				logger.Info("ws", "host %s paired successfully", hostID)
			}
		} else {
			logger.Warn("ws", "pairing failed for host %s: %s", hostID, errorMsg)
		}

	case "shell_result":
		commandID, _ := data["command_id"].(string)
		stdout, _ := data["stdout"].(string)
		stderr, _ := data["stderr"].(string)
		exitCodeFloat, _ := data["exit_code"].(float64)
		exitCode := int(exitCodeFloat)
		shellService := services.GetShellService()
		shellService.DeliverResult(commandID, stdout, stderr, exitCode)

	case "screen_capture_result":
		commandID, _ := data["command_id"].(string)
		imageBase64, _ := data["image_base64"].(string)
		display, _ := data["display"].(string)
		widthFloat, _ := data["width"].(float64)
		heightFloat, _ := data["height"].(float64)
		errorMsg, _ := data["error"].(string)
		svc := services.GetScreenCaptureService()
		svc.DeliverResult(commandID, &services.CaptureResult{
			ImageBase64: imageBase64,
			Display:     display,
			Width:       int(widthFloat),
			Height:      int(heightFloat),
			Error:       errorMsg,
		})
	}
}
