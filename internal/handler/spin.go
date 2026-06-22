// Package handler provides HTTP request handlers for the RockGame platform.
//
// spin.go — Spin Wheel (C++ SpinHandler port)
//
// This file implements the spin wheel feature ported from C++ SpinHandler.cpp, including:
//   - Config management: loading spin, plot, invite, and poster configs from DB,
//     with in-memory caching protected by sync.RWMutex.
//   - Free spin logic: daily free spins advance the user through a pre-determined
//     plot/script that controls the amount increment at each step.
//   - Invite spin logic: when a user invites a friend, a probabilistic check
//     determines whether the wheel fills to full (hit) or advances one step (miss).
//   - Withdrawal: users can withdraw when accumulated amount reaches full_gold,
//     creating a pending order for manual review (auto-audit is a placeholder).
//   - Concurrency safety: per-user Redis distributed lock prevents duplicate spins.
package handler

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "math/rand/v2"
        "net/http"
        "strconv"
        "strings"
        "sync"
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

// ── Spin Redis key helpers ──

func spinLockKey(userID int64) string {
        return fmt.Sprintf("spin:lock:%d", userID)
}

// ── Config cache (thread-safe) ──

var (
        spinConfigMu    sync.RWMutex
        spinConfigCache map[string]*model.SpinConfig // key = spin_id
        spinConfigInited bool

        spinPlotMu    sync.RWMutex
        spinPlotCache map[int]*plotCacheEntry // key = plot_id

        spinInviteMu    sync.RWMutex
        spinInviteCache map[int][]model.SpinInviteConfig // key = group_id
)

// plotCacheEntry caches a parsed plot config for fast access.
type plotCacheEntry struct {
        StepInc int
        FreeInc []int
}

// loadSpinConfigs loads all active spin configs from DB into memory cache.
// Thread-safe: acquires write lock on the cache.
func loadSpinConfigs() error {
        db := database.DB()
        if db == nil {
                return errors.New("database not initialized")
        }
        var configs []model.SpinConfig
        if err := db.Where("status = 1").Find(&configs).Error; err != nil {
                return fmt.Errorf("failed to load spin configs: %w", err)
        }
        m := make(map[string]*model.SpinConfig, len(configs))
        for i := range configs {
                c := configs[i]
                m[c.SpinID] = &c
        }
        spinConfigMu.Lock()
        spinConfigCache = m
        spinConfigInited = true
        spinConfigMu.Unlock()
        logger.Infof("[SPIN] loaded %d spin configs into cache", len(m))
        return nil
}

// getSpinConfigCache returns the cached spin configs, loading from DB on first call.
func getSpinConfigCache() (map[string]*model.SpinConfig, error) {
        spinConfigMu.RLock()
        if spinConfigInited {
                defer spinConfigMu.RUnlock()
                return spinConfigCache, nil
        }
        spinConfigMu.RUnlock()
        if err := loadSpinConfigs(); err != nil {
                return nil, err
        }
        spinConfigMu.RLock()
        defer spinConfigMu.RUnlock()
        return spinConfigCache, nil
}

// getSpinConfig returns a single SpinConfig by spin_id, using the default (highest priority) if not found.
func getSpinConfig(spinID string) (*model.SpinConfig, error) {
        cache, err := getSpinConfigCache()
        if err != nil {
                return nil, err
        }
        if cfg, ok := cache[spinID]; ok {
                return cfg, nil
        }
        // Fallback: return highest priority config
        var best *model.SpinConfig
        for _, cfg := range cache {
                if best == nil || cfg.Priority > best.Priority {
                        best = cfg
                }
        }
        if best == nil {
                return nil, bizerr.ErrSpinNotActive
        }
        return best, nil
}

