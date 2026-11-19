package models

import (
	"time"
)

// RegisteredDevice represents a device in the whitelist
type RegisteredDevice struct {
	ID            string     `gorm:"type:char(36);primaryKey" json:"id"`
	MachineCode   string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"machine_code"`
	HostID        string     `gorm:"type:varchar(100);not null" json:"host_id"`
	DeviceName    string     `gorm:"type:varchar(255);not null;default:''" json:"device_name"`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	IP            string     `gorm:"type:varchar(45)" json:"ip"`
	Remark          string     `gorm:"type:text" json:"remark"`
	ShutdownEnabled bool       `gorm:"default:false" json:"shutdown_enabled"`
	ShutdownTime    string     `gorm:"type:varchar(5);default:'23:00'" json:"shutdown_time"`
	PoweronEnabled  bool       `gorm:"default:false" json:"poweron_enabled"`
	PoweronTime     string     `gorm:"type:varchar(5);default:'08:00'" json:"poweron_time"`
	BypassChargeEnabled bool       `gorm:"default:false" json:"bypass_charge_enabled"`
	TenantID      *string    `gorm:"type:char(36);index" json:"tenant_id"`
	Tenant        *Tenant    `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	LastAttemptAt *time.Time `gorm:"index" json:"last_attempt_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName specifies the table name for RegisteredDevice
func (RegisteredDevice) TableName() string {
	return "registered_devices"
}
