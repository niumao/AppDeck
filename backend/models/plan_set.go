package models

import (
	"time"

	"gorm.io/gorm"
)

type PlanSet struct {
	ID               string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	Name             string    `gorm:"column:name;size:100;not null;uniqueIndex" json:"name"`
	Description      string    `gorm:"column:description;size:500;default:''" json:"description"`
	IsDefault        int       `gorm:"column:is_default;default:0" json:"is_default"`
	StandardPlanID   *string   `gorm:"column:standard_plan_id;size:36" json:"standard_plan_id"`
	GlobalRatePlanID *string   `gorm:"column:global_rate_plan_id;size:36" json:"global_rate_plan_id"`
	Status           int            `gorm:"column:status;default:1" json:"status"`
	CreatedAt        time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`

	StandardPlan   *BillingPlan  `gorm:"foreignKey:StandardPlanID" json:"standard_plan,omitempty"`
	GlobalRatePlan *BillingPlan  `gorm:"foreignKey:GlobalRatePlanID" json:"global_rate_plan,omitempty"`
	Plans          []BillingPlan `gorm:"foreignKey:PlanSetID" json:"plans,omitempty"`
}

func (PlanSet) TableName() string { return "plan_sets" }
func (ps *PlanSet) IsActive() bool { return ps.Status == 1 }
