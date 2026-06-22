// Package handler provides HTTP request handlers for the RockGame platform.
//
// wheel.go — Lucky Wheel (Spin Wheel) Activity Handler
//
// This file implements the lucky wheel / spin wheel activity feature, including:
//   - Spin logic: a multi-step flow that finds the active activity, checks remaining
//     spins, acquires a distributed lock, determines a prize via weighted random
//     selection, credits the reward, and records the spin.
//   - Prize probability: weighted random selection where each prize's chance is
//     proportional to its configured weight relative to the total weight pool.
//   - Reward crediting: atomic reward delivery (bonus, coin, or item) within the
//     same database transaction as the spin, replacing the old fire-and-forget pattern.
//   - Concurrency safety: per-user Redis lock to prevent duplicate spins, plus
//     DB-level checks (e.g., atomic inventory decrement) for correctness.
//   - State queries: endpoints for retrieving the user's current wheel state,
//     recent spin history, and the activity configuration including prize pool.
package handler

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "math/rand/v2"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// ── Wheel Redis key helpers ──

// wheelSpinLockKey returns the Redis key used for the per-user spin lock.
// This prevents a single user from submitting concurrent spin requests that
// could lead to double-spending or duplicate rewards.
func wheelSpinLockKey(userID int64) string {
        // Per-user spinning lock to prevent concurrent spin requests
        return fmt.Sprintf("wheel:spinning:%d", userID)
}

