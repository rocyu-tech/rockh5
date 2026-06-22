package handler

import (
	"testing"

	"github.com/rocyu-tech/rockgame/internal/model"
)

func TestFindReward(t *testing.T) {
	rewards := []model.CheckInRewardConfig{
		{Day: 1, RewardType: "bonus", RewardValue: 1.0},
		{Day: 2, RewardType: "bonus", RewardValue: 2.0},
		{Day: 3, RewardType: "bonus", RewardValue: 3.0},
		{Day: 4, RewardType: "bonus", RewardValue: 5.0},
		{Day: 5, RewardType: "bonus", RewardValue: 8.0},
		{Day: 6, RewardType: "coin", RewardValue: 10.0},
		{Day: 7, RewardType: "item", RewardValue: 0, ItemID: 100},
	}

	tests := []struct {
		name      string
		totalDays int
		wantDay   int
		wantType  string
		wantValue float64
	}{
		{"day 1 returns first reward", 1, 1, "bonus", 1.0},
		{"day 3 returns third reward", 3, 3, "bonus", 3.0},
		{"day 7 returns last reward", 7, 7, "item", 0},
		{"day 8 wraps to day 1 (new cycle)", 8, 1, "bonus", 1.0},
		{"day 14 wraps to day 7", 14, 7, "item", 0},
		{"day 15 wraps to day 1 again", 15, 1, "bonus", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findReward(rewards, tt.totalDays)
			if got == nil {
				t.Fatal("findReward returned nil")
			}
			if got.Day != tt.wantDay {
				t.Errorf("day = %d, want %d", got.Day, tt.wantDay)
			}
			if got.RewardType != tt.wantType {
				t.Errorf("reward_type = %q, want %q", got.RewardType, tt.wantType)
			}
			if got.RewardValue != tt.wantValue {
				t.Errorf("reward_value = %.2f, want %.2f", got.RewardValue, tt.wantValue)
			}
		})
	}
}

func TestFindRewardEmpty(t *testing.T) {
	got := findReward(nil, 1)
	if got != nil {
		t.Error("findReward(nil, ...) should return nil")
	}
	got = findReward([]model.CheckInRewardConfig{}, 1)
	if got != nil {
		t.Error("findReward(empty, ...) should return nil")
	}
}

func TestFindRewardFallback(t *testing.T) {
	// If specific day not found, should return last reward
	rewards := []model.CheckInRewardConfig{
		{Day: 1, RewardType: "bonus", RewardValue: 5.0},
		{Day: 5, RewardType: "bonus", RewardValue: 50.0}, // gap: no day 2,3,4
	}
	got := findReward(rewards, 3) // day 3 doesn't exist
	if got == nil {
		t.Fatal("findReward should return fallback (last reward)")
	}
	if got.RewardValue != 50.0 {
		t.Errorf("fallback reward_value = %.2f, want 50.00", got.RewardValue)
	}
}

func TestCheckInStateDataDefaults(t *testing.T) {
	state := model.CheckInStateData{}
	if state.Streak != 0 {
		t.Errorf("default streak = %d, want 0", state.Streak)
	}
	if state.Cycle != 0 {
		t.Errorf("default cycle = %d, want 0", state.Cycle)
	}
}

func TestCheckInConfigDefaults(t *testing.T) {
	cfg := model.CheckInConfig{}
	if cfg.CycleDays != 0 {
		t.Errorf("default cycle_days = %d, want 0", cfg.CycleDays)
	}
	if cfg.ResetOnMiss != false {
		t.Error("default reset_on_miss should be false")
	}
}

func TestValidRoles(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"super", true},
		{"admin", true},
		{"operator", true},
		{"viewer", true},
		{"Super", false},     // case sensitive
		{"SUPER", false},     // case sensitive
		{"", false},          // empty
		{"hacker", false},    // unknown
		{"root", false},      // not a valid role
		{"superadmin", false}, // not a valid role
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := IsValidRole(tt.role)
			if got != tt.want {
				t.Errorf("IsValidRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}