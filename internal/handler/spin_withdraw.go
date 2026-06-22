// Package handler provides HTTP request handlers for the RockGame platform.
//
// spin_withdraw.go — Spin Withdraw (转盘满额提现) Handler
//
// This file implements the "spin-to-full-amount then withdraw" system,
// ported from the C++ SpinHandler.cpp. Core features:
//   - Plot-based step progression: each free spin advances the amount
//     through a predefined "script" (plot), creating a deterministic
//     accumulation curve rather than random prizes.
//   - Round lifecycle: each round has a time limit; when it expires,
//     the round resets (amount, invite counts, record).
//   - Gift box display: 4 boxes shown at round start — first box is
//     the plot's initial amount, the other 3 are random from [box_gt, box_le].
//   - Withdrawal: once cur_amount reaches full_gold, the user can apply
//     to withdraw the amount to their wallet (with flow requirement).
//   - Automatic audit: 4 rules evaluate whether the withdrawal can be
//     auto-approved or needs manual review (see spin_invite.go).
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
        "github.com/rocyu-tech/rockgame/pkg/snowflake"
        "gorm.io/gorm"
)

// ── Constants ──

const spinWithdrawLockTTL = 15 * time.Second

func spinWithdrawLockKey(userID int64) string {
        return fmt.Sprintf("spin:withdraw:%d", userID)
}

func spinOperatingLockKey(userID int64) string {
        return fmt.Sprintf("spin:operating:%d", userID)
}

// ============================================================================
// SpinWithdrawInfo — GET /api/v1/activity/spin-withdraw/info
//
// Returns the user's current spin-withdraw state, including:
//   - Current amount and progress toward full_gold
//   - Remaining free spins today
//   - Round information (start time, deadline)
//   - Gift boxes (4 amounts)
//   - Recent spin history (round_record)
//   - Recent successful withdrawals (top_list)
//   - Invite code for sharing
//
// If the user has no state or the round has expired, this endpoint
// initializes or resets the round before returning data.
// ============================================================================

