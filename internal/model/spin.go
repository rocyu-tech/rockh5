package model

import "time"

// ── Constants ──

const (
	SpinRATIO_BASE    = 10000 // probability base (10000 = 100%)
	SpinCURRENCY_BASE = 100   // internal amount to display conversion

	// Spin user type
	SpinUserTypeAll   = 0 // all users (exclude tagged)
	SpinUserTypePart  = 1 // only show to tagged users
	SpinUserTypeUID   = 2 // only specific UIDs

	// Spin order status
	SpinOrderPending  = 0
	SpinOrderApproved = 1
	SpinOrderDelayed  = 2
	SpinOrderRejected = 3

	// Audit rule type
	AuditRuleNone = 0
	AuditRule1    = 1 // recent invitee recharge check
	AuditRule2    = 2 // recharging user with sufficient flow
	AuditRule3    = 3 // non-recharger with non-recharging invitees
	AuditRule4    = 4 // suspect label check

	// Spin error codes (business)
	SpinErrSuccess       = 0
	SpinErrServerError   = 1
	SpinErrDayLimit      = 2
	SpinErrNoChance      = 3
	SpinErrAmountLimit   = 4
	SpinErrBindPhone     = 5
	SpinErrUserDataError = 6
)

// ── Spin Config Models ──

// SpinConfig represents the spin_config table — main wheel configuration.
// One row defines a complete spin wheel variant identified by SpinID.
type SpinConfig struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SpinID              string    `gorm:"size:32;uniqueIndex;not null" json:"spin_id"`
	FullGold            int64     `gorm:"not null;default:0" json:"full_gold"`
	FlowMulti           int       `gorm:"not null;default:0" json:"flow_multi"`
	TimeLimitHour       int       `gorm:"not null;default:72" json:"time_limit_hour"`
	AuditUserCnt        int       `gorm:"not null;default:-1" json:"audit_usercnt"`
	AuditRecharge       int64     `gorm:"not null;default:0" json:"audit_recharge"`
	Rule2InviteTotalLT  int       `gorm:"not null;default:0" json:"audit_rule_2_invitetotal_lt"`
	Rule2FlowMulti      int64     `gorm:"not null;default:0" json:"audit_rule_2_flowmutil"`
	Rule3InviteTotalGE  int       `gorm:"not null;default:0" json:"audit_rule_3_invtetotal_ge"`
	Rule4Users          int       `gorm:"not null;default:-1" json:"audit_rule_4_users"`
	Rule4Labels         string    `gorm:"size:256;not null;default:''" json:"audit_rule_4_labels"`
	StartTime           int64     `gorm:"not null;default:0" json:"start_time"`
	EndTime             int64     `gorm:"not null;default:0" json:"end_time"`
	UserType            int       `gorm:"not null;default:0" json:"user_type"`
	TagList             string    `gorm:"size:512;not null;default:''" json:"tag_list"`
	UserList            string    `gorm:"size:512;not null;default:''" json:"user_list"`
	PlotList            string    `gorm:"size:256;not null;default:''" json:"plot_list"`
	InviteGroupID       int       `gorm:"not null;default:0" json:"invite_group_id"`
	Priority            int       `gorm:"not null;default:0" json:"priority"`
	BoxGT               int       `gorm:"not null;default:0" json:"box_gt"`
	BoxLE               int       `gorm:"not null;default:0" json:"box_le"`
	ItemsJSON           string    `gorm:"type:text;not null;default:'[]'" json:"items_json"`
	Status              int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (SpinConfig) TableName() string { return "spin_config" }

// SpinItem represents a single item/segment on the spin wheel.
// The position is determined by matching the step increment diff against [NumGT, NumLE].
type SpinItem struct {
	ID     int   `json:"id"`
	PropID int   `json:"prop_id"`
	NumGT  int64 `json:"num_gt"`
	NumLE  int64 `json:"num_le"`
}

// SpinPlotConfig represents the spin_plot_config table — plot/script configurations.
// Each plot defines a pre-determined sequence of cumulative amounts (the "script").
type SpinPlotConfig struct {
	ID      int    `gorm:"primaryKey;autoIncrement" json:"id"`
	StepInc int    `gorm:"not null;default:0" json:"step_inc"`
	FreeInc string `gorm:"type:text;not null;default:'[]'" json:"free_inc"`
	Status  int8   `gorm:"type:tinyint;not null;default:1" json:"status"`
}

func (SpinPlotConfig) TableName() string { return "spin_plot_config" }

// SpinInviteConfig represents the spin_invite_config table — invite probability settings.
// Multiple rows per group, one per VIP level.
type SpinInviteConfig struct {
	ID           int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID      int   `gorm:"not null;index" json:"group_id"`
	VIPLevel     int   `gorm:"not null;default:0" json:"vip"`
	NewCount     int   `gorm:"not null;default:0" json:"new_count"`
	NewRatio     int   `gorm:"not null;default:0" json:"new_ratio"`
	DefaultRatio int   `gorm:"not null;default:0" json:"default_ratio"`
	ReduceRatio  int   `gorm:"not null;default:0" json:"reduce_ratio"`
	BaseRatio    int   `gorm:"not null;default:0" json:"base_ratio"`
	MaxCount     int   `gorm:"not null;default:0" json:"max_count"`
	MaxAmount    int64 `gorm:"not null;default:0" json:"max_amount"`
}

func (SpinInviteConfig) TableName() string { return "spin_invite_config" }

