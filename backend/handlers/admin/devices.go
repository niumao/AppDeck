package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// DevicesHandler 设备管理处理器
type DevicesHandler struct {
	deviceStateService      *services.DeviceStateService
	userClientService       *services.UserClientService
	sessionService          *services.SessionService
	registeredDeviceService *services.RegisteredDeviceService
	shellService            *services.ShellService
	screenCaptureService    *services.ScreenCaptureService
}

// NewDevicesHandler 创建设备管理处理器
func NewDevicesHandler() *DevicesHandler {
	return &DevicesHandler{
		deviceStateService:      services.GetDeviceStateService(),
		userClientService:       services.NewUserClientService(),
		sessionService:          services.NewSessionService(),
		registeredDeviceService: services.GetRegisteredDeviceService(),
		shellService:            services.GetShellService(),
		screenCaptureService:    services.GetScreenCaptureService(),
	}
}

// GetAll 获取所有设备列表 (ADM_DEV_LIST)
func (h *DevicesHandler) GetAll(c *gin.Context) {
	// 获取查询参数
	search := c.DefaultQuery("search", "")
	stateFilter := c.DefaultQuery("state", "") // idle/logged_in/in_use/offline
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// 从 DeviceStateService 获取所有设备
	allDevices := h.deviceStateService.GetAllDevices()

	// 从 RegisteredDeviceService 获取设备名称映射 (machine_code -> device_name)
	deviceNameMap := h.registeredDeviceService.GetDeviceNameMap()

	// 构建响应列表
	var items []map[string]interface{}

	// 统计摘要
	summary := map[string]interface{}{
		"total":      0,
		"online":     0,
		"in_session": 0,
		"offline":    0,
	}

	for _, device := range allDevices {
		// 获取用户信息（如果设备绑定了用户）
		var userID *string
		var userNickname *string
		var userClientID *string

		if device.BoundUserID != "" {
			// 通过 HostID 和 MachineCode 查找 UserClient
			userClient, err := h.userClientService.GetClientByHostInfo(device.HostID, device.MachineCode)
			if err == nil && userClient != nil {
				userClientID = &userClient.ID
				userID = &userClient.UserID
				if userClient.User != nil {
					userNickname = &userClient.User.Nickname
				}
			}
		}

		// 获取活跃会话信息（如果有）
		var sessionID *string
		var sessionDuration *int
		var currentCost *int
		var planRateCents *int
		var planUnit *string
		if userClientID != nil && device.State == services.DeviceStateLoggedIn {
			session, err := h.sessionService.GetActiveSessionByClientID(*userClientID)
			if err == nil && session != nil {
				sessionID = &session.ID
				// 计算会话持续时间（秒）
				duration := int(time.Since(session.StartedAt).Seconds())
				sessionDuration = &duration

				// Calculate current cost
				cost := h.sessionService.CalculateSessionCost(session)
				currentCost = &cost

				// Plan info
				if session.Plan != nil {
					planRateCents = &session.Plan.Rate
					planUnit = &session.Plan.Unit
				}
			}
		}

		// 确定设备在线状态
		isOnline := device.WSConn != nil && device.State != services.DeviceStateOffline

		// Use registered device name from DB; fall back to in-memory name
		displayName := device.DeviceName
		if regName, ok := deviceNameMap[device.MachineCode]; ok {
			displayName = regName
		}

		// 构建设备项
		item := map[string]interface{}{
			"id":              device.HostID + ":" + device.MachineCode, // 组合ID
			"name":            displayName,
			"machine_code":    device.MachineCode,
			"host_id":         device.HostID,
			"ip":              device.IP,
			"state":           string(device.State),
			"battery_level":   device.BatteryLevel,
			"charging_state":  device.ChargingState,
			"current_app_name": device.CurrentAppName,
			"device_temp":     device.DeviceTemp,
		}

		// 添加连接时间（如果在线）
		if isOnline {
			connectedSince := device.ConnectedAt.Format(time.RFC3339)
			item["connected_since"] = connectedSince
		} else {
			item["connected_since"] = nil
		}

		// 添加最后断开时间
		if device.DisconnectedAt != nil {
			item["last_seen"] = device.DisconnectedAt.Format(time.RFC3339)
		} else if isOnline {
			item["last_seen"] = nil
		}

		// 添加用户信息（如果绑定）
		if userID != nil {
			item["user_id"] = *userID
		}
		if userNickname != nil {
			item["user_nickname"] = *userNickname
		}

		// 添加会话信息（如果活跃）
		if sessionID != nil {
			item["session_id"] = *sessionID
		}
		if sessionDuration != nil {
			item["session_duration_seconds"] = *sessionDuration
		}
		if currentCost != nil {
			item["current_cost_cents"] = *currentCost
		}
		if planRateCents != nil {
			item["plan_rate_cents"] = *planRateCents
		}
		if planUnit != nil {
			item["plan_unit"] = *planUnit
		}
		// 应用过滤
		// 搜索过滤：设备名称或用户昵称
		if search != "" {
			matched := false
			if displayName != "" && strings.Contains(strings.ToLower(displayName), strings.ToLower(search)) {
				matched = true
			}
			if userNickname != nil && strings.Contains(strings.ToLower(*userNickname), strings.ToLower(search)) {
				matched = true
			}
			if !matched {
				continue
			}
		}

		// 状态过滤
		if stateFilter != "" {
			if stateFilter == "online" {
				// "online" matches idle + logged_in (everything except offline)
				if device.State == services.DeviceStateOffline {
					continue
				}
			} else if string(device.State) != stateFilter {
				continue
			}
		}

		items = append(items, item)

		// 更新统计
		summary["total"] = summary["total"].(int) + 1
		if isOnline {
			summary["online"] = summary["online"].(int) + 1
		}
		if device.State == services.DeviceStateLoggedIn {
			summary["in_session"] = summary["in_session"].(int) + 1
		}
		if device.State == services.DeviceStateOffline {
			summary["offline"] = summary["offline"].(int) + 1
		}
	}

	// 分页
	total := len(items)
	start := (page - 1) * limit
	end := start + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	pagedItems := []map[string]interface{}{}
	if start < total {
		pagedItems = items[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary": summary,
			"items":   pagedItems,
			"total":   total,
			"page":    page,
			"limit":   limit,
		},
	})
}