// SpinWheel handles a single wheel spin request.
//
// POST /api/v1/activity/spin-wheel
// Request body (optional): {"use_free": true}  (default: auto, uses free first)
//
// Multi-step flow:
//  1. Authenticate the user and validate the database connection.
//  2. Find the currently active wheel activity (status=1, within time window).
//  3. Parse the wheel configuration (prize list, free spins, cost, limits).
//  4. Parse the optional request body to determine free vs. paid spin intent.
//  5. Load or create the user's activity record; refresh daily counters if needed.
//  6. Determine spin type (free or paid) and validate availability.
//  7. Check daily maximum spin limit.
//  8. Check cooldown between consecutive spins.
//  9. Acquire a per-user Redis lock for concurrency protection.
// 10. Inside a DB transaction:
//     a. Select a prize via weighted random (probability proportional to weight).
//     b. Check and atomically decrement inventory for limited-stock prizes.
//     c. Deduct the spin cost for paid spins.
//     d. Update the user's activity state (counters, last spin time).
//     e. Record the spin as an activity record.
//     f. Credit the reward (bonus, coin, or item) atomically within the transaction.
// 11. Build and return the response with prize details and updated state.
func SpinWheel(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "SpinWheel", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[SpinWheel] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinWheel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()
        today := now.Format("20060102")

        // 1. Find active wheel activity
        var activity model.ActivityDefine
        if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
                "wheel", now, now).
                Order("priority DESC, id DESC").
                First(&activity).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "SpinWheel.FindActivity", "no active wheel activity")
                        return bizerr.ErrWheelNotActive
                }
                middleware.LogError(c, "SpinWheel.FindActivity", err)
                return bizerr.ErrInternal
        }

        // 2. Parse wheel config
        config := model.WheelConfig{}
        if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
                middleware.LogError(c, "SpinWheel.ParseConfig", err)
                return bizerr.ErrInternal
        }
        // Default to 1 free spin per day if not explicitly configured
        if config.FreeSpinsPerDay == 0 {
                config.FreeSpinsPerDay = 1
        }

        // 3. Parse request body (optional: {"use_free": true/false})
        // Empty body is valid (defaults to auto), so we tolerate unmarshal errors gracefully
        reqBody := struct {
                UseFree *bool `json:"use_free"` // nil = auto (use free first), true = force free, false = force paid
        }{}
        bodyBytes := c.Body()
        if len(bodyBytes) > 0 {
                if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
                        middleware.LogWarn(c, "SpinWheel.ParseBody", "invalid request body")
                }
        }

        // 4. Get or create user activity record
        var userAct model.UserActivity
        err := db.Where("user_id = ? AND activity_id = ?", userID, activity.ID).First(&userAct).Error
        if errors.Is(err, gorm.ErrRecordNotFound) {
                // First spin ever for this user on this activity — initialize state
                state := model.WheelStateData{
                        TotalSpins:      0,
                        TodayFreeSpins:  0,
                        TodayTotalSpins: 0,
                        LastSpinDate:    "",
                        LastSpinTime:    0,
                }
                stateJSON, _ := json.Marshal(state)
                userAct = model.UserActivity{
                        UserID:     userID,
                        ActivityID: activity.ID,
                        Progress:   0,
                        StateData:  string(stateJSON),
                }
                if err := db.Create(&userAct).Error; err != nil {
                        middleware.LogError(c, "SpinWheel.CreateUserActivity", err)
                        return bizerr.ErrInternal
                }
        } else if err != nil {
                middleware.LogError(c, "SpinWheel.FindUserActivity", err)
                return bizerr.ErrInternal
        }

        // 5. Parse and refresh state (reset daily counters if new day)
        state := model.WheelStateData{}
        if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
                middleware.LogError(c, "SpinWheel.UnmarshalState", err)
                return bizerr.ErrInternal
        }
        state = refreshWheelDailyState(state, today)

        // 6. Determine spin type (free or paid)
        useFree := false
        if reqBody.UseFree != nil {
                useFree = *reqBody.UseFree
        } else {
                // Auto: use free spins first
                useFree = state.TodayFreeSpins < config.FreeSpinsPerDay
        }

        if useFree {
                // Check free spin availability
                if state.TodayFreeSpins >= config.FreeSpinsPerDay {
                        middleware.LogWarn(c, "SpinWheel.NoFreeSpins", fmt.Sprintf("user %d exhausted free spins today", userID))
                        return bizerr.ErrWheelSpinsExhausted
                }
        } else {
                // Paid spin: balance check moved inside the transaction (step 12)
                // to prevent race condition between check and deduction.
        }

        // 7. Check daily max spins limit
        if config.MaxSpinsPerDay > 0 && state.TodayTotalSpins >= config.MaxSpinsPerDay {
                middleware.LogWarn(c, "SpinWheel.DailyLimit", fmt.Sprintf("user %d exceeded daily spin limit", userID))
                return bizerr.ErrWheelSpinsExhausted
        }

        // 8. Check cooldown
        if config.CooldownSec > 0 && state.LastSpinTime > 0 {
                elapsed := now.Unix() - state.LastSpinTime
                if elapsed < int64(config.CooldownSec) {
                        middleware.LogWarn(c, "SpinWheel.Cooldown", fmt.Sprintf("user %d cooldown %d seconds remaining", userID, int64(config.CooldownSec)-elapsed))
                        return bizerr.ErrWheelCooldown
                }
        }

        // 9. Concurrency protection via Redis (per-user spin lock)
        // This prevents the same user from submitting multiple concurrent spin
        // requests which could otherwise bypass single-spin checks and cause
        // duplicate rewards. The lock has a 10-second TTL as a safety net.
        rdb := cache.Client()
        if rdb != nil {
                ctx := context.Background()
                lockKey := wheelSpinLockKey(userID)
                lockToken, err := cache.Lock(ctx, lockKey, 10*time.Second)
                if err != nil {
                        logger.Warnf("[Wheel] redis lock error: %v", err)
                        // Continue without lock if Redis is unavailable (DB-level checks still apply)
                }
                if lockToken == "" && err == nil {
                        // Another spin is in progress for this user
                        middleware.LogWarn(c, "SpinWheel.Concurrent", fmt.Sprintf("user %d spin already in progress", userID))
                        return bizerr.ErrWheelCooldown
                }
                if lockToken != "" {
                        // Ensure the lock is always released when the handler returns,
                        // whether the spin succeeds or fails.
                        defer cache.Unlock(ctx, lockKey, lockToken)
                }
        }

        // 10-14. Transactional: select prize + check/deduct inventory + deduct cost + update state + record spin
        // All financial and state changes must be atomic to prevent data inconsistency.
        var prizeResult *model.WheelPrize
        var prizeIndex int
        err = db.Transaction(func(tx *gorm.DB) error {
                // 10. Select prize via weighted random
                p, idx := selectWheelPrize(config.Prizes)
                if p == nil {
                        return fmt.Errorf("no prizes configured")
                }
                prizeResult = p
                prizeIndex = idx

                // 11. Check and deduct inventory for limited prizes
                // Stock > 0 means the prize has a finite quantity; Stock == 0 means unlimited.
                // The atomic decrement uses a WHERE clause (remaining > 0) to handle
                // concurrent transactions safely — if two transactions race, only one
                // will succeed and the other will see RowsAffected == 0.
                if prizeResult.Stock > 0 {
                        var inv model.ActivityInventory
                        if err := tx.Where("activity_id = ? AND reward_id = ?", activity.ID, prizeResult.ID).First(&inv).Error; err != nil {
                                if errors.Is(err, gorm.ErrRecordNotFound) {
                                        // First time this prize's inventory is accessed — initialize
                                        // with the configured stock quantity.
                                        inv = model.ActivityInventory{
                                                ActivityID: activity.ID,
                                                RewardID:   prizeResult.ID,
                                                Total:      prizeResult.Stock,
                                                Remaining:  prizeResult.Stock,
                                        }
                                        if err := tx.Create(&inv).Error; err != nil {
                                                return fmt.Errorf("create inventory: %w", err)
                                        }
                                } else {
                                        return fmt.Errorf("find inventory: %w", err)
                                }
                        }
                        if inv.Remaining <= 0 {
                                return bizerr.ErrWheelPrizeStockEmpty
                        }
                        // Atomic decrement: the WHERE remaining > 0 guard ensures that
                        // concurrent transactions cannot over-deduct below zero.
                        result := tx.Exec("UPDATE activity_inventory SET remaining = remaining - 1 WHERE id = ? AND remaining > 0", inv.ID)
                        if result.Error != nil {
                                return fmt.Errorf("decrement inventory: %w", result.Error)
                        }
                        if result.RowsAffected == 0 {
                                // Another transaction claimed the last unit before us
                                return bizerr.ErrWheelPrizeStockEmpty
                        }
                }

                // 12. Deduct cost for paid spins
                if !useFree && config.SpinCost > 0 {
                        if err := deductSpinCostTx(tx, userID, config.SpinCostType, config.SpinCost); err != nil {
                                return err
                        }
                }

                // 13. Update user state
                if useFree {
                        state.TodayFreeSpins++
                }
                state.TodayTotalSpins++
                state.TotalSpins++
                state.LastSpinDate = today
                state.LastSpinTime = now.Unix()

                stateJSON, _ := json.Marshal(state)
                userAct.StateData = string(stateJSON)
                userAct.Progress = state.TotalSpins
                nowPtr := now
                userAct.LastDrawAt = &nowPtr
                if err := tx.Save(&userAct).Error; err != nil {
                        return fmt.Errorf("update user activity state: %w", err)
                }

                // 14. Record the spin
                rewardJSON, _ := json.Marshal(prizeResult)
                record := model.ActivityRecord{
                        UserID:     userID,
                        ActivityID: activity.ID,
                        Action:     "draw",
                        RewardJSON: string(rewardJSON),
                }
                if err := tx.Create(&record).Error; err != nil {
                        return fmt.Errorf("create spin record: %w", err)
                }

                // 15. Credit reward inside the same transaction (no more fire-and-forget)
                if err := creditWheelRewardTx(tx, userID, activity.ID, prizeResult); err != nil {
                        return fmt.Errorf("credit reward: %w", err)
                }

                return nil
        })

        if err != nil {
                // Distinguish business errors from internal errors
                if errors.Is(err, bizerr.ErrWheelPrizeStockEmpty) {
                        middleware.LogWarn(c, "SpinWheel.StockEmpty", fmt.Sprintf("prize out of stock for user %d", userID))
                        return bizerr.ErrWheelPrizeStockEmpty
                }
                if bizErr, ok := err.(*bizerr.BizError); ok {
                        return bizErr
                }
                middleware.LogError(c, "SpinWheel.Transaction", err)
                return bizerr.ErrInternal
        }

        // 16. Build response
        remainingFree := config.FreeSpinsPerDay - state.TodayFreeSpins
        if remainingFree < 0 {
                remainingFree = 0
        }

        logger.Infof("[SpinWheel] completed: user_id=%d activity_id=%d prize_id=%d prize_name=%s prize_type=%s spin_type=%s prize_index=%d total_spins=%d",
                userID, activity.ID, prizeResult.ID, prizeResult.Name, prizeResult.Type,
                spinTypeLabel(useFree), prizeIndex, state.TotalSpins)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "spin_type":       spinTypeLabel(useFree),
                "prize_index":     prizeIndex,
                "prize": fiber.Map{
                        "id":     prizeResult.ID,
                        "name":   prizeResult.Name,
                        "type":   prizeResult.Type,
                        "value":  prizeResult.Value,
                        "item_id": prizeResult.ItemID,
                        "rarity": prizeResult.Rarity,
                        "icon":   prizeResult.Icon,
                },
                "total_spins":       state.TotalSpins,
                "remaining_free":    remainingFree,
                "today_total_spins": state.TodayTotalSpins,
        }))
}

