package client

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"appdeck/config"
	"appdeck/database"
	"appdeck/middleware"
	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// RechargeHandler 充值处理器
type RechargeHandler struct {
	rechargeService *services.RechargeService
	authService     *services.AuthService
	planSetService  *services.PlanSetService
	wechatToken     string
}

// NewRechargeHandler 创建充值处理器
func NewRechargeHandler(cfg *config.Config) *RechargeHandler {
	return &RechargeHandler{
		rechargeService: services.NewRechargeService(cfg),
		authService:     services.NewAuthService(cfg),
		planSetService:  services.NewPlanSetService(),
		wechatToken:     cfg.WeChat.Token,
	}
}

// GetPackages 获取充值套餐列表
// GET /api/v1/recharge/packages
func (h *RechargeHandler) GetPackages(c *gin.Context) {
	packages, err := h.rechargeService.GetPackages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取套餐失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    packages,
	})
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	PackageID string `json:"packageId" binding:"required"`
}

// CreateOrderJSAPIRequest JSAPI 创建订单请求
type CreateOrderJSAPIRequest struct {
	PackageID   string `json:"package_id" binding:"required"`
	MachineCode string `json:"machine_code"` // 扫码入口时传入，用于识别租户套餐
}

// CreateOrder 创建充值订单
// POST /api/v1/recharge/create
func (h *RechargeHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请选择充值套餐",
			"error":   err.Error(),
		})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	// 获取客户端IP
	clientIP := c.ClientIP()

	result, err := h.rechargeService.CreateOrder(userID, req.PackageID, clientIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// CreateOrderJSAPI 创建 JSAPI 充值订单
// POST /api/v1/recharge/create-h5
func (h *RechargeHandler) CreateOrderJSAPI(c *gin.Context) {
	var req CreateOrderJSAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请选择充值套餐",
			"error":   err.Error(),
		})
		return
	}

	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	// 从 JWT context 获取 OpenID（middleware 已设置）
	openID, _ := c.Get("openID")
	openIDStr, _ := openID.(string)
	if openIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 OpenID，请重新登录",
		})
		return
	}

	// 获取客户端IP
	clientIP := c.ClientIP()

	// 如果有 machine_code，尝试作为租户套餐订单处理
	if req.MachineCode != "" {
		var device models.RegisteredDevice
		if err := database.DB.First(&device, "machine_code = ?", req.MachineCode).Error; err == nil && device.TenantID != nil {
			// 解析计费方案集
			planSet, err := h.planSetService.ResolvePlanSetForTenant(*device.TenantID)
			if err == nil {
				// 在方案集中查找匹配的 package 类型计划
				for _, plan := range planSet.Plans {
					if plan.ID == req.PackageID && plan.Type == "package" && plan.Status == 1 {
						result, err := h.rechargeService.CreateTenantOrderJSAPI(userID, *device.TenantID, planSet.ID, &plan, openIDStr, clientIP)
						if err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{
								"code":    500,
								"message": err.Error(),
							})
							return
						}
						c.JSON(http.StatusOK, gin.H{
							"code":    0,
							"message": "success",
							"data":    result,
						})
						return
					}
				}
			}
		}
		// 未找到匹配的租户套餐，继续尝试全局套餐
	}

	result, err := h.rechargeService.CreateOrderJSAPI(userID, req.PackageID, openIDStr, clientIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// PollOrder 轮询订单状态
// GET /api/v1/recharge/poll/:order_no
func (h *RechargeHandler) PollOrder(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少订单号",
		})
		return
	}

	order, err := h.rechargeService.PollOrderStatus(orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "订单不存在",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"orderNo":   order.OrderNo,
			"status":    order.Status,
			"isPaid":    order.IsPaid(),
			"isExpired": order.IsExpired(),
			"paidAt":    order.PaidAt,
		},
	})
}