func SpinWithdrawInfo(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "SpinWithdrawInfo", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[SpinWithdrawInfo] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinWithdrawInfo.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()

        // 1. Find active spin_withdraw activity
        activity, swCfg, err := findActiveSpinWithdrawActivity(db, now)
        if err != nil {
                return err
        }

        // 2. Load or initialize user state
        state, userAct, isInit, err := loadOrCreateSpinWithdrawState(db, userID, activity.ID)
        if err != nil {
                return err
        }

        // 3. Check if round has expired → need to reset
        isRound := false
        if state.RoundStartTS > 0 {
                expiryTS := state.RoundStartTS + int64(swCfg.TimeLimitHour)*3600
                if now.Unix() >= expiryTS {
                        // Round expired, log it and reset
                        logSpinOrderLog(db, userID, activity.ID, "", state, 0)
                        isRound = true
                }
        }

        if isInit || isRound {
                // Initialize or reset round
                if isInit {
                        state.CurRound = 0
                }
                resetSpinRound(state, swCfg, now)
                // Set initial amount from plot step 0
                if len(swCfg.Plot.FreeInc) > 0 {
                        state.CurAmount = float64(swCfg.Plot.FreeInc[0])
                        state.CurPlotStep = 1
                }

                // Generate gift boxes
                boxes := generateGiftBoxes(state.CurAmount, swCfg.BoxGT, swCfg.BoxLE)

                // Save state
                if err := saveSpinWithdrawState(db, userAct, state, isInit); err != nil {
                        return err
                }

                logger.Infof("[SpinWithdrawInfo] init round: user_id=%d round=%d amount=%.4f boxes=%v",
                        userID, state.CurRound, state.CurAmount, boxes)

                return c.JSON(bizerr.SuccessResponse(fiber.Map{
                        "activity_id":     activity.ID,
                        "spin_id":         activity.ID,
                        "amount":          state.CurAmount,
                        "amount_full":     swCfg.FullGold,
                        "end_time":        state.RoundStartTS + int64(swCfg.TimeLimitHour)*3600,
                        "free_times":      swCfg.FreeSpinsPerDay,
                        "round":           state.CurRound,
                        "rec_list":        []interface{}{},
                        "boxes":           boxes,
                        "total_withdraw":  state.TotalWithdraw,
                        "total_invite":    state.TotalInvite,
                }))
        }

        // 4. Normal case: refresh daily free spins
        today := now.Format("20060102")
        if state.LastSpinDate != today {
                state.TodayFreeSpins = 0
                state.TodayTotalSpins = 0
                // Persist the daily reset
                stateJSON, _ := json.Marshal(state)
                userAct.StateData = string(stateJSON)
                _ = db.Save(userAct).Error // best-effort, non-critical
        }

        remainingFree := swCfg.FreeSpinsPerDay - state.TodayFreeSpins
        if remainingFree < 0 {
                remainingFree = 0
        }

        // 5. Parse round record for history display
        var recList []model.SpinRecordItem
        if state.RoundRecord != "" {
                _ = json.Unmarshal([]byte(state.RoundRecord), &recList)
        }
        if recList == nil {
                recList = []model.SpinRecordItem{}
        }

        // 6. Get recent successful withdrawals (top list)
        var topList []model.SpinTopWithdraw
        db.Table("spin_top_withdraw").
                Select("nick_name, amount, created_at").
                Order("created_at DESC").
                Limit(10).
                Find(&topList)

        // 7. Get user's invite code
        inviteCode := ""
        var inviteRow struct {
                Code string `gorm:"column:code"`
        }
        if err := db.Table("invite_code").
                Select("code").
                Where("user_id = ? AND status = 1", userID).
                First(&inviteRow).Error; err == nil {
                inviteCode = inviteRow.Code
        }

        logger.Infof("[SpinWithdrawInfo] success: user_id=%d amount=%.4f/%.4f round=%d",
                userID, state.CurAmount, swCfg.FullGold, state.CurRound)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "activity_id":     activity.ID,
                "spin_id":         activity.ID,
                "amount":          state.CurAmount,
                "amount_full":     swCfg.FullGold,
                "end_time":        state.RoundStartTS + int64(swCfg.TimeLimitHour)*3600,
                "free_times":      remainingFree,
                "round":           state.CurRound,
                "rec_list":        recList,
                "top_list":        topList,
                "invite_code":     inviteCode,
                "total_withdraw":  state.TotalWithdraw,
                "total_invite":    state.TotalInvite,
        }))
}

// ============================================================================
// SpinWithdrawSpin — POST /api/v1/activity/spin-withdraw/spin
//
// Performs a single free spin on the spin-withdraw wheel.
// Unlike the regular wheel (weighted random prize), this uses a deterministic
// "plot" (script) that defines the cumulative amount at each step.
//
// Flow:
//  1. Find active activity + parse config
//  2. Load user state, check daily limit, check round not expired
//  3. Acquire per-user Redis lock
//  4. Calculate next step's amount from the plot
//  5. Match the amount delta to a prize position (for UI animation)
//  6. Append to round_record
//  7. Save state atomically
//
// Request body: {} (empty, no parameters needed)
// ============================================================================