// ── WheelState returns the user's current wheel state ──
// GET /api/v1/activity/spin-wheel/state
func WheelState(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "WheelState", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "WheelState.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()
        today := now.Format("20060102")

        // Find active wheel activity
        var activity model.ActivityDefine
        if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
                "wheel", now, now).
                Order("priority DESC, id DESC").
                First(&activity).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                                "has_activity": false,
                        }))
                }
                middleware.LogError(c, "WheelState.FindActivity", err)
                return bizerr.ErrInternal
        }

        // Parse config
        config := model.WheelConfig{}
        if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
                middleware.LogError(c, "WheelState.ParseConfig", err)
                return bizerr.ErrInternal
        }
        if config.FreeSpinsPerDay == 0 {
                config.FreeSpinsPerDay = 1
        }

        // Get user activity state
        var userAct model.UserActivity
        state := model.WheelStateData{}
        if err := db.Where("user_id = ? AND activity_id = ?", userID, activity.ID).First(&userAct).Error; err == nil {
                if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
                        logger.Warnf("[WheelState] unmarshal state failed for user %d: %v", userID, err)
                }
        }
        state = refreshWheelDailyState(state, today)

        remainingFree := config.FreeSpinsPerDay - state.TodayFreeSpins
        if remainingFree < 0 {
                remainingFree = 0
        }

        // Calculate cooldown remaining
        cooldownRemaining := 0
        if config.CooldownSec > 0 && state.LastSpinTime > 0 {
                elapsed := now.Unix() - state.LastSpinTime
                if elapsed < int64(config.CooldownSec) {
                        cooldownRemaining = config.CooldownSec - int(elapsed)
                }
        }

        // Calculate if paid spin is affordable
        canAffordPaid := true
        if config.SpinCost > 0 {
                hasBalance, _ := checkSpinBalance(userID, config.SpinCostType, config.SpinCost)
                canAffordPaid = hasBalance
        }

        // Check daily max spins
        dailyLimitReached := false
        if config.MaxSpinsPerDay > 0 && state.TodayTotalSpins >= config.MaxSpinsPerDay {
                dailyLimitReached = true
        }

        // Get recent spin history (last 10)
        var recentRecords []model.ActivityRecord
        db.Where("user_id = ? AND activity_id = ? AND action = ?",
                userID, activity.ID, "draw").
                Order("created_at DESC").
                Limit(10).
                Find(&recentRecords)

        history := make([]fiber.Map, 0, len(recentRecords))
        for _, rec := range recentRecords {
                var p model.WheelPrize
                if err := json.Unmarshal([]byte(rec.RewardJSON), &p); err != nil {
                        logger.Warnf("[WheelHistory] unmarshal reward failed for record %d: %v", rec.ID, err)
                        continue
                }
                history = append(history, fiber.Map{
                        "prize_name": p.Name,
                        "prize_type": p.Type,
                        "prize_rarity": p.Rarity,
                        "value":     p.Value,
                        "created_at": rec.CreatedAt,
                })
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "has_activity":      true,
                "activity_id":       activity.ID,
                "remaining_free":    remainingFree,
                "total_spins":       state.TotalSpins,
                "today_total_spins": state.TodayTotalSpins,
                "cooldown_remaining": cooldownRemaining,
                "can_afford_paid":   canAffordPaid,
                "spin_cost":         config.SpinCost,
                "spin_cost_type":    config.SpinCostType,
                "daily_limit_reached": dailyLimitReached,
                "history":           history,
        }))
}

