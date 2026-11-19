package services

import (
	"appdeck/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WeChatOAService WeChat Official Account service
// Manages OA access token with caching and provides follow check & customer service message APIs
type WeChatOAService struct {
	oaAppID        string
	oaAppSecret    string
	qrcodeURL      string // QR code image URL for following the Official Account
	accessToken    string
	tokenExpiresAt time.Time
	mu             sync.RWMutex
}

// WeChatOAAccessTokenResponse OA access token response
type WeChatOAAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// WeChatOAUserInfo OA user info response (from cgi-bin/user/info)
type WeChatOAUserInfo struct {
	Subscribe     int    `json:"subscribe"`      // 0=not followed, 1=followed
	OpenID        string `json:"openid"`
	Nickname      string `json:"nickname"`
	Sex           int    `json:"sex"`
	Language      string `json:"language"`
	City          string `json:"city"`
	Province      string `json:"province"`
	Country       string `json:"country"`
	HeadImgURL    string `json:"headimgurl"`
	SubscribeTime int64  `json:"subscribe_time"`
	UnionID       string `json:"unionid"`
	Remark        string `json:"remark"`
	GroupID       int    `json:"groupid"`
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
}

// WeChatCustomerMessageRequest customer service message request
type WeChatCustomerMessageRequest struct {
	ToUser  string                `json:"touser"`
	MsgType string                `json:"msgtype"`
	Text    *WeChatTextMessageContent `json:"text,omitempty"`
}

// WeChatTextMessageContent text message content
type WeChatTextMessageContent struct {
	Content string `json:"content"`
}

// WeChatCustomerMessageResponse customer service message response
type WeChatCustomerMessageResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// WeChatQRCodeCreateRequest parametric QR code creation request
type WeChatQRCodeCreateRequest struct {
	ExpireSeconds int                      `json:"expire_seconds"`
	ActionName    string                   `json:"action_name"`
	ActionInfo    WeChatQRCodeActionInfo   `json:"action_info"`
}

// WeChatQRCodeActionInfo QR code action info
type WeChatQRCodeActionInfo struct {
	Scene WeChatQRCodeScene `json:"scene"`
}

// WeChatQRCodeScene QR code scene
type WeChatQRCodeScene struct {
	SceneStr string `json:"scene_str"`
}

// WeChatQRCodeCreateResponse QR code creation response
type WeChatQRCodeCreateResponse struct {
	Ticket    string `json:"ticket"`
	ExpireSeconds int `json:"expire_seconds"`
	URL       string `json:"url"`
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
}

var wechatOAServiceInstance *WeChatOAService
var wechatOAServiceOnce sync.Once

// NewWeChatOAService creates WeChat OA service singleton
func NewWeChatOAService(cfg *config.WeChatConfig) *WeChatOAService {
	wechatOAServiceOnce.Do(func() {
		wechatOAServiceInstance = &WeChatOAService{
			oaAppID:     cfg.OAAppID,
			oaAppSecret: cfg.OAAppSecret,
			qrcodeURL:   cfg.OAFollowURL,
		}
	})
	return wechatOAServiceInstance
}

// GetWeChatOAService returns the WeChat OA service singleton
func GetWeChatOAService() *WeChatOAService {
	return wechatOAServiceInstance
}

// GetOAAccessToken gets OA access token with caching (TTL ~7000s, auto-refresh)
func (s *WeChatOAService) GetOAAccessToken() (string, error) {
	// Read lock to check cache
	s.mu.RLock()
	if s.accessToken != "" && time.Now().Before(s.tokenExpiresAt) {
		token := s.accessToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	// Write lock to refresh token
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock (prevent concurrent refresh)
	if s.accessToken != "" && time.Now().Before(s.tokenExpiresAt) {
		return s.accessToken, nil
	}

	// Fetch new token
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		s.oaAppID, s.oaAppSecret,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get OA access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OA access token response: %w", err)
	}

	var result WeChatOAAccessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse OA access token response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat OA API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	// Cache token with 7000s TTL (7200s expiry - 200s buffer)
	s.accessToken = result.AccessToken
	s.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-200) * time.Second)

	return s.accessToken, nil
}