// Disconnect 强制断开设备连接 (ADM_DEV_DISCONNECT)
func (h *DevicesHandler) Disconnect(c *gin.Context) {
	deviceID := c.Param("id")

	// 解析 deviceID (格式: "hostID:machineCode")
	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
			"data":    nil,
		})
		return
	}

	// 获取设备信息
	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Device not found",
			"data":    nil,
		})
		return
	}

	// 检查设备是否在线
	if device.State == services.DeviceStateOffline || device.WSConn == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device is already offline",
			"data":    nil,
		})
		return
	}

	// Explicit disconnect: end sessions immediately (no grace period)
	if device.BoundUserID != "" {
		// Send session_end WS message before closing
		if device.ActiveSessionID != "" {
			h.deviceStateService.SendMessage(hostID, machineCode, "session_end", gin.H{
				"session_id": device.ActiveSessionID,
				"reason":     "admin_force",
			})
		}
		// Send client_quit WS message
		h.deviceStateService.SendMessage(hostID, machineCode, "client_quit", gin.H{
			"reason": "admin",
		})
		// Clear ActiveSessionID first so the billing ticker skips this device
		h.deviceStateService.SetActiveSession(hostID, machineCode, "")
		// End sessions immediately
		h.sessionService.EndSessionsByDevice(hostID, machineCode, "admin_kick")
	}

	// Mark as explicit so the WS defer block skips grace period
	h.deviceStateService.MarkExplicitDisconnect(hostID, machineCode)

	// 关闭 WebSocket 连接
	if device.WSConn != nil {
		device.WSConn.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"success": true,
		},
	})
}

// Reboot 重启设备 (ADM_DEV_REBOOT)
func (h *DevicesHandler) Reboot(c *gin.Context) {
	deviceID := c.Param("id")

	// 解析 deviceID (格式: "hostID:machineCode")
	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
			"data":    nil,
		})
		return
	}

	// 获取设备信息
	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Device not found",
			"data":    nil,
		})
		return
	}

	// 检查设备是否在线
	if device.State == services.DeviceStateOffline || device.WSConn == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device is already offline",
			"data":    nil,
		})
		return
	}

	// End active sessions if device has a bound user
	if device.BoundUserID != "" {
		if device.ActiveSessionID != "" {
			h.deviceStateService.SendMessage(hostID, machineCode, "session_end", gin.H{
				"session_id": device.ActiveSessionID,
				"reason":     "admin_force",
			})
		}
		// Clear ActiveSessionID first so the billing ticker skips this device
		h.deviceStateService.SetActiveSession(hostID, machineCode, "")
		h.sessionService.EndSessionsByDevice(hostID, machineCode, "admin_reboot")
	}

	// Send reboot command to device
	h.deviceStateService.SendMessage(hostID, machineCode, "system_command", gin.H{
		"action": "reboot",
	})

	// Mark as explicit so the WS defer block skips grace period
	h.deviceStateService.MarkExplicitDisconnect(hostID, machineCode)

	// 关闭 WebSocket 连接
	if device.WSConn != nil {
		device.WSConn.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"success": true,
		},
	})
}