// ── WheelConfig returns the active wheel activity configuration ──
// GET /api/v1/activity/spin-wheel/config
func WheelConfigHandler(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "WheelConfig.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()

        var activity model.ActivityDefine
        if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
                "wheel", now, now).
                Order("priority DESC, id DESC").
                First(&activity).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                                "has_activity": false,
                        }))
                }
                middleware.LogError(c, "WheelConfig.FindActivity", err)
                return bizerr.ErrInternal
        }

        config := model.WheelConfig{}
        if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
                middleware.LogError(c, "WheelConfig.ParseConfig", err)
                return bizerr.ErrInternal
        }
        if config.FreeSpinsPerDay == 0 {
                config.FreeSpinsPerDay = 1
        }

        // Build prize list with current stock info
        prizeList := make([]fiber.Map, 0, len(config.Prizes))
        for _, p := range config.Prizes {
                pm := fiber.Map{
                        "id":      p.ID,
                        "name":    p.Name,
                        "type":    p.Type,
                        "value":   p.Value,
                        "item_id": p.ItemID,
                        "weight":  p.Weight,
                        "icon":    p.Icon,
                        "rarity":  p.Rarity,
                        "stock":   p.Stock,
                }
                // For limited prizes (Stock > 0), fetch remaining stock from the
                // activity_inventory table. Unlimited prizes (Stock == 0) show -1.
                if p.Stock > 0 {
                        var inv model.ActivityInventory
                        if err := database.DB().Where("activity_id = ? AND reward_id = ?", activity.ID, p.ID).First(&inv).Error; err == nil {
                                pm["remaining"] = inv.Remaining
                        } else {
                                pm["remaining"] = p.Stock
                        }
                } else {
                        pm["remaining"] = -1 // unlimited
                }
                prizeList = append(prizeList, pm)
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "has_activity":     true,
                "activity_id":      activity.ID,
                "name":             activity.Name,
                "start_time":       activity.StartTime,
                "end_time":         activity.EndTime,
                "free_spins_per_day": config.FreeSpinsPerDay,
                "spin_cost":        config.SpinCost,
                "spin_cost_type":   config.SpinCostType,
                "cooldown_sec":     config.CooldownSec,
                "max_spins_per_day": config.MaxSpinsPerDay,
                "prizes":           prizeList,
        }))
}