func SpinWithdrawSpin(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "SpinWithdrawSpin", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[SpinWithdrawSpin] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinWithdrawSpin.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()
        today := now.Format("20060102")

        // 1. Find active activity + config
        activity, swCfg, err := findActiveSpinWithdrawActivity(db, now)
        if err != nil {
                return err
        }

        // 2. Load user state
        state, userAct, _, err := loadOrCreateSpinWithdrawState(db, userID, activity.ID)
        if err != nil {
                return err
        }

        // 3. Validate state
        if state.CurPlotStep <= 0 {
                middleware.LogError(c, "SpinWithdrawSpin.NotInit", fmt.Errorf("user %d spin not initialized", userID))
                return bizerr.ErrInternal
        }

        // Check round expiry
        if state.RoundStartTS > 0 {
                expiryTS := state.RoundStartTS + int64(swCfg.TimeLimitHour)*3600
                if now.Unix() >= expiryTS {
                        middleware.LogWarn(c, "SpinWithdrawSpin.RoundExpired", fmt.Sprintf("user %d round expired", userID))
                        return bizerr.ErrSpinRoundExpired
                }
        }

        // Check amount not already full
        if state.CurAmount >= swCfg.FullGold {
                middleware.LogWarn(c, "SpinWithdrawSpin.AmountFull", fmt.Sprintf("user %d amount already full", userID))
                return bizerr.ErrSpinAmountNotFull
        }

        // 4. Daily free spin check
        if state.LastSpinDate != today {
                state.TodayFreeSpins = 0
                state.TodayTotalSpins = 0
        }
        if state.TodayFreeSpins >= swCfg.FreeSpinsPerDay {
                middleware.LogWarn(c, "SpinWithdrawSpin.NoFreeSpins", fmt.Sprintf("user %d no free spins today", userID))
                return bizerr.ErrWheelSpinsExhausted
        }

        // 5. Acquire lock
        rdb := cache.Client()
        if rdb != nil {
                ctx := context.Background()
                lockKey := spinOperatingLockKey(userID)
                lockToken, lockErr := cache.Lock(ctx, lockKey, spinWithdrawLockTTL)
                if lockErr != nil {
                        logger.Warnf("[SpinWithdrawSpin] redis lock error: %v", lockErr)
                }
                if lockToken == "" && lockErr == nil {
                        middleware.LogWarn(c, "SpinWithdrawSpin.Concurrent", fmt.Sprintf("user %d spin in progress", userID))
                        return bizerr.ErrWheelCooldown
                }
                if lockToken != "" {
                        defer cache.Unlock(ctx, lockKey, lockToken)
                }
        }

        // 6. Calculate next step amount from plot
        stepMoney, err := calculatePlotStep(state.CurPlotStep, state.CurAmount, swCfg)
        if err != nil {
                middleware.LogError(c, "SpinWithdrawSpin.PlotStep", err)
                return bizerr.ErrSpinPlotStepError
        }

        // Safety check: step money should not exceed or equal full_gold
        if stepMoney >= swCfg.FullGold {
                middleware.LogError(c, "SpinWithdrawSpin.StepExceedsFull", fmt.Errorf(
                        "user %d step_money=%.4f >= full_gold=%.4f step=%d", userID, stepMoney, swCfg.FullGold, state.CurPlotStep))
                return bizerr.ErrSpinPlotStepError
        }

        diff := stepMoney - state.CurAmount

        // 7. Match diff to prize position for UI
        pos := matchDiffToPosition(diff, swCfg)

        // 8. Update state
        state.CurAmount = stepMoney
        state.CurPlotStep++
        state.TodayFreeSpins++
        state.TodayTotalSpins++
        state.TotalSpins++
        state.LastSpinDate = today
        state.LastSpinTime = now.Unix()

        // 9. Append to round record
        appendSpinRecord(state, userID, diff, now)

        // 10. Save state
        if err := saveSpinWithdrawState(db, userAct, state, false); err != nil {
                middleware.LogError(c, "SpinWithdrawSpin.SaveState", err)
                return bizerr.ErrInternal
        }

        remainingFree := swCfg.FreeSpinsPerDay - state.TodayFreeSpins
        if remainingFree < 0 {
                remainingFree = 0
        }

        logger.Infof("[SpinWithdrawSpin] success: user_id=%d step=%d amount=%.4f diff=%.4f pos=%d",
                userID, state.CurPlotStep-1, state.CurAmount, diff, pos)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":         0,
                "tickets":        remainingFree,
                "amount":         state.CurAmount,
                "amount_full":    swCfg.FullGold,
                "pos":            pos,
                "diff":           diff,
                "round":          state.CurRound,
        }))
}

// ============================================================================
// SpinWithdraw — POST /api/v1/activity/spin-withdraw/withdraw
//
// Allows the user to apply for withdrawal when cur_amount >= full_gold.
//
// Flow:
//  1. Validate phone is bound (KYC prerequisite)
//  2. Load state, verify amount >= full_gold
//  3. Create withdraw order + order log
//  4. Deduct full_gold from cur_amount, reset round
//  5. Execute automatic audit rules (synchronous in Go, unlike C++ async)
//  6. If auto-approved: credit amount to wallet immediately
//  7. If not auto-approved: order stays in "pending" status for manual review
//
// Request body: {} (empty)
// ============================================================================