// SpinPosterConfig represents the spin_poster_config table — poster sharing configurations.
type SpinPosterConfig struct {
	ID            int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Language      string `gorm:"size:10;not null;default:'en'" json:"language"`
	ShareURL      string `gorm:"size:512;not null;default:''" json:"share_url"`
	TelegramURL   string `gorm:"size:512;not null;default:''" json:"telegram_url"`
	WhatsappURL   string `gorm:"size:512;not null;default:''" json:"whatsapp_url"`
	ShareURLPrefix string `gorm:"size:256;not null;default:''" json:"share_url_prefix"`
	PostersJSON   string `gorm:"type:text;not null;default:'[]'" json:"posters_json"`
	Status        int8   `gorm:"type:tinyint;not null;default:1" json:"status"`
}

func (SpinPosterConfig) TableName() string { return "spin_poster_config" }

// SpinPosterItem represents a single poster in the posters list.
type SpinPosterItem struct {
	ID             int    `json:"id"`
	Type           int    `json:"type"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	ShareTitle     string `json:"share_title"`
	ShareTextFront string `json:"share_text_front"`
	ShareTextAfter string `json:"share_text_after"`
}

// ── User Spin Data Models ──

// UserSpinData represents the user_spin_data table — per-user spin wheel state.
// Each user has one row tracking their current round progress.
type UserSpinData struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         int64     `gorm:"uniqueIndex;not null" json:"user_id"`
	SpinID         string    `gorm:"size:32;not null;default:''" json:"spin_id"`
	CurRound       int       `gorm:"not null;default:0" json:"cur_round"`
	RoundStartTS   int64     `gorm:"not null;default:0" json:"round_start_ts"`
	CurAmount      int64     `gorm:"not null;default:0" json:"cur_amount"`
	CurPlotStep    int       `gorm:"not null;default:0" json:"cur_plot_step"`
	PlotID         int       `gorm:"not null;default:0" json:"plot_id"`
	FreeTimes      int       `gorm:"not null;default:1" json:"free_times"`
	LastFreeSpinTS int64     `gorm:"not null;default:0" json:"last_free_spin_ts"`
	InviteCount    int       `gorm:"not null;default:0" json:"invite_count"`
	TotalInvite    int       `gorm:"not null;default:0" json:"total_invite"`
	LevelInvite    int       `gorm:"not null;default:0" json:"level_invite"`
	TotalWithdraw  int64     `gorm:"not null;default:0" json:"total_withdraw"`
	RoundRecord    string    `gorm:"type:text;not null;default:'[]'" json:"round_record"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (UserSpinData) TableName() string { return "user_spin_data" }

// SpinRecord represents a single spin record in the round_record JSON array.
type SpinRecord struct {
	UID        int64 `json:"uid"`
	InviteUID  int64 `json:"invite_uid"`
	Amount     int64 `json:"amount"`
	Record     int64 `json:"record"`
	Total      int64 `json:"total"`
	SpinID     string `json:"spin_id"`
	IsFirst    bool  `json:"is_first"`
	Timestamp  int64 `json:"timestamp"`
}

// ── Spin Withdraw Order Models ──

// SpinWithdrawOrder represents the spin_withdraw_order table — withdrawal requests.
type SpinWithdrawOrder struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderNo       string    `gorm:"size:64;uniqueIndex;not null" json:"order_no"`
	UserID        int64     `gorm:"not null;index" json:"user_id"`
	NickName      string    `gorm:"size:64;not null;default:''" json:"nick_name"`
	Amount        int64     `gorm:"not null;default:0" json:"amount"`
	Flow          int64     `gorm:"not null;default:0" json:"flow"`
	Round         int       `gorm:"not null;default:0" json:"round"`
	SpinID        string    `gorm:"size:32;not null;default:''" json:"spin_id"`
	Status        int8      `gorm:"type:tinyint;not null;default:0" json:"status"`
	AuditUID      int64     `gorm:"not null;default:0" json:"audit_uid"`
	AuditName     string    `gorm:"size:64;not null;default:''" json:"audit_name"`
	Reason        string    `gorm:"size:512;not null;default:''" json:"reason"`
	AuditRuleType int       `gorm:"not null;default:0" json:"audit_rule_type"`
	AuditJSON     string    `gorm:"type:text;not null;default:'{}'" json:"audit_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SpinWithdrawOrder) TableName() string { return "spin_withdraw_order" }

// SpinOrderLog represents the spin_order_log table — order lifecycle audit trail.
type SpinOrderLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID   int64     `gorm:"not null;index" json:"order_id"`
	UserID    int64     `gorm:"not null" json:"user_id"`
	Type      int8      `gorm:"type:tinyint;not null;default:1" json:"type"` // 1=withdraw, 2=audit
	Detail    string    `gorm:"type:text;not null;default:''" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (SpinOrderLog) TableName() string { return "spin_order_log" }

// SpinAuditData represents the audit JSON stored in SpinWithdrawOrder.AuditJSON.
// Tracks which audit rule was applied and the data used for the decision.
type SpinAuditData struct {
	AuditRuleType      int   `json:"audit_rule_type"`
	BlackLabelArray    string `json:"blackLabelArray"`
	SuspectNumber      int    `json:"suspect_number"`
	TotalFlow          int64  `json:"total_flow"`
	TotalRecharge      int64  `json:"toatal_recharge"`
	InviteTotal        int    `json:"invite_total"`
	InviteTotalRecharge int64 `json:"invite_total_recharge"`
}