// UnifiedWechatMessage 微信统一消息（事件推送+支付回调）
type UnifiedWechatMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
	// 支付回调字段
	ReturnCode    string `xml:"return_code"`
	ReturnMsg     string `xml:"return_msg"`
	ResultCode    string `xml:"result_code"`
	OpenID        string `xml:"openid"`
	IsSubscribe   string `xml:"is_subscribe"`
	TradeType     string `xml:"trade_type"`
	BankType      string `xml:"bank_type"`
	TotalFee      int    `xml:"total_fee"`
	CashFee       string `xml:"cash_fee"`
	FeeType       string `xml:"fee_type"`
	TransactionID string `xml:"transaction_id"`
	OutTradeNo    string `xml:"out_trade_no"`
	TimeEnd       string `xml:"time_end"`
	MchID         string `xml:"mch_id"`
	AppID         string `xml:"appid"`
	NonceStr      string `xml:"nonce_str"`
	Sign          string `xml:"sign"`
}

// WechatPayCallback 微信统一回调（处理事件推送和支付回调）
// POST /api/v1/recharge/callback
func (h *RechargeHandler) WechatPayCallback(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("[WeChat Callback] 读取请求体失败: %v\n", err)
		h.respondToWechat(c, "FAIL", "读取请求失败")
		return
	}

	fmt.Printf("[WeChat Callback] 收到请求: %s\n", string(body))

	var msg UnifiedWechatMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		fmt.Printf("[WeChat Callback] XML解析失败: %v\n", err)
		h.respondToWechat(c, "FAIL", "解析XML失败")
		return
	}

	// 判断是支付回调还是事件推送
	if msg.ReturnCode != "" {
		// 支付回调
		fmt.Printf("[WeChat Callback] 处理支付回调: OrderNo=%s, ReturnCode=%s\n", msg.OutTradeNo, msg.ReturnCode)
		h.handlePaymentCallback(&msg, c)
		return
	}

	if msg.MsgType == "event" {
		// 事件推送（QR扫码登录）
		fmt.Printf("[WeChat Callback] 处理事件推送: Event=%s, EventKey=%s, FromUser=%s\n", msg.Event, msg.EventKey, msg.FromUserName)
		h.handleEventCallback(&msg, c)
		return
	}

	fmt.Printf("[WeChat Callback] 未知消息类型: MsgType=%s\n", msg.MsgType)
	c.String(http.StatusOK, "success")
}

// handlePaymentCallback 处理支付回调
func (h *RechargeHandler) handlePaymentCallback(msg *UnifiedWechatMessage, c *gin.Context) {
	// 检查返回状态
	if msg.ReturnCode != "SUCCESS" || msg.ResultCode != "SUCCESS" {
		h.respondToWechat(c, "SUCCESS", "OK")
		return
	}

	// 转换为支付通知结构
	notify := &services.WechatPayNotify{
		ReturnCode:    msg.ReturnCode,
		ReturnMsg:     msg.ReturnMsg,
		AppID:         msg.AppID,
		MchID:         msg.MchID,
		NonceStr:      msg.NonceStr,
		Sign:          msg.Sign,
		ResultCode:    msg.ResultCode,
		OpenID:        msg.OpenID,
		IsSubscribe:   msg.IsSubscribe,
		TradeType:     msg.TradeType,
		BankType:      msg.BankType,
		TotalFee:      msg.TotalFee,
		CashFee:       msg.CashFee,
		FeeType:       msg.FeeType,
		TransactionID: msg.TransactionID,
		OutTradeNo:    msg.OutTradeNo,
		TimeEnd:       msg.TimeEnd,
	}

	// 处理支付回调
	if err := h.rechargeService.ProcessPaymentCallback(notify); err != nil {
		fmt.Printf("[WeChat Callback] 支付回调处理失败: %v\n", err)
		h.respondToWechat(c, "FAIL", err.Error())
		return
	}

	fmt.Printf("[WeChat Callback] 支付回调处理成功: OrderNo=%s\n", msg.OutTradeNo)
	h.respondToWechat(c, "SUCCESS", "OK")
}

// handleEventCallback 处理事件回调（QR扫码登录）
func (h *RechargeHandler) handleEventCallback(msg *UnifiedWechatMessage, c *gin.Context) {
	// 验证签名
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	if !h.verifySignature(signature, timestamp, nonce) {
		fmt.Printf("[WeChat Callback] 签名验证失败\n")
		c.String(http.StatusForbidden, "signature verification failed")
		return
	}

	// 转换为服务层消息结构
	wechatMsg := &services.WechatMessage{
		ToUserName:   msg.ToUserName,
		FromUserName: msg.FromUserName,
		CreateTime:   msg.CreateTime,
		MsgType:      msg.MsgType,
		Event:        msg.Event,
		EventKey:     msg.EventKey,
		Ticket:       msg.Ticket,
	}

	// 处理事件
	if err := h.authService.ProcessWechatEvent(wechatMsg); err != nil {
		fmt.Printf("[WeChat Callback] 事件处理失败: %v\n", err)
		// 即使失败也返回success，避免微信重试
		c.String(http.StatusOK, "success")
		return
	}

	fmt.Printf("[WeChat Callback] 事件处理成功: Event=%s, EventKey=%s\n", msg.Event, msg.EventKey)
	c.String(http.StatusOK, "success")
}

