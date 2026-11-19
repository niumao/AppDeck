package client

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"appdeck/config"
	"appdeck/middleware"
	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService        *services.AuthService
	wechatService      *services.WechatService
	userService        *services.UserService
	billingPlanService *services.BillingPlanService
	sessionService     *services.SessionService
	wechatToken        string
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:        services.NewAuthService(cfg),
		wechatService:      services.GetWechatService(),
		userService:        services.NewUserService(),
		billingPlanService: services.NewBillingPlanService(),
		sessionService:     services.NewSessionService(),
		wechatToken:        cfg.WeChat.Token,
	}
}

// GetQRCode 获取登录二维码
// GET /api/v1/auth/qrcode
func (h *AuthHandler) GetQRCode(c *gin.Context) {
	result, err := h.authService.GenerateLoginQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成二维码失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetOAQRCode 获取公众号参数二维码
// GET /api/v1/auth/oa-qrcode?host_id=xxx&machine_code=xxx
func (h *AuthHandler) GetOAQRCode(c *gin.Context) {
	hostID := c.Query("host_id")
	machineCode := c.Query("machine_code")

	// Validate required parameters
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Missing host_id or machine_code",
			"data":    nil,
		})
		return
	}

	// Create scene string: "bind:{host_id}:{machine_code}:{timestamp}"
	ts := c.Query("ts")
	if ts == "" {
		ts = fmt.Sprintf("%d", time.Now().Unix()) // fallback for old clients
	}
	sceneStr := fmt.Sprintf("bind:%s:%s:%s", hostID, machineCode, ts)

	// Get OA service
	oaService := services.GetWeChatOAService()
	if oaService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "WeChat OA service not initialized",
			"data":    nil,
		})
		return
	}

	// Create parametric QR code
	qrURL, err := oaService.CreateParametricQR(sceneStr)
	if err != nil {
		log.Printf("Error creating OA parametric QR: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create QR code",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"qr_url": qrURL,
		},
	})
}

// PollStatus 轮询登录状态
// GET /api/v1/auth/poll/:scene_str
func (h *AuthHandler) PollStatus(c *gin.Context) {
	sceneStr := c.Param("scene_str")
	if sceneStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少场景字符串",
		})
		return
	}

	result, err := h.authService.PollLoginStatus(sceneStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "登录会话不存在",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// WechatXMLMessage 微信XML消息
type WechatXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
}

// WechatCallback 微信回调
// GET/POST /api/v1/auth/callback
func (h *AuthHandler) WechatCallback(c *gin.Context) {
	// GET请求用于验证服务器
	if c.Request.Method == "GET" {
		h.verifyServer(c)
		return
	}

	// POST请求用于接收消息
	h.handleMessage(c)
}

// verifyServer 验证服务器
func (h *AuthHandler) verifyServer(c *gin.Context) {
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echostr := c.Query("echostr")

	if h.verifySignature(signature, timestamp, nonce) {
		c.String(http.StatusOK, echostr)
	} else {
		c.String(http.StatusForbidden, "signature verification failed")
	}
}

// handleMessage 处理微信消息
func (h *AuthHandler) handleMessage(c *gin.Context) {
	// 验证签名
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	if !h.verifySignature(signature, timestamp, nonce) {
		c.String(http.StatusForbidden, "signature verification failed")
		return
	}

	// 读取消息体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "failed to read body")
		return
	}

	// 解析XML
	var msg WechatXMLMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		c.String(http.StatusBadRequest, "failed to parse xml")
		return
	}

	// 处理事件
	wechatMsg := &services.WechatMessage{
		ToUserName:   msg.ToUserName,
		FromUserName: msg.FromUserName,
		CreateTime:   msg.CreateTime,
		MsgType:      msg.MsgType,
		Event:        msg.Event,
		EventKey:     msg.EventKey,
		Ticket:       msg.Ticket,
	}

	if err := h.authService.ProcessWechatEvent(wechatMsg); err != nil {
		// 记录错误但返回成功（避免微信重试）
		c.String(http.StatusOK, "success")
		return
	}

	c.String(http.StatusOK, "success")
}