// ── Internal helpers ──

// refreshWheelDailyState resets daily counters (TodayFreeSpins, TodayTotalSpins)
// when the recorded LastSpinDate differs from today. This ensures each new UTC
// day starts with a fresh set of free spins and the daily spin count is reset.
func refreshWheelDailyState(state model.WheelStateData, today string) model.WheelStateData {
        if state.LastSpinDate != today {
                state.TodayFreeSpins = 0
                state.TodayTotalSpins = 0
        }
        return state
}

// selectWheelPrize selects a prize using weighted random selection.
// Returns the selected prize and its index in the config.
//
// Probability calculation:
//   - Each prize has a Weight value. The probability of being selected is
//     prize.Weight / totalWeight, where totalWeight is the sum of all
//     non-zero weights in the prize pool.
//   - A random integer in [0, totalWeight) is generated, then we walk the
//     prize list accumulating weights until the cumulative sum exceeds the
//     roll value. This is the classic "cumulative weight" algorithm.
//   - Prizes with Weight <= 0 are skipped entirely (effectively zero probability).
//   - If all weights are zero or the total weight is zero, the function falls
//     back to equal distribution (uniform random) across all prizes.
func selectWheelPrize(prizes []model.WheelPrize) (*model.WheelPrize, int) {
        if len(prizes) == 0 {
                return nil, -1
        }

        // Calculate total weight across all prizes with positive weight
        totalWeight := 0
        for _, p := range prizes {
                if p.Weight > 0 {
                        totalWeight += p.Weight
                }
        }

        if totalWeight == 0 {
                // Fallback: equal distribution when no weights are configured
                idx := rand.IntN(len(prizes))
                return &prizes[idx], idx
        }

        // Weighted random selection using the cumulative weight algorithm.
        // Example: prizes with weights [10, 30, 60] → totalWeight=100.
        // Roll in [0,100): 0-9 → prize 0 (10%), 10-39 → prize 1 (30%), 40-99 → prize 2 (60%).
        roll := rand.IntN(totalWeight)
        cumulative := 0
        for i, p := range prizes {
                if p.Weight <= 0 {
                        continue // Skip zero-weight prizes (probability = 0)
                }
                cumulative += p.Weight
                if roll < cumulative {
                        return &prizes[i], i
                }
        }

        // Fallback: return last prize (should not normally be reached due to
        // floating-point/exact integer arithmetic; here as a safety net)
        return &prizes[len(prizes)-1], len(prizes) - 1
}

