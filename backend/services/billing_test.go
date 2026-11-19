package services

import (
	"appdeck/models"
	"testing"
)

func TestCalculateCost_Minute(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 100, Unit: "minute"}

	tests := []struct {
		name        string
		durationSec int
		wantCost    int
	}{
		{"zero duration", 0, 0},
		{"negative duration", -10, 0},
		{"exact 1 minute", 60, 100},
		{"exact 2 minutes", 120, 200},
		{"partial minute rounds up", 61, 200},
		{"1 second rounds up to 1 minute", 1, 100},
		{"59 seconds rounds up to 1 minute", 59, 100},
		{"90 seconds rounds up to 2 minutes", 90, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.calculateCost(plan, tt.durationSec)
			if got != tt.wantCost {
				t.Errorf("calculateCost(%d sec) = %d, want %d", tt.durationSec, got, tt.wantCost)
			}
		})
	}
}

func TestCalculateCost_Hour(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 600, Unit: "hour"}

	tests := []struct {
		name        string
		durationSec int
		wantCost    int
	}{
		{"zero", 0, 0},
		{"exact 1 hour", 3600, 600},
		{"exact 2 hours", 7200, 1200},
		{"1 second rounds up to 1 hour", 1, 600},
		{"3601 seconds rounds up to 2 hours", 3601, 1200},
		{"half hour rounds up to 1 hour", 1800, 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.calculateCost(plan, tt.durationSec)
			if got != tt.wantCost {
				t.Errorf("calculateCost(%d sec) = %d, want %d", tt.durationSec, got, tt.wantCost)
			}
		})
	}
}

func TestCalculateCost_Day(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 5000, Unit: "day"}

	tests := []struct {
		name        string
		durationSec int
		wantCost    int
	}{
		{"zero", 0, 0},
		{"exact 1 day", 86400, 5000},
		{"1 second rounds up to 1 day", 1, 5000},
		{"86401 seconds rounds up to 2 days", 86401, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.calculateCost(plan, tt.durationSec)
			if got != tt.wantCost {
				t.Errorf("calculateCost(%d sec) = %d, want %d", tt.durationSec, got, tt.wantCost)
			}
		})
	}
}

func TestCalculateCost_DefaultUnit(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 100, Unit: "unknown_unit"}

	got := s.calculateCost(plan, 60)
	if got != 100 {
		t.Errorf("calculateCost with unknown unit should default to minute, got %d, want 100", got)
	}
}

func TestGetRatePerSecond(t *testing.T) {
	s := &SessionService{}

	tests := []struct {
		name     string
		rate     int
		unit     string
		wantRate float64
	}{
		{"minute rate 60", 60, "minute", 1.0},
		{"minute rate 600", 600, "minute", 10.0},
		{"hour rate 3600", 3600, "hour", 1.0},
		{"hour rate 600", 600, "hour", 600.0 / 3600.0},
		{"day rate 86400", 86400, "day", 1.0},
		{"zero rate", 0, "hour", 0.0},
		{"default unit", 60, "unknown", 60.0 / 3600.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.getRatePerSecond(tt.rate, tt.unit)
			if diff := got - tt.wantRate; diff > 0.0001 || diff < -0.0001 {
				t.Errorf("getRatePerSecond(%d, %q) = %f, want %f", tt.rate, tt.unit, got, tt.wantRate)
			}
		})
	}
}

func TestCalculateCost_PartialUnitRoundsUp(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 100, Unit: "minute"}

	if got := s.calculateCost(plan, 61); got != 200 {
		t.Errorf("ceiling cost for 61s at 100/min should be 200, got %d", got)
	}
}

func TestCalculateCost_ExactBoundary(t *testing.T) {
	s := &SessionService{}
	plan := &models.BillingPlan{Rate: 100, Unit: "minute"}

	if got := s.calculateCost(plan, 60); got != 100 {
		t.Errorf("cost at exact 1 minute boundary should be 100, got %d", got)
	}
}
