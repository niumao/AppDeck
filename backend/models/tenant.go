package models

import "time"

type Tenant struct {
	ID            string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	Username      string    `gorm:"column:username;size:50;uniqueIndex;not null" json:"username"`
	Password      string    `gorm:"column:password;size:255;not null" json:"-"`
	DisplayName   string    `gorm:"column:display_name;size:100" json:"display_name"`
	Phone         string    `gorm:"column:phone;size:20" json:"phone"`
	Status        int       `gorm:"column:status;default:1" json:"status"` // 1=active, 0=disabled
	PlanSetID     *string   `gorm:"column:plan_set_id;size:36;index" json:"plan_set_id"`
	GlobalEnabled int       `gorm:"column:global_enabled;default:1" json:"global_enabled"`
	FreePlay      int       `gorm:"column:free_play;default:0" json:"free_play"` // 0=paid, 1=free
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`

	PlanSet *PlanSet `gorm:"foreignKey:PlanSetID" json:"plan_set,omitempty"`
}

func (Tenant) TableName() string { return "tenants" }
func (t *Tenant) IsActive() bool { return t.Status == 1 }