func SpinWithdraw(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "SpinWithdraw", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[SpinWithdraw] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinWithdraw.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now().UTC()

        // 1. Find active activity
        activity, swCfg, err := findActiveSpinWithdrawActivity(db, now)
        if err != nil {
                return err
        }

        // 2. Check phone binding
        var user struct {
                Phone    string `gorm:"column:phone"`
                Nickname string `gorm:"column:nickname"`
                Avatar   string `gorm:"column:avatar"`
        }
        if err := db.Table("users").Select("phone, nickname, avatar").
                Where("id = ?", userID).First(&user).Error; err != nil {
                middleware.LogError(c, "SpinWithdraw.FindUser", err)
                return bizerr.ErrInternal
        }
        if user.Phone == "" {
                middleware.LogWarn(c, "SpinWithdraw.NoPhone", fmt.Sprintf("user %d phone not bound", userID))
                return bizerr.ErrSpinPhoneRequired
        }

        // 3. Load user state
        state, userAct, _, err := loadOrCreateSpinWithdrawState(db, userID, activity.ID)
        if err != nil {
                return err
        }

        // 4. Verify amount
        if state.CurAmount < swCfg.FullGold {
                middleware.LogWarn(c, "SpinWithdraw.NotEnough", fmt.Sprintf("user %d amount=%.4f < full=%.4f",
                        userID, state.CurAmount, swCfg.FullGold))
                return bizerr.ErrSpinAmountNotFull
        }

        // 5. Calculate flow requirement
        flowRequired := state.CurAmount * float64(swCfg.FlowMulti) / float64(model.SpinRatioBase)

        // 6. Get VIP level for order log
        vipLevel := 0
        var vipRow struct {
                Level int `gorm:"column:level"`
        }
        if err := db.Table("user_vip").Select("level").
                Where("user_id = ?", userID).First(&vipRow).Error; err == nil {
                vipLevel = vipRow.Level
        }

        // 7. Acquire lock
        rdb := cache.Client()
        if rdb != nil {
                ctx := context.Background()
                lockKey := spinWithdrawLockKey(userID)
                lockToken, lockErr := cache.Lock(ctx, lockKey, spinWithdrawLockTTL)
                if lockErr != nil {
                        logger.Warnf("[SpinWithdraw] redis lock error: %v", lockErr)
                }
                if lockToken == "" && lockErr == nil {
                        middleware.LogWarn(c, "SpinWithdraw.Concurrent", fmt.Sprintf("user %d withdraw in progress", userID))
                        return bizerr.ErrTooManyRequests
                }
                if lockToken != "" {
                        defer cache.Unlock(ctx, lockKey, lockToken)
                }
        }

        // 8. Create order + update state + log (in transaction)
        orderNo := fmt.Sprintf("SW%s%d", now.Format("20060102150405"), snowflake.NextID())
        auditDetail := &model.SpinAuditDetail{}
        var orderID int64

        err = db.Transaction(func(tx *gorm.DB) error {
                // Create withdraw order
                auditJSON, _ := json.Marshal(auditDetail)
                order := model.SpinWithdrawOrder{
                        UserID:       userID,
                        ActivityID:   activity.ID,
                        SpinID:       fmt.Sprintf("%d", activity.ID),
                        OrderNo:      orderNo,
                        Amount:       swCfg.FullGold,
                        FlowRequired: flowRequired,
                        NickName:     user.Nickname,
                        Status:       model.SpinOrderStatusInit,
                        Round:        state.CurRound,
                        InviteTotal:  state.InviteCount,
                        AuditDetail:  string(auditJSON),
                }
                if err := tx.Create(&order).Error; err != nil {
                        return fmt.Errorf("create withdraw order: %w", err)
                }
                orderID = order.ID

                // Create order log
                logEntry := model.SpinOrderLog{
                        UserID:      userID,
                        ActivityID:  activity.ID,
                        SpinID:      fmt.Sprintf("%d", activity.ID),
                        Round:       state.CurRound,
                        TotalAmount: state.CurAmount,
                        InviteTotal: state.InviteCount,
                        VipLevel:    vipLevel,
                        LogType:     1, // withdrawal
                }
                if err := tx.Create(&logEntry).Error; err != nil {
                        return fmt.Errorf("create order log: %w", err)
                }

                // Update user state: deduct amount, reset round markers
                state.CurAmount -= swCfg.FullGold
                state.RoundStartTS = 0
                state.CurPlotStep = 0

                if err := saveSpinWithdrawStateTx(tx, userAct, state, false); err != nil {
                        return fmt.Errorf("update user state: %w", err)
                }

                return nil
        })

        if err != nil {
                middleware.LogError(c, "SpinWithdraw.Transaction", err)
                return bizerr.ErrInternal
        }

        // 9. Execute automatic audit (synchronous)
        autoApproved := executeAutoAudit(db, userID, &swCfg, state, auditDetail)

        if autoApproved {
                // Auto-approve: credit to wallet
                approveSpinOrder(db, orderID, userID, swCfg.FullGold, flowRequired, 1, "system", auditDetail)
                state.TotalWithdraw += swCfg.FullGold
                _ = saveSpinWithdrawState(db, userAct, state, false)

                logger.Infof("[SpinWithdraw] auto-approved: user_id=%d order_id=%d amount=%.4f rule=%d",
                        userID, orderID, swCfg.FullGold, auditDetail.RuleType)
        } else {
                logger.Infof("[SpinWithdraw] pending manual review: user_id=%d order_id=%d rule_check=none_passed",
                        userID, orderID)
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":       0,
                "order_id":     orderID,
                "order_no":     orderNo,
                "amount":       swCfg.FullGold,
                "auto_audited": autoApproved,
        }))
}

