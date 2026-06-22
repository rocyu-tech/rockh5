package handler

// vip.go — VIP level query handlers.
//
// Endpoints:
//   - VipLevels: list all active VIP tier configurations and their benefits
//   - VipInfo:  current user's VIP level, growth points, and progress to next tier

import (
        "encoding/json"
        "errors"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// VipLevels returns all active VIP level configurations sorted by level ascending.
//
// WHAT: Queries the vip_level_config table for all tiers with status=1 and maps
// each tier's benefits JSON (a set of boolean flags) into a human-readable
// string list using friendlyBenefitName.
//
// GET /api/v1/vip/levels
func VipLevels(c *fiber.Ctx) error {
        logger.Infof("[VipLevels] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "VipLevels.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var levels []model.VipLevelConfig
        if err := db.Where("status = 1").
                Order("level ASC").
                Find(&levels).Error; err != nil {
                middleware.LogError(c, "VipLevels.Find", err)
                return bizerr.ErrInternal
        }

        // Transform each VIP level config into a client-friendly response map,
        // parsing the benefits JSON to produce a flat list of enabled benefit labels.
        list := make([]fiber.Map, 0, len(levels))
        for _, lv := range levels {
                var benefits []string
                if lv.BenefitsJSON != "" {
                        var bm map[string]interface{}
                        if err := json.Unmarshal([]byte(lv.BenefitsJSON), &bm); err != nil {
                                logger.Warnf("[VipLevels] unmarshal benefits failed for level %d: %v", lv.Level, err)
                        } else {
                                for k, v := range bm {
                                        label := friendlyBenefitName(k)
                                        if boolVal, ok := v.(bool); ok && boolVal {
                                                benefits = append(benefits, label)
                                        }
                                }
                        }
                }

                list = append(list, fiber.Map{
                        "level":              lv.Level,
                        "name":               lv.Name,
                        "growth_required":    lv.GrowthRequired,
                        "icon":               lv.Icon,
                        "withdraw_fee_rate":  lv.WithdrawFeeRate,
                        "daily_signin_bonus": lv.DailySigninBonus,
                        "benefits":           benefits,
                })
        }

        logger.Infof("[VipLevels] completed: count=%d", len(list))

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "list": list,
        }))
}

// VipInfo returns the current user's VIP level, growth points, and progress toward the next level.
//
// WHAT: Reads the user's VIP record from the sharded user_vip table, looks up
// the next tier configuration, and calculates a percentage progress value.
//
// WHY the progress calculation is non-trivial:
//   - Growth points are cumulative across all tiers (e.g., VIP 2 requires 1000
//     total growth, VIP 3 requires 5000 total growth).
//   - To show progress within the CURRENT tier, we subtract the base growth
//     required for the current tier from both the user's total growth and the
//     next tier's requirement. The fraction of the remaining gap represents
//     progress (0–100%). Clamping guards against negative or >100% values
//     caused by data inconsistencies or admin config changes.
//
// Edge cases:
//   - No VIP record exists yet: returns level 0 with progress 0 and next_level
//     set to the first available tier.
//   - User is already at the maximum tier: progress is set to 100 and
//     next_level is an empty object (no further tier to upgrade to).
//
// GET /api/v1/vip/info
func VipInfo(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "VipInfo", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[VipInfo] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "VipInfo.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Query user_vip from sharded table
        shardTable := database.ShardTable("user_vip", userID)
        var userVip model.UserVip
        err := db.Table(shardTable).Where("user_id = ?", userID).First(&userVip).Error
        if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
                middleware.LogError(c, "VipInfo.QueryUserVip", err)
                return bizerr.ErrInternal
        }

        // No VIP record yet — the user is at level 0 with zero growth.
        // Show the first available tier as their next_level target.
        if errors.Is(err, gorm.ErrRecordNotFound) {
                var firstLevel model.VipLevelConfig
                if e := db.Where("status = 1 AND level = 1").First(&firstLevel).Error; e != nil {
                        middleware.LogError(c, "VipInfo.FirstLevel", e)
                        return bizerr.ErrInternal
                }

                logger.Infof("[VipInfo] completed: user_id=%d level=0 progress=0", userID)

                return c.JSON(bizerr.SuccessResponse(fiber.Map{
                        "level":    0,
                        "growth":   0,
                        "progress": 0,
                        "next_level": fiber.Map{
                                "level":           firstLevel.Level,
                                "name":            firstLevel.Name,
                                "growth_required": firstLevel.GrowthRequired,
                        },
                }))
        }

        currentLevel := userVip.Level
        currentGrowth := userVip.Growth

        // Attempt to find the next tier above the user's current level.
        var nextLevel model.VipLevelConfig
        err = db.Where("status = 1 AND level = ?", currentLevel+1).First(&nextLevel).Error

        var progress float64
        nextLevelInfo := fiber.Map{}
        if errors.Is(err, gorm.ErrRecordNotFound) {
                // User is already at the maximum configured VIP level — no further progress to show.
                progress = 100
        } else {
                // Calculate progress within the current tier as a percentage.
                //
                // Growth points are cumulative across tiers, so we need the current
                // tier's growth_required as the "base" to subtract from both the user's
                // total growth and the next tier's requirement. This yields the fraction
                // of the gap the user has covered within this tier.
                //
                // Example: current tier requires 1000, next tier requires 5000, user has 3000.
                //   base = 1000, needed = 5000 - 1000 = 4000, earned = 3000 - 1000 = 2000
                //   progress = 2000 / 4000 * 100 = 50%
                var curLevelCfg model.VipLevelConfig
                if e := db.Where("status = 1 AND level = ?", currentLevel).First(&curLevelCfg).Error; e != nil {
                        middleware.LogError(c, "VipInfo.CurLevelCfg", e)
                        return bizerr.ErrInternal
                }

                baseGrowth := curLevelCfg.GrowthRequired
                needed := nextLevel.GrowthRequired - baseGrowth
                if needed > 0 {
                        progress = float64(currentGrowth-baseGrowth) / float64(needed) * 100
                        // Clamp to [0, 100] to handle edge cases such as admin lowering
                        // growth_required after the user has already accumulated points.
                        if progress < 0 {
                                progress = 0
                        }
                        if progress > 100 {
                                progress = 100
                        }
                }
                nextLevelInfo = fiber.Map{
                        "level":           nextLevel.Level,
                        "name":            nextLevel.Name,
                        "growth_required": nextLevel.GrowthRequired,
                }
        }

        logger.Infof("[VipInfo] completed: user_id=%d level=%d growth=%d progress=%.1f",
                userID, currentLevel, currentGrowth, progress)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "level":      currentLevel,
                "growth":     currentGrowth,
                "progress":   progress,
                "next_level": nextLevelInfo,
        }))
}

// friendlyBenefitName converts a benefit JSON key to a display name.
// NOTE: Currently returns hardcoded English strings. This should use i18n
// in the future to support multiple languages based on the lang parameter.
func friendlyBenefitName(key string, lang ...string) string {
        // TODO(i18n): Integrate with i18n system to return localized benefit names.
        // Default to "en" when no lang is provided.
        _ = lang
        names := map[string]string{
                "max_withdraw_daily": "Daily Withdraw Limit",
                "birthday_bonus":     "Birthday Bonus",
                "exclusive_activity": "Exclusive Activities",
                "exclusive_support":  "VIP Dedicated Support",
                "all_benefits":       "All Benefits Unlocked",
        }
        if name, ok := names[key]; ok {
                return name
        }
        return key
}