package model

import "time"

// VipLevelConfig represents the vip_level_config table (non-sharded)
type VipLevelConfig struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Level            int       `gorm:"not null" json:"level"`
	Name             string    `gorm:"size:64;not null" json:"name"`
	GrowthRequired   int64     `gorm:"not null" json:"growth_required"`
	Icon             string    `gorm:"size:512;not null;default:''" json:"icon"`
	WithdrawFeeRate  float64   `gorm:"type:decimal(5,2);not null;default:0.00" json:"withdraw_fee_rate"`
	DailySigninBonus float64   `gorm:"type:decimal(18,4);not null;default:0.0000" json:"daily_signin_bonus"`
	BenefitsJSON     string    `gorm:"type:json" json:"benefits_json"`
	Status           int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (VipLevelConfig) TableName() string {
	return "vip_level_config"
}

// UserVip represents the user_vip table (hash-sharded by user_id)
type UserVip struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64      `gorm:"not null" json:"user_id"`
	Level       int        `gorm:"not null;default:0" json:"level"`
	Growth      int64      `gorm:"not null;default:0" json:"growth"`
	UpgradeAt   *time.Time `json:"upgrade_at"`
	LastRefresh *time.Time `json:"last_refresh"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (UserVip) TableName() string {
	return "user_vip"
}