// verifySignature 验证签名
func (h *AuthHandler) verifySignature(signature, timestamp, nonce string) bool {
	arr := []string{h.wechatToken, timestamp, nonce}
	sort.Strings(arr)
	str := strings.Join(arr, "")

	hash := sha1.New()
	hash.Write([]byte(str))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	return hashStr == signature
}

// GetMe 获取当前用户信息
// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    user,
	})
}

// Logout 登出
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// 客户端清除token即可，服务端无需特殊处理
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "登出成功",
	})
}

// GetBalance 获取用户余额
// GET /api/v1/user/balance?machine_code=xxx (optional)
func (h *AuthHandler) GetBalance(c *gin.Context) {
	userInterface := middleware.GetUser(c)
	if userInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	user := userInterface.(*models.User)

	// Check for active billing session and calculate accumulated cost
	totalBalance := user.GetTotalBalance()
	effectiveBalance := totalBalance
	activeSession, err := h.sessionService.GetActiveSessionByUserID(user.ID)
	if err == nil && activeSession != nil {
		// User has an active session, calculate current cost
		currentCost := h.sessionService.CalculateSessionCost(activeSession)
		// Subtract accumulated cost from displayed balance (but don't modify DB)
		effectiveBalance = totalBalance - currentCost
		if effectiveBalance < 0 {
			effectiveBalance = 0
		}
	}

	// Calculate remaining_minutes using session's actual plan (fall back to default)
	remainingMinutes := 0
	if effectiveBalance > 0 {
		var rate int
		var unit string

		// Prefer the active session's plan over the default plan
		if activeSession != nil && activeSession.Plan != nil && activeSession.Plan.Rate > 0 {
			rate = activeSession.Plan.Rate
			unit = activeSession.Plan.Unit
		} else {
			plan, err := h.billingPlanService.GetDefaultPlan()
			if err == nil && plan != nil && plan.Rate > 0 {
				rate = plan.Rate
				unit = plan.Unit
			}
		}

		if rate > 0 {
			// Use float division to avoid truncation
			var ratePerMinute float64
			switch unit {
			case "hour":
				ratePerMinute = float64(rate) / 60.0
			default:
				ratePerMinute = float64(rate)
			}
			if ratePerMinute > 0 {
				remainingMinutes = int(float64(effectiveBalance) / ratePerMinute)
			}
		}
	}

	responseData := gin.H{
		"balance_cents":     effectiveBalance,
		"balance_expiry":    user.BalanceExpiry,
		"remaining_minutes": remainingMinutes,
		"has_balance":       effectiveBalance > 0,
	}

	// If machine_code is provided, also include tenant balance
	machineCode := c.Query("machine_code")
	if machineCode != "" {
		regDeviceService := services.GetRegisteredDeviceService()
		tenantID := regDeviceService.GetTenantIDByMachineCode(machineCode)
		if tenantID != "" {
			tenantBalanceService := services.NewTenantBalanceService()
			tenantMinutes, err := tenantBalanceService.GetBalance(user.ID, tenantID)
			if err == nil {
				responseData["tenant_balance_minutes"] = tenantMinutes
				// Add tenant minutes to remaining_minutes so H5 gets total time
				remainingMinutes += tenantMinutes
				responseData["remaining_minutes"] = remainingMinutes
				responseData["has_balance"] = effectiveBalance > 0 || tenantMinutes > 0
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    responseData,
	})
}

// GetDeviceStatus returns the current status of a device (idle/in_use/offline/in_use_by_self)
// GET /api/v1/auth/device-status?host_id=X&machine_code=Y
func (h *AuthHandler) GetDeviceStatus(c *gin.Context) {
	hostID := c.Query("host_id")
	machineCode := c.Query("machine_code")
	if hostID == "" || machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Missing host_id or machine_code"})
		return
	}

	// Get authenticated user
	user := middleware.GetUser(c)
	var currentUserID string
	if user != nil {
		currentUserID = user.(*models.User).ID
	}

	// Prefer the admin-configured name (set on /admin/devices) over the
	// runtime-reported name so the H5 page matches what the admin sees.
	adminDeviceName := services.GetRegisteredDeviceService().GetDeviceName(machineCode)

	deviceStateService := services.GetDeviceStateService()
	device, err := deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		// Device not in memory — offline or not registered
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{"status": "offline", "device_name": adminDeviceName},
		})
		return
	}

	deviceName := adminDeviceName
	if deviceName == "" {
		deviceName = device.DeviceName
	}

	status := "idle"
	responseData := gin.H{"status": status, "device_name": deviceName}

	if device.WSConn == nil || device.State == services.DeviceStateOffline {
		responseData["status"] = "offline"
	} else if device.BoundUserID != "" {
		if currentUserID != "" && device.BoundUserID == currentUserID {
			responseData["status"] = "in_use_by_self"
			// Include client_id so H5 can call quit
			userClientService := services.NewUserClientService()
			client, err := userClientService.GetClientByHostInfo(hostID, machineCode)
			if err == nil && client != nil {
				responseData["client_id"] = client.ID
			}
		} else {
			responseData["status"] = "in_use"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": responseData,
	})
}