// getActiveSpinConfig returns the best matching active spin config for a user.
// Matches by time range, user type, labels, and selects highest priority.
func getActiveSpinConfig(uid int64, now int64) (*model.SpinConfig, error) {
        cache, err := getSpinConfigCache()
        if err != nil {
                return nil, err
        }
        if len(cache) == 0 {
                return nil, bizerr.ErrSpinNotActive
        }
        // Find matching configs
        var matches []*model.SpinConfig
        for _, cfg := range cache {
                if now < cfg.StartTime || now > cfg.EndTime {
                        continue
                }
                // User type filtering
                switch cfg.UserType {
                case model.SpinUserTypeAll:
                        // All users except tagged ones — skip for now (no label system)
                case model.SpinUserTypeUID:
                        // Specific UIDs — check if user is in the list
                        uids := parseIDList(cfg.UserList)
                        if !containsInt64(uids, uid) {
                                continue
                        }
                }
                matches = append(matches, cfg)
        }
        if len(matches) == 0 {
                return nil, bizerr.ErrSpinNotActive
        }
        // Select highest priority
        best := matches[0]
        for _, cfg := range matches[1:] {
                if cfg.Priority > best.Priority {
                        best = cfg
                }
        }
        return best, nil
}

// loadPlotConfigs loads all spin plot configs from DB into memory cache.
func loadPlotConfigs() error {
        db := database.DB()
        if db == nil {
                return errors.New("database not initialized")
        }
        var plots []model.SpinPlotConfig
        if err := db.Where("status = 1").Find(&plots).Error; err != nil {
                return fmt.Errorf("failed to load plot configs: %w", err)
        }
        m := make(map[int]*plotCacheEntry, len(plots))
        for _, p := range plots {
                var inc []int
                if err := json.Unmarshal([]byte(p.FreeInc), &inc); err != nil {
                        logger.Warnf("[SPIN] invalid free_inc JSON for plot_id=%d: %v", p.ID, err)
                        continue
                }
                m[p.ID] = &plotCacheEntry{StepInc: p.StepInc, FreeInc: inc}
        }
        spinPlotMu.Lock()
        spinPlotCache = m
        spinPlotMu.Unlock()
        logger.Infof("[SPIN] loaded %d plot configs into cache", len(m))
        return nil
}

// getPlotEntry returns the cached plot config for a given plot_id.
// If plot_id <= 0, returns a random plot's entry.
func getPlotEntry(plotID int) (*plotCacheEntry, error) {
        spinPlotMu.RLock()
        defer spinPlotMu.RUnlock()
        if len(spinPlotCache) == 0 {
                return nil, fmt.Errorf("no plot configs loaded")
        }
        if plotID > 0 {
                if entry, ok := spinPlotCache[plotID]; ok {
                        return entry, nil
                }
        }
        // Random plot
        keys := make([]int, 0, len(spinPlotCache))
        for k := range spinPlotCache {
                keys = append(keys, k)
        }
        picked := keys[rand.IntN(len(keys))]
        return spinPlotCache[picked], nil
}

// loadInviteConfigs loads all spin invite configs from DB into memory cache.
func loadInviteConfigs() error {
        db := database.DB()
        if db == nil {
                return errors.New("database not initialized")
        }
        var cfgs []model.SpinInviteConfig
        if err := db.Find(&cfgs).Error; err != nil {
                return fmt.Errorf("failed to load invite configs: %w", err)
        }
        m := make(map[int][]model.SpinInviteConfig)
        for _, c := range cfgs {
                m[c.GroupID] = append(m[c.GroupID], c)
        }
        spinInviteMu.Lock()
        spinInviteCache = m
        spinInviteMu.Unlock()
        logger.Infof("[SPIN] loaded invite configs for %d groups", len(m))
        return nil
}

// getInviteConfig returns the invite config matching the user's VIP level within a group.
func getInviteConfig(groupID int, vipLevel int) (*model.SpinInviteConfig, error) {
        spinInviteMu.RLock()
        defer spinInviteMu.RUnlock()
        cfgs, ok := spinInviteCache[groupID]
        if !ok || len(cfgs) == 0 {
                return nil, fmt.Errorf("no invite config for group %d", groupID)
        }
        // Find exact VIP match, or highest VIP <= user's level
        var best *model.SpinInviteConfig
        for i := range cfgs {
                c := cfgs[i]
                if c.VIPLevel == vipLevel {
                        return &c, nil
                }
                if c.VIPLevel < vipLevel {
                        if best == nil || c.VIPLevel > best.VIPLevel {
                                best = &c
                        }
                }
        }
        if best == nil {
                // Fallback to lowest VIP config
                best = &cfgs[0]
        }
        return best, nil
}

