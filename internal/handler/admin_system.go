// Admin system configuration handlers.
//
// This file provides read-only handlers for admin-facing system configuration
// data: payment channels, VIP level definitions, and the admin user roster.
// All handlers exclude sensitive fields (e.g. payment secrets, password hashes)
// by using explicit SELECT columns or dedicated DTO structs.
package handler

import (
        "errors"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// PaymentChannelItem represents a payment channel in API responses.
// Intentionally omits sensitive fields such as API keys, secrets, and
// webhook URLs that exist in the database table.
type PaymentChannelItem struct {
        ID               int64   `json:"id"`
        Name             string  `json:"name"`
        Type             string  `json:"type"`
        SupportedRegions string  `json:"supported_regions"`
        MinAmount        float64 `json:"min_amount"`
        MaxAmount        float64 `json:"max_amount"`
        SortOrder        int     `json:"sort_order"`
        Status           int8    `json:"status"`
}

// GetAdminPaymentChannels returns all payment channels sorted by sort_order.
// Only safe, non-sensitive columns are selected — credentials and webhook
// URLs are excluded from the response.
//
// GET /api/v1/admin/system/payment-channels
func GetAdminPaymentChannels(c *fiber.Ctx) error {
        logger.Infof("[GetAdminPaymentChannels] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminPaymentChannels.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var channels []PaymentChannelItem
        if err := db.Table("payment_channel").
                Select("id, name, type, supported_regions, min_amount, max_amount, sort_order, status").
                Order("sort_order ASC, id ASC").
                Find(&channels).Error; err != nil {
                middleware.LogError(c, "GetAdminPaymentChannels.Find", err)
                return bizerr.ErrInternal
        }

        // Ensure an empty array (not null) is returned when no channels exist
        if channels == nil {
                channels = []PaymentChannelItem{}
        }

        return c.JSON(bizerr.SuccessResponse(channels))
}

// VIPLevelItem represents a VIP level configuration in API responses.
// Contains the business parameters that define each tier: growth threshold,
// withdraw fee rate, and daily sign-in bonus.
type VIPLevelItem struct {
        ID               int64   `json:"id"`
        Level            int     `json:"level"`
        Name             string  `json:"name"`
        GrowthRequired   int64   `json:"growth_required"`
        WithdrawFeeRate  float64 `json:"withdraw_fee_rate"`
        DailySigninBonus float64 `json:"daily_signin_bonus"`
        Status           int8    `json:"status"`
}

// GetAdminVIPLevels returns all VIP level configurations ordered by level.
// These define the tiered benefits system (e.g. fee discounts, sign-in bonuses)
// that apply to players based on their growth points.
//
// GET /api/v1/admin/system/vip-levels
func GetAdminVIPLevels(c *fiber.Ctx) error {
        logger.Infof("[GetAdminVIPLevels] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminVIPLevels.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var levels []VIPLevelItem
        if err := db.Table("vip_level_config").
                Select("id, level, name, growth_required, withdraw_fee_rate, daily_signin_bonus, status").
                Order("level ASC").
                Find(&levels).Error; err != nil {
                middleware.LogError(c, "GetAdminVIPLevels.Find", err)
                return bizerr.ErrInternal
        }

        // Ensure an empty array (not null) is returned when no levels exist
        if levels == nil {
                levels = []VIPLevelItem{}
        }

        return c.JSON(bizerr.SuccessResponse(levels))
}

// AdminUserListItem represents an admin user in the list response.
// Password hash is intentionally excluded for security.
type AdminUserListItem struct {
        ID          int64  `json:"id"`
        Username    string `json:"username"`
        RealName    string `json:"real_name"`
        Email       string `json:"email"`
        Role        string `json:"role"`
        Status      int8   `json:"status"`
        LastLoginAt string `json:"last_login_at" gorm:"column:last_login_at"`
        CreatedAt   string `json:"created_at"`
}

// GetAdminAdminUsers returns all admin users (without password hashes) ordered
// by ID. Used by the admin panel to display the team roster and manage roles.
//
// GET /api/v1/admin/system/admin-users
func GetAdminAdminUsers(c *fiber.Ctx) error {
        logger.Infof("[GetAdminAdminUsers] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminAdminUsers.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var admins []AdminUserListItem
        if err := db.Table("admin_user").
                Select("id, username, real_name, email, role, status, last_login_at, created_at").
                Order("id ASC").
                Find(&admins).Error; err != nil {
                middleware.LogError(c, "GetAdminAdminUsers.Find", err)
                return bizerr.ErrInternal
        }

        // Ensure an empty array (not null) is returned when no admins exist
        if admins == nil {
                admins = []AdminUserListItem{}
        }

        return c.JSON(bizerr.SuccessResponse(admins))
}