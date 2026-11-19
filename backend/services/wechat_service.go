package services

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"appdeck/config"
)

// WechatService 微信服务
type WechatService struct {
	appID          string
	appSecret      string
	token          string
	mchID          string
	mchAPIKey      string
	accessToken    string
	tokenExpiresAt time.Time
	mu             sync.RWMutex
}

// WechatAccessTokenResponse 微信access_token响应
type WechatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// WechatQRCodeResponse 微信临时二维码响应
type WechatQRCodeResponse struct {
	Ticket        string `json:"ticket"`
	ExpireSeconds int    `json:"expire_seconds"`
	URL           string `json:"url"`
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
}

// WechatUserInfo 微信用户信息
type WechatUserInfo struct {
	Subscribe     int    `json:"subscribe"`
	OpenID        string `json:"openid"`
	UnionID       string `json:"unionid"`
	Nickname      string `json:"nickname"`
	HeadImgURL    string `json:"headimgurl"`
	SubscribeTime int64  `json:"subscribe_time"`
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
}

// WechatMessage 微信消息
type WechatMessage struct {
	ToUserName   string `json:"ToUserName"`
	FromUserName string `json:"FromUserName"`
	CreateTime   int64  `json:"CreateTime"`
	MsgType      string `json:"MsgType"`
	Event        string `json:"Event"`
	EventKey     string `json:"EventKey"`
	Ticket       string `json:"Ticket"`
}

var wechatServiceInstance *WechatService
var wechatServiceOnce sync.Once

// NewWechatService 创建微信服务单例
func NewWechatService(cfg *config.WeChatConfig) *WechatService {
	wechatServiceOnce.Do(func() {
		wechatServiceInstance = &WechatService{
			appID:     cfg.AppID,
			appSecret: cfg.AppSecret,
			token:     cfg.Token,
			mchID:     cfg.MchID,
			mchAPIKey: cfg.MchAPIKey,
		}
	})
	return wechatServiceInstance
}

// GetWechatService 获取微信服务实例
func GetWechatService() *WechatService {
	return wechatServiceInstance
}

// GetAccessToken 获取access_token（带缓存）
func (s *WechatService) GetAccessToken() (string, error) {
	s.mu.RLock()
	if s.accessToken != "" && time.Now().Before(s.tokenExpiresAt) {
		token := s.accessToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 再次检查，防止并发时重复请求
	if s.accessToken != "" && time.Now().Before(s.tokenExpiresAt) {
		return s.accessToken, nil
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		s.appID, s.appSecret,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result WechatAccessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat api error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	s.accessToken = result.AccessToken
	// 提前5分钟过期
	s.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return s.accessToken, nil
}

// CreateTempQRCode 创建临时二维码
func (s *WechatService) CreateTempQRCode(sceneStr string, expireSeconds int) (*WechatQRCodeResponse, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=%s", token)

	// 构建请求体
	reqBody := map[string]interface{}{
		"expire_seconds": expireSeconds,
		"action_name":    "QR_STR_SCENE",
		"action_info": map[string]interface{}{
			"scene": map[string]string{
				"scene_str": sceneStr,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create qr code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result WechatQRCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat api error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &result, nil
}

// GetQRCodeURL 获取二维码图片URL
func (s *WechatService) GetQRCodeURL(ticket string) string {
	return fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=%s", ticket)
}

// GetUserInfo 获取用户信息
func (s *WechatService) GetUserInfo(openID string) (*WechatUserInfo, error) {
	token, err := s.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/user/info?access_token=%s&openid=%s&lang=zh_CN",
		token, openID,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result WechatUserInfo
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat api error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &result, nil
}

// VerifySignature 验证微信消息签名
func (s *WechatService) VerifySignature(signature, timestamp, nonce string) bool {
	arr := []string{s.token, timestamp, nonce}
	sort.Strings(arr)
	str := strings.Join(arr, "")

	h := sha1.New()
	h.Write([]byte(str))
	hash := hex.EncodeToString(h.Sum(nil))

	return hash == signature
}

// WechatOAuthTokenResponse OAuth access_token 响应
type WechatOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// GetOAuthURL 生成 OAuth 授权链接
// scope: snsapi_base (静默授权，只获取openid) 或 snsapi_userinfo (弹窗授权，获取用户信息)
func (s *WechatService) GetOAuthURL(redirectURI, state, scope string) string {
	if scope == "" {
		scope = "snsapi_base"
	}
	return fmt.Sprintf(
		"https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s#wechat_redirect",
		s.appID, redirectURI, scope, state,
	)
}

// GetOAuthToken 通过 code 换取 access_token 和 openid
func (s *WechatService) GetOAuthToken(code string) (*WechatOAuthTokenResponse, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		s.appID, s.appSecret, code,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result WechatOAuthTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat oauth error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &result, nil
}

// GetOAuthUserInfo 通过 OAuth access_token 获取用户信息 (SNS API)
// 仅当 scope 为 snsapi_userinfo 时可用
func (s *WechatService) GetOAuthUserInfo(accessToken, openID string) (*WechatUserInfo, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN",
		accessToken, openID,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result WechatUserInfo
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[NICKNAME-DEBUG] sns/userinfo raw response for openID=%s: nickname=%q, headimgurl=%q, unionid=%q, errcode=%d",
		openID, result.Nickname, result.HeadImgURL, result.UnionID, result.ErrCode)

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat oauth userinfo error: %d - %s", result.ErrCode, result.ErrMsg)
	}

	return &result, nil
}

// GetAppID 获取 AppID
func (s *WechatService) GetAppID() string {
	return s.appID
}