// InvalidateToken clears the cached token, forcing a refresh on next call
func (s *WeChatOAService) InvalidateToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ""
	s.tokenExpiresAt = time.Time{}
}

// isTokenInvalidErr returns true if the error code indicates an invalid/expired token
func isTokenInvalidErr(errCode int) bool {
	// 40001: invalid credential / access_token invalid or not latest
	// 42001: access_token expired
	return errCode == 40001 || errCode == 42001
}

// CheckFollow checks if user follows the Official Account
// Returns (followed bool, error)
func (s *WeChatOAService) CheckFollow(openid string) (bool, error) {
	return s.checkFollowWithRetry(openid, true)
}

func (s *WeChatOAService) checkFollowWithRetry(openid string, canRetry bool) (bool, error) {
	token, err := s.GetOAAccessToken()
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/user/info?access_token=%s&openid=%s&lang=zh_CN",
		token, openid,
	)

	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read user info response: %w", err)
	}

	var result WeChatOAUserInfo
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("failed to parse user info response: %w", err)
	}

	if result.ErrCode != 0 {
		if canRetry && isTokenInvalidErr(result.ErrCode) {
			log.Printf("OA access token invalid (errcode=%d), refreshing and retrying CheckFollow", result.ErrCode)
			s.InvalidateToken()
			return s.checkFollowWithRetry(openid, false)
		}
		return false, fmt.Errorf("wechat OA user/info API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	// subscribe == 1 means followed, 0 means not followed
	return result.Subscribe == 1, nil
}

// SendCustomerMessage sends text message via WeChat Customer Service Message API
// This is async/fire-and-forget - errors are logged but not returned
func (s *WeChatOAService) SendCustomerMessage(openid string, content string) error {
	return s.sendCustomerMessageWithRetry(openid, content, true)
}

func (s *WeChatOAService) sendCustomerMessageWithRetry(openid string, content string, canRetry bool) error {
	token, err := s.GetOAAccessToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s",
		token,
	)

	reqBody := WeChatCustomerMessageRequest{
		ToUser:  openid,
		MsgType: "text",
		Text: &WeChatTextMessageContent{
			Content: content,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal customer message request: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to send customer message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read customer message response: %w", err)
	}

	var result WeChatCustomerMessageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse customer message response: %w", err)
	}

	if result.ErrCode != 0 {
		if canRetry && isTokenInvalidErr(result.ErrCode) {
			log.Printf("OA access token invalid (errcode=%d), refreshing and retrying SendCustomerMessage", result.ErrCode)
			s.InvalidateToken()
			return s.sendCustomerMessageWithRetry(openid, content, false)
		}
		return fmt.Errorf("wechat customer message API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// GetFollowURL returns the configured QR code image URL for following the Official Account
func (s *WeChatOAService) GetFollowURL() string {
	return s.qrcodeURL
}

// CreateParametricQR creates a parametric QR code with scene string
// Returns the QR image URL (https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket={url_encoded_ticket})
func (s *WeChatOAService) CreateParametricQR(sceneStr string) (string, error) {
	return s.createParametricQRWithRetry(sceneStr, true)
}

func (s *WeChatOAService) createParametricQRWithRetry(sceneStr string, canRetry bool) (string, error) {
	token, err := s.GetOAAccessToken()
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=%s",
		token,
	)

	reqBody := WeChatQRCodeCreateRequest{
		ExpireSeconds: 300, // 5 min — matches ±5min timestamp validation
		ActionName:    "QR_STR_SCENE",
		ActionInfo: WeChatQRCodeActionInfo{
			Scene: WeChatQRCodeScene{
				SceneStr: sceneStr,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal QR create request: %w", err)
	}

	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create parametric QR: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read QR create response: %w", err)
	}

	var result WeChatQRCodeCreateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse QR create response: %w", err)
	}

	if result.ErrCode != 0 {
		if canRetry && isTokenInvalidErr(result.ErrCode) {
			log.Printf("OA access token invalid (errcode=%d), refreshing and retrying QR create", result.ErrCode)
			s.InvalidateToken()
			return s.createParametricQRWithRetry(sceneStr, false)
		}
		return "", fmt.Errorf("wechat QR create API error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	// Build QR image URL
	qrURL := fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=%s", url.QueryEscape(result.Ticket))
	return qrURL, nil
}

