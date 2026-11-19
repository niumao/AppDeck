package models

import (
	"time"
)

// UserClient 用户客户端模型
type UserClient struct {
	ID           string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID       string     `gorm:"column:user_id;size:36;not null;index" json:"user_id"`
	Name         string     `gorm:"column:name;size:100;not null" json:"name"`
	MachineID    string     `gorm:"column:machine_id;size:100;index" json:"machine_id"`
	HostID       string     `gorm:"column:host_id;size:100;index" json:"host_id"`
	MachineCode  string     `gorm:"column:machine_code;size:100;index" json:"machine_code"`
	DeviceID     string     `gorm:"column:device_id;size:100;index" json:"device_id"`
	PlanID       *string    `gorm:"column:plan_id;size:36;index" json:"plan_id"`
	SteamID      string     `gorm:"column:steam_id;size:100;index" json:"steam_id"`
	LastActiveAt *time.Time `gorm:"column:last_active_at" json:"last_active_at"`
	Status       int        `gorm:"column:status;default:1" json:"status"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`

	User *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Plan *BillingPlan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}

func (UserClient) TableName() string {
	return "user_clients"
}

func (uc *UserClient) IsOnline() bool {
	return uc.Status == 1
}
