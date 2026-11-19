package services

import (
	"crypto/md5"
	"encoding/base64"
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
	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RechargeService 充值服务
type RechargeService struct {
	balanceService *BalanceService
	mchID          string
	mchAPIKey      string
	appID          string
	notifyURL      string
	baseURL        string
}

// WechatPayUnifiedOrderRequest 微信统一下单请求
type WechatPayUnifiedOrderRequest struct {
	XMLName        xml.Name `xml:"xml"`
	AppID          string   `xml:"appid"`
	MchID          string   `xml:"mch_id"`
	NonceStr       string   `xml:"nonce_str"`
	Sign           string   `xml:"sign"`
	Body           string   `xml:"body"`
	OutTradeNo     string   `xml:"out_trade_no"`
	TotalFee       int      `xml:"total_fee"`
	SpbillCreateIP string   `xml:"spbill_create_ip"`
	NotifyURL      string   `xml:"notify_url"`
	TradeType      string   `xml:"trade_type"`
	ProductID      string   `xml:"product_id"`
}

// WechatPayUnifiedOrderResponse 微信统一下单响应
type WechatPayUnifiedOrderResponse struct {
	XMLName    xml.Name `xml:"xml"`
	ReturnCode string   `xml:"return_code"`
	ReturnMsg  string   `xml:"return_msg"`
	ResultCode string   `xml:"result_code"`
	ErrCode    string   `xml:"err_code"`
	ErrCodeDes string   `xml:"err_code_des"`
	PrepayID   string   `xml:"prepay_id"`
	CodeURL    string   `xml:"code_url"`
}

// WechatPayNotify 微信支付回调
type WechatPayNotify struct {
	XMLName       xml.Name `xml:"xml"`
	ReturnCode    string   `xml:"return_code"`
	ReturnMsg     string   `xml:"return_msg"`
	AppID         string   `xml:"appid"`
	MchID         string   `xml:"mch_id"`
	NonceStr      string   `xml:"nonce_str"`
	Sign          string   `xml:"sign"`
	ResultCode    string   `xml:"result_code"`
	OpenID        string   `xml:"openid"`
	IsSubscribe   string   `xml:"is_subscribe"`
	TradeType     string   `xml:"trade_type"`
	BankType      string   `xml:"bank_type"`
	TotalFee      int      `xml:"total_fee"`
	CashFee       string   `xml:"cash_fee"` // CDATA 包装，用 string 解析
	FeeType       string   `xml:"fee_type"`
	TransactionID string   `xml:"transaction_id"`
	OutTradeNo    string   `xml:"out_trade_no"`
	TimeEnd       string   `xml:"time_end"`
}

// CreateOrderResult 创建订单结果
type CreateOrderResult struct {
	OrderNo   string    `json:"orderNo"`
	QRCodeURL string    `json:"qrCodeUrl"`
	Amount    int       `json:"amount"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// WechatPayJSAPIOrderRequest JSAPI 下单请求
type WechatPayJSAPIOrderRequest struct {
	XMLName        xml.Name `xml:"xml"`
	AppID          string   `xml:"appid"`
	MchID          string   `xml:"mch_id"`
	NonceStr       string   `xml:"nonce_str"`
	Sign           string   `xml:"sign"`
	Body           string   `xml:"body"`
	OutTradeNo     string   `xml:"out_trade_no"`
	TotalFee       int      `xml:"total_fee"`
	SpbillCreateIP string   `xml:"spbill_create_ip"`
	NotifyURL      string   `xml:"notify_url"`
	TradeType      string   `xml:"trade_type"`
	OpenID         string   `xml:"openid"`
}

// JSAPIPayParams 返回给前端的 JSAPI 支付参数
type JSAPIPayParams struct {
	AppID     string `json:"app_id"`
	TimeStamp string `json:"time_stamp"`
	NonceStr  string `json:"nonce_str"`
	Package   string `json:"package"`
	SignType  string `json:"sign_type"`
	PaySign   string `json:"pay_sign"`
}

// CreateOrderJSAPIResult JSAPI 订单创建结果
type CreateOrderJSAPIResult struct {
	OrderNo   string          `json:"order_no"`
	Amount    int             `json:"amount"`
	ExpiresAt time.Time       `json:"expires_at"`
	PayParams *JSAPIPayParams `json:"jsapi_params"`
}

// NewRechargeService 创建充值服务
func NewRechargeService(cfg *config.Config) *RechargeService {
	return &RechargeService{
		balanceService: NewBalanceService(),
		mchID:          cfg.WeChat.MchID,
		mchAPIKey:      cfg.WeChat.MchAPIKey,
		appID:          cfg.WeChat.AppID,
		notifyURL:      cfg.WeChat.PayNotifyURL,
		baseURL:        cfg.Server.BaseURL,
	}
}

// GetPackages 获取充值套餐列表（仅活跃）
func (s *RechargeService) GetPackages() ([]models.GlobalRechargePackage, error) {
	var packages []models.GlobalRechargePackage
	if err := database.DB.Where("status = ?", 1).Order("sort_order ASC").Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

// GetPackageByID 根据ID获取套餐
func (s *RechargeService) GetPackageByID(id string) (*models.GlobalRechargePackage, error) {
	var pkg models.GlobalRechargePackage
	if err := database.DB.First(&pkg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

// CreateOrder 创建充值订单
func (s *RechargeService) CreateOrder(userID, packageID, clientIP string) (*CreateOrderResult, error) {
	// 获取套餐信息
	pkg, err := s.GetPackageByID(packageID)
	if err != nil {
		return nil, fmt.Errorf("套餐不存在")
	}

	if pkg.Status != 1 {
		return nil, fmt.Errorf("套餐已下架")
	}

	// 生成订单号
	orderNo := fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	expiresAt := time.Now().Add(10 * time.Minute)

	// 调用微信支付创建预支付订单（返回 weixin:// URL）
	wechatURL, err := s.createWechatPayOrder(orderNo, pkg.Name, pkg.PriceCents, clientIP)
	if err != nil {
		return nil, fmt.Errorf("创建支付订单失败: %w", err)
	}

	// 将微信URL转换为二维码Base64图片
	qrCodeDataURL, err := s.generateQRCodeBase64(wechatURL)
	if err != nil {
		return nil, fmt.Errorf("生成二维码失败: %w", err)
	}

	// 保存订单到数据库（存储原始微信URL）
	order := &models.RechargeOrder{
		ID:               uuid.New().String(),
		UserID:           userID,
		OrderNo:          orderNo,
		PackageID:        pkg.ID,
		PackageName:      pkg.Name,
		AmountCents:      pkg.PriceCents,
		BonusCents:       pkg.BonusCents,
		TotalCreditCents: pkg.PriceCents + pkg.BonusCents,
		Status:           models.RechargeOrderStatusPending,
		QRCodeURL:        wechatURL, // 存储原始URL
		CreatedAt:        time.Now(),
		ExpiresAt:        expiresAt,
	}

	if err := database.DB.Create(order).Error; err != nil {
		return nil, fmt.Errorf("保存订单失败: %w", err)
	}

	return &CreateOrderResult{
		OrderNo:   orderNo,
		QRCodeURL: qrCodeDataURL, // 返回Base64图片给客户端
		Amount:    pkg.PriceCents,
		ExpiresAt: expiresAt,
	}, nil
}

// CreateOrderJSAPI 创建 JSAPI 充值订单
func (s *RechargeService) CreateOrderJSAPI(userID, packageID, openID, clientIP string) (*CreateOrderJSAPIResult, error) {
	// 获取套餐信息
	pkg, err := s.GetPackageByID(packageID)
	if err != nil {
		return nil, fmt.Errorf("套餐不存在")
	}

	if pkg.Status != 1 {
		return nil, fmt.Errorf("套餐已下架")
	}

	if openID == "" {
		return nil, fmt.Errorf("缺少 OpenID")
	}

	// 生成订单号
	orderNo := fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	expiresAt := time.Now().Add(10 * time.Minute)

	// 调用微信支付创建 JSAPI 预支付订单
	prepayID, err := s.createWechatPayOrderJSAPI(orderNo, pkg.Name, pkg.PriceCents, openID, clientIP)
	if err != nil {
		return nil, fmt.Errorf("创建支付订单失败: %w", err)
	}

	// 生成 JSAPI 支付参数
	payParams := s.generateJSAPIPayParams(prepayID)

	// 保存订单到数据库
	order := &models.RechargeOrder{
		ID:               uuid.New().String(),
		UserID:           userID,
		OrderNo:          orderNo,
		PackageID:        pkg.ID,
		PackageName:      pkg.Name,
		AmountCents:      pkg.PriceCents,
		BonusCents:       pkg.BonusCents,
		TotalCreditCents: pkg.PriceCents + pkg.BonusCents,
		Status:           models.RechargeOrderStatusPending,
		QRCodeURL:        "", // JSAPI 不需要二维码
		CreatedAt:        time.Now(),
		ExpiresAt:        expiresAt,
	}

	if err := database.DB.Create(order).Error; err != nil {
		return nil, fmt.Errorf("保存订单失败: %w", err)
	}

	return &CreateOrderJSAPIResult{
		OrderNo:   orderNo,
		Amount:    pkg.PriceCents,
		ExpiresAt: expiresAt,
		PayParams: payParams,
	}, nil
}

// CreateTenantOrder 创建租户套餐订单
func (s *RechargeService) CreateTenantOrder(userID, tenantID, planSetID string, plan *models.BillingPlan, clientIP string) (*CreateOrderResult, error) {
	// Validate plan type
	if plan.Type != "package" {
		return nil, fmt.Errorf("计费方案类型错误")
	}

	// Get the plan set's standard plan for rate snapshot
	var planSet models.PlanSet
	err := database.DB.Preload("StandardPlan").Where("id = ?", planSetID).First(&planSet).Error
	if err != nil || planSet.StandardPlan == nil {
		return nil, fmt.Errorf("计费方案集配置错误")
	}

	// Generate order number (reuse existing logic)
	orderNo := fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	expiresAt := time.Now().Add(30 * time.Minute)

	// Call WeChat Pay to create native order
	wechatURL, err := s.createWechatPayOrder(orderNo, plan.Name, plan.PriceCents, clientIP)
	if err != nil {
		return nil, fmt.Errorf("创建支付订单失败: %w", err)
	}

	// Convert WeChat URL to QR code Base64 image
	qrCodeDataURL, err := s.generateQRCodeBase64(wechatURL)
	if err != nil {
		return nil, fmt.Errorf("生成二维码失败: %w", err)
	}

	// Create order in database
	order := &models.RechargeOrder{
		ID:                   uuid.New().String(),
		UserID:               userID,
		OrderNo:              orderNo,
		PackageID:            plan.ID,
		PackageName:          plan.Name,
		AmountCents:          plan.PriceCents,
		BonusCents:           0,
		TotalCreditCents:     plan.PriceCents,
		Status:               models.RechargeOrderStatusPending,
		Scope:                "tenant",
		TenantID:             &tenantID,
		PlanSetID:            &planSetID,
		PlanID:               &plan.ID,
		StandardRateSnapshot: planSet.StandardPlan.Rate,
		StandardUnitSnapshot: planSet.StandardPlan.Unit,
		MinutesCredited:      plan.GivesMinutes,
		MinutesRemaining:     0, // will be set on payment
		RealCents:            plan.PriceCents,
		RealCentsRemaining:   0,         // not applicable for tenant orders
		QRCodeURL:            wechatURL, // store original WeChat URL
		CreatedAt:            time.Now(),
		ExpiresAt:            expiresAt,
	}

	if err := database.DB.Create(order).Error; err != nil {
		return nil, fmt.Errorf("保存订单失败: %w", err)
	}

	return &CreateOrderResult{
		OrderNo:   orderNo,
		QRCodeURL: qrCodeDataURL, // return Base64 image to client
		Amount:    plan.PriceCents,
		ExpiresAt: expiresAt,
	}, nil
}

// CreateTenantOrderJSAPI 创建租户套餐订单 (JSAPI)
func (s *RechargeService) CreateTenantOrderJSAPI(userID, tenantID, planSetID string, plan *models.BillingPlan, openID, clientIP string) (*CreateOrderJSAPIResult, error) {
	// Validate plan type
	if plan.Type != "package" {
		return nil, fmt.Errorf("计费方案类型错误")
	}

	if openID == "" {
		return nil, fmt.Errorf("缺少 OpenID")
	}

	// Get the plan set's standard plan for rate snapshot
	var planSet models.PlanSet
	err := database.DB.Preload("StandardPlan").Where("id = ?", planSetID).First(&planSet).Error
	if err != nil || planSet.StandardPlan == nil {
		return nil, fmt.Errorf("计费方案集配置错误")
	}

	// Generate order number
	orderNo := fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8])
	expiresAt := time.Now().Add(30 * time.Minute)

	// Call WeChat Pay to create JSAPI prepay order
	prepayID, err := s.createWechatPayOrderJSAPI(orderNo, plan.Name, plan.PriceCents, openID, clientIP)
	if err != nil {
		return nil, fmt.Errorf("创建支付订单失败: %w", err)
	}

	// Generate JSAPI pay parameters
	payParams := s.generateJSAPIPayParams(prepayID)

	// Create order in database
	order := &models.RechargeOrder{
		ID:                   uuid.New().String(),
		UserID:               userID,
		OrderNo:              orderNo,
		PackageID:            plan.ID,
		PackageName:          plan.Name,
		AmountCents:          plan.PriceCents,
		BonusCents:           0,
		TotalCreditCents:     plan.PriceCents,
		Status:               models.RechargeOrderStatusPending,
		Scope:                "tenant",
		TenantID:             &tenantID,
		PlanSetID:            &planSetID,
		PlanID:               &plan.ID,
		StandardRateSnapshot: planSet.StandardPlan.Rate,
		StandardUnitSnapshot: planSet.StandardPlan.Unit,
		MinutesCredited:      plan.GivesMinutes,
		MinutesRemaining:     0, // will be set on payment
		RealCents:            plan.PriceCents,
		RealCentsRemaining:   0,  // not applicable for tenant orders
		QRCodeURL:            "", // JSAPI doesn't need QR code
		CreatedAt:            time.Now(),
		ExpiresAt:            expiresAt,
	}

	if err := database.DB.Create(order).Error; err != nil {
		return nil, fmt.Errorf("保存订单失败: %w", err)
	}

	return &CreateOrderJSAPIResult{
		OrderNo:   orderNo,
		Amount:    plan.PriceCents,
		ExpiresAt: expiresAt,
		PayParams: payParams,
	}, nil
}

// createWechatPayOrderJSAPI 调用微信支付统一下单接口 (JSAPI)
func (s *RechargeService) createWechatPayOrderJSAPI(orderNo, body string, totalFee int, openID, clientIP string) (string, error) {
	nonceStr := uuid.New().String()[:32]

	// 构建请求参数
	params := map[string]string{
		"appid":            s.appID,
		"mch_id":           s.mchID,
		"nonce_str":        nonceStr,
		"body":             body,
		"out_trade_no":     orderNo,
		"total_fee":        fmt.Sprintf("%d", totalFee),
		"spbill_create_ip": clientIP,
		"notify_url":       s.notifyURL,
		"trade_type":       "JSAPI",
		"openid":           openID,
	}

	// 生成签名
	sign := s.generateSign(params)

	// 构建XML请求
	req := &WechatPayJSAPIOrderRequest{
		AppID:          s.appID,
		MchID:          s.mchID,
		NonceStr:       nonceStr,
		Sign:           sign,
		Body:           body,
		OutTradeNo:     orderNo,
		TotalFee:       totalFee,
		SpbillCreateIP: clientIP,
		NotifyURL:      s.notifyURL,
		TradeType:      "JSAPI",
		OpenID:         openID,
	}

	xmlData, err := xml.Marshal(req)
	if err != nil {
		return "", err
	}

	// 发送请求
	resp, err := http.Post(
		"https://api.mch.weixin.qq.com/pay/unifiedorder",
		"application/xml",
		strings.NewReader(string(xmlData)),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var result WechatPayUnifiedOrderResponse
	if err := xml.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("微信支付响应解析失败: %s, 原始响应: %s\n", err.Error(), string(respBody))
		return "", err
	}

	fmt.Printf("微信支付JSAPI响应: ReturnCode=%s, ReturnMsg=%s, ResultCode=%s, ErrCode=%s, ErrCodeDes=%s, PrepayID=%s\n",
		result.ReturnCode, result.ReturnMsg, result.ResultCode, result.ErrCode, result.ErrCodeDes, result.PrepayID)

	if result.ReturnCode != "SUCCESS" {
		return "", fmt.Errorf("微信支付错误: %s", result.ReturnMsg)
	}

	if result.ResultCode != "SUCCESS" {
		return "", fmt.Errorf("微信支付错误: %s - %s", result.ErrCode, result.ErrCodeDes)
	}

	return result.PrepayID, nil
}

// generateJSAPIPayParams 生成 JSAPI 支付参数
func (s *RechargeService) generateJSAPIPayParams(prepayID string) *JSAPIPayParams {
	timeStamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := uuid.New().String()[:32]
	packageStr := fmt.Sprintf("prepay_id=%s", prepayID)

	// 构建签名参数
	params := map[string]string{
		"appId":     s.appID,
		"timeStamp": timeStamp,
		"nonceStr":  nonceStr,
		"package":   packageStr,
		"signType":  "MD5",
	}

	// 生成签名
	paySign := s.generateSign(params)

	return &JSAPIPayParams{
		AppID:     s.appID,
		TimeStamp: timeStamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "MD5",
		PaySign:   paySign,
	}
}

// generateQRCodeBase64 生成二维码Base64图片
func (s *RechargeService) generateQRCodeBase64(content string) (string, error) {
	// 生成256x256的二维码PNG图片
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}

	// 转换为Base64 Data URL
	base64Str := base64.StdEncoding.EncodeToString(png)
	dataURL := fmt.Sprintf("data:image/png;base64,%s", base64Str)

	return dataURL, nil
}

// createWechatPayOrder 调用微信支付统一下单接口
func (s *RechargeService) createWechatPayOrder(orderNo, body string, totalFee int, clientIP string) (string, error) {
	nonceStr := uuid.New().String()[:32]

	// 构建请求参数
	params := map[string]string{
		"appid":            s.appID,
		"mch_id":           s.mchID,
		"nonce_str":        nonceStr,
		"body":             body,
		"out_trade_no":     orderNo,
		"total_fee":        fmt.Sprintf("%d", totalFee),
		"spbill_create_ip": clientIP,
		"notify_url":       s.notifyURL,
		"trade_type":       "NATIVE",
		"product_id":       orderNo,
	}

	// 生成签名
	sign := s.generateSign(params)

	// 构建XML请求
	req := &WechatPayUnifiedOrderRequest{
		AppID:          s.appID,
		MchID:          s.mchID,
		NonceStr:       nonceStr,
		Sign:           sign,
		Body:           body,
		OutTradeNo:     orderNo,
		TotalFee:       totalFee,
		SpbillCreateIP: clientIP,
		NotifyURL:      s.notifyURL,
		TradeType:      "NATIVE",
		ProductID:      orderNo,
	}

	xmlData, err := xml.Marshal(req)
	if err != nil {
		return "", err
	}

	// 发送请求
	resp, err := http.Post(
		"https://api.mch.weixin.qq.com/pay/unifiedorder",
		"application/xml",
		strings.NewReader(string(xmlData)),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var result WechatPayUnifiedOrderResponse
	if err := xml.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("微信支付响应解析失败: %s, 原始响应: %s\n", err.Error(), string(respBody))
		return "", err
	}

	fmt.Printf("微信支付响应: ReturnCode=%s, ReturnMsg=%s, ResultCode=%s, ErrCode=%s, ErrCodeDes=%s\n",
		result.ReturnCode, result.ReturnMsg, result.ResultCode, result.ErrCode, result.ErrCodeDes)

	if result.ReturnCode != "SUCCESS" {
		return "", fmt.Errorf("微信支付错误: %s", result.ReturnMsg)
	}

	if result.ResultCode != "SUCCESS" {
		return "", fmt.Errorf("微信支付错误: %s - %s", result.ErrCode, result.ErrCodeDes)
	}

	return result.CodeURL, nil
}

// PollOrderStatus 轮询订单状态
func (s *RechargeService) PollOrderStatus(orderNo string) (*models.RechargeOrder, error) {
	var order models.RechargeOrder
	if err := database.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// ProcessPaymentCallback 处理微信支付回调
func (s *RechargeService) ProcessPaymentCallback(notify *WechatPayNotify) error {
	// 验证签名
	if !s.verifyNotifySign(notify) {
		return fmt.Errorf("签名验证失败")
	}

	// Atomically lock order row, check status, and mark paid to prevent double crediting
	var order models.RechargeOrder
	var alreadyProcessed bool

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", notify.OutTradeNo).First(&order).Error; err != nil {
			return fmt.Errorf("订单不存在: %w", err)
		}

		if order.Status != models.RechargeOrderStatusPending {
			alreadyProcessed = true
			return nil
		}

		if notify.TotalFee != order.AmountCents {
			return fmt.Errorf("金额不匹配")
		}

		now := time.Now()
		order.Status = models.RechargeOrderStatusPaid
		order.WxTransactionID = notify.TransactionID
		order.PaidAt = &now
		return tx.Save(&order).Error
	}); err != nil {
		return err
	}

	if alreadyProcessed {
		return nil
	}

	// Handle balance crediting based on order scope
	if order.Scope == "tenant" {
		// Tenant package: add minutes to tenant balance
		if order.TenantID == nil {
			return fmt.Errorf("tenant order missing tenant_id")
		}
		tenantBalanceService := NewTenantBalanceService()

		// Begin transaction for tenant balance update
		tx := database.DB.Begin()
		err := tenantBalanceService.AddMinutes(tx, order.UserID, *order.TenantID, order.MinutesCredited, order.ID)
		if err != nil {
			tx.Rollback()
			log.Printf("Error adding tenant minutes: %v", err)
			return fmt.Errorf("增加租户时间失败: %w", err)
		}

		// Set minutes_remaining on the order
		if err := tx.Model(&order).Update("minutes_remaining", order.MinutesCredited).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("更新订单minutes_remaining失败: %w", err)
		}

		// Create balance_log entry for tenant recharge so it appears in admin Balance History
		balanceLog := models.BalanceLog{
			ID:           uuid.New().String(),
			UserID:       order.UserID,
			Type:         models.BalanceLogTypeRecharge,
			Scope:        "tenant",
			TenantID:     order.TenantID,
			AmountCents:  order.AmountCents,
			DeltaMinutes: order.MinutesCredited,
			OrderID:      &order.ID,
			Remark:       fmt.Sprintf("租户充值%d分钟", order.MinutesCredited),
			CreatedAt:    time.Now(),
		}
		if err := tx.Create(&balanceLog).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("创建余额日志失败: %w", err)
		}

		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("提交事务失败: %w", err)
		}

		// Push balance_update to client if there's an active session
		s.PushBalanceUpdateToActiveDevices(order.UserID)
	} else {
		// Global package: add real + bonus balance
		if err := s.balanceService.AddBalanceSplit(
			order.UserID,
			order.AmountCents,
			order.BonusCents,
			&order.ID,
			models.BalanceLogTypeRecharge,
			fmt.Sprintf("充值%d分（含赠送%d分）", order.TotalCreditCents, order.BonusCents),
		); err != nil {
			return fmt.Errorf("增加余额失败: %w", err)
		}

		// Set real_cents and real_cents_remaining on the order
		database.DB.Model(&order).Updates(map[string]interface{}{
			"scope":                "global",
			"real_cents":           order.AmountCents,
			"real_cents_remaining": order.AmountCents,
		})

		// Push balance_update to client if there's an active session
		s.PushBalanceUpdateToActiveDevices(order.UserID)
	}

	// 更新用户累计充值金额
	database.DB.Model(&models.User{}).Where("id = ?", order.UserID).
		UpdateColumn("total_recharge_amount", database.DB.Raw("total_recharge_amount + ?", order.AmountCents))

	return nil
}

// generateSign 生成签名
func (s *RechargeService) generateSign(params map[string]string) string {
	// 按键名排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接字符串
	var buf strings.Builder
	for _, k := range keys {
		if params[k] != "" {
			buf.WriteString(k)
			buf.WriteString("=")
			buf.WriteString(params[k])
			buf.WriteString("&")
		}
	}
	buf.WriteString("key=")
	buf.WriteString(s.mchAPIKey)

	// 调试：打印待签名字符串（隐藏 key）
	debugStr := buf.String()
	keyIdx := strings.LastIndex(debugStr, "&key=")
	if keyIdx > 0 {
		fmt.Printf("[WeChat Callback] 待签名字符串: %s&key=***\n", debugStr[:keyIdx])
	}

	// MD5加密
	hash := md5.Sum([]byte(buf.String()))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// verifyNotifySign 验证回调签名
func (s *RechargeService) verifyNotifySign(notify *WechatPayNotify) bool {
	params := map[string]string{
		"appid":          notify.AppID,
		"mch_id":         notify.MchID,
		"nonce_str":      notify.NonceStr,
		"result_code":    notify.ResultCode,
		"return_code":    notify.ReturnCode,
		"openid":         notify.OpenID,
		"is_subscribe":   notify.IsSubscribe,
		"trade_type":     notify.TradeType,
		"bank_type":      notify.BankType,
		"total_fee":      fmt.Sprintf("%d", notify.TotalFee),
		"cash_fee":       notify.CashFee, // 已经是 string
		"fee_type":       notify.FeeType,
		"transaction_id": notify.TransactionID,
		"out_trade_no":   notify.OutTradeNo,
		"time_end":       notify.TimeEnd,
	}

	// 调试：打印所有参数
	fmt.Printf("[WeChat Callback] 签名参数:\n")
	for k, v := range params {
		fmt.Printf("  %s = %s\n", k, v)
	}

	sign := s.generateSign(params)
	fmt.Printf("[WeChat Callback] 签名验证: 计算签名=%s, 回调签名=%s\n", sign, notify.Sign)
	return sign == notify.Sign
}

// CancelExpiredOrders 取消过期订单
func (s *RechargeService) CancelExpiredOrders() error {
	return database.DB.Model(&models.RechargeOrder{}).
		Where("status = ? AND expires_at < ?", models.RechargeOrderStatusPending, time.Now()).
		Update("status", models.RechargeOrderStatusCancelled).Error
}

// GetUserOrders 获取用户订单列表
func (s *RechargeService) GetUserOrders(userID string, page, pageSize int) ([]models.RechargeOrder, int64, error) {
	var orders []models.RechargeOrder
	var total int64

	query := database.DB.Model(&models.RechargeOrder{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// --- 管理端方法 ---

// GetAllOrders 获取所有订单（管理端）
func (s *RechargeService) GetAllOrders(page, pageSize int, status *int) ([]models.RechargeOrder, int64, error) {
	var orders []models.RechargeOrder
	var total int64

	query := database.DB.Model(&models.RechargeOrder{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// GetAllOrdersV2 获取所有订单（管理端，支持更多过滤条件）
func (s *RechargeService) GetAllOrdersV2(page, limit int, search, userID string, status *int, dateFrom, dateTo string) ([]models.RechargeOrderWithUser, int64, error) {
	var orders []models.RechargeOrderWithUser
	var total int64

	query := database.DB.Table("recharge_orders").
		Select("recharge_orders.*, users.nickname as user_nickname, users.avatar_url as user_avatar_url, users.status as user_status").
		Joins("LEFT JOIN users ON users.id = recharge_orders.user_id")

	if status != nil {
		query = query.Where("recharge_orders.status = ?", *status)
	}

	if userID != "" {
		query = query.Where("recharge_orders.user_id = ?", userID)
	}

	if search != "" {
		query = query.Where("recharge_orders.order_no LIKE ? OR recharge_orders.package_name LIKE ? OR users.nickname LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if dateFrom != "" {
		query = query.Where("recharge_orders.created_at >= ?", dateFrom)
	}

	if dateTo != "" {
		query = query.Where("recharge_orders.created_at <= ?", dateTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if err := query.Order("recharge_orders.created_at DESC").Offset(offset).Limit(limit).Scan(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// GetOrderByID 根据ID获取订单
func (s *RechargeService) GetOrderByID(id string) (*models.RechargeOrderWithUser, error) {
	var order models.RechargeOrderWithUser
	if err := database.DB.Table("recharge_orders").
		Select("recharge_orders.*, users.nickname as user_nickname, users.avatar_url as user_avatar_url, users.status as user_status").
		Joins("LEFT JOIN users ON users.id = recharge_orders.user_id").
		Where("recharge_orders.id = ? OR recharge_orders.order_no = ?", id, id).
		Scan(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// PushBalanceUpdateToActiveDevices pushes balance_correction to all devices with active sessions for a user
func (s *RechargeService) PushBalanceUpdateToActiveDevices(userID string) {
	deviceStateService := GetDeviceStateService()
	devices := deviceStateService.GetDevicesByUser(userID)

	// Get user's current balance
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("Failed to get user balance for push: %v", err)
		return
	}

	// Count total active devices for this user (for other_session_count)
	totalActive := 0
	for _, device := range devices {
		if device.ActiveSessionID != "" {
			totalActive++
		}
	}

	sessionService := NewSessionService()

	for _, device := range devices {
		if device.ActiveSessionID == "" {
			continue
		}

		// Fetch actual session data for accurate balance info
		reconnData, err := sessionService.GetSessionReconnectData(device.ActiveSessionID)
		if err != nil {
			// Fallback: send basic balance with recharge flag
			var fallbackTenantMinutes int
			if device.TenantID != "" {
				tenantBalanceService := NewTenantBalanceService()
				tb, tbErr := tenantBalanceService.GetBalance(userID, device.TenantID)
				if tbErr == nil {
					fallbackTenantMinutes = tb
				}
			}
			deviceStateService.SendMessage(device.HostID, device.MachineCode, "device_state_change", map[string]interface{}{
				"event":                  "balance_correction",
				"state":                  "logged_in",
				"balance_cents":          user.BalanceCents + user.BonusBalanceCents,
				"tenant_balance_minutes": fallbackTenantMinutes,
				"global_balance_cents":   user.BalanceCents + user.BonusBalanceCents,
				"other_session_count":    totalActive - 1,
				"server_ts":              time.Now().Unix(),
			})
			continue
		}

		// Convert plan_unit to plan_unit_seconds
		var planUnitSeconds int
		if reconnData.PlanUnit == "hour" {
			planUnitSeconds = 3600
		} else if reconnData.PlanUnit == "minute" {
			planUnitSeconds = 60
		} else {
			planUnitSeconds = 60
		}

		deviceStateService.SendMessage(device.HostID, device.MachineCode, "device_state_change", map[string]interface{}{
			"event":                  "balance_correction",
			"state":                  "logged_in",
			"balance_cents":          reconnData.BalanceCents,
			"remaining_seconds":      reconnData.RemainingSeconds,
			"balance_source":         reconnData.BalanceSource,
			"plan_rate_cents":        reconnData.PlanRateCents,
			"plan_unit":              reconnData.PlanUnit,
			"plan_unit_seconds":      planUnitSeconds,
			"tenant_balance_minutes": reconnData.TenantBalanceMinutes,
			"global_balance_cents":   reconnData.GlobalBalanceCents,
			"other_session_count":    reconnData.OtherSessionCount,
			"server_ts":              time.Now().Unix(),
		})

		// Reschedule the server-side one-shot expiry timer against the
		// post-recharge remaining time now, instead of leaving the stale
		// pre-recharge timer armed until the next ~60s billing tick catches
		// up -- otherwise a session can still get force-ended right after
		// the user paid to extend it.
		sessionService.scheduleSessionExpiryForDevice(device.HostID, device.MachineCode, device.ActiveSessionID, userID, reconnData.RemainingSeconds)
	}
}
