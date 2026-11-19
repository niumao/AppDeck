package models

import (
	"time"
)

// RechargeOrder 订单状态
const (
	RechargeOrderStatusPending   = 0 // 待支付
	RechargeOrderStatusPaid      = 1 // 已支付
	RechargeOrderStatusCancelled = 2 // 已取消
	RechargeOrderStatusRefunded  = 3 // 已退款
)

// RechargeOrder 充值订单模型
type RechargeOrder struct {
	ID                   string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID               string     `gorm:"column:user_id;size:36;not null;index" json:"user_id"`
	OrderNo              string     `gorm:"column:order_no;size:64;uniqueIndex;not null" json:"order_no"`
	WxTransactionID      string     `gorm:"column:wx_transaction_id;size:64" json:"wx_transaction_id"`
	PackageID            string     `gorm:"column:package_id;size:36" json:"package_id"`
	PackageName          string     `gorm:"column:package_name;size:50" json:"package_name"`
	AmountCents          int        `gorm:"column:amount_cents;not null" json:"amount_cents"`
	BonusCents           int        `gorm:"column:bonus_cents;default:0" json:"bonus_cents"`
	TotalCreditCents     int        `gorm:"column:total_credit_cents;not null" json:"total_credit_cents"`
	Status               int        `gorm:"column:status;default:0;index" json:"status"`
	Scope                string     `gorm:"column:scope;size:10;default:'global';index" json:"scope"`
	TenantID             *string    `gorm:"column:tenant_id;size:36;index" json:"tenant_id"`
	PlanSetID            *string    `gorm:"column:plan_set_id;size:36" json:"plan_set_id"`
	PlanID               *string    `gorm:"column:plan_id;size:36" json:"plan_id"`
	StandardRateSnapshot int        `gorm:"column:standard_rate_snapshot;default:0" json:"standard_rate_snapshot"`
	StandardUnitSnapshot string     `gorm:"column:standard_unit_snapshot;size:20;default:'hour'" json:"standard_unit_snapshot"`
	MinutesCredited      int        `gorm:"column:minutes_credited;default:0" json:"minutes_credited"`
	MinutesRemaining     int        `gorm:"column:minutes_remaining;default:0" json:"minutes_remaining"`
	RealCents            int        `gorm:"column:real_cents;default:0" json:"real_cents"`
	RealCentsRemaining   int        `gorm:"column:real_cents_remaining;default:0" json:"real_cents_remaining"`
	RefundedCents        int        `gorm:"column:refunded_cents;default:0" json:"refunded_cents"`
	RefundStatus         string     `gorm:"column:refund_status;size:20;default:'none'" json:"refund_status"`
	QRCodeURL            string     `gorm:"column:qr_code_url;size:500" json:"qr_code_url"`
	CreatedAt            time.Time  `gorm:"column:created_at" json:"created_at"`
	PaidAt               *time.Time `gorm:"column:paid_at" json:"paid_at"`
	ExpiresAt            time.Time  `gorm:"column:expires_at" json:"expires_at"`

}

// RechargeOrderWithUser 订单+用户信息（用于管理端查询）
type RechargeOrderWithUser struct {
	RechargeOrder
	UserNickname  string `json:"user_nickname"`
	UserAvatarURL string `json:"user_avatar_url"`
	UserStatus    int    `json:"user_status"`
}

// TableName 指定表名
func (RechargeOrder) TableName() string {
	return "recharge_orders"
}

// IsPending 检查是否待支付
func (o *RechargeOrder) IsPending() bool {
	return o.Status == RechargeOrderStatusPending
}

// IsPaid 检查是否已支付
func (o *RechargeOrder) IsPaid() bool {
	return o.Status == RechargeOrderStatusPaid
}

// IsExpired 检查订单是否已过期
func (o *RechargeOrder) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

// GetAmountYuan 获取金额（元）
func (o *RechargeOrder) GetAmountYuan() float64 {
	return float64(o.AmountCents) / 100
}
