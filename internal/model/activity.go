package model

import (
        "time"
)

// ActivityDefine represents the activity_define table
// Defines an activity with its type, configuration, and time range.
// Type values: signin, wheel, recharge, gift
type ActivityDefine struct {
        ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        Name        string    `gorm:"size:128;not null" json:"name"`
        Type        string    `gorm:"size:16;not null" json:"type"`           // signin/wheel/recharge/gift
        HandlerName string    `gorm:"size:64;not null" json:"handler_name"`    // plugin handler name
        ConfigJSON  string    `gorm:"type:json;not null" json:"config_json"`  // activity config JSON
        StartTime   time.Time `gorm:"not null" json:"start_time"`
        EndTime     time.Time `gorm:"not null" json:"end_time"`
        Status      int8      `gorm:"type:tinyint;not null;default:1" json:"status"` // 1=active, 0=disabled
        Priority    int       `gorm:"not null;default:0" json:"priority"`
        CreatedAt   time.Time `json:"created_at"`
        UpdatedAt   time.Time `json:"updated_at"`
}

func (ActivityDefine) TableName() string {
        return "activity_define"
}

// ActivityInventory represents the activity_inventory table
// Tracks reward stock for an activity (optional, for limited rewards).
type ActivityInventory struct {
        ID         int64  `gorm:"primaryKey;autoIncrement" json:"id"`
        ActivityID int64  `gorm:"not null" json:"activity_id"`
        RewardID   int64  `gorm:"not null" json:"reward_id"`
        Total      int    `gorm:"not null;default:0" json:"total"`
        Remaining  int    `gorm:"not null;default:0" json:"remaining"`
        CreatedAt  time.Time `json:"created_at"`
}

func (ActivityInventory) TableName() string {
        return "activity_inventory"
}

// UserActivity represents the user_activity table (hash sharded)
// Tracks a user's participation in a specific activity.
// StateData stores JSON for activity-specific state (e.g., consecutive check-in days).
type UserActivity struct {
        ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
        UserID     int64      `gorm:"not null" json:"user_id"`
        ActivityID int64      `gorm:"not null" json:"activity_id"`
        Progress   int        `gorm:"not null;default:0" json:"progress"`
        StateData  string     `gorm:"type:json" json:"state_data"` // JSON: {"streak":3,"cycle":1,...}
        LastDrawAt *time.Time `json:"last_draw_at"`
        CreatedAt  time.Time  `json:"created_at"`
        UpdatedAt  time.Time  `json:"updated_at"`
}

func (UserActivity) TableName() string {
        return "user_activity"
}

// ActivityRecord represents the activity_records table (hash sharded)
// Records each user action within an activity (checkin, draw, claim).
type ActivityRecord struct {
        ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        UserID      int64     `gorm:"not null" json:"user_id"`
        ActivityID  int64     `gorm:"not null" json:"activity_id"`
        Action      string    `gorm:"size:32;not null" json:"action"`       // checkin/draw/claim
        RewardJSON  string    `gorm:"type:json" json:"reward_json"`       // reward details JSON
        CreatedAt   time.Time `json:"created_at"`
}

func (ActivityRecord) TableName() string {
        return "activity_records"
}

// CheckInStateData represents the state_data JSON for signin-type activities
type CheckInStateData struct {
        Streak      int `json:"streak"`       // consecutive check-in days
        Cycle       int `json:"cycle"`        // which 7-day cycle (1-indexed)
        LastDay     int `json:"last_day"`     // last checked-in day within cycle (1-7)
        TotalDays   int `json:"total_days"`   // lifetime total check-in days
}

// CheckInRewardConfig represents a single day's reward in the config JSON
type CheckInRewardConfig struct {
        Day        int     `json:"day"`          // day number (1-7)
        RewardType  string  `json:"reward_type"`  // "bonus" / "coin" / "item"
        RewardValue float64 `json:"reward_value"` // reward amount
        ItemID      int64   `json:"item_id"`      // item ID (when reward_type=item)
}

// CheckInConfig represents the full config_json for a signin-type activity
type CheckInConfig struct {
        CycleDays    int                  `json:"cycle_days"`    // cycle length (default 7)
        Rewards      []CheckInRewardConfig `json:"rewards"`       // per-day reward list
        ResetOnMiss  bool                 `json:"reset_on_miss"` // reset streak on missed day (default true)
}

// ── Lucky Wheel (转盘抽奖) Models ──

// WheelConfig represents the full config_json for a wheel-type activity
type WheelConfig struct {
        FreeSpinsPerDay int          `json:"free_spins_per_day"` // daily free spins granted per user
        SpinCost        float64      `json:"spin_cost"`          // cost per paid spin
        SpinCostType    string       `json:"spin_cost_type"`     // "bonus" / "coin"
        CooldownSec     int          `json:"cooldown_sec"`       // cooldown between spins in seconds (0 = no cooldown)
        MaxSpinsPerDay  int          `json:"max_spins_per_day"`  // max total spins per day (0 = unlimited, free+paid combined)
        Prizes          []WheelPrize `json:"prizes"`             // prize slots on the wheel
}

// WheelPrize represents a single prize slot on the wheel
type WheelPrize struct {
        ID     int64   `json:"id"`      // prize identifier (references activity_inventory.reward_id)
        Name   string  `json:"name"`    // display name
        Type   string  `json:"type"`    // "bonus" / "coin" / "item" / "empty"
        Value  float64 `json:"value"`   // reward amount (for bonus/coin)
        ItemID int64   `json:"item_id"` // item_define.id (when type=item)
        Weight int     `json:"weight"`  // probability weight (relative; higher = more likely)
        Icon   string  `json:"icon"`    // prize icon URL
        Rarity string  `json:"rarity"`  // "common" / "rare" / "epic" / "legendary"
        Stock  int     `json:"stock"`   // total stock (-1 = unlimited)
}

// WheelStateData represents the state_data JSON for wheel-type activities
type WheelStateData struct {
        TotalSpins     int    `json:"total_spins"`       // lifetime total spins
        TodayFreeSpins int    `json:"today_free_spins"`  // free spins used today
        TodayTotalSpins int   `json:"today_total_spins"` // total spins used today (free + paid)
        LastSpinDate   string `json:"last_spin_date"`    // UTC date string "20260607" of last spin
        LastSpinTime   int64  `json:"last_spin_time"`    // Unix timestamp of last spin (for cooldown)
}
