package models

import (
	"time"

	"gorm.io/gorm"
)

type GlobalRechargePackage struct {
	ID         string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	Name       string    `gorm:"column:name;size:100;not null" json:"name"`
	PriceCents int       `gorm:"column:price_cents;not null" json:"price_cents"`
	RealCents  int       `gorm:"column:real_cents;not null" json:"real_cents"`
	BonusCents int       `gorm:"column:bonus_cents;default:0" json:"bonus_cents"`
	SortOrder  int            `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status     int            `gorm:"column:status;default:1" json:"status"`
	CreatedAt  time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (GlobalRechargePackage) TableName() string { return "global_recharge_packages" }
func (p *GlobalRechargePackage) IsActive() bool  { return p.Status == 1 }