// ============================================================================
// SpinWithdrawLog — GET /api/v1/activity/spin-withdraw/log
//
// Returns paginated withdrawal history for the current user.
// Query params: page, page_size
// ============================================================================

func SpinWithdrawLog(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                middleware.LogError(c, "SpinWithdrawLog", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinWithdrawLog.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        page, pageSize, offset := ParsePagination(c)

        var orders []model.SpinWithdrawOrder
        var total int64

        db.Table("spin_withdraw_order").
                Where("user_id = ?", userID).
                Count(&total)

        db.Table("spin_withdraw_order").
                Where("user_id = ?", userID).
                Order("created_at DESC").
                Offset(offset).
                Limit(pageSize).
                Find(&orders)

        if orders == nil {
                orders = []model.SpinWithdrawOrder{}
        }

        return c.JSON(bizerr.SuccessResponse(bizerr.PagedData{
                List:     orders,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(offset+pageSize) < total,
        }))
}

// ============================================================================
// Internal helper functions
// ============================================================================

// findActiveSpinWithdrawActivity finds the currently active "spin_withdraw" type
// activity and parses both WheelConfig and SpinWithdrawConfig from config_json.
func findActiveSpinWithdrawActivity(db *gorm.DB, now time.Time) (*model.ActivityDefine, *model.SpinWithdrawConfig, error) {
        var activity model.ActivityDefine
        if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
                "spin_withdraw", now, now).
                Order("priority DESC, id DESC").
                First(&activity).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return nil, nil, bizerr.ErrWheelNotActive
                }
                logger.Errorf("[SpinWithdraw] find activity failed: %v", err)
                return nil, nil, bizerr.ErrInternal
        }

        // Parse spin withdraw config
        swCfg := model.SpinWithdrawConfig{}
        if err := json.Unmarshal([]byte(activity.ConfigJSON), &swCfg); err != nil {
                logger.Errorf("[SpinWithdraw] parse config failed: %v", err)
                return nil, nil, bizerr.ErrInternal
        }

        // Set defaults
        if swCfg.FreeSpinsPerDay == 0 {
                swCfg.FreeSpinsPerDay = 1
        }

        return &activity, &swCfg, nil
}