// InitSpinConfigs loads all spin-related configs from DB into memory.
// Should be called once at service startup.
func InitSpinConfigs() {
        if err := loadSpinConfigs(); err != nil {
                logger.Warnf("[SPIN] failed to load spin configs: %v", err)
        }
        if err := loadPlotConfigs(); err != nil {
                logger.Warnf("[SPIN] failed to load plot configs: %v", err)
        }
        if err := loadInviteConfigs(); err != nil {
                logger.Warnf("[SPIN] failed to load invite configs: %v", err)
        }
}

// ReloadSpinConfigs reloads all configs from DB (for hot-reload after admin changes).
func ReloadSpinConfigs() {
        spinConfigMu.Lock()
        spinConfigInited = false
        spinConfigMu.Unlock()
        InitSpinConfigs()
}

// ── User Spin Data helpers ──

// getUserSpinData loads or creates UserSpinData for a user.
func getUserSpinData(db *gorm.DB, userID int64) (*model.UserSpinData, bool, error) {
        var data model.UserSpinData
        err := db.Where("user_id = ?", userID).First(&data).Error
        if err == nil {
                return &data, false, nil
        }
        if !errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, false, err
        }
        // Create new record
        data = model.UserSpinData{
                UserID:    userID,
                FreeTimes: 1,
        }
        if err := db.Create(&data).Error; err != nil {
                return nil, false, err
        }
        return &data, true, nil
}

// isSameDay checks if two Unix timestamps are on the same day (UTC).
func isSameDay(ts1, ts2 int64) bool {
        t1 := time.Unix(ts1, 0).UTC()
        t2 := time.Unix(ts2, 0).UTC()
        return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

// parseIDList parses a comma-separated string of int64 IDs into a slice.
func parseIDList(s string) []int64 {
        if s == "" {
                return nil
        }
        parts := strings.Split(s, ",")
        result := make([]int64, 0, len(parts))
        for _, p := range parts {
                p = strings.TrimSpace(p)
                if id, err := strconv.ParseInt(p, 10, 64); err == nil && id > 0 {
                        result = append(result, id)
                }
        }
        return result
}

// containsInt64 checks if a slice contains an int64 value.
func containsInt64(slice []int64, val int64) bool {
        for _, v := range slice {
                if v == val {
                        return true
                }
        }
        return false
}

// parseItems parses the ItemsJSON field of SpinConfig into a slice of SpinItem.
func parseItems(itemsJSON string) ([]model.SpinItem, error) {
        var items []model.SpinItem
        if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
                return nil, fmt.Errorf("failed to parse items_json: %w", err)
        }
        return items, nil
}

// parsePlotList parses the PlotList field into a slice of int IDs.
func parsePlotList(s string) []int {
        if s == "" {
                return nil
        }
        parts := strings.Split(s, ",")
        result := make([]int, 0, len(parts))
        for _, p := range parts {
                p = strings.TrimSpace(p)
                if id, err := strconv.Atoi(p); err == nil && id > 0 {
                        result = append(result, id)
                }
        }
        return result
}

// generateOrderNo generates a unique order number: SPIN + timestamp + random.
func generateOrderNo() string {
        return fmt.Sprintf("SPIN%d%04d", time.Now().UnixMilli(), rand.IntN(10000))
}

// ── Core Spin Logic ──

