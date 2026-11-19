package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthService 认证服务
type AuthService struct {
	wechatService *WechatService
	userService   *UserService
	jwtSecret     string
	jwtExpiry     time.Duration
	baseURL       string
}

// JWTClaims JWT claims
type JWTClaims struct {
	UserID   string `json:"userId"`
	OpenID   string `json:"openId"`
	Nickname string `json:"nickname"`
	jwt.RegisteredClaims
}

// LoginQRCodeResult 登录二维码结果
type LoginQRCodeResult struct {
	SceneStr  string    `json:"sceneStr"`
	QRCodeURL string    `json:"qrCodeUrl"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// LoginPollResult 登录轮询结果
type LoginPollResult struct {
	Status   int          `json:"status"`
	Token    string       `json:"token,omitempty"`
	User     *models.User `json:"user,omitempty"`
	Message  string       `json:"message"`
}

// NewAuthService 创建认证服务
func NewAuthService(cfg *config.Config) *AuthService {
	return &AuthService{
		wechatService: GetWechatService(),
		userService:   NewUserService(),
		jwtSecret:     cfg.JWT.Secret,
		jwtExpiry:     time.Duration(cfg.JWT.ExpiryHours) * time.Hour,
		baseURL:       cfg.Server.BaseURL,
	}
}

// GenerateLoginQRCode 生成登录二维码
func (s *AuthService) GenerateLoginQRCode() (*LoginQRCodeResult, error) {
	// 生成场景字符串
	sceneStr := uuid.New().String()
	expireSeconds := 300 // 5分钟

	// 调用微信API创建临时二维码
	qrResult, err := s.wechatService.CreateTempQRCode(sceneStr, expireSeconds)
	if err != nil {
		return nil, fmt.Errorf("failed to create qr code: %w", err)
	}

	// 创建登录会话
	expiresAt := time.Now().Add(time.Duration(expireSeconds) * time.Second)
	session := &models.LoginSession{
		ID:        uuid.New().String(),
		SceneStr:  sceneStr,
		Status:    models.LoginSessionStatusPending,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if err := database.DB.Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create login session: %w", err)
	}

	return &LoginQRCodeResult{
		SceneStr:  sceneStr,
		QRCodeURL: s.wechatService.GetQRCodeURL(qrResult.Ticket),
		ExpiresAt: expiresAt,
	}, nil
}

// PollLoginStatus 轮询登录状态
func (s *AuthService) PollLoginStatus(sceneStr string) (*LoginPollResult, error) {
	var session models.LoginSession
	if err := database.DB.Where("scene_str = ?", sceneStr).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found")
	}

	// 检查是否过期
	if session.IsExpired() {
		// 更新状态为过期
		database.DB.Model(&session).Update("status", models.LoginSessionStatusExpired)
		return &LoginPollResult{
			Status:  models.LoginSessionStatusExpired,
			Message: "二维码已过期，请重新获取",
		}, nil
	}

	switch session.Status {
	case models.LoginSessionStatusPending:
		return &LoginPollResult{
			Status:  models.LoginSessionStatusPending,
			Message: "等待扫码",
		}, nil

	case models.LoginSessionStatusScanned:
		return &LoginPollResult{
			Status:  models.LoginSessionStatusScanned,
			Message: "已扫码，等待确认",
		}, nil

	case models.LoginSessionStatusAuthorized:
		// 获取用户信息
		if session.UserID != nil {
			user, err := s.userService.GetByID(*session.UserID)
			if err != nil {
				return nil, fmt.Errorf("failed to get user: %w", err)
			}
			return &LoginPollResult{
				Status:  models.LoginSessionStatusAuthorized,
				Token:   session.Token,
				User:    user,
				Message: "登录成功",
			}, nil
		}
		return nil, fmt.Errorf("user not found in session")

	default:
		return &LoginPollResult{
			Status:  session.Status,
			Message: "未知状态",
		}, nil
	}
}

// ProcessWechatEvent 处理微信事件（扫码、关注等）
func (s *AuthService) ProcessWechatEvent(msg *WechatMessage) error {
	// 只处理扫码事件
	if msg.MsgType != "event" {
		return nil
	}

	var sceneStr string
	switch msg.Event {
	case "subscribe":
		// 新用户关注（通过扫码关注）
		if msg.EventKey != "" {
			// EventKey 格式: qrscene_xxx
			if len(msg.EventKey) > 8 && msg.EventKey[:8] == "qrscene_" {
				sceneStr = msg.EventKey[8:]
			}
		}
	case "SCAN":
		// 已关注用户扫码
		sceneStr = msg.EventKey
	default:
		return nil
	}

	if sceneStr == "" {
		return nil
	}

	// Check if this is a parametric bind QR (format: "bind:{host_id}:{machine_code}")
	if len(sceneStr) > 5 && sceneStr[:5] == "bind:" {
		return s.processParametricBind(msg.FromUserName, sceneStr)
	}

	// Otherwise, handle normal login session flow
	// 查找登录会话
	var session models.LoginSession
	if err := database.DB.Where("scene_str = ?", sceneStr).First(&session).Error; err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// 检查会话是否有效
	if session.IsExpired() || session.Status != models.LoginSessionStatusPending {
		return fmt.Errorf("session expired or already used")
	}

	// 获取用户信息
	openID := msg.FromUserName
	userInfo, err := s.wechatService.GetUserInfo(openID)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Filter out WeChat's default placeholder nickname
	nickname := userInfo.Nickname
	if nickname == "微信用户" || nickname == "" {
		nickname = "用户" + openID[len(openID)-4:]
	}

	// 创建或更新用户
	user, _, err := s.userService.FindOrCreateByOpenID(
		openID,
		userInfo.UnionID,
		nickname,
		userInfo.HeadImgURL,
	)
	if err != nil {
		return fmt.Errorf("failed to find or create user: %w", err)
	}

	// 检查用户状态
	if !user.IsActive() {
		return fmt.Errorf("user is disabled")
	}

	// 生成JWT token
	token, err := s.GenerateToken(user)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// 更新登录会话
	session.Status = models.LoginSessionStatusAuthorized
	session.UserID = &user.ID
	session.Token = token
	if err := database.DB.Save(&session).Error; err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// GenerateToken 生成JWT token
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	claims := JWTClaims{
		UserID:   user.ID,
		OpenID:   user.OpenID,
		Nickname: user.Nickname,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "appdeck",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// ValidateToken 验证JWT token
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GetUserFromToken 从token获取用户
func (s *AuthService) GetUserFromToken(tokenString string) (*models.User, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return s.userService.GetByID(claims.UserID)
}

// CleanExpiredSessions 清理过期的登录会话
func (s *AuthService) CleanExpiredSessions() error {
	return database.DB.Where("expires_at < ?", time.Now()).Delete(&models.LoginSession{}).Error
}

// GenerateQRURLForDevice generates a QR URL for device binding
func (s *AuthService) GenerateQRURLForDevice(hostID, machineCode string) string {
	sceneStr := fmt.Sprintf("bind:%s:%s:%d", hostID, machineCode, time.Now().Unix())
	oaService := GetWeChatOAService()
	if oaService == nil {
		return ""
	}
	qrURL, err := oaService.CreateParametricQR(sceneStr)
	if err != nil {
		log.Printf("Failed to create QR URL for device %s:%s: %v", hostID, machineCode, err)
		return ""
	}
	return qrURL
}

// processParametricBind handles parametric QR bind flow
// sceneStr format: "bind:{host_id}:{machine_code}" or "bind:{host_id}:{machine_code}:{timestamp}"
func (s *AuthService) processParametricBind(openID, sceneStr string) error {
	// Parse scene string — supports 3-part (legacy) or 4-part (with timestamp)
	parts := strings.Split(sceneStr, ":")
	if len(parts) < 3 || len(parts) > 4 {
		return fmt.Errorf("invalid bind scene format: %s", sceneStr)
	}

	hostID := parts[1]
	machineCode := parts[2]

	// Validate timestamp if present (4-part format)
	if len(parts) == 4 {
		ts, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid bind timestamp: %s", parts[3])
		}
		elapsed := time.Since(time.Unix(ts, 0))
		if elapsed < 0 {
			elapsed = -elapsed
		}
		if elapsed > 5*time.Minute {
			log.Printf("Rejecting stale QR scan: sceneStr=%s, elapsed=%v", sceneStr, elapsed)
			go func() {
				oaService := GetWeChatOAService()
				if oaService != nil {
					oaService.SendCustomerMessage(openID, "二维码已过期，请扫描设备上最新的二维码。")
				}
			}()
			return fmt.Errorf("QR code expired (elapsed: %v)", elapsed)
		}
	}

	log.Printf("Processing parametric bind: openID=%s, hostID=%s, machineCode=%s", openID, hostID, machineCode)

	// Fetch real WeChat user info to get nickname and avatar (same as normal login flow)
	userInfo, err := s.wechatService.GetUserInfo(openID)
	var user *models.User
	var nickname, avatarURL, unionID string

	if err != nil {
		log.Printf("Warning: Failed to fetch WeChat user info for %s: %v. Using default nickname.", openID, err)
		// Fallback to default values if WeChat API fails
		nickname = "用户" + openID[len(openID)-4:]
		avatarURL = ""
		unionID = ""
	} else {
		// Use real WeChat user info, but filter out placeholder nickname
		nickname = userInfo.Nickname
		if nickname == "微信用户" || nickname == "" {
			nickname = "用户" + openID[len(openID)-4:]
		}
		avatarURL = userInfo.HeadImgURL
		unionID = userInfo.UnionID
	}

	// Find or create user with fetched info (or defaults if API failed)
	user, _, err = s.userService.FindOrCreateByOpenID(openID, unionID, nickname, avatarURL)
	if err != nil {
		return fmt.Errorf("failed to find/create user: %w", err)
	}

	// Check if user is active
	if !user.IsActive() {
		return fmt.Errorf("user is disabled")
	}

	deviceStateService := GetDeviceStateService()
	userClientService := NewUserClientService()

	// Get device
	device, err := deviceStateService.GetDevice(hostID, machineCode)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Check if device is online
	if device.WSConn == nil || device.State == DeviceStateOffline {
		return fmt.Errorf("device is offline")
	}

	// Check balance: tenant time balance first, then global balance
	hasTenantBalance := false
	var registeredDevice models.RegisteredDevice
	if err := database.DB.Where("machine_code = ?", machineCode).First(&registeredDevice).Error; err == nil && registeredDevice.TenantID != nil {
		tenantBalanceService := NewTenantBalanceService()
		tenantBalance, err := tenantBalanceService.GetBalance(user.ID, *registeredDevice.TenantID)
		if err == nil && tenantBalance > 0 {
			hasTenantBalance = true
		}
	}

	if user.HasValidBalance() || hasTenantBalance {
		// Balance > 0: auto-bind device
		log.Printf("User %s has balance %d, auto-binding to device %s:%s", user.ID, user.GetTotalBalance(), hostID, machineCode)

		// If same user already bound with active session, resync device UI and skip re-bind
		if device.BoundUserID == user.ID && device.ActiveSessionID != "" {
			log.Printf("Duplicate login: user %s already has active session %s on device %s:%s, resyncing UI",
				user.ID, device.ActiveSessionID, hostID, machineCode)

			// Resend login state to device so WebView recovers if it lost state
			sessionService := NewSessionService()
			reconnData, reconErr := sessionService.GetSessionReconnectData(device.ActiveSessionID)
			if reconErr == nil {
				token, _ := s.GenerateToken(user)
				var planUnitSeconds int
				switch reconnData.PlanUnit {
				case "hour":
					planUnitSeconds = 3600
				default:
					planUnitSeconds = 60
				}
				freePlay := GetRegisteredDeviceService().GetFreePlayByMachineCode(machineCode)
				deviceStateService.SendMessage(hostID, machineCode, "device_state_change", map[string]interface{}{
					"event": "login",
					"state": "logged_in",
					"user": map[string]interface{}{
						"id":         user.ID,
						"nickname":   user.Nickname,
						"avatar_url": user.AvatarURL,
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

			go func() {
				oaService := GetWeChatOAService()
				if oaService == nil {
					return
				}
				message := fmt.Sprintf("您已在该设备上登录，无需重复扫码。\n设备: %s", device.DeviceName)
				if sendErr := oaService.SendCustomerMessage(openID, message); sendErr != nil {
					log.Printf("Warning: Failed to send duplicate-login message to user %s: %v", openID, sendErr)
				}
			}()
			return nil
		}

		// If device is bound to another user, reject
		if device.BoundUserID != "" && device.BoundUserID != user.ID {
			go func() {
				oaService := GetWeChatOAService()
				if oaService == nil {
					return
				}
				message := fmt.Sprintf("该设备正被其他用户使用中，请等待或使用其他设备。\n设备: %s", device.DeviceName)
				if sendErr := oaService.SendCustomerMessage(openID, message); sendErr != nil {
					log.Printf("Warning: Failed to send device-occupied message to user %s: %v", openID, sendErr)
				}
			}()
			return fmt.Errorf("device already bound to another user")
		}

		// Generate JWT token
		token, err := s.GenerateToken(user)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		// Bind user to device
		if err := deviceStateService.BindUser(hostID, machineCode, user.ID); err != nil {
			// Device is bound to another user — send feedback via OA message
			go func() {
				oaService := GetWeChatOAService()
				if oaService == nil {
					return
				}
				message := fmt.Sprintf("该设备正被其他用户使用中，请等待或使用其他设备。\n设备: %s", device.DeviceName)
				if sendErr := oaService.SendCustomerMessage(openID, message); sendErr != nil {
					log.Printf("Warning: Failed to send device-occupied message to user %s: %v", openID, sendErr)
				}
			}()
			return fmt.Errorf("failed to bind device: %w", err)
		}

		// Create/update UserClient record
		clientID, err := userClientService.CreateOrUpdateClientWithHostInfo(user.ID, device.DeviceName, hostID, machineCode)
		if err != nil {
			log.Printf("Warning: Failed to create/update UserClient: %v", err)
		}

		// Start billing session
		sessionService := NewSessionService()
		var sessionID string
		var planID string
		var planRateCents int
		var planUnit string
		var remainingSeconds int
		var otherSessionCount int
		var otherDeviceNames []string

		if clientID != "" {
			result, err := sessionService.StartSessionOnBind(user.ID, clientID)
			if err != nil {
				log.Printf("Warning: Failed to start billing session on OA bind: %v", err)
			} else {
				sessionID = result.ID
				planID = result.PlanID
				planRateCents = result.PlanRateCents
				planUnit = result.PlanUnit
				remainingSeconds = result.RemainingSeconds
				otherSessionCount = result.OtherSessionCount
				otherDeviceNames = result.OtherDeviceNames
				// Track active session on device
				deviceStateService.SetActiveSession(hostID, machineCode, sessionID)
			}
		}

		// Convert plan_unit to plan_unit_seconds
		var planUnitSeconds int
		switch planUnit {
		case "hour":
			planUnitSeconds = 3600
		case "minute":
			planUnitSeconds = 60
		default:
			planUnitSeconds = 60
		}

		// Update device state and push WS message
		deviceStateService.UpdateDeviceState(hostID, machineCode, DeviceStateLoggedIn)

		// Get both balance pools for combined client countdown
		var tenantBalanceMinutes int
		if registeredDevice.TenantID != nil {
			tenantBalanceService := NewTenantBalanceService()
			tb, _ := tenantBalanceService.GetBalance(user.ID, *registeredDevice.TenantID)
			tenantBalanceMinutes = tb
		}

		if err := deviceStateService.SendMessage(hostID, machineCode, "device_state_change", map[string]interface{}{
			"event": "login",
			"state": "logged_in",
			"user": map[string]interface{}{
				"id":         user.ID,
				"nickname":   user.Nickname,
				"avatar_url": user.AvatarURL,
			},
			"token":                  token,
			"balance_cents":          user.GetTotalBalance(),
			"session_id":             sessionID,
			"plan_id":                planID,
			"plan_rate_cents":        planRateCents,
			"plan_unit":              planUnit,
			"plan_unit_seconds":      planUnitSeconds,
			"remaining_seconds":      remainingSeconds,
			"tenant_balance_minutes": tenantBalanceMinutes,
			"global_balance_cents":   user.GetTotalBalance(),
			"other_session_count":    otherSessionCount,
			"other_device_names":     otherDeviceNames,
			"server_ts":              time.Now().Unix(),
		}); err != nil {
			log.Printf("Warning: Failed to send login WS message to device %s:%s: %v", hostID, machineCode, err)
		}

		// Notify sibling devices so they get corrected remaining_seconds and other_session_count
		if otherSessionCount > 0 {
			go (&RechargeService{}).PushBalanceUpdateToActiveDevices(user.ID)
		}

		// Send login success OA message (async, fire-and-forget)
		go func() {
			oaService := GetWeChatOAService()
			if oaService == nil {
				return
			}
			loc, err := time.LoadLocation("Asia/Shanghai")
			if err != nil {
				loc = time.UTC
			}
			timestamp := time.Now().In(loc).Format("2006-01-02 15:04:05")

			message := fmt.Sprintf(
				"登录成功！\n设备: %s\n时间: %s\n\n<a href=\"%s/h5?host_id=%s&machine_code=%s\">点击查看账户信息</a>",
				device.DeviceName,
				timestamp,
				s.baseURL,
				hostID,
				machineCode,
			)
			if err := oaService.SendCustomerMessage(openID, message); err != nil {
				log.Printf("Warning: Failed to send login success message to user %s: %v", user.ID, err)
			}
		}()

		log.Printf("Parametric bind success: user=%s device=%s:%s", user.ID, hostID, machineCode)
	} else {
		// Balance == 0: send OA recharge link, device stays idle.
		// User recharges externally then scans again.
		log.Printf("User %s has zero balance, sending recharge link (device %s:%s stays idle)", user.ID, hostID, machineCode)

		oaService := GetWeChatOAService()
		if oaService != nil {
			rechargeURL := fmt.Sprintf("%s/h5?host_id=%s&machine_code=%s", s.baseURL, hostID, machineCode)
			message := fmt.Sprintf("您的余额不足，请点击下方链接充值后重新扫码登录：\n%s", rechargeURL)
			go func() {
				if err := oaService.SendCustomerMessage(openID, message); err != nil {
					log.Printf("Warning: Failed to send recharge message to user %s: %v", user.ID, err)
				} else {
					log.Printf("Recharge message sent to user %s", user.ID)
				}
			}()
		}
	}

	return nil
}
