package biz

import (
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
)

func TestDropBasisPoints(t *testing.T) {
	tests := []struct {
		baseline int64
		current  int64
		want     int
	}{
		{10000, 8000, 2000},
		{10000, 7999, 2001},
		{10000, 10000, 0},
		{0, 100, 0},
	}
	for _, tt := range tests {
		if got := DropBasisPoints(tt.baseline, tt.current); got != tt.want {
			t.Fatalf("DropBasisPoints(%d, %d) = %d, want %d", tt.baseline, tt.current, got, tt.want)
		}
	}
}

func TestEvaluateAlertState(t *testing.T) {
	now := time.Now()
	setting := &schema.ShopSetting{CandidateDropPercent: 15, AlertDropPercent: 20, RecoveryDropPercent: 15}

	t.Run("exactly twenty percent does not trigger", func(t *testing.T) {
		state, trigger, _ := EvaluateAlertState(schema.AlertStateArmed, now.Add(-25*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 3}, 8000, setting)
		if state != schema.AlertStateArmed || trigger {
			t.Fatalf("state=%s trigger=%v", state, trigger)
		}
	})

	t.Run("more than twenty percent triggers", func(t *testing.T) {
		state, trigger, drop := EvaluateAlertState(schema.AlertStateArmed, now.Add(-25*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 3}, 7999, setting)
		if state != schema.AlertStateAlerting || !trigger || drop != 2001 {
			t.Fatalf("state=%s trigger=%v drop=%d", state, trigger, drop)
		}
	})

	t.Run("fractionally more than twenty percent triggers", func(t *testing.T) {
		state, trigger, drop := EvaluateAlertState(schema.AlertStateArmed, now.Add(-25*time.Hour), now, &schema.PriceStats{AverageFen: 10001, Count: 3}, 8000, setting)
		if state != schema.AlertStateAlerting || !trigger || drop != 2000 {
			t.Fatalf("state=%s trigger=%v drop=%d", state, trigger, drop)
		}
	})

	t.Run("cold by age", func(t *testing.T) {
		state, trigger, _ := EvaluateAlertState(schema.AlertStateArmed, now.Add(-23*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 3}, 7000, setting)
		if state != schema.AlertStateArmed || trigger {
			t.Fatalf("state=%s trigger=%v", state, trigger)
		}
	})

	t.Run("cold by sample count", func(t *testing.T) {
		state, trigger, _ := EvaluateAlertState(schema.AlertStateArmed, now.Add(-48*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 2}, 7000, setting)
		if state != schema.AlertStateArmed || trigger {
			t.Fatalf("state=%s trigger=%v", state, trigger)
		}
	})

	t.Run("rearms at fifteen percent", func(t *testing.T) {
		state, trigger, drop := EvaluateAlertState(schema.AlertStateAlerting, now.Add(-48*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 5}, 8500, setting)
		if state != schema.AlertStateArmed || trigger || drop != 1500 {
			t.Fatalf("state=%s trigger=%v drop=%d", state, trigger, drop)
		}
	})

	t.Run("does not rearm fractionally above fifteen percent", func(t *testing.T) {
		state, trigger, drop := EvaluateAlertState(schema.AlertStateAlerting, now.Add(-48*time.Hour), now, &schema.PriceStats{AverageFen: 10001, Count: 5}, 8500, setting)
		if state != schema.AlertStateAlerting || trigger || drop != 1500 {
			t.Fatalf("state=%s trigger=%v drop=%d", state, trigger, drop)
		}
	})

	t.Run("persistent low price is suppressed", func(t *testing.T) {
		state, trigger, _ := EvaluateAlertState(schema.AlertStateAlerting, now.Add(-48*time.Hour), now, &schema.PriceStats{AverageFen: 10000, Count: 5}, 7000, setting)
		if state != schema.AlertStateAlerting || trigger {
			t.Fatalf("state=%s trigger=%v", state, trigger)
		}
	})
}