// respondToWechat 响应微信
func (h *RechargeHandler) respondToWechat(c *gin.Context, code, msg string) {
	type WechatResponse struct {
		XMLName    xml.Name `xml:"xml"`
		ReturnCode string   `xml:"return_code"`
		ReturnMsg  string   `xml:"return_msg"`
	}

	resp := WechatResponse{
		ReturnCode: code,
		ReturnMsg:  msg,
	}

	xmlData, _ := xml.Marshal(resp)
	c.Data(http.StatusOK, "application/xml", xmlData)
}

// WechatServerVerify 微信服务器验证
// GET /api/v1/recharge/callback
func (h *RechargeHandler) WechatServerVerify(c *gin.Context) {
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

// verifySignature 验证签名
func (h *RechargeHandler) verifySignature(signature, timestamp, nonce string) bool {
	arr := []string{h.wechatToken, timestamp, nonce}
	sort.Strings(arr)
	str := strings.Join(arr, "")

	hash := sha1.New()
	hash.Write([]byte(str))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	return hashStr == signature
}

// GetOrders 获取用户订单列表
// GET /api/v1/recharge/orders
func (h *RechargeHandler) GetOrders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}

	page := 1
	pageSize := 20

	orders, total, err := h.rechargeService.GetUserOrders(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取订单失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"orders": orders,
			"total":  total,
		},
	})
}

// GetDevicePlanInfo 获取设备所属商户的计费方案信息
// GET /api/v1/device/plan-info?machine_code=Y
func (h *RechargeHandler) GetDevicePlanInfo(c *gin.Context) {
	machineCode := c.Query("machine_code")
	if machineCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 machine_code 参数",
		})
		return
	}

	// Look up device
	var device models.RegisteredDevice
	if err := database.DB.First(&device, "machine_code = ?", machineCode).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备未找到",
		})
		return
	}

	if device.TenantID == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备未关联商户",
		})
		return
	}

	// Look up tenant
	var tenant models.Tenant
	if err := database.DB.First(&tenant, "id = ?", *device.TenantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "商户未找到",
		})
		return
	}

	// Resolve plan set
	planSet, err := h.planSetService.ResolvePlanSetForTenant(*device.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取计费方案失败",
		})
		return
	}

	// Build response
	rateCents := 0
	rateUnit := "minute"
	if planSet.GlobalRatePlan != nil {
		rateCents = planSet.GlobalRatePlan.Rate
		rateUnit = planSet.GlobalRatePlan.Unit
	}

	unitDisplay := "分钟"
	if rateUnit == "hour" {
		unitDisplay = "小时"
	}

	diamonds := rateCents * 100
	priceYuan := float64(rateCents) / 100.0
	rateDisplay := fmt.Sprintf("%d钻石/%s (¥%.2f/%s)", diamonds, unitDisplay, priceYuan, unitDisplay)

	tenantName := tenant.DisplayName
	if tenantName == "" {
		tenantName = tenant.Username
	}

	// Build plans list
	plans := []gin.H{}
	for _, plan := range planSet.Plans {
		if plan.Status != 1 {
			continue
		}
		plans = append(plans, gin.H{
			"id":            plan.ID,
			"name":          plan.Name,
			"type":          plan.Type,
			"rate_cents":    plan.Rate,
			"unit":          plan.Unit,
			"price_cents":   plan.PriceCents,
			"gives_minutes": plan.GivesMinutes,
			"description":   plan.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"tenant_name":  tenantName,
			"plan_name":    planSet.Name,
			"rate_cents":   rateCents,
			"rate_unit":    rateUnit,
			"rate_display": rateDisplay,
			"plans":        plans,
		},
	})
}