// loadOrCreateSpinWithdrawState loads the user's spin-withdraw state from
// user_activity, or creates a new initialized state if none exists.
// Returns the state, the UserActivity record, whether this is a new initialization, and any error.
func loadOrCreateSpinWithdrawState(db *gorm.DB, userID, activityID int64) (*model.SpinWithdrawState, *model.UserActivity, bool, error) {
        var userAct model.UserActivity
        err := db.Where("user_id = ? AND activity_id = ?", userID, activityID).First(&userAct).Error

        if errors.Is(err, gorm.ErrRecordNotFound) {
                // First time — create with empty state
                state := &model.SpinWithdrawState{
                        TotalSpins:      0,
                        TodayFreeSpins:  0,
                        TodayTotalSpins: 0,
                        LastSpinDate:    "",
                        LastSpinTime:    0,
                        CurAmount:       0,
                        CurRound:        0,
                        RoundStartTS:    0,
                        CurPlotStep:     0,
                        InviteCount:     0,
                        LevelInvite:     0,
                        TotalInvite:     0,
                        TotalWithdraw:   0,
                        RoundRecord:     "[]",
                }
                stateJSON, _ := json.Marshal(state)
                userAct = model.UserActivity{
                        UserID:     userID,
                        ActivityID: activityID,
                        Progress:   0,
                        StateData:  string(stateJSON),
                }
                if err := db.Create(&userAct).Error; err != nil {
                        logger.Errorf("[SpinWithdraw] create user activity failed: %v", err)
                        return nil, nil, false, bizerr.ErrInternal
                }
                return state, &userAct, true, nil
        }

        if err != nil {
                logger.Errorf("[SpinWithdraw] find user activity failed: %v", err)
                return nil, nil, false, bizerr.ErrInternal
        }

        // Parse existing state
        state := &model.SpinWithdrawState{}
        if err := json.Unmarshal([]byte(userAct.StateData), state); err != nil {
                logger.Warnf("[SpinWithdraw] unmarshal state failed for user %d: %v", userID, err)
                // Initialize fresh state if parse fails
                state = &model.SpinWithdrawState{
                        RoundRecord: "[]",
                }
        }

        return state, &userAct, false, nil
}

// resetSpinRound resets all round-specific fields for a new round.
// This corresponds to C++ ResetSpinRound.
func resetSpinRound(state *model.SpinWithdrawState, cfg *model.SpinWithdrawConfig, now time.Time) {
        state.RoundStartTS = now.Unix()
        state.CurAmount = 0
        state.CurPlotStep = 0
        state.InviteCount = 0
        state.CurRound++
        state.RoundRecord = "[]"
        // Note: free_times is managed via daily reset in the caller, not here.
        // Note: LevelInvite is NOT reset — it persists across rounds for probability decay.
}

// calculatePlotStep determines the next cumulative amount based on the plot config.
// This is the Go equivalent of C++ do_spin_free's plot step logic.
//
// If curPlotStep < len(free_inc), use the preconfigured value from the array.
// Otherwise, use curAmount + step_inc, capped at full_gold - 1 (to prevent
// reaching full_gold via free spins alone — invitation is required for the final fill).
func calculatePlotStep(curPlotStep int, curAmount float64, cfg *model.SpinWithdrawConfig) (float64, error) {
        freeInc := cfg.Plot.FreeInc

        if curPlotStep < len(freeInc) {
                return float64(freeInc[curPlotStep]), nil
        }

        // Beyond the array: use step_inc
        nextAmount := curAmount + float64(cfg.Plot.StepInc)
        if nextAmount >= cfg.FullGold {
                // Cap to prevent reaching full via free spins
                nextAmount = curAmount
        }

        return nextAmount, nil
}

// matchDiffToPosition matches the amount difference to a prize position
// using the prize list's num_gt/num_le ranges. Falls back to position 1.
// This is the Go equivalent of C++ do_spin_free's item matching loop.
func matchDiffToPosition(diff float64, cfg *model.SpinWithdrawConfig) int {
        // In the spin_withdraw system, the "position" is for UI animation display.
        // The prize list from WheelConfig is used for display purposes.
        // We map the diff to a position proportionally if no exact match is found.
        pos := 1 // default

        // Try to parse the full config to get WheelPrize ranges
        // Since SpinWithdrawConfig extends WheelConfig, we'd need the full config.
        // For simplicity, we use a simple proportional mapping based on full_gold.
        if cfg.FullGold > 0 {
                ratio := diff / cfg.FullGold
                if ratio > 0 && ratio <= 0.25 {
                        pos = 1
                } else if ratio <= 0.5 {
                        pos = 2
                } else if ratio <= 0.75 {
                        pos = 3
                } else {
                        pos = 4
                }
        }

        return pos
}