// GetOAuthURL 获取 OAuth 授权链接
// GET /api/v1/auth/oauth/url
func (h *AuthHandler) GetOAuthURL(c *gin.Context) {
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	scope := c.Query("scope") // snsapi_base 或 snsapi_userinfo

	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 redirect_uri 参数",
		})
		return
	}

	url := h.wechatService.GetOAuthURL(redirectURI, state, scope)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"url": url,
		},
	})
}

// WechatOAuthCallback 微信 OAuth 回调
// GET /api/v1/auth/oauth/callback
func (h *AuthHandler) WechatOAuthCallback(c *gin.Context) {
	code := c.Query("code")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 code 参数",
		})
		return
	}

	// 通过 code 换取 access_token 和 openid
	tokenResp, err := h.wechatService.GetOAuthToken(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取 OAuth Token 失败",
			"error":   err.Error(),
		})
		return
	}

	// 根据 scope 确定 auth_level 和是否需要获取用户信息
	var user *models.User
	var authLevel string

	log.Printf("[NICKNAME-DEBUG] OAuth callback: scope=%q, openID=%s", tokenResp.Scope, tokenResp.OpenID)

	if tokenResp.Scope == "snsapi_userinfo" {
		// 完整授权：获取用户详细信息
		userInfo, err := h.wechatService.GetOAuthUserInfo(tokenResp.AccessToken, tokenResp.OpenID)
		if err != nil {
			log.Printf("[NICKNAME-DEBUG] GetOAuthUserInfo FAILED: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "获取用户信息失败",
				"error":   err.Error(),
			})
			return
		}

		log.Printf("[NICKNAME-DEBUG] WeChat returned: nickname=%q, avatar=%q", userInfo.Nickname, userInfo.HeadImgURL)

		// 创建或查找用户（完整信息）
		user, _, err = h.userService.FindOrCreateByOpenID(
			tokenResp.OpenID,
			userInfo.UnionID,
			userInfo.Nickname,
			userInfo.HeadImgURL,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建/更新用户失败",
				"error":   err.Error(),
			})
			return
		}

		authLevel = "full"
	} else {
		// 基础授权：只有 openid，生成可区分的默认昵称
		defaultNickname := "用户" + tokenResp.OpenID[len(tokenResp.OpenID)-4:]
		user, _, err = h.userService.FindOrCreateByOpenID(
			tokenResp.OpenID,
			"",              // unionID 为空
			defaultNickname, // 用 OpenID 后4位生成默认昵称
			"",              // avatarURL 为空
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "创建/更新用户失败",
				"error":   err.Error(),
			})
			return
		}

		authLevel = "basic"
	}

	// 更新用户的 auth_level
	user.AuthLevel = authLevel
	if err := h.userService.UpdateAuthLevel(user.ID, authLevel); err != nil {
		// 非致命错误，继续执行
	}

	// 检查用户状态
	if !user.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "用户已被禁用",
		})
		return
	}

	// 生成 JWT token
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成 Token 失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"token": token,
		},
	})
}

