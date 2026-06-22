package model

import "time"

// ── Shop Constants ──

// Charge (deposit/recharge) order statuses
const (
        ChargeStatusInit    = 0 // Created, awaiting payment
        ChargeStatusPaid    = 1 // Payment confirmed, balance credited
        ChargeStatusFailed  = 2 // Payment failed / expired
        ChargeStatusRefunded = 3 // Refunded by admin
)

// Withdraw order statuses
const (
        WithdrawStatusInit     = 0 // Created, pending processing
        WithdrawStatusApproved = 1 // Approved by admin
        WithdrawStatusDone     = 2 // Transfer completed
        WithdrawStatusRejected = 3 // Rejected, amount refunded
        WithdrawStatusCancelled = 4 // Cancelled by user
)

// Payment channel types
const (
        ChannelTypeCharge  = 0 // Used for deposits
        ChannelTypeWithdraw = 1 // Used for withdrawals
        ChannelTypeBoth    = 2 // Used for both
)

// Order type tags
const (
        OrderTagNormal  = 0
        OrderTagFirst   = 1
        OrderTagBanner  = 2
        OrderTagQRCode  = 3
        OrderTagActivity = 4 // From activity/match control
)

// ── Payment Channel Model ──

// PaymentChannel represents the payment_channel table (non-sharded).
// Stores channel configurations for both charge and withdrawal.
type PaymentChannel struct {
        ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        Name        string    `gorm:"size:64;not null" json:"name"`
        Type        string    `gorm:"size:32;not null;default:\"general\" json:"type"` // usdt, bank, upi, crypto, stripe, paypal
        Icon        string    `gorm:"size:256;not null;default:\"\" json:"icon"`
        SubTitle    string    `gorm:"size:128;not null;default:\"\" json:"sub_title"`
        ChannelType int8      `gorm:"type:tinyint;not null;default:2" json:"channel_type"` // 0=charge, 1=withdraw, 2=both
        MinCharge   float64   `gorm:"type:decimal(18,4);not null;default:0" json:"min_charge"`
        MaxCharge   float64   `gorm:"type:decimal(18,4);not null;default:0" json:"max_charge"`
        MinWithdraw float64   `gorm:"type:decimal(18,4);not null;default:0" json:"min_withdraw"`
        MaxWithdraw float64   `gorm:"type:decimal(18,4);not null;default:0" json:"max_withdraw"`
        DailyLimit  float64   `gorm:"type:decimal(18,4);not null;default:0" json:"daily_limit"`
        IsHot       bool      `gorm:"not null;default:false" json:"is_hot"`
        SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
        Status      int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        ConfigJSON  string    `gorm:"type:text;not null;default:\"{}\"" json:"config_json"`
        CreatedAt   time.Time `json:"created_at"`
        UpdatedAt   time.Time `json:"updated_at"`
}

func (PaymentChannel) TableName() string { return "payment_channel" }

// ── User Wallet Model ──

// UserWallet represents the user_wallet table (non-sharded, one row per user).
// Uses optimistic locking via Version field for balance updates.
type UserWallet struct {
        ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        UserID           int64     `gorm:"uniqueIndex;not null" json:"user_id"`
        CashBalance      float64   `gorm:"type:decimal(18,4);not null;default:0" json:"cash_balance"`
        BonusBalance     float64   `gorm:"type:decimal(18,4);not null;default:0" json:"bonus_balance"`
        FrozenBalance    float64   `gorm:"type:decimal(18,4);not null;default:0" json:"frozen_balance"`
        TotalRecharge    float64   `gorm:"type:decimal(18,4);not null;default:0" json:"total_recharge"`
        TotalWithdraw    float64   `gorm:"type:decimal(18,4);not null;default:0" json:"total_withdraw"`
        RechargeCount    int       `gorm:"not null;default:0" json:"recharge_count"`
        WithdrawCount    int       `gorm:"not null;default:0" json:"withdraw_count"`
        TotalBet         float64   `gorm:"type:decimal(18,4);not null;default:0" json:"total_bet"`
        TotalWin         float64   `gorm:"type:decimal(18,4);not null;default:0" json:"total_win"`
        FlowRequired     float64   `gorm:"type:decimal(18,4);not null;default:0" json:"flow_required"` // wagering requirement remaining
        FlowCompleted    float64   `gorm:"type:decimal(18,4);not null;default:0" json:"flow_completed"`
        Version          int       `gorm:"not null;default:0" json:"version"` // optimistic lock
        WithdrawPwdHash  string    `gorm:"size:128;not null;default:\"\" json:"-"`
        WithdrawPwdSet   bool      `gorm:"not null;default:false" json:"withdraw_pwd_set"`
        CreatedAt        time.Time `json:"created_at"`
        UpdatedAt        time.Time `json:"updated_at"`
}

func (UserWallet) TableName() string { return "user_wallet" }

// ── Recharge Order Model (sharded by user_id) ──