// GetSpinInfo handles GET /spin/info — returns full spin wheel page data.
// Main entry point when a user opens the spin wheel page:
//   1. Load or create user spin data
//   2. If round expired, reset round
//   3. If new day, grant free spin
//   4. Return full info: items, progress, records, invite code, etc.
func GetSpinInfo(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        logger.Infof("[GetSpinInfo] start: user_id=%d", uid)

        db := MustDB(c, "GetSpinInfo")
        if db == nil {
                return bizerr.ErrInternal
        }

        // Load user spin data
        rdata, isNew, err := getUserSpinData(db, uid)
        if err != nil {
                middleware.LogError(c, "GetSpinInfo.GetUserSpinData", err)
                return bizerr.ErrSpinUserDataErr
        }

        now := time.Now().Unix()
        var cfg *model.SpinConfig

        if isNew || rdata.SpinID == "" {
                // New user: match to best config
                cfg, err = getActiveSpinConfig(uid, now)
                if err != nil {
                        return err
                }
                rdata.SpinID = cfg.SpinID
                // Initialize first round
                initSpinRound(cfg, rdata, now, true)
        } else {
                cfg, err = getSpinConfig(rdata.SpinID)
                if err != nil {
                        return err
                }
                // Check if round expired
                roundEnd := rdata.RoundStartTS + int64(cfg.TimeLimitHour)*3600
                if rdata.RoundStartTS > 0 && now >= roundEnd {
                        // Round expired — reset
                        initSpinRound(cfg, rdata, now, false)
                } else if rdata.RoundStartTS == 0 {
                        // Data was reset after withdrawal — re-initialize
                        initSpinRound(cfg, rdata, now, false)
                }
        }

        // Grant daily free spin if new day
        if !isSameDay(rdata.LastFreeSpinTS, now) {
                rdata.FreeTimes = 1
                rdata.LastFreeSpinTS = now
        }

        // Parse items
        items, err := parseItems(cfg.ItemsJSON)
        if err != nil {
                middleware.LogError(c, "GetSpinInfo.ParseItems", err)
                return bizerr.ErrInternal
        }

        // Generate gift boxes
        boxes := generateGiftBoxes(cfg, rdata.CurAmount)

        // Parse round records
        var records []model.SpinRecord
        _ = json.Unmarshal([]byte(rdata.RoundRecord), &records)

        // Get user info for display
        var user model.User
        _ = db.Select("nickname, avatar").First(&user, uid).Error
        inviteCode := fmt.Sprintf("SPIN%d", uid)

        // Calculate round end time
        roundEndTime := rdata.RoundStartTS + int64(cfg.TimeLimitHour)*3600

        // Save updated data
        if err := db.Save(rdata).Error; err != nil {
                middleware.LogError(c, "GetSpinInfo.Save", err)
                // Don't fail the request, just log
        }

        logger.Infof("[GetSpinInfo] success: user_id=%d spin_id=%s round=%d amount=%d/%d tickets=%d",
                uid, rdata.SpinID, rdata.CurRound, rdata.CurAmount, cfg.FullGold, rdata.FreeTimes)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":     model.SpinErrSuccess,
                "tickets":    rdata.FreeTimes,
                "amount":     rdata.CurAmount,
                "full_amount": cfg.FullGold,
                "end_time":   roundEndTime,
                "items":      items,
                "boxes":      boxes,
                "rec_list":   records,
                "invite_code": inviteCode,
                "nickname":   user.Nickname,
                "avatar":     user.Avatar,
                "cur_round":  rdata.CurRound,
        }))
}

// DoSpin handles POST /spin/do — executes a free spin for the user.
//   1. Acquire distributed lock
//   2. Load and validate user data
//   3. Calculate next amount from plot/script
//   4. Find matching wheel item by diff range
//   5. Update user data and record
func DoSpin(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        logger.Infof("[DoSpin] start: user_id=%d", uid)

        db := MustDB(c, "DoSpin")
        if db == nil {
                return bizerr.ErrInternal
        }

        // Acquire per-user distributed lock
        ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
        defer cancel()
        token, err := cache.Lock(ctx, spinLockKey(uid), 5*time.Second)
        if err != nil {
                middleware.LogError(c, "DoSpin.Lock", err)
                return bizerr.ErrInternal
        }
        if token == "" {
                return bizerr.ErrDuplicateRequest
        }
        defer func() {
                if err := cache.Unlock(ctx, spinLockKey(uid), token); err != nil {
                        logger.Warnf("[DoSpin] unlock failed: %v", err)
                }
        }()

        // Load user spin data
        rdata, _, err := getUserSpinData(db, uid)
        if err != nil {
                middleware.LogError(c, "DoSpin.GetUserSpinData", err)
                return bizerr.ErrSpinUserDataErr
        }

        cfg, err := getSpinConfig(rdata.SpinID)
        if err != nil {
                return err
        }

        // Execute free spin
        result, err := doSpinFree(uid, rdata, cfg, 0)
        if err != nil {
                return err
        }

        // Save updated data
        if err := db.Save(rdata).Error; err != nil {
                middleware.LogError(c, "DoSpin.Save", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[DoSpin] success: user_id=%d pos=%d amount=%d/%d",
                uid, result.Pos, rdata.CurAmount, cfg.FullGold)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":  model.SpinErrSuccess,
                "tickets": rdata.FreeTimes,
                "amount":  rdata.CurAmount,
                "pos":     result.Pos,
        }))
}