// checkSpinBalance checks if the user has enough balance for a paid spin.
// The costType determines which wallet column to query ("coin" → cash_balance,
// anything else → bonus_balance).
func checkSpinBalance(userID int64, costType string, cost float64) (bool, error) {
        db := database.DB()
        if db == nil {
                return false, fmt.Errorf("database not initialized")
        }

        var balance float64
        switch costType {
        case "coin":
                // NOTE: schema uses cash_balance (not coin_balance)
                err := db.Raw("SELECT cash_balance FROM user_wallet WHERE user_id = ?", userID).Scan(&balance).Error
                if err != nil {
                        return false, err
                }
        case "bonus":
                fallthrough
        default:
                err := db.Raw("SELECT bonus_balance FROM user_wallet WHERE user_id = ?", userID).Scan(&balance).Error
                if err != nil {
                        return false, err
                }
        }

        return balance >= cost, nil
}

// deductSpinCost deducts the spin cost from the user's wallet.
func deductSpinCost(userID int64, costType string, cost float64) error {
        db := database.DB()
        if db == nil {
                return fmt.Errorf("database not initialized")
        }
        return deductSpinCostTx(db, userID, costType, cost)
}

// deductSpinCostTx deducts the spin cost within an existing transaction.
// Uses allowlisted column names to prevent SQL injection.
// The costType maps to a fixed column ("coin" → cash_balance, else → bonus_balance),
// so the column name is never interpolated from user input.
func deductSpinCostTx(tx *gorm.DB, userID int64, costType string, cost float64) error {
        var column string
        switch costType {
        case "coin":
                column = "cash_balance"
        default:
                column = "bonus_balance"
        }

        // Use allowlisted column name (not fmt.Sprintf with user input) to prevent SQL injection.
        // The WHERE column >= cost guard prevents negative balances; RowsAffected == 0
        // indicates either insufficient funds or a missing wallet row.
        result := tx.Exec(
                "UPDATE user_wallet SET "+column+" = "+column+" - ?, updated_at = NOW() WHERE user_id = ? AND "+column+" >= ?",
                cost, userID, cost,
        )
        if result.Error != nil {
                return result.Error
        }
        if result.RowsAffected == 0 {
                return fmt.Errorf("insufficient balance or wallet not found")
        }
        return nil
}