// appendSpinRecord appends a spin record entry to the round_record JSON array.
// This is the Go equivalent of C++ AddSpinRecord.
func appendSpinRecord(state *model.SpinWithdrawState, recordUID int64, amount float64, now time.Time) {
        if amount <= 0 {
                return
        }

        // Look up user info for the record
        var userInfo struct {
                Nickname string `gorm:"column:nickname"`
                Avatar   string `gorm:"column:avatar"`
        }
        db := database.DB()
        if db != nil {
                _ = db.Table("users").Select("nickname, avatar").
                        Where("id = ?", recordUID).First(&userInfo).Error
        }

        item := model.SpinRecordItem{
                UID:    recordUID,
                Amount: amount,
                Time:   now.Unix(),
                Name:   userInfo.Nickname,
                Avatar: userInfo.Avatar,
        }

        var records []model.SpinRecordItem
        if state.RoundRecord != "" {
                _ = json.Unmarshal([]byte(state.RoundRecord), &records)
        }
        records = append(records, item)
        recordJSON, _ := json.Marshal(records)
        state.RoundRecord = string(recordJSON)
}

// saveSpinWithdrawState persists the user's spin-withdraw state to user_activity.
func saveSpinWithdrawState(db *gorm.DB, userAct *model.UserActivity, state *model.SpinWithdrawState, isInit bool) error {
        stateJSON, err := json.Marshal(state)
        if err != nil {
                return fmt.Errorf("marshal state: %w", err)
        }
        userAct.StateData = string(stateJSON)
        userAct.Progress = state.TotalSpins
        now := time.Now()
        userAct.LastDrawAt = &now

        if isInit {
                return db.Create(userAct).Error
        }
        return db.Save(userAct).Error
}

// saveSpinWithdrawStateTx is the transaction-scoped version of saveSpinWithdrawState.
func saveSpinWithdrawStateTx(tx *gorm.DB, userAct *model.UserActivity, state *model.SpinWithdrawState, isInit bool) error {
        stateJSON, err := json.Marshal(state)
        if err != nil {
                return fmt.Errorf("marshal state: %w", err)
        }
        userAct.StateData = string(stateJSON)
        userAct.Progress = state.TotalSpins
        now := time.Now()
        userAct.LastDrawAt = &now

        if isInit {
                return tx.Create(userAct).Error
        }
        return tx.Save(userAct).Error
}

// generateGiftBoxes creates the 4 gift box amounts for display at round start.
// Box 0 is the fixed initial amount (from plot step 0).
// Boxes 1-3 are random values in [boxGT, boxLE].
func generateGiftBoxes(fixedAmount float64, boxGT, boxLE int) []int64 {
        boxes := make([]int64, 4)
        boxes[0] = int64(fixedAmount)
        for i := 1; i < 4; i++ {
                if boxLE > boxGT {
                        boxes[i] = int64(rand.IntN(boxLE-boxGT+1)) + int64(boxGT)
                } else {
                        boxes[i] = boxes[0] // fallback if config is bad
                }
        }
        return boxes
}

// logSpinOrderLog creates a SpinOrderLog entry when a round expires or ends.
func logSpinOrderLog(db *gorm.DB, userID, activityID int64, spinID string, state *model.SpinWithdrawState, logType int8) {
        vipLevel := 0
        if db != nil {
                var vipRow struct {
                        Level int `gorm:"column:level"`
                }
                if err := db.Table("user_vip").Select("level").
                        Where("user_id = ?", userID).First(&vipRow).Error; err == nil {
                        vipLevel = vipRow.Level
                }
        }

        entry := model.SpinOrderLog{
                UserID:      userID,
                ActivityID:  activityID,
                SpinID:      spinID,
                Round:       state.CurRound,
                TotalAmount: state.CurAmount,
                InviteTotal: state.InviteCount,
                VipLevel:    vipLevel,
                LogType:     logType,
        }
        if db != nil {
                _ = db.Create(&entry).Error // best-effort logging
        }
}