// DoInviteSpin handles POST /spin/invite-spin — processes an invite-triggered spin.
// Called when an invited friend successfully registers.
func DoInviteSpin(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        logger.Infof("[DoInviteSpin] start: user_id=%d", uid)

        var req struct {
                InviteUID int64 `json:"invite_uid"`
        }
        if err := c.BodyParser(&req); err != nil || req.InviteUID <= 0 {
                return bizerr.ErrInvalidParams
        }

        db := MustDB(c, "DoInviteSpin")
        if db == nil {
                return bizerr.ErrInternal
        }

        // Acquire lock
        ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
        defer cancel()
        token, err := cache.Lock(ctx, spinLockKey(uid), 5*time.Second)
        if err != nil {
                middleware.LogError(c, "DoInviteSpin.Lock", err)
                return bizerr.ErrInternal
        }
        if token == "" {
                return bizerr.ErrDuplicateRequest
        }
        defer func() {
                if err := cache.Unlock(ctx, spinLockKey(uid), token); err != nil {
                        logger.Warnf("[DoInviteSpin] unlock failed: %v", err)
                }
        }()

        rdata, _, err := getUserSpinData(db, uid)
        if err != nil {
                middleware.LogError(c, "DoInviteSpin.GetUserSpinData", err)
                return bizerr.ErrSpinUserDataErr
        }

        cfg, err := getSpinConfig(rdata.SpinID)
        if err != nil {
                return err
        }

        // Execute invite spin
        result, err := doSpinInvite(uid, rdata, cfg, req.InviteUID)
        if err != nil {
                return err
        }

        if err := db.Save(rdata).Error; err != nil {
                middleware.LogError(c, "DoInviteSpin.Save", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[DoInviteSpin] success: user_id=%d invite_uid=%d pos=%d hit=%v amount=%d/%d",
                uid, req.InviteUID, result.Pos, result.IsHit, rdata.CurAmount, cfg.FullGold)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":  model.SpinErrSuccess,
                "tickets": rdata.FreeTimes,
                "amount":  rdata.CurAmount,
                "pos":     result.Pos,
                "is_hit":  result.IsHit,
        }))
}

// spinResult holds the result of a single spin operation.
type spinResult struct {
        Pos   int   `json:"pos"`
        IsHit bool  `json:"is_hit,omitempty"`
}

// doSpinFree executes a free spin step, advancing the user through the plot script.
func doSpinFree(uid int64, rdata *model.UserSpinData, cfg *model.SpinConfig, inviteUID int64) (*spinResult, error) {
        now := time.Now().Unix()

        // Check daily limit
        if isSameDay(rdata.LastFreeSpinTS, now) && rdata.FreeTimes <= 0 {
                return nil, bizerr.ErrSpinDayLimit
        }

        // Check remaining chances
        if rdata.FreeTimes <= 0 {
                return nil, bizerr.ErrSpinNoChance
        }

        // Check amount not full
        if rdata.CurAmount >= cfg.FullGold {
                return nil, bizerr.ErrSpinAmountFull
        }

        // Consume free spin
        rdata.FreeTimes--
        rdata.LastFreeSpinTS = now

        // Get plot step amount
        stepMoney, err := getPlotStepMoney(rdata, cfg)
        if err != nil {
                logger.Warnf("[doSpinFree] getPlotStepMoney failed: user_id=%d err=%v", uid, err)
                return nil, bizerr.ErrSpinUserDataErr
        }

        // Calculate diff and find matching item position
        diff := stepMoney - rdata.CurAmount
        items, err := parseItems(cfg.ItemsJSON)
        if err != nil {
                return nil, bizerr.ErrInternal
        }
        pos := findItemPosition(items, diff)

        // Update user data
        rdata.CurAmount = stepMoney
        rdata.CurPlotStep++

        // Add spin record (first spin of round when CurPlotStep was 0 before increment)
        addSpinRecord(rdata, uid, inviteUID, diff, stepMoney, cfg.SpinID, rdata.CurPlotStep == 1)

        return &spinResult{Pos: pos}, nil
}

