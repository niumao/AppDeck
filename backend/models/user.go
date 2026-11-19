package models

import (
	"time"
)

// User 用户模型
type User struct {
	ID                  string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	OpenID              string     `gorm:"column:openid;size:100;uniqueIndex;not null" json:"-"`
	UnionID             string     `gorm:"column:unionid;size:100" json:"-"`
	Nickname            string     `gorm:"column:nickname;size:100;not null" json:"nickname"`
	AvatarURL           string     `gorm:"column:avatar_url;size:500" json:"avatar_url"`
	AuthLevel           string     `gorm:"column:auth_level;size:20;default:'basic'" json:"auth_level"`
	BalanceCents        int        `gorm:"column:balance_cents;default:0" json:"balance_cents"`
	BonusBalanceCents   int        `gorm:"column:bonus_balance_cents;default:0" json:"bonus_balance_cents"`
	BalanceExpiry       *time.Time `gorm:"column:balance_expiry" json:"balance_expiry"`
	TotalRechargeAmount int        `gorm:"column:total_recharge_amount;default:0" json:"total_recharge_amount"`
	Status              int        `gorm:"column:status;default:1" json:"status"` // 1=active, 0=disabled
	CreatedAt           time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at" json:"updated_at"`
	LastLoginAt         *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// HasValidBalance 检查用户是否有有效余额
func (u *User) HasValidBalance() bool {
	return u.BalanceCents > 0 || u.BonusBalanceCents > 0
}

// GetTotalBalance 获取总余额（实际余额 + 赠送余额）
func (u *User) GetTotalBalance() int {
	return u.BalanceCents + u.BonusBalanceCents
}

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == 1
}
