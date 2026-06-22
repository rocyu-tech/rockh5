package model

import "time"

// ── Lobby/CMS Models ──

// LobbyConfig represents the lobby_config table
type LobbyConfig struct {
        ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        LobbyName string    `gorm:"size:64;not null" json:"lobby_name"`
        ParentID  int64     `gorm:"not null;default:0" json:"parent_id"`
        SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
        Status    int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        Language  string    `gorm:"size:10;not null;default:'en'" json:"language"`
        Icon      string    `gorm:"size:512;not null;default:''" json:"icon"`
        CreatedAt time.Time `json:"created_at"`
        UpdatedAt time.Time `json:"updated_at"`
}

func (LobbyConfig) TableName() string { return "lobby_config" }

// Banner represents the banner table
type Banner struct {
        ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
        LobbyID    int64      `gorm:"not null;default:0" json:"lobby_id"`
        ImageURL   string     `gorm:"size:512;not null;default:''" json:"image_url"`
        LinkURL    string     `gorm:"size:512;not null;default:''" json:"link_url"`
        Weight     int        `gorm:"not null;default:0" json:"weight"`
        StartTime  *time.Time `json:"start_time"`
        EndTime    *time.Time `json:"end_time"`
        TargetLang string     `gorm:"size:10;not null;default:''" json:"target_lang"`
        Status     int8       `gorm:"type:tinyint;not null;default:1" json:"status"`
        CreatedAt  time.Time  `json:"created_at"`
        UpdatedAt  time.Time  `json:"updated_at"`
}

func (Banner) TableName() string { return "banner" }

// SplashPopup represents the splash_popup table
type SplashPopup struct {
        ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
        Title        string     `gorm:"size:128;not null;default:''" json:"title"`
        Content      string     `gorm:"type:text" json:"content"`
        ImageURL     string     `gorm:"size:512;not null;default:''" json:"image_url"`
        LinkURL      string     `gorm:"size:512;not null;default:''" json:"link_url"`
        TriggerRules string     `gorm:"type:json" json:"trigger_rules"`
        DailyLimit   int        `gorm:"not null;default:1" json:"daily_limit"`
        Priority     int        `gorm:"not null;default:0" json:"priority"`
        Status       int8       `gorm:"type:tinyint;not null;default:1" json:"status"`
        CreatedAt    time.Time  `json:"created_at"`
        UpdatedAt    time.Time  `json:"updated_at"`
}

func (SplashPopup) TableName() string { return "splash_popup" }

// GameCategory represents the game_category table
type GameCategory struct {
        ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        LobbyID   int64     `gorm:"not null;default:0" json:"lobby_id"`
        Name      string    `gorm:"size:64;not null" json:"name"`
        Icon      string    `gorm:"size:512;not null;default:''" json:"icon"`
        SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
        Status    int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        CreatedAt time.Time `json:"created_at"`
}

func (GameCategory) TableName() string { return "game_category" }

// GameInfo represents the game_info table
type GameInfo struct {
        ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        VendorID   int64     `gorm:"not null" json:"vendor_id"`
        GameID     string    `gorm:"size:64;not null" json:"game_id"`
        Name       string    `gorm:"size:128;not null" json:"name"`
        Icon       string    `gorm:"size:512;not null;default:''" json:"icon"`
        URL        string    `gorm:"size:512;not null;default:''" json:"url"`
        CategoryID int64     `gorm:"not null;default:0" json:"category_id"`
        Status     int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        SortOrder  int       `gorm:"not null;default:0" json:"sort_order"`
        Hot        int8      `gorm:"type:tinyint;not null;default:0" json:"hot"`
        New        int8      `gorm:"type:tinyint;not null;default:0" json:"new"`
        CreatedAt  time.Time `json:"created_at"`
        UpdatedAt  time.Time `json:"updated_at"`
}

func (GameInfo) TableName() string { return "game_info" }

// GameVendor represents the game_vendor table
type GameVendor struct {
        ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
        Name          string    `gorm:"size:64;not null" json:"name"`
        APIKey        string    `gorm:"size:256;not null;default:''" json:"-"`
        APISecret     string    `gorm:"size:256;not null;default:''" json:"-"`
        CallbackURL   string    `gorm:"size:512;not null;default:''" json:"callback_url"`
        Status        int8      `gorm:"type:tinyint;not null;default:1" json:"status"`
        TimeoutConfig string    `gorm:"type:json" json:"timeout_config"`
        RetryPolicy   string    `gorm:"type:json" json:"retry_policy"`
        ErrorThreshold int      `gorm:"not null;default:30" json:"error_threshold"`
        CreatedAt     time.Time `json:"created_at"`
        UpdatedAt     time.Time `json:"updated_at"`
}

func (GameVendor) TableName() string { return "game_vendor" }

// ── Response DTOs (for API serialization, excludes internal fields) ──

// BannerItem is the API response for a banner
type BannerItem struct {
        ID       int64  `json:"id"`
        Title    string `json:"title,omitempty"`
        Image    string `json:"image"`
        Link     string `json:"link,omitempty"`
        Sort     int    `json:"sort_order"`
}

// CategoryItem is the API response for a game category (supports tree)
type CategoryItem struct {
        ID       int64           `json:"id"`
        ParentID int64           `json:"parent_id"`
        Name     string          `json:"name"`
        Icon     string          `json:"icon,omitempty"`
        Sort     int             `json:"sort_order"`
        Children []CategoryItem  `json:"children,omitempty"`
}

// GameItem is the API response for a game
type GameItem struct {
        ID         int64  `json:"id"`
        Name       string `json:"name"`
        Icon       string `json:"icon"`
        Cover      string `json:"cover,omitempty"`
        CategoryID int64  `json:"category_id"`
        Vendor     string `json:"vendor,omitempty"`
        Status     int8   `json:"status"`
        Hot        int8   `json:"hot"`
        New        int8   `json:"new"`
}

// LobbyHomeData is the BFF aggregation response for the lobby home page
type LobbyHomeData struct {
        Banners   []BannerItem   `json:"banners"`
        Categories []CategoryItem `json:"categories"`
        HotGames  []GameItem     `json:"hot_games"`
        NewGames  []GameItem     `json:"new_games"`
}
