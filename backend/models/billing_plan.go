package models

import (
	"time"
)

// BillingPlan 计费方案模型
type BillingPlan struct {
	ID           string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	Name         string    `gorm:"column:name;size:100;not null" json:"name"`
	Rate         int       `gorm:"column:rate;not null" json:"rate_cents"`
	Unit         string    `gorm:"column:unit;size:20;not null;default:'minute'" json:"unit"`
	PlanSetID    *string   `gorm:"column:plan_set_id;size:36;index" json:"plan_set_id"`
	Type         string    `gorm:"column:type;size:20;default:'rate'" json:"type"`
	PriceCents   int       `gorm:"column:price_cents;default:0" json:"price_cents"`
	GivesMinutes int       `gorm:"column:gives_minutes;default:0" json:"gives_minutes"`
	Description  string    `gorm:"column:description;size:500" json:"description"`
	IsDefault    int       `gorm:"column:is_default;default:0" json:"is_default"`
	Status       int       `gorm:"column:status;default:1" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`

	PlanSet *PlanSet `gorm:"foreignKey:PlanSetID" json:"plan_set,omitempty"`
}

func (BillingPlan) TableName() string {
	return "billing_plans"
}

func (bp *BillingPlan) IsActive() bool {
	return bp.Status == 1
}