// creditWheelRewardTx credits the wheel prize to the user inside an existing transaction.
// This replaces the old fire-and-forget goroutine to ensure reward consistency.
//
// Reward types:
//   - "bonus": adds prize.Value to bonus_balance.
//   - "coin":  adds prize.Value to cash_balance.
//   - "item":  inserts/updates a row in user_inventory.
//   - "empty": no reward, returns immediately.
//
// For wallet updates, if the wallet row doesn't exist (RowsAffected == 0),
// an UPSERT (INSERT ... ON DUPLICATE KEY UPDATE) is used to atomically create it.
func creditWheelRewardTx(tx *gorm.DB, userID int64, activityID int64, prize *model.WheelPrize) error {
        if prize == nil || prize.Type == "empty" {
                return nil
        }

        switch prize.Type {
        case "bonus":
                result := tx.Exec("UPDATE user_wallet SET bonus_balance = bonus_balance + ?, updated_at = NOW() WHERE user_id = ?",
                        prize.Value, userID)
                if result.Error != nil {
                        return fmt.Errorf("update bonus_balance: %w", result.Error)
                }
                if result.RowsAffected == 0 {
                        // Wallet doesn't exist — create atomically via UPSERT.
                        // ON DUPLICATE KEY UPDATE handles the race where another transaction
                        // creates the wallet between the failed UPDATE and this INSERT.
                        if err := tx.Exec("INSERT INTO user_wallet (user_id, cash_balance, bonus_balance, created_at, updated_at) VALUES (?, 0, ?, NOW(), NOW()) "+
                                "ON DUPLICATE KEY UPDATE bonus_balance = bonus_balance + ?",
                                userID, prize.Value, prize.Value).Error; err != nil {
                                return fmt.Errorf("insert wallet for bonus: %w", err)
                        }
                }
                logger.Infof("[Wheel] creditReward: user_id=%d activity_id=%d type=bonus value=%.4f",
                        userID, activityID, prize.Value)

        case "coin":
                // NOTE: schema uses cash_balance (not coin_balance)
                result := tx.Exec("UPDATE user_wallet SET cash_balance = cash_balance + ?, updated_at = NOW() WHERE user_id = ?",
                        prize.Value, userID)
                if result.Error != nil {
                        return fmt.Errorf("update cash_balance: %w", result.Error)
                }
                if result.RowsAffected == 0 {
                        // Wallet doesn't exist — create atomically via UPSERT
                        if err := tx.Exec("INSERT INTO user_wallet (user_id, cash_balance, bonus_balance, created_at, updated_at) VALUES (?, ?, 0, NOW(), NOW()) "+
                                "ON DUPLICATE KEY UPDATE cash_balance = cash_balance + ?",
                                userID, prize.Value, prize.Value).Error; err != nil {
                                return fmt.Errorf("insert wallet for coin: %w", err)
                        }
                }
                logger.Infof("[Wheel] creditReward: user_id=%d activity_id=%d type=coin value=%.4f",
                        userID, activityID, prize.Value)

        case "item":
                // Use UPSERT to atomically insert or increment item quantity.
                // ON DUPLICATE KEY UPDATE handles the case where the user already
                // owns this item, avoiding a unique constraint violation.
                if err := tx.Exec("INSERT INTO user_inventory (user_id, item_id, quantity, source, created_at) VALUES (?, ?, 1, 'wheel', NOW()) "+
                        "ON DUPLICATE KEY UPDATE quantity = quantity + 1, updated_at = NOW()",
                        userID, prize.ItemID).Error; err != nil {
                        return fmt.Errorf("insert inventory: %w", err)
                }
                logger.Infof("[Wheel] creditReward: user_id=%d activity_id=%d type=item item_id=%d",
                        userID, activityID, prize.ItemID)

        default:
                logger.Infof("[Wheel] creditReward: unsupported reward type=%s for user_id=%d", prize.Type, userID)
        }
        return nil
}

// spinTypeLabel returns a human-readable label for the spin type.
func spinTypeLabel(useFree bool) string {
        if useFree {
                return "free"
        }
        return "paid"
}