// Poweroff 关机设备 (ADM_DEV_POWEROFF)
func (h *DevicesHandler) Poweroff(c *gin.Context) {
	deviceID := c.Param("id")

	// 解析 deviceID (格式: "hostID:machineCode")
	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
			"data":    nil,
		})
		return
	}

	// 获取设备信息
	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Device not found",
			"data":    nil,
		})
		return
	}

	// 检查设备是否在线
	if device.State == services.DeviceStateOffline || device.WSConn == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device is already offline",
			"data":    nil,
		})
		return
	}

	// End active sessions if device has a bound user
	if device.BoundUserID != "" {
		if device.ActiveSessionID != "" {
			h.deviceStateService.SendMessage(hostID, machineCode, "session_end", gin.H{
				"session_id": device.ActiveSessionID,
				"reason":     "admin_force",
			})
		}
		// Clear ActiveSessionID first so the billing ticker skips this device
		h.deviceStateService.SetActiveSession(hostID, machineCode, "")
		h.sessionService.EndSessionsByDevice(hostID, machineCode, "admin_poweroff")
	}

	// Send poweroff command to device
	h.deviceStateService.SendMessage(hostID, machineCode, "system_command", gin.H{
		"action": "poweroff",
	})

	// Mark as explicit so the WS defer block skips grace period
	h.deviceStateService.MarkExplicitDisconnect(hostID, machineCode)

	// 关闭 WebSocket 连接
	if device.WSConn != nil {
		device.WSConn.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"success": true,
		},
	})
}

// ShellExec executes a shell command on a remote device
func (h *DevicesHandler) ShellExec(c *gin.Context) {
	deviceID := c.Param("id")

	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
		})
		return
	}

	// Check device is online
	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Device not found",
		})
		return
	}

	if device.State == services.DeviceStateOffline || device.WSConn == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device is offline",
		})
		return
	}

	// Bind request
	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Command is required",
		})
		return
	}

	// Get admin ID from context (set by AdminAuthMiddleware)
	adminID, _ := c.Get("admin_id")
	adminIDStr, _ := adminID.(string)

	// Execute remote command (blocks up to 30s)
	shellLog, err := h.shellService.ExecuteRemoteCommand(hostID, machineCode, req.Command, adminIDStr)
	if err != nil {
		// Even on timeout/error, shellLog may have useful info
		if shellLog != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "success",
				"data": gin.H{
					"id":          shellLog.ID,
					"command":     shellLog.Command,
					"stdout":      shellLog.Stdout,
					"stderr":      shellLog.Stderr,
					"exit_code":   shellLog.ExitCode,
					"status":      shellLog.Status,
					"duration_ms": shellLog.DurationMs,
					"created_at":  shellLog.CreatedAt,
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":          shellLog.ID,
			"command":     shellLog.Command,
			"stdout":      shellLog.Stdout,
			"stderr":      shellLog.Stderr,
			"exit_code":   shellLog.ExitCode,
			"status":      shellLog.Status,
			"duration_ms": shellLog.DurationMs,
			"created_at":  shellLog.CreatedAt,
		},
	})
}

// ShellHistory returns shell command history for a device
func (h *DevicesHandler) ShellHistory(c *gin.Context) {
	deviceID := c.Param("id")

	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
		})
		return
	}

	_ = hostID // machineCode is what we need for DB query

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	logs, total, err := h.shellService.GetHistory(machineCode, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to load shell history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": logs,
			"total": total,
		},
	})
}

// Screenshot captures a screenshot from a remote device
func (h *DevicesHandler) Screenshot(c *gin.Context) {
	deviceID := c.Param("id")

	hostID, machineCode := parseDeviceID(deviceID)
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid device ID format",
		})
		return
	}

	device, err := h.deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Device not found",
		})
		return
	}

	if device.State == services.DeviceStateOffline || device.WSConn == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device is offline",
		})
		return
	}

	display := c.DefaultQuery("display", "phone")
	if display != "phone" && display != "external" {
		display = "phone"
	}

	result, err := h.screenCaptureService.ExecuteCapture(hostID, machineCode, display)
	if err != nil {
		if result != nil && result.Error != "" {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": result.Error,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"image_base64": result.ImageBase64,
			"display":      result.Display,
			"width":        result.Width,
			"height":       result.Height,
		},
	})
}

// 辅助函数：解析设备 ID
func parseDeviceID(deviceID string) (string, string) {
	// 格式: "hostID:machineCode"
	parts := strings.Split(deviceID, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
