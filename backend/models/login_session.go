package models

import (
	"time"
)

// LoginSession 登录会话状态
const (
	LoginSessionStatusPending    = 0 // 等待扫码
	LoginSessionStatusScanned    = 1 // 已扫码
	LoginSessionStatusAuthorized = 2 // 已授权
	LoginSessionStatusExpired    = 3 // 已过期
)

// LoginSession 登录会话模型（用于QR码登录轮询）
type LoginSession struct {
	ID        string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	SceneStr  string    `gorm:"column:scene_str;size:100;uniqueIndex;not null" json:"sceneStr"`
	UserID    *string   `gorm:"column:user_id;size:36" json:"userId"`
	Token     string    `gorm:"column:token;type:text" json:"-"`
	Status    int       `gorm:"column:status;default:0" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	ExpiresAt time.Time `gorm:"column:expires_at" json:"expiresAt"`
}

// TableName 指定表名
func (LoginSession) TableName() string {
	return "login_sessions"
}

// IsExpired 检查会话是否已过期
func (s *LoginSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsPending 检查是否等待扫码
func (s *LoginSession) IsPending() bool {
	return s.Status == LoginSessionStatusPending
}

// IsAuthorized 检查是否已授权
func (s *LoginSession) IsAuthorized() bool {
	return s.Status == LoginSessionStatusAuthorized
}
