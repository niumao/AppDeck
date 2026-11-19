package models

import "time"

type UserTenantBalance struct {
	ID             string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID         string    `gorm:"column:user_id;size:36;not null;uniqueIndex:uk_user_tenant" json:"user_id"`
	TenantID       string    `gorm:"column:tenant_id;size:36;not null;uniqueIndex:uk_user_tenant" json:"tenant_id"`
	BalanceMinutes int       `gorm:"column:balance_minutes;default:0" json:"balance_minutes"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`

	User   *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

func (UserTenantBalance) TableName() string { return "user_tenant_balances" }