// doSpinInvite executes an invite-triggered spin with probability check.
func doSpinInvite(uid int64, rdata *model.UserSpinData, cfg *model.SpinConfig, inviteUID int64) (*spinResult, error) {
        // Get invite config
        inviteCfg, err := getInviteConfig(cfg.InviteGroupID, 0) // VIP level 0 for now
        if err != nil {
                logger.Warnf("[doSpinInvite] getInviteConfig failed: user_id=%d err=%v", uid, err)
                // Fallback to free spin
                return doSpinFree(uid, rdata, cfg, inviteUID)
        }

        // Update invite counts
        rdata.InviteCount++
        rdata.TotalInvite++
        rdata.LevelInvite++

        // Check guaranteed hit (max_count)
        if rdata.InviteCount >= inviteCfg.MaxCount {
                // Guaranteed hit — fill to full_gold
                diff := cfg.FullGold - rdata.CurAmount
                rdata.CurAmount = cfg.FullGold
                addSpinRecord(rdata, uid, inviteUID, diff, rdata.CurAmount, cfg.SpinID, false)

                items, _ := parseItems(cfg.ItemsJSON)
                pos := findItemPosition(items, diff)
                return &spinResult{Pos: pos, IsHit: true}, nil
        }

        // Probability check
        num := rand.IntN(model.SpinRATIO_BASE) + 1 // 1 to RATIO_BASE inclusive
        var hitRatio int
        if rdata.LevelInvite <= inviteCfg.NewCount {
                hitRatio = inviteCfg.NewRatio
        } else {
                hitRatio = inviteCfg.DefaultRatio - inviteCfg.ReduceRatio*(rdata.LevelInvite-inviteCfg.NewCount)
        }
        if hitRatio < inviteCfg.BaseRatio {
                hitRatio = inviteCfg.BaseRatio
        }

        if num <= hitRatio {
                // Hit — fill to full_gold
                diff := cfg.FullGold - rdata.CurAmount
                rdata.CurAmount = cfg.FullGold
                addSpinRecord(rdata, uid, inviteUID, diff, rdata.CurAmount, cfg.SpinID, false)

                items, _ := parseItems(cfg.ItemsJSON)
                pos := findItemPosition(items, diff)
                return &spinResult{Pos: pos, IsHit: true}, nil
        }

        // Miss — fall through to free spin
        return doSpinFree(uid, rdata, cfg, inviteUID)
}

// getPlotStepMoney returns the next target amount from the plot script.
func getPlotStepMoney(rdata *model.UserSpinData, cfg *model.SpinConfig) (int64, error) {
        entry, err := getPlotEntry(rdata.PlotID)
        if err != nil {
                return 0, err
        }
        if rdata.CurPlotStep < len(entry.FreeInc) {
                return int64(entry.FreeInc[rdata.CurPlotStep]), nil
        }
        // Script exhausted — use linear increment from plot config
        stepInc := entry.StepInc
        if stepInc <= 0 {
                stepInc = 1
        }
        stepMoney := rdata.CurAmount + int64(stepInc)
        // Cap at FullGold - 1
        if stepMoney >= cfg.FullGold {
                stepMoney = cfg.FullGold - 1
        }
        return stepMoney, nil
}

// findItemPosition matches diff against item ranges to determine wheel position.
func findItemPosition(items []model.SpinItem, diff int64) int {
        for _, item := range items {
                if diff > item.NumGT && diff <= item.NumLE {
                        return item.ID
                }
        }
        // Fallback: return first item
        if len(items) > 0 {
                return items[0].ID
        }
        return 0
}

// initSpinRound initializes or resets a spin round for the user.
func initSpinRound(cfg *model.SpinConfig, rdata *model.UserSpinData, now int64, isNew bool) {
        rdata.RoundStartTS = now
        rdata.CurAmount = 0
        rdata.FreeTimes = 1
        rdata.CurRound++
        rdata.CurPlotStep = 0
        rdata.InviteCount = 0
        rdata.RoundRecord = "[]"

        // Pick a random plot from config's plot list
        plots := parsePlotList(cfg.PlotList)
        if len(plots) > 0 {
                rdata.PlotID = plots[rand.IntN(len(plots))]
        }

        // Set initial amount to first plot step
        if !isNew && rdata.CurPlotStep == 0 {
                entry, err := getPlotEntry(rdata.PlotID)
                if err == nil && len(entry.FreeInc) > 0 {
                        rdata.CurAmount = int64(entry.FreeInc[0])
                        rdata.CurPlotStep = 1
                }
        }
}