// GetPromoPlan 获取推广计费方案
// GET /api/v1/plans/promo
func (h *AuthHandler) GetPromoPlan(c *gin.Context) {
	plan, err := h.billingPlanService.GetPromoPlan()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "没有可用的计费方案",
		})
		return
	}

	// 计算 duration_display
	var durationDisplay string
	if plan.Unit == "hour" {
		hours := 1
		if plan.Rate > 0 {
			// 假设用户有 plan.Rate 分钱，可以使用的时长
			// 对于推广，我们显示1个单位的时长
			hours = 1
		}
		durationDisplay = fmt.Sprintf("%d小时", hours)
	} else {
		// minute
		minutes := 1
		if plan.Rate > 0 {
			minutes = 1
		}
		durationDisplay = fmt.Sprintf("%d分钟", minutes)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"name":             plan.Name,
			"price_cents":      plan.Rate,
			"unit":             plan.Unit,
			"duration_display": durationDisplay,
		},
	})
}

// BindDeviceRequest 绑定设备请求
type BindDeviceRequest struct {
	HostID      string `json:"host_id" binding:"required"`
	MachineCode string `json:"machine_code" binding:"required"`
}

// BindDevice 绑定设备 (H5_BIND_DEVICE)
// POST /api/v1/auth/bind-device
func (h *AuthHandler) BindDevice(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Unauthorized",
			"data":    nil,
		})
		return
	}

	userModel := user.(*models.User)

	var req BindDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"data":    nil,
		})
		return
	}

	deviceStateService := services.GetDeviceStateService()
	userClientService := services.NewUserClientService()

	// 1. 查找设备 - 先查内存中的在线设备
	device, err := deviceStateService.GetDevice(req.HostID, req.MachineCode)

	// 如果内存中没有，尝试从数据库查找（设备可能存在但未连接）
	if err != nil {
		// 尝试从数据库查找设备
		dbClient, dbErr := userClientService.GetClientByHostInfo(req.HostID, req.MachineCode)
		if dbErr != nil {
			// 数据库中也没有，设备真的不存在
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "Device not found",
				"data":    nil,
			})
			return
		}
		// 设备存在但离线
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Device is offline",
			"data": gin.H{
				"device_name": dbClient.Name,
			},
		})
		return
	}

	// 2. 检查设备是否在线（有活跃的 WebSocket 连接）
	if device.WSConn == nil || device.State == services.DeviceStateOffline {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Device is offline",
			"data": gin.H{
				"device_name": device.DeviceName,
			},
		})
		return
	}

	// 3. 检查设备是否已被绑定
	if device.BoundUserID != "" && device.BoundUserID != userModel.ID {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "Device already in use by another user",
			"data":    nil,
		})
		return
	}

	// 3.1 同一用户重复登录同一设备 — resync device UI and return current session info
	if device.BoundUserID == userModel.ID && device.ActiveSessionID != "" {
		log.Printf("Duplicate login: user %s already logged in on device %s:%s (session %s), resyncing UI",
			userModel.ID, req.HostID, req.MachineCode, device.ActiveSessionID)

		// Resend login state to device so WebView recovers if it lost state
		sessionService := services.NewSessionService()
		reconnData, reconErr := sessionService.GetSessionReconnectData(device.ActiveSessionID)
		if reconErr == nil {
			token, _ := h.authService.GenerateToken(userModel)
			var planUnitSeconds int
			switch reconnData.PlanUnit {
			case "hour":
				planUnitSeconds = 3600
			default:
				planUnitSeconds = 60
			}
			registeredDeviceService := services.GetRegisteredDeviceService()
			freePlay := registeredDeviceService.GetFreePlayByMachineCode(req.MachineCode)
			deviceStateService.SendMessage(req.HostID, req.MachineCode, "device_state_change", map[string]interface{}{
				"event": "login",
				"state": "logged_in",
				"user": map[string]interface{}{
					"id":         userModel.ID,
					"nickname":   userModel.Nickname,
					"avatar_url": userModel.AvatarURL,
				},
				"token":                  token,
				"balance_cents":          reconnData.BalanceCents,
				"session_id":             reconnData.SessionID,
				"plan_rate_cents":        reconnData.PlanRateCents,
				"plan_unit":              reconnData.PlanUnit,
				"plan_unit_seconds":      planUnitSeconds,
				"remaining_seconds":      reconnData.RemainingSeconds,
				"tenant_balance_minutes": reconnData.TenantBalanceMinutes,
				"global_balance_cents":   reconnData.GlobalBalanceCents,
				"free_play":              freePlay,
				"server_ts":              time.Now().Unix(),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "already_logged_in",
			"data": gin.H{
				"device_name": device.DeviceName,
				"status":      "already_logged_in",
				"session_id":  device.ActiveSessionID,
			},
		})
		return
	}

	// 3.5. 检查用户是否关注公众号 (WeChat Official Account follow check)
	oaService := services.GetWeChatOAService()
	if oaService != nil {
		followed, err := oaService.CheckFollow(userModel.OpenID)
		if err != nil {
			log.Printf("Warning: Failed to check OA follow status for user %s: %v", userModel.ID, err)
			// Continue - don't block bind-device on follow check failure
		} else if !followed {
			// User has not followed the OA - return 403 with oa_qrcode_url (consistent with H5_CHECK_FOLLOW)
			followURL := oaService.GetFollowURL()
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "ERR_NOT_FOLLOWED",
				"data": gin.H{
					"oa_qrcode_url": followURL,
					"follow_url":    followURL, // Deprecated, kept for backward compatibility
				},
			})
			return
		}
	}

	// 4. 生成 JWT token
	token, err := h.authService.GenerateToken(userModel)
	if err != nil {
		log.Printf("Error: Failed to generate JWT token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate token",
			"data":    nil,
		})
		return
	}

	// 5. 绑定用户到设备
	if err := deviceStateService.BindUser(req.HostID, req.MachineCode, userModel.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to bind device",
			"data":    nil,
		})
		return
	}

	// 5.5 创建或更新 UserClient 记录，关联 host_id 和 machine_code
	clientID, err := userClientService.CreateOrUpdateClientWithHostInfo(
		userModel.ID,
		device.DeviceName,
		req.HostID,
		req.MachineCode,
	)
	if err != nil {
		log.Printf("Error: Failed to create/update UserClient record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create device record",
			"data":    nil,
		})
		return
	}

	// 6. Try to start a billing session (handles both global and tenant balance)
	balanceCents := userModel.GetTotalBalance()
	status := "zero_balance"

	var sessionID string
	var planID string
	var planRateCents int
	var planUnit string
	var remainingSeconds int
	var otherSessionCount int
	var otherDeviceNames []string
	var balanceSource string
	var tenantBalanceMinutes int

	// Try session start — StartSessionOnBind checks both tenant and global balance
	existingSession, _ := h.sessionService.GetActiveSessionByClientID(clientID)
	if existingSession == nil {
		result, err := h.sessionService.StartSessionOnBind(userModel.ID, clientID)
		if err != nil {
			// Session start failed — could be zero balance on both sources
			log.Printf("Info: Session not started on bind for user %s: %v", userModel.ID, err)
			// Don't return error — show zero_balance page so user can recharge
		} else {
			sessionID = result.ID
			planID = result.PlanID
			planRateCents = result.PlanRateCents
			planUnit = result.PlanUnit
			remainingSeconds = result.RemainingSeconds
			otherSessionCount = result.OtherSessionCount
			otherDeviceNames = result.OtherDeviceNames
			balanceSource = result.BalanceSource
			tenantBalanceMinutes = result.TenantBalanceMinutes
			status = "ok"
		}
	} else {
		sessionID = existingSession.ID
		if existingSession.Plan != nil {
			planID = existingSession.PlanID
			planRateCents = existingSession.Plan.Rate
			planUnit = existingSession.Plan.Unit
		}
		// Recalculate remaining seconds
		if existingSession.Plan != nil {
			var ratePerSecond float64
			switch existingSession.Plan.Unit {
			case "minute":
				ratePerSecond = float64(existingSession.Plan.Rate) / 60.0
			case "hour":
				ratePerSecond = float64(existingSession.Plan.Rate) / 3600.0
			default:
				ratePerSecond = float64(existingSession.Plan.Rate) / 60.0
			}
			if ratePerSecond > 0 {
				remainingSeconds = int(float64(balanceCents) / ratePerSecond)
			}
		}
		status = "ok"
	}

	if status == "ok" {
		// Track active session on device
		if sessionID != "" {
			deviceStateService.SetActiveSession(req.HostID, req.MachineCode, sessionID)
		}

		// Convert plan_unit to plan_unit_seconds
		var planUnitSeconds int
		if planUnit == "hour" {
			planUnitSeconds = 3600
		} else if planUnit == "minute" {
			planUnitSeconds = 60
		} else {
			planUnitSeconds = 60
		}

		// 余额充足，推送 device_state_change 到客户端
		deviceStateService.UpdateDeviceState(req.HostID, req.MachineCode, services.DeviceStateLoggedIn)
		deviceStateService.SendMessage(req.HostID, req.MachineCode, "device_state_change", gin.H{
			"event": "login",
			"state": "logged_in",
			"user": gin.H{
				"id":         userModel.ID,
				"nickname":   userModel.Nickname,
				"avatar_url": userModel.AvatarURL,
			},
			"token":                  token,
			"balance_cents":          balanceCents,
			"session_id":             sessionID,
			"plan_id":                planID,
			"plan_rate_cents":        planRateCents,
			"plan_unit":              planUnit,
			"plan_unit_seconds":      planUnitSeconds,
			"remaining_seconds":      remainingSeconds,
			"tenant_balance_minutes": tenantBalanceMinutes,
			"global_balance_cents":   balanceCents,
			"other_session_count":    otherSessionCount,
			"other_device_names":     otherDeviceNames,
			"server_ts":              time.Now().Unix(),
		})

		// Notify sibling devices so they get corrected remaining_seconds and other_session_count
		if otherSessionCount > 0 {
			go (&services.RechargeService{}).PushBalanceUpdateToActiveDevices(userModel.ID)
		}
	}

	// 7. 返回响应
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device_name":            device.DeviceName,
			"balance_cents":          balanceCents,
			"status":                 status,
			"session_id":             sessionID,
			"remaining_seconds":      remainingSeconds,
			"balance_source":         balanceSource,
			"tenant_balance_minutes": tenantBalanceMinutes,
		},
	})

	// 8. 异步发送登录成功客服消息 (H5_LOGIN_MSG)
	// Fire-and-forget goroutine - don't block the response
	if oaService != nil {
		go func() {
			// Format timestamp in Asia/Shanghai timezone
			loc, err := time.LoadLocation("Asia/Shanghai")
			if err != nil {
				loc = time.UTC // fallback to UTC if timezone load fails
			}
			timestamp := time.Now().In(loc).Format("2006-01-02 15:04:05")

			// Build message content
			message := fmt.Sprintf(
				"Login successful!\nDevice: %s\nTime: %s\nIf this was not you, please contact support immediately.",
				device.DeviceName,
				timestamp,
			)

			// Send customer service message
			err = oaService.SendCustomerMessage(userModel.OpenID, message)
			if err != nil {
				log.Printf("Warning: Failed to send login success message to user %s: %v", userModel.ID, err)
				// Don't propagate error - this is best-effort notification
			} else {
				log.Printf("Login success message sent to user %s (device: %s)", userModel.ID, device.DeviceName)
			}
		}()
	}
}

// CheckFollow 检查用户是否关注公众号 (H5_CHECK_FOLLOW)
// GET /api/v1/h5/check-follow
func (h *AuthHandler) CheckFollow(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Unauthorized",
			"data":    nil,
		})
		return
	}

	userModel := user.(*models.User)

	// Get OA service
	oaService := services.GetWeChatOAService()
	if oaService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "WeChat OA service not initialized",
			"data":    nil,
		})
		return
	}

	// Check follow status
	followed, err := oaService.CheckFollow(userModel.OpenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to check follow status",
			"data":    nil,
		})
		return
	}

	// Build response
	responseData := gin.H{
		"followed": followed,
	}

	// Always include oa_qrcode_url (per API spec H5_CHECK_FOLLOW)
	qrcodeURL := oaService.GetFollowURL()
	if qrcodeURL != "" {
		responseData["oa_qrcode_url"] = qrcodeURL
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    responseData,
	})
}
