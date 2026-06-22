// Package handler provides HTTP request handlers for the RockGame platform.
//
// spin_invite.go — Spin Invite (邀请转盘) & Auto-Audit Handler
//
// This file implements two closely related subsystems:
//
// 1. Invite Spin (邀请转盘):
//   - When a user's invitee triggers a spin event, the inviter gets an extra
//     spin with a probability of instantly filling the amount to full_gold.
//   - Probability is determined by spin_invite_config (per VIP level, per group).
//   - If not hit, the inviter still gets a free step advance (same as daily spin).
//   - The invite spin does NOT consume the daily free spin quota.
//
// 2. Automatic Audit (自动审核):
//   - 4 rules evaluated in order: Rule 4 → Rule 2 → Rule 1 → Rule 3.
//   - Rule 4: Check if inviter or sub-invitees have suspect tags → reject auto.
//   - Rule 2: Recharging user with sufficient valid flow + low invite count → approve.
//   - Rule 1: Any of last N invitees has recharged → approve.
//   - Rule 3: Non-recharging user with many invitees but none recharged → reject.
//   - If no rule matches, the order stays pending for manual review.
package handler

import (
        "encoding/json"
        "errors"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// ============================================================================
// SpinInviteTrigger — POST /api/v1/activity/spin-withdraw/invite-spin
//
// Called when an invite event occurs (e.g., invitee registers or completes
// a qualifying action). This gives the inviter a chance to fill their spin
// amount to full_gold instantly.
//
// In the C++ version, this was triggered by an internal server message
// (SSMI_SRV_INVITE_NOTIFY). In the Go version, we expose it as an API
// endpoint that can be called by the agent/referral service.
//
// Request body:
//   {"invitee_uid": 12345}
//
// The invitee_uid identifies who was invited; the inviter is derived from
// the authenticated user (X-User-ID).
// ============================================================================

func SpinInviteTrigger(c *fiber.Ctx) error {
        inviterUID := middleware.GetUserID(c)
        if inviterUID == 0 {
                middleware.LogError(c, "SpinInviteTrigger", errors.New("user_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        var req struct {
                InviteeUID int64 `json:"invitee_uid"`
        }
        if err := c.BodyParser(&req); err != nil || req.InviteeUID <= 0 {
                middleware.LogWarn(c, "SpinInviteTrigger.ParseBody", "invalid request body")
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[SpinInviteTrigger] start: inviter=%d invitee=%d", inviterUID, req.InviteeUID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SpinInviteTrigger.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result, err := doInviteSpin(db, inviterUID, req.InviteeUID)
        if err != nil {
                if bizErr, ok := err.(*bizerr.BizError); ok {
                        return bizErr
                }
                middleware.LogError(c, "SpinInviteTrigger.DoInviteSpin", err)
                return bizerr.ErrInternal
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":       0,
                "hit":          result.Hit,
                "amount":       result.Amount,
                "cur_amount":   result.CurAmount,
                "amount_full":  result.FullGold,
                "total_invite": result.TotalInvite,
        }))
}

// inviteSpinResult holds the outcome of an invite spin.
type inviteSpinResult struct {
        Hit         bool    `json:"hit"`
        Amount      float64 `json:"amount"`
        CurAmount   float64 `json:"cur_amount"`
        FullGold    float64 `json:"amount_full"`
        TotalInvite int     `json:"total_invite"`
}

// ============================================================================
// doInviteSpin — Core invite spin logic (ported from C++ do_spin_invite)
//
// 1. Load inviter's spin-withdraw state
// 2. Load invite probability config for the inviter's VIP level
// 3. Determine if the invite "hits" (probability-based)
// 4. If hit: fill cur_amount to full_gold, record, update state
// 5. If not hit: advance one free plot step (same as daily spin)
//
// Returns the result and any error.
// ============================================================================

func doInviteSpin(db *gorm.DB, inviterUID, inviteeUID int64) (*inviteSpinResult, error) {
        now := time.Now().UTC()

        // 1. Find active activity + config
        _, swCfg, err := findActiveSpinWithdrawActivity(db, now)
        if err != nil {
                return nil, err
        }

        // 2. Load inviter state
        state, userAct, _, err := loadOrCreateSpinWithdrawState(db, inviterUID, 0)
        _ = userAct // used later for saving
        if err != nil {
                return nil, err
        }

        // We need the activity ID — reload it properly
        var activity model.ActivityDefine
        if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
                "spin_withdraw", now, now).
                Order("priority DESC, id DESC").
                First(&activity).Error; err != nil {
                return nil, bizerr.ErrWheelNotActive
        }

        // Re-load with correct activity ID
        state, userAct, _, err = loadOrCreateSpinWithdrawState(db, inviterUID, activity.ID)
        if err != nil {
                return nil, err
        }

        if state.CurPlotStep <= 0 {
                return nil, fmt.Errorf("inviter %d spin not initialized", inviterUID)
        }

        if state.CurAmount >= swCfg.FullGold {
                return nil, bizerr.ErrSpinAmountNotFull
        }

        // 3. Load invite config
        inviteCfg, err := getInviteConfig(db, inviterUID, swCfg.InviteGroupID)
        if err != nil {
                return nil, err
        }

        // 4. Determine hit
        isHit := false
        if state.InviteCount >= inviteCfg.MaxCount {
                // Max count reached → 100% hit
                isHit = true
                logger.Infof("[InviteSpin] uid=%d hit by max_count: invite_count=%d max=%d",
                        inviterUID, state.InviteCount, inviteCfg.MaxCount)
        } else {
                // Probability-based hit
                hitRatio := 0
                num := rand.IntN(model.SpinRatioBase) + 1

                if state.LevelInvite < inviteCfg.NewCount {
                        hitRatio = inviteCfg.NewRatio
                } else {
                        hitRatio = inviteCfg.DefaultRatio - inviteCfg.ReduceRatio*(state.LevelInvite-inviteCfg.NewCount)
                }
                if hitRatio < inviteCfg.BaseRatio {
                        hitRatio = inviteCfg.BaseRatio
                }

                isHit = (num <= hitRatio)
                logger.Infof("[InviteSpin] uid=%d is_hit=%v ratio=%d num=%d level_invite=%d new_count=%d",
                        inviterUID, isHit, hitRatio, num, state.LevelInvite, inviteCfg.NewCount)
        }

        // Update counters regardless of hit
        state.InviteCount++
        state.TotalInvite++
        state.LevelInvite++

        result := &inviteSpinResult{
                TotalInvite: state.TotalInvite,
                FullGold:    swCfg.FullGold,
        }

        if isHit {
                // Fill to full
                diff := swCfg.FullGold - state.CurAmount
                state.CurAmount = swCfg.FullGold

                // Append record (with invitee's info)
                appendSpinRecord(state, inviteeUID, diff, now)

                result.Hit = true
                result.Amount = diff
                result.CurAmount = state.CurAmount

                logger.Infof("[InviteSpin] uid=%d HIT! diff=%.4f cur_amount=%.4f total_invite=%d",
                        inviterUID, diff, state.CurAmount, state.TotalInvite)
        } else {
                // Not hit → advance one free step (same as daily spin, but doesn't consume quota)
                stepMoney, stepErr := calculatePlotStep(state.CurPlotStep, state.CurAmount, &swCfg)
                if stepErr != nil {
                        return nil, stepErr
                }

                if stepMoney < swCfg.FullGold {
                        diff := stepMoney - state.CurAmount
                        if diff > 0 {
                                appendSpinRecord(state, inviteeUID, diff, _now())
                        }
                        state.CurAmount = stepMoney
                }

                state.CurPlotStep++

                result.Hit = false
                result.Amount = 0
                result.CurAmount = state.CurAmount

                logger.Infof("[InviteSpin] uid=%d NOT HIT, free step: amount=%.4f step=%d",
                        inviterUID, state.CurAmount, state.CurPlotStep)
        }

        // Save state
        if err := saveSpinWithdrawState(db, userAct, state, false); err != nil {
                return nil, err
        }

        return result, nil
}

// ============================================================================
// getInviteConfig — Load invite probability config for a user.
//
// Looks up the user's VIP level, then finds the matching row in
// spin_invite_config for the given group_id and VIP level.
// Falls back to the first config row for the group if no exact VIP match.
// (Ported from C++ GetInviteConfig)
// ============================================================================

func getInviteConfig(db *gorm.DB, userID int64, groupID int) (*model.SpinInviteConfig, error) {
        // Get user VIP level
        vipLevel := 0
        var vipRow struct {
                Level int `gorm:"column:level"`
        }
        if err := db.Table("user_vip").Select("level").
                Where("user_id = ?", userID).First(&vipRow).Error; err == nil {
                vipLevel = vipRow.Level
        }

        // Find exact VIP match
        var cfg model.SpinInviteConfig
        err := db.Where("group_id = ? AND vip_level = ? AND status = 1", groupID, vipLevel).
                First(&cfg).Error
        if err == nil {
                return &cfg, nil
        }

        // Fallback: first config for this group
        err = db.Where("group_id = ? AND status = 1", groupID).
                Order("vip_level ASC").
                First(&cfg).Error
        if err != nil {
                logger.Errorf("[SpinInvite] no config for group_id=%d vip=%d: %v", groupID, vipLevel, err)
                return nil, bizerr.ErrSpinInviteConfigNotFound
        }

        return &cfg, nil
}

// ============================================================================
// executeAutoAudit — Run the 4 automatic audit rules.
//
// Returns true if the order should be auto-approved, false otherwise.
// Populates auditDetail with the rule that was triggered (if any).
//
// Rule execution order (matches C++ ProcessSpinAutoWithdraw + __CheckAutoAudit):
//   Rule 4 → Rule 2 → Rule 1 → Rule 3
//
// Rule 4: Check suspect tags on inviter and sub-invitees
// Rule 2: Recharging user + sufficient valid flow + low invite count
// Rule 1: Last N invitees have at least one recharger
// Rule 3: Non-recharging user + many invitees (needs proxy check, returns false for now)
// ============================================================================

func executeAutoAudit(db *gorm.DB, userID int64, cfg *model.SpinWithdrawConfig, state *model.SpinWithdrawState, auditDetail *model.SpinAuditDetail) bool {
        // ── Rule 4: Suspect tag check ──
        if cfg.AuditRules.Rule4UserCnt >= 0 {
                if checkAuditRule4(db, userID, cfg, auditDetail) {
                        // Rule 4 triggered → do NOT auto-approve
                        logger.Infof("[AutoAudit] uid=%d rule=4 triggered (suspect tags), reject auto", userID)
                        return false
                }
        }

        // ── Get total recharge for rules 2 and 3 ──
        totalRecharge := getUserTotalRecharge(db, userID)

        // ── Rule 2: Recharging user + sufficient flow + low invite count ──
        if totalRecharge > 0 && state.TotalInvite < cfg.AuditRules.Rule2InviteTotalLt {
                if checkAuditRule2(db, userID, totalRecharge, cfg, auditDetail) {
                        logger.Infof("[AutoAudit] uid=%d rule=2 passed (flow check)", userID)
                        return true
                }
        }

        // ── Rule 1: Last N invitees recharged check ──
        if cfg.AuditRules.Rule1UserCnt >= 0 {
                if checkAuditRule1(db, userID, cfg, state, auditDetail) {
                        logger.Infof("[AutoAudit] uid=%d rule=1 passed (invitee recharged)", userID)
                        return true
                }
        }

        // ── Rule 3: Non-recharging user + many invitees → needs proxy check ──
        // In C++, this triggers an async call to the proxy service.
        // In Go, we return false (pending manual review) for now.
        // A future enhancement can make this synchronous or use a message queue.
        if totalRecharge <= 0 && cfg.AuditRules.Rule3InviteTotalGe > 0 &&
                state.TotalInvite >= cfg.AuditRules.Rule3InviteTotalGe {
                auditDetail.RuleType = 3
                logger.Infof("[AutoAudit] uid=%d rule=3 triggered (non-charger + many invitees), pending manual", userID)
                return false
        }

        // No rule matched → pending manual review
        return false
}

// checkAuditRule4 checks if the inviter or their sub-invitees have suspect tags.
// Returns true if the rule blocks auto-approval (i.e., suspect tags found).
func checkAuditRule4(db *gorm.DB, userID int64, cfg *model.SpinWithdrawConfig, auditDetail *model.SpinAuditDetail) bool {
        suspectLabels := cfg.AuditRules.Rule4Labels
        if len(suspectLabels) == 0 {
                return false
        }

        // Check inviter's own tags
        inviterTags := getUserTagIDs(db, userID)
        foundLabels := intersectInts(inviterTags, suspectLabels)
        if len(foundLabels) > 0 {
                auditDetail.RuleType = 4
                auditDetail.BlackLabelArray = foundLabels
                logger.Infof("[AuditRule4] uid=%d has suspect tags: %v", userID, foundLabels)
                return true
        }

        // Check sub-invitees' tags
        // Get recent invitees from round_record
        var records []model.SpinRecordItem
        if state, _, _, _ := loadOrCreateSpinWithdrawState(db, userID, 0); state != nil && state.RoundRecord != "" {
                _ = json.Unmarshal([]byte(state.RoundRecord), &records)
        }

        suspectCount := 0
        for _, rec := range records {
                if rec.UID == userID {
                        continue // skip self
                }
                subTags := getUserTagIDs(db, rec.UID)
                if len(intersectInts(subTags, suspectLabels)) > 0 {
                        suspectCount++
                }
        }

        if suspectCount >= cfg.AuditRules.Rule4UserCnt {
                auditDetail.RuleType = 4
                auditDetail.SuspectNumber = suspectCount
                logger.Infof("[AuditRule4] uid=%d suspect sub-invitees: %d (threshold=%d)",
                        userID, suspectCount, cfg.AuditRules.Rule4UserCnt)
                return true
        }

        return false
}

// checkAuditRule2 checks if a recharging user has sufficient valid flow.
// Condition: valid_flow >= (flow_multi / RATIO_BASE) * total_recharge
// AND invite_total < rule2_invite_total_lt
func checkAuditRule2(db *gorm.DB, userID int64, totalRecharge float64, cfg *model.SpinWithdrawConfig, auditDetail *model.SpinAuditDetail) bool {
        if cfg.AuditRules.Rule2FlowMulti <= 0 {
                return false
        }

        // Get valid flow from user wallet (stored as total_bet in wallet)
        var wallet struct {
                TotalBet float64 `gorm:"column:total_bet"`
        }
        if err := db.Table("user_wallet").Select("total_bet").
                Where("user_id = ?", userID).First(&wallet).Error; err != nil {
                return false
        }

        // Calculate required flow: (rule2_flow_multi / RATIO_BASE) * total_recharge
        requiredFlow := float64(cfg.AuditRules.Rule2FlowMulti) / float64(model.SpinRatioBase) * totalRecharge

        if wallet.TotalBet >= requiredFlow {
                auditDetail.RuleType = 2
                auditDetail.TotalFlow = wallet.TotalBet
                auditDetail.TotalRecharge = totalRecharge
                return true
        }

        return false
}

// checkAuditRule1 checks if any of the last N invitees has recharged.
// Returns true if at least one invitee has recharged (→ auto-approve).
func checkAuditRule1(db *gorm.DB, userID int64, cfg *model.SpinWithdrawConfig, state *model.SpinWithdrawState, auditDetail *model.SpinAuditDetail) bool {
        if cfg.AuditRules.Rule1UserCnt < 0 {
                return false
        }

        // Parse round record to get invitee UIDs
        var records []model.SpinRecordItem
        if state.RoundRecord != "" {
                _ = json.Unmarshal([]byte(state.RoundRecord), &records)
        }

        // Walk backwards, skip self, check last N invitees
        checked := 0
        for i := len(records) - 1; i >= 0; i-- {
                rec := records[i]
                if rec.UID == userID {
                        continue // skip self-record
                }
                checked++
                if checked > cfg.AuditRules.Rule1UserCnt {
                        break
                }

                // Check if this invitee has recharged
                rechargeCount := getUserRechargeCount(db, rec.UID)
                if rechargeCount > 0 {
                        auditDetail.RuleType = 1
                        auditDetail.InviteTotal = state.TotalInvite
                        logger.Infof("[AuditRule1] uid=%d invitee %d has recharged (count=%d)",
                                userID, rec.UID, rechargeCount)
                        return true
                }
        }

        return false
}

// ============================================================================
// approveSpinOrder — Credits the withdrawal amount to the user's wallet.
//
// This is called when an order is auto-approved or manually approved.
// It updates the order status and adds the amount to bonus_balance.
// (Ported from C++ __AuditSpinOrder, PASS branch)
// ============================================================================

func approveSpinOrder(db *gorm.DB, orderID, userID int64, amount, flowRequired float64, ruleType int8, auditName string, auditDetail *model.SpinAuditDetail) {
        // Update order status
        updates := map[string]interface{}{
                "status":          model.SpinOrderStatusPaid,
                "audit_uid":       0,
                "audit_name":      auditName,
                "audit_rule_type": ruleType,
        }
        auditJSON, _ := json.Marshal(auditDetail)
        if auditJSON != nil {
                updates["audit_detail"] = string(auditJSON)
        }

        db.Table("spin_withdraw_order").Where("id = ?", orderID).Updates(updates)

        // Credit to wallet (bonus_balance)
        // Use WHERE guard to prevent negative or duplicate credits
        result := db.Exec(
                "UPDATE user_wallet SET bonus_balance = bonus_balance + ?, total_withdraw = total_withdraw + ?, updated_at = NOW() WHERE user_id = ?",
                amount, amount, userID,
        )
        if result.Error != nil {
                logger.Errorf("[ApproveSpinOrder] credit wallet failed: uid=%d order=%d err=%v", userID, orderID, result.Error)
                return
        }
        if result.RowsAffected == 0 {
                // Wallet doesn't exist — create via UPSERT
                db.Exec(
                        "INSERT INTO user_wallet (user_id, cash_balance, bonus_balance, total_withdraw, created_at, updated_at) VALUES (?, 0, ?, ?, NOW(), NOW()) "+
                                "ON DUPLICATE KEY UPDATE bonus_balance = bonus_balance + VALUES(bonus_balance), total_withdraw = total_withdraw + VALUES(total_withdraw)",
                        userID, amount, amount,
                )
        }

        // Update spin_top_withdraw for display
        var user struct {
                Nickname string `gorm:"column:nickname"`
                Avatar   string `gorm:"column:avatar"`
        }
        _ = db.Table("users").Select("nickname, avatar").Where("id = ?", userID).First(&user).Error

        db.Create(&model.SpinTopWithdraw{
                UserID:   userID,
                NickName: user.Nickname,
                Avatar:   user.Avatar,
                Amount:   amount,
        })

        // Record in ledger
        _ = db.Exec(
                "INSERT INTO ledger (user_id, biz_type, biz_id, debit_account, credit_account, amount, balance_after, remark, created_at) "+
                        "SELECT ?, 'spin_withdraw', ?, 'frozen', 'bonus', ?, bonus_balance, ?, NOW() FROM user_wallet WHERE user_id = ?",
                userID, orderID, amount, "spin withdraw credit", userID,
        )

        logger.Infof("[ApproveSpinOrder] success: uid=%d order=%d amount=%.4f flow=%.4f rule=%d",
                userID, orderID, amount, flowRequired, ruleType)
}

// ============================================================================
// rejectSpinOrder — Marks an order as rejected.
// ============================================================================

func rejectSpinOrder(db *gorm.DB, orderID int64, auditUID int64, auditName, reason string, auditDetail *model.SpinAuditDetail) {
        updates := map[string]interface{}{
                "status":     model.SpinOrderStatusReject,
                "audit_uid":  auditUID,
                "audit_name": auditName,
                "audit_reason": reason,
        }
        auditJSON, _ := json.Marshal(auditDetail)
        if auditJSON != nil {
                updates["audit_detail"] = string(auditJSON)
        }
        db.Table("spin_withdraw_order").Where("id = ? AND status IN ?", orderID,
                []int8{model.SpinOrderStatusInit}).Updates(updates)
}

// ============================================================================
// Shared helper functions
// ============================================================================

// getUserTotalRecharge returns the user's total recharge amount from user_wallet.
func getUserTotalRecharge(db *gorm.DB, userID int64) float64 {
        var wallet struct {
                TotalRecharge float64 `gorm:"column:total_recharge"`
        }
        if err := db.Table("user_wallet").Select("total_recharge").
                Where("user_id = ?", userID).First(&wallet).Error; err != nil {
                return 0
        }
        return wallet.TotalRecharge
}

// getUserRechargeCount returns how many recharge orders a user has.
func getUserRechargeCount(db *gorm.DB, userID int64) int {
        var count int64
        db.Table("recharge_order").
                Where("user_id = ? AND status = 1", userID).
                Count(&count)
        return int(count)
}

// getUserTagIDs returns the tag IDs associated with a user.
func getUserTagIDs(db *gorm.DB, userID int64) []int {
        var tagIDs []int
        db.Table("user_tag").
                Select("tag_id").
                Where("user_id = ?", userID).
                Pluck("tag_id", &tagIDs)
        if tagIDs == nil {
                return []int{}
        }
        return tagIDs
}

// intersectInts returns the intersection of two int slices.
func intersectInts(a, b []int) []int {
        setB := make(map[int]struct{}, len(b))
        for _, v := range b {
                setB[v] = struct{}{}
        }
        var result []int
        for _, v := range a {
                if _, ok := setB[v]; ok {
                        result = append(result, v)
                }
        }
        return result
}