// generateGiftBoxes creates 4 shuffled gift box amounts for display.
// One box holds the actual current amount, the other 3 are random in [box_gt, box_le].
func generateGiftBoxes(cfg *model.SpinConfig, curAmount int64) []int64 {
        boxes := make([]int64, 4)
        lo, hi := int64(cfg.BoxGT), int64(cfg.BoxLE)
        if hi <= lo {
                hi = lo + 1 // ensure valid range
        }
        for i := range boxes {
                boxes[i] = lo + rand.Int64N(hi-lo)
        }
        // Place real amount at a random position
        boxes[rand.IntN(4)] = curAmount
        return boxes
}

// addSpinRecord appends a spin record to the round_record JSON array.
func addSpinRecord(rdata *model.UserSpinData, uid, inviteUID int64, amount, total int64, spinID string, isFirst bool) {
        var records []model.SpinRecord
        _ = json.Unmarshal([]byte(rdata.RoundRecord), &records)
        records = append(records, model.SpinRecord{
                UID:       uid,
                InviteUID: inviteUID,
                Amount:    amount,
                Total:     total,
                SpinID:    spinID,
                IsFirst:   isFirst,
                Timestamp: time.Now().Unix(),
        })
        // Keep last 100 records
        if len(records) > 100 {
                records = records[len(records)-100:]
        }
        data, _ := json.Marshal(records)
        rdata.RoundRecord = string(data)
}

// ── Withdrawal Handlers ──

// SpinWithdraw handles POST /spin/withdraw — creates a withdrawal request.
//   1. Validate phone binding and amount
//   2. Create order in transaction
//   3. Trigger async auto-audit (currently a no-op placeholder)
func SpinWithdraw(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        logger.Infof("[SpinWithdraw] start: user_id=%d", uid)

        db := MustDB(c, "SpinWithdraw")
        if db == nil {
                return bizerr.ErrInternal
        }

        // Check phone binding
        var user model.User
        if err := db.Select("phone, nickname").First(&user, uid).Error; err != nil {
                middleware.LogError(c, "SpinWithdraw.FindUser", err)
                return bizerr.ErrSpinBindPhone
        }
        if user.Phone == "" {
                return bizerr.ErrSpinBindPhone
        }

        // Load user spin data
        rdata, _, err := getUserSpinData(db, uid)
        if err != nil {
                middleware.LogError(c, "SpinWithdraw.GetUserSpinData", err)
                return bizerr.ErrSpinUserDataErr
        }

        cfg, err := getSpinConfig(rdata.SpinID)
        if err != nil {
                return err
        }

        // Check amount has reached full_gold threshold
        if rdata.CurAmount < cfg.FullGold {
                return bizerr.New(70004, "amount not enough to withdraw").WithHTTP(http.StatusBadRequest)
        }

        // Calculate flow requirement
        flow := rdata.CurAmount * int64(cfg.FlowMulti) / int64(model.SpinRATIO_BASE)

        // Create order
        orderNo := generateOrderNo()
        order := model.SpinWithdrawOrder{
                OrderNo:   orderNo,
                UserID:    uid,
                NickName:  user.Nickname,
                Amount:    rdata.CurAmount,
                Flow:      flow,
                Round:     rdata.CurRound,
                SpinID:    rdata.SpinID,
                Status:    model.SpinOrderPending,
        }

        // Use transaction for order creation + data update
        err = db.Transaction(func(tx *gorm.DB) error {
                if err := tx.Create(&order).Error; err != nil {
                        return err
                }
                // Create order log
                log := model.SpinOrderLog{
                        OrderID: order.ID,
                        UserID:  uid,
                        Type:    1,
                        Detail:  fmt.Sprintf("withdraw request: amount=%d flow=%d round=%d", rdata.CurAmount, flow, rdata.CurRound),
                }
                if err := tx.Create(&log).Error; err != nil {
                        return err
                }
                // Deduct amount from user data
                rdata.TotalWithdraw += rdata.CurAmount
                rdata.CurAmount = 0
                rdata.RoundStartTS = 0
                rdata.CurPlotStep = 0
                rdata.RoundRecord = "[]"
                if err := tx.Save(rdata).Error; err != nil {
                        return err
                }
                return nil
        })
        if err != nil {
                middleware.LogError(c, "SpinWithdraw.Transaction", err)
                return bizerr.ErrInternal
        }

        // Run auto-audit asynchronously (placeholder — no rules configured yet)
        go runAutoAudit(order.ID)

        logger.Infof("[SpinWithdraw] success: user_id=%d order_id=%d amount=%d", uid, order.ID, order.Amount)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":   model.SpinErrSuccess,
                "order_id": order.ID,
        }))
}

// runAutoAudit is a placeholder for the auto-audit system.
// No audit rules are configured yet; orders remain in pending status for manual review.
func runAutoAudit(orderID int64) {
        logger.Infof("[SpinWithdraw] auto-audit skipped (no rules): order_id=%d", orderID)
}

// ── Query Handlers ──

// GetSpinWithdrawLog handles GET /spin/withdraw-log — paginated withdrawal history.
func GetSpinWithdrawLog(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        page, pageSize, offset := ParsePagination(c)

        logger.Infof("[GetSpinWithdrawLog] start: user_id=%d page=%d page_size=%d", uid, page, pageSize)

        db := MustDB(c, "GetSpinWithdrawLog")
        if db == nil {
                return bizerr.ErrInternal
        }

        var total int64
        db.Table("spin_withdraw_order").Where("user_id = ?", uid).Count(&total)

        var orders []model.SpinWithdrawOrder
        if err := db.Table("spin_withdraw_order").
                Where("user_id = ?", uid).
                Order("id DESC").Offset(offset).Limit(pageSize).
                Find(&orders).Error; err != nil {
                middleware.LogError(c, "GetSpinWithdrawLog.Find", err)
                return bizerr.ErrInternal
        }

        if orders == nil {
                orders = []model.SpinWithdrawOrder{}
        }

        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     orders,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}

// GetCurSpinData handles GET /spin/cur-data — lightweight current progress query.
func GetCurSpinData(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        logger.Infof("[GetCurSpinData] start: user_id=%d", uid)

        db := MustDB(c, "GetCurSpinData")
        if db == nil {
                return bizerr.ErrInternal
        }

        rdata, _, err := getUserSpinData(db, uid)
        if err != nil {
                middleware.LogError(c, "GetCurSpinData.GetUserSpinData", err)
                return bizerr.ErrSpinUserDataErr
        }

        var fullGold int64
        if rdata.SpinID != "" {
                cfg, err := getSpinConfig(rdata.SpinID)
                if err != nil {
                        return err
                }
                fullGold = cfg.FullGold
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result":  model.SpinErrSuccess,
                "target":  fullGold,
                "amount":  rdata.CurAmount,
                "tickets": rdata.FreeTimes,
        }))
}

// GetSpinPoster handles GET /spin/poster — returns poster sharing config.
func GetSpinPoster(c *fiber.Ctx) error {
        uid := middleware.GetUserID(c)
        language := c.Query("language", "en")

        logger.Infof("[GetSpinPoster] start: user_id=%d language=%s", uid, language)

        db := MustDB(c, "GetSpinPoster")
        if db == nil {
                return bizerr.ErrInternal
        }

        var poster model.SpinPosterConfig
        err := db.Where("language = ? AND status = 1", language).First(&poster).Error
        if err != nil {
                // Fallback to English
                err = db.Where("language = 'en' AND status = 1").First(&poster).Error
                if err != nil {
                        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                                "invite_code":       fmt.Sprintf("%d", uid),
                                "share_url":         "",
                                "telegram_url":      "",
                                "whatsapp_url":      "",
                                "share_url_prefix":  "",
                                "list":              []interface{}{},
                        }))
                }
        }

        // Parse posters JSON
        var posters []model.SpinPosterItem
        _ = json.Unmarshal([]byte(poster.PostersJSON), &posters)

        // Replace #code# placeholder in URLs
        inviteCode := fmt.Sprintf("SPIN%d", uid)
        shareURL := strings.ReplaceAll(poster.ShareURL, "#code#", inviteCode)
        tgURL := strings.ReplaceAll(poster.TelegramURL, "#code#", inviteCode)
        waURL := strings.ReplaceAll(poster.WhatsappURL, "#code#", inviteCode)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "invite_code":       inviteCode,
                "share_url":         shareURL,
                "telegram_url":      tgURL,
                "whatsapp_url":      waURL,
                "share_url_prefix":  poster.ShareURLPrefix,
                "list":              posters,
        }))
}