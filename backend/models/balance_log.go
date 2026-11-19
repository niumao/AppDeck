package models

import (
	"time"
)

// BalanceLog 操作类型
const (
	BalanceLogTypeRecharge    = "recharge"     // 充值
	BalanceLogTypeAdminAdjust = "admin_adjust" // 管理员调整
	BalanceLogTypeConsume     = "consume"      // 消费（预留）
)

// BalanceLog 余额操作日志模型
type BalanceLog struct {
	ID                 string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID             string     `gorm:"column:user_id;size:36;not null;index" json:"user_id"`
	Type               string     `gorm:"column:type;size:20;not null" json:"type"`
	Scope              string     `gorm:"column:scope;size:10;default:'global'" json:"scope"`
	TenantID           *string    `gorm:"column:tenant_id;size:36" json:"tenant_id"`
	BalanceType        string     `gorm:"column:balance_type;size:10;default:'real'" json:"balance_type"`
	AmountCents        int        `gorm:"column:amount_cents;default:0" json:"amount_cents"`
	BalanceBeforeCents int        `gorm:"column:balance_before_cents;default:0" json:"balance_before_cents"`
	BalanceAfterCents  int        `gorm:"column:balance_after_cents;default:0" json:"balance_after_cents"`
	DeltaMinutes       int        `gorm:"column:delta_minutes;default:0" json:"delta_minutes"`
	BalanceBefore      *time.Time `gorm:"column:balance_before" json:"balance_before"`
	BalanceAfter       *time.Time `gorm:"column:balance_after" json:"balance_after"`
	OrderID            *string    `gorm:"column:order_id;size:64" json:"order_id"`
	AdminID            *string    `gorm:"column:admin_id;size:36" json:"admin_id"`
	Remark             string     `gorm:"column:remark;size:200" json:"remark"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (BalanceLog) TableName() string {
	return "balance_logs"
}
