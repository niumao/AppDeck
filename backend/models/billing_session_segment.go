package models

import "time"

type BillingSessionSegment struct {
	ID                 string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SessionID          string     `gorm:"column:session_id;size:36;not null;index" json:"session_id"`
	BalanceSource      string     `gorm:"column:balance_source;size:20;not null;index" json:"balance_source"`
	RateSnapshot       int        `gorm:"column:rate_snapshot;default:0" json:"rate_snapshot"`
	RateUnitSnapshot   string     `gorm:"column:rate_unit_snapshot;size:20;default:'hour'" json:"rate_unit_snapshot"`
	StartedAt          time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	EndedAt            *time.Time `gorm:"column:ended_at" json:"ended_at"`
	DurationSec        int        `gorm:"column:duration_sec;default:0" json:"duration_sec"`
	MinutesConsumed    int        `gorm:"column:minutes_consumed;default:0" json:"minutes_consumed"`
	RealCentsConsumed  int        `gorm:"column:real_cents_consumed;default:0" json:"real_cents_consumed"`
	BonusCentsConsumed int        `gorm:"column:bonus_cents_consumed;default:0" json:"bonus_cents_consumed"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`

	Session *BillingSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

func (BillingSessionSegment) TableName() string { return "billing_session_segments" }
func (s *BillingSessionSegment) IsActive() bool  { return s.EndedAt == nil }