// RechargeOrder represents the recharge_order table (sharded).
type RechargeOrder struct {
        ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        OrderNo         string    `gorm:"size:64;uniqueIndex;not null" json:"order_no"`
        UserID          int64     `gorm:"not null;index" json:"user_id"`
        ChannelID       int64     `gorm:"not null;default:0" json:"channel_id"`
        Amount          float64   `gorm:"type:decimal(18,4);not null;default:0" json:"amount"` // actual payment amount
        AmountUSD       float64   `gorm:"type:decimal(18,4);not null;default:0" json:"amount_usd"` // USD equivalent
        CreditBalance   float64   `gorm:"type:decimal(18,4);not null;default:0" json:"credit_balance"` // credited to user
        BonusAmount     float64   `gorm:"type:decimal(18,4);not null;default:0" json:"bonus_amount"` // extra bonus
        BonusFlow       float64   `gorm:"type:decimal(18,4);not null;default:0" json:"bonus_flow"` // flow requirement from bonus
        ThirdOrderNo    string    `gorm:"size:128;not null;default:\"\" json:"third_order_no"`
        ThirdPayURL     string    `gorm:"size:1024;not null;default:\"\" json:"third_pay_url"`
        PaymentType     int       `gorm:"not null;default:0" json:"payment_type"` // 0=normal, 100=USDT
        PaymentChannel  string    `gorm:"size:64;not null;default:\"\" json:"payment_channel"`
        PaymentName     string    `gorm:"size:64;not null;default:\"\" json:"payment_name"`
        Status          int8      `gorm:"type:tinyint;not null;default:0" json:"status"`
        Tag             int8      `gorm:"type:tinyint;not null;default:0" json:"tag"` // 0=normal, 1=first, 2=banner, 3=qrcode, 4=activity
        AccountType     int       `gorm:"not null;default:0" json:"account_type"`
        Account         string    `gorm:"size:256;not null;default:\"\" json:"account"`
        BankName        string    `gorm:"size:64;not null;default:\"\" json:"bank_name"`
        SrcType         int       `gorm:"not null;default:0" json:"src_type"`
        ExtJSON         string    `gorm:"type:text;not null;default:\"{}\"" json:"ext_json"`
        ExchangeRate    float64   `gorm:"type:decimal(18,8);not null;default:0" json:"exchange_rate"`
        USDTAmount      float64   `gorm:"type:decimal(18,8);not null;default:0" json:"usdt_amount"`
        USPTRatio       float64   `gorm:"type:decimal(18,8);not null;default:0" json:"usdt_ratio"`
        Sequence        int       `gorm:"not null;default:0" json:"sequence"` // order sequence for anti-abuse check
        CreatedAt       time.Time `json:"created_at"`
        FinishedAt      *time.Time `json:"finished_at"`
}

func (RechargeOrder) TableName() string { return "recharge_order" }

// ── Withdraw Order Model (sharded by user_id) ──

// WithdrawOrder represents the withdraw_order table (sharded).
type WithdrawOrder struct {
        ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
        OrderNo         string     `gorm:"size:64;uniqueIndex;not null" json:"order_no"`
        UserID          int64      `gorm:"not null;index" json:"user_id"`
        ChannelID       int64      `gorm:"not null;default:0" json:"channel_id"`
        Amount          float64    `gorm:"type:decimal(18,4);not null;default:0" json:"amount"`
        Fee             float64    `gorm:"type:decimal(18,4);not null;default:0" json:"fee"`
        RealAmount      float64    `gorm:"type:decimal(18,4);not null;default:0" json:"real_amount"`
        AccountType     int        `gorm:"not null;default:0" json:"account_type"`
        Account         string     `gorm:"size:256;not null;default:\"\" json:"account"`
        AccountName     string     `gorm:"size:128;not null;default:\"\" json:"account_name"`
        BankCode        string     `gorm:"size:32;not null;default:\"\" json:"bank_code"`
        Status          int8       `gorm:"type:tinyint;not null;default:0" json:"status"`
        Reason          string     `gorm:"size:512;not null;default:\"\" json:"reason"`
        PaymentChannel  string     `gorm:"size:64;not null;default:\"\" json:"payment_channel"`
        PaymentType     int        `gorm:"not null;default:0" json:"payment_type"`
        ThirdOrderNo    string     `gorm:"size:128;not null;default:\"\" json:"third_order_no"`
        ExtJSON         string     `gorm:"type:text;not null;default:\"{}\"" json:"ext_json"`
        CreatedAt       time.Time  `json:"created_at"`
        FinishedAt      *time.Time `json:"finished_at"`
}

func (WithdrawOrder) TableName() string { return "withdraw_order" }

// ── User Payment Account Model (sharded by user_id) ──

// UserPaymentAccount represents user payment accounts for withdrawal.
type UserPaymentAccount struct {
        ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        UserID       int64     `gorm:"not null;index" json:"user_id"`
        AccountType  int       `gorm:"not null;default:0" json:"account_type"` // 0=bank, 1=usdt, 2=upi
        Title        string    `gorm:"size:64;not null;default:\"\" json:"title"`
        Account      string    `gorm:"size:256;not null" json:"account"`
        Code         string    `gorm:"size:64;not null;default:\"\" json:"code"` // bank code / UPI handle
        Username     string    `gorm:"size:128;not null;default:\"\" json:"username"` // account holder name
        ModifyCount  int       `gorm:"not null;default:0" json:"modify_count"`
        CreatedAt    time.Time `json:"created_at"`
        UpdatedAt    time.Time `json:"updated_at"`
}

func (UserPaymentAccount) TableName() string { return "user_payment_account" }
