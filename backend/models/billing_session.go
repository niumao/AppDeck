package models

import (
	"time"
)

// BillingSession 计费会话模型
type BillingSession struct {
	ID                       string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID                   string     `gorm:"column:user_id;size:36;not null;index" json:"user_id"`
	ClientID                 string     `gorm:"column:client_id;size:36;not null;index" json:"client_id"`
	AppID                    *string    `gorm:"column:app_id;size:36;index" json:"app_id"`
	PlanID                   string     `gorm:"column:plan_id;size:36;not null;index" json:"plan_id"`
	TenantID                 string     `gorm:"column:tenant_id;size:36;index" json:"tenant_id"`
	PlanSetID                string     `gorm:"column:plan_set_id;size:36" json:"plan_set_id"`
	TenantMinutesConsumed    int        `gorm:"column:tenant_minutes_consumed;default:0" json:"tenant_minutes_consumed"`
	GlobalRealCentsConsumed  int        `gorm:"column:global_real_cents_consumed;default:0" json:"global_real_cents_consumed"`
	GlobalBonusCentsConsumed int        `gorm:"column:global_bonus_cents_consumed;default:0" json:"global_bonus_cents_consumed"`
	StartedAt                time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	EndedAt                  *time.Time `gorm:"column:ended_at" json:"ended_at"`
	DurationSec              int        `gorm:"column:duration_sec;default:0" json:"duration_sec"`
	TotalCost                int        `gorm:"column:total_cost;default:0" json:"total_cost"`
	LastBilledCost           int        `gorm:"column:last_billed_cost;default:0" json:"last_billed_cost"`
	CreatedAt                time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt                time.Time  `gorm:"column:updated_at" json:"updated_at"`

	User   *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Client *UserClient  `gorm:"foreignKey:ClientID" json:"client,omitempty"`
	App    *App         `gorm:"foreignKey:AppID" json:"app,omitempty"`
	Plan   *BillingPlan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	Tenant *Tenant      `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

func (BillingSession) TableName() string {
	return "billing_sessions"
}

func (bs *BillingSession) IsActive() bool {
	return bs.EndedAt == nil
}

func (bs *BillingSession) GetDurationMinutes() int {
	if bs.DurationSec <= 0 {
		return 0
	}
	return bs.DurationSec / 60
}
