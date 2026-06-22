package model

import "time"

// PaymentChannel represents the payment_channel table (non-sharded)
type PaymentChannel struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string    `gorm:"size:64;not null" json:"name"`
	Type             string    `gorm:"size:32;not null" json:"type"` // stripe/paypal/usdt/bank
	ConfigJSON       string    `gorm:"type:json;not null" json:"-"`
	Status           int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
	SupportedRegions string    `gorm:"size:512;not null;default:''" json:"supported_regions"`
	MinAmount        float64   `gorm:"type:decimal(18,4);not null;default:0" json:"min_amount"`
	MaxAmount        float64   `gorm:"type:decimal(18,4);not null;default:0" json:"max_amount"`
	SortOrder        int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (PaymentChannel) TableName() string {
	return "payment_channel"
}

// RechargeOrder represents the recharge_order table (hash-sharded by user_id)
type RechargeOrder struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64      `gorm:"not null" json:"user_id"`
	OrderNo      string     `gorm:"size:64;not null" json:"order_no"`
	Amount       float64    `gorm:"type:decimal(18,4);not null" json:"amount"`
	AmountUSD    float64    `gorm:"type:decimal(18,4);not null;default:0" json:"amount_usd"`
	Currency     string     `gorm:"size:8;not null;default:'USD'" json:"currency"`
	ChannelID    int64      `gorm:"not null" json:"channel_id"`
	ThirdOrderNo string     `gorm:"size:128;not null;default:''" json:"third_order_no"`
	Status       int8       `gorm:"type:tinyint;not null;default:0" json:"status"` // 0=pending 1=paid 2=failed 3=refunded
	PaidAt       *time.Time `json:"paid_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (RechargeOrder) TableName() string {
	return "recharge_order"
}

// WithdrawOrder represents the withdraw_order table (hash-sharded by user_id)
type WithdrawOrder struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64      `gorm:"not null" json:"user_id"`
	OrderNo    string     `gorm:"size:64;not null" json:"order_no"`
	Amount     float64    `gorm:"type:decimal(18,4);not null" json:"amount"`
	Currency   string     `gorm:"size:8;not null;default:'USD'" json:"currency"`
	BankInfo   string     `gorm:"type:json;not null" json:"bank_info"` // encrypted JSON
	ChannelID  int64      `gorm:"not null;default:0" json:"channel_id"`
	Fee        float64    `gorm:"type:decimal(18,4);not null;default:0" json:"fee"`
	RealAmount float64    `gorm:"type:decimal(18,4);not null;default:0" json:"real_amount"`
	Status     int8       `gorm:"type:tinyint;not null;default:0" json:"status"` // 0=pending 1=approved 2=completed 3=rejected 4=cancelled
	ReviewedBy *int64     `json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	Remark     string     `gorm:"size:512;not null;default:''" json:"remark"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (WithdrawOrder) TableName() string {
	return "withdraw_order"
}

// UserWallet represents the user_wallet table (non-sharded, high-frequency R/W)
type UserWallet struct {
	ID             int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         int64   `gorm:"not null;uniqueIndex" json:"user_id"`
	CashBalance    float64 `gorm:"type:decimal(18,4);not null;default:0" json:"cash_balance"`
	BonusBalance   float64 `gorm:"type:decimal(18,4);not null;default:0" json:"bonus_balance"`
	FrozenBalance  float64 `gorm:"type:decimal(18,4);not null;default:0" json:"frozen_balance"`
	TotalRecharge  float64 `gorm:"type:decimal(18,4);not null;default:0" json:"total_recharge"`
	TotalWithdraw  float64 `gorm:"type:decimal(18,4);not null;default:0" json:"total_withdraw"`
	TotalWin       float64 `gorm:"type:decimal(18,4);not null;default:0" json:"total_win"`
	TotalBet       float64 `gorm:"type:decimal(18,4);not null;default:0" json:"total_bet"`
	Version        int     `gorm:"not null;default:0" json:"version"` // optimistic lock
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (UserWallet) TableName() string {
	return "user_wallet"
}

// UserPaymentAccount represents user's payment accounts for withdrawal (hash-sharded)
type UserPaymentAccount struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"not null" json:"user_id"`
	AccountType int8      `gorm:"type:tinyint;not null" json:"account_type"` // 1=bank 2=usdt 3=paypal
	Title       string    `gorm:"size:64;not null" json:"title"`              // display name
	Account     string    `gorm:"size:256;not null" json:"account"`           // account number / address
	Code        string    `gorm:"size:64;not null;default:''" json:"code"`    // bank code / network
	Username    string    `gorm:"size:128;not null;default:''" json:"username"` // account holder name
	IsDefault   int8      `gorm:"type:tinyint;not null;default:0" json:"is_default"`
	Status      int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UserPaymentAccount) TableName() string {
	return "user_payment_account"
}

// UserWithdrawPassword stores the user's withdraw password hash (non-sharded)
type UserWithdrawPassword struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"not null;uniqueIndex" json:"user_id"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	HasSet       int8      `gorm:"type:tinyint;not null;default:0" json:"has_set"` // 0=not set 1=set
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (UserWithdrawPassword) TableName() string {
	return "user_withdraw_password"
}
