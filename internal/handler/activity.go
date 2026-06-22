// Package handler provides HTTP request handlers for the game platform.
//
// activity.go — Handles check-in (daily sign-in) activity, including:
//   - Check-in state queries (streak, cycle, calendar view)
//   - Daily check-in execution with idempotency and concurrency safety
//   - Streak/cycle tracking and reward logic (bonus wallet crediting)
//   - Activity configuration and listing endpoints
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// ── Check-in Redis key helpers ──

// checkinRedisKey returns the Redis key used for idempotency / distributed locking
// on a per-user, per-day basis. Format: "checkin:{userID}:{YYYYMMDD}".
func checkinRedisKey(userID int64, date string) string {
	return fmt.Sprintf("checkin:%d:%s", userID, date)
}

// todayUTC returns the current date in UTC as a compact string "YYYYMMDD",
// used as the idempotency key suffix.
func todayUTC() string {
	return time.Now().UTC().Format("20060102")
}

// ── Check-in Handler ──

// CheckIn handles daily check-in requests.
// POST /api/v1/activity/check-in
//
// Multi-step flow:
//  1. Validate user context and database availability.
//  2. Find the currently active signin activity definition.
//  3. Idempotency check — prevent duplicate check-ins via Redis lock (with DB fallback).
//  4. Parse the activity's check-in config (cycle length, reset policy, rewards).
//  5. Get or create the user's activity state; update streak/cycle counters.
//  6. Determine the reward for the current day.
//  7. Record the check-in and credit the reward inside a single DB transaction.
//  8. Return the updated state to the client.
func CheckIn(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.LogError(c, "CheckIn", errors.New("user_id not found in context"))
		return bizerr.ErrUnauthorized
	}

	logger.Infof("[CheckIn] start: user_id=%d", userID)

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "CheckIn.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	today := todayUTC()
	now := time.Now().UTC()

	// ── Step 1: Find the active signin activity (highest priority first) ──
	var activity model.ActivityDefine
	if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
		"signin", now, now).
		Order("priority DESC, id DESC").
		First(&activity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.LogWarn(c, "CheckIn.FindActivity", "no active signin activity")
			return bizerr.ErrActivityExpired
		}
		middleware.LogError(c, "CheckIn.FindActivity", err)
		return bizerr.ErrInternal
	}
	logger.Infof("[CheckIn] found activity: activity_id=%d name=%s", activity.ID, activity.Name)

	// ── Step 2: Idempotency check via Redis (distributed lock for concurrency safety) ──
	rdb := cache.Client()
	if rdb != nil {
		ctx := context.Background()
		lockKey := checkinRedisKey(userID, today)
		lockToken, err := cache.Lock(ctx, lockKey, 48*time.Hour) // 48h TTL covers timezone edge cases
		if err != nil {
			middleware.LogError(c, "CheckIn.RedisLock", err)
			// Fall through to DB check if Redis is unavailable
		}
		if lockToken == "" && err == nil {
			middleware.LogWarn(c, "CheckIn.Duplicate", fmt.Sprintf("user %d already checked in today", userID))
			return bizerr.ErrAlreadyCheckedIn
		}
	}

	// ── Step 3: Check for existing record in DB as fallback ──
	var existingRecord model.ActivityRecord
	if err := db.Where("user_id = ? AND activity_id = ? AND action = ? AND DATE(created_at) = ?",
		userID, activity.ID, "checkin", now.Format("2006-01-02")).
		First(&existingRecord).Error; err == nil {
		middleware.LogWarn(c, "CheckIn.DuplicateDB", fmt.Sprintf("user %d already checked in today (DB)", userID))
		return bizerr.ErrAlreadyCheckedIn
	}

	// ── Step 4: Parse check-in config ──
	config := model.CheckInConfig{}
	if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
		middleware.LogError(c, "CheckIn.ParseConfig", err)
		return bizerr.ErrInternal
	}
	if config.CycleDays == 0 {
		config.CycleDays = 7 // Default to a 7-day cycle if not configured
	}
	logger.Infof("[CheckIn] parsed config: activity_id=%d cycle_days=%d reset_on_miss=%v rewards_count=%d",
		activity.ID, config.CycleDays, config.ResetOnMiss, len(config.Rewards))

	// ── Step 5: Get or create user activity record; update streak/cycle ──
	var userAct model.UserActivity
	err := db.Where("user_id = ? AND activity_id = ?", userID, activity.ID).First(&userAct).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// First time participating — create record with initial state
		state := model.CheckInStateData{
			Streak:    1,
			Cycle:     1,
			LastDay:   1,
			TotalDays: 1,
		}
		stateJSON, _ := json.Marshal(state)
		userAct = model.UserActivity{
			UserID:     userID,
			ActivityID: activity.ID,
			Progress:   1,
			StateData:  string(stateJSON),
		}
		if err := db.Create(&userAct).Error; err != nil {
			middleware.LogError(c, "CheckIn.CreateUserActivity", err)
			return bizerr.ErrInternal
		}
		logger.Infof("[CheckIn] created user activity: user_id=%d activity_id=%d", userID, activity.ID)
	} else if err != nil {
		middleware.LogError(c, "CheckIn.FindUserActivity", err)
		return bizerr.ErrInternal
	} else {
		// Update existing record — check streak continuity
		state := model.CheckInStateData{}
		if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
			middleware.LogError(c, "CheckIn.UnmarshalState", err)
			return bizerr.ErrInternal
		}

		// Streak calculation: compare the last check-in date with today to decide
		// whether the streak continues, was broken, or is a duplicate.
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		lastCheckIn := userAct.UpdatedAt.Format("2006-01-02")

		if lastCheckIn == today {
			// Already checked in today (edge case: duplicate request slipped through)
			return bizerr.ErrAlreadyCheckedIn
		} else if lastCheckIn == yesterday {
			// Consecutive day — increment streak and day-within-cycle counter.
			// If day exceeds the cycle length, roll over to a new cycle.
			state.Streak++
			state.LastDay++
			if state.LastDay > config.CycleDays {
				// Cycle reset: the user completed a full cycle; start a new one.
				// Streak is preserved across cycles (it counts consecutive days),
				// but LastDay resets to 1 and Cycle increments.
				state.LastDay = 1
				state.Cycle++
				logger.Infof("[CheckIn] cycle completed, starting new cycle: user_id=%d new_cycle=%d streak=%d",
					userID, state.Cycle, state.Streak)
			}
		} else if config.ResetOnMiss {
			// Streak broken (gap > 1 day) and reset policy is enabled —
			// reset streak, cycle position, and cycle number back to 1.
			state.Streak = 1
			state.LastDay = 1
			state.Cycle = 1
			logger.Infof("[CheckIn] streak reset (missed day): user_id=%d", userID)
		}
		// Note: if !config.ResetOnMiss and there's a gap, streak and LastDay are NOT
		// incremented — the user simply continues from where they left off.
		state.TotalDays++

		stateJSON, _ := json.Marshal(state)
		userAct.StateData = string(stateJSON)
		userAct.Progress = state.TotalDays
		if err := db.Save(&userAct).Error; err != nil {
			middleware.LogError(c, "CheckIn.UpdateUserActivity", err)
			return bizerr.ErrInternal
		}
		logger.Infof("[CheckIn] updated state: user_id=%d streak=%d cycle=%d day=%d total_days=%d",
			userID, state.Streak, state.Cycle, state.LastDay, state.TotalDays)
	}

	// ── Step 6: Determine reward for current day ──
	reward := findReward(config.Rewards, userAct.Progress)
	if reward == nil {
		// No reward configured for this day — still record the check-in
		record := model.ActivityRecord{
			UserID:     userID,
			ActivityID: activity.ID,
			Action:     "checkin",
			RewardJSON: "{}",
		}
		if err := db.Create(&record).Error; err != nil {
			middleware.LogError(c, "CheckIn.CreateRecord", err)
			return bizerr.ErrInternal
		}

		// Parse updated state for response
		state := model.CheckInStateData{}
		if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
			logger.Warnf("[CheckIn] unmarshal state for response failed: %v", err)
		}

		logger.Infof("[CheckIn] completed (no reward): user_id=%d activity_id=%d day=%d total_days=%d",
			userID, activity.ID, state.LastDay, state.TotalDays)

		return c.JSON(bizerr.SuccessResponse(fiber.Map{
			"checked_in": true,
			"day":         state.LastDay,
			"streak":      state.Streak,
			"cycle":       state.Cycle,
			"total_days":  state.TotalDays,
			"reward":      nil,
		}))
	}

	// ── Step 7: Record the check-in with reward AND credit reward in a single transaction ──
	rewardJSON, _ := json.Marshal(reward)
	err = db.Transaction(func(tx *gorm.DB) error {
		record := model.ActivityRecord{
			UserID:     userID,
			ActivityID: activity.ID,
			Action:     "checkin",
			RewardJSON: string(rewardJSON),
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		// Credit reward in the same transaction to ensure atomicity
		return creditCheckInRewardTx(tx, userID, activity.ID, reward)
	})
	if err != nil {
		middleware.LogError(c, "CheckIn.CreditReward", err)
		return bizerr.ErrInternal
	}

	// ── Step 8: Parse final state for response ──
	state := model.CheckInStateData{}
	if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
		logger.Warnf("[CheckIn] unmarshal final state failed: %v", err)
	}

	logger.Infof("[CheckIn] completed: user_id=%d activity_id=%d day=%d streak=%d cycle=%d reward_type=%s reward_value=%.4f",
		userID, activity.ID, state.LastDay, state.Streak, state.Cycle, reward.RewardType, reward.RewardValue)

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"checked_in": true,
		"day":        state.LastDay,
		"streak":     state.Streak,
		"cycle":      state.Cycle,
		"total_days": state.TotalDays,
		"reward": fiber.Map{
			"type":  reward.RewardType,
			"value": reward.RewardValue,
		},
	}))
}

// CheckInState returns the user's current check-in state and calendar view.
// GET /api/v1/activity/check-in/state
//
// This endpoint is read-only and returns:
//   - Whether the user has checked in today
//   - Current streak, cycle number, day within cycle, and total days
//   - The reward configuration and a calendar map of checked-in days in the current cycle
func CheckInState(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.LogError(c, "CheckInState", errors.New("user_id not found in context"))
		return bizerr.ErrUnauthorized
	}

	logger.Infof("[CheckInState] start: user_id=%d", userID)

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "CheckInState.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	now := time.Now().UTC()

	// Find active signin activity
	var activity model.ActivityDefine
	if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
		"signin", now, now).
		Order("priority DESC, id DESC").
		First(&activity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No active activity — return empty state
			return c.JSON(bizerr.SuccessResponse(fiber.Map{
				"has_activity": false,
			}))
		}
		middleware.LogError(c, "CheckInState.FindActivity", err)
		return bizerr.ErrInternal
	}

	// Check if already checked in today
	var todayRecord model.ActivityRecord
	todayStr := now.Format("2006-01-02")
	checkedInToday := false
	if err := db.Where("user_id = ? AND activity_id = ? AND action = ? AND DATE(created_at) = ?",
		userID, activity.ID, "checkin", todayStr).
		First(&todayRecord).Error; err == nil {
		checkedInToday = true
	}

	// Get user activity state
	var userAct model.UserActivity
	state := model.CheckInStateData{}
	if err := db.Where("user_id = ? AND activity_id = ?", userID, activity.ID).First(&userAct).Error; err == nil {
		if err := json.Unmarshal([]byte(userAct.StateData), &state); err != nil {
			logger.Warnf("[CheckInState] unmarshal state failed for user %d: %v", userID, err)
		}
	}

	// Parse reward config
	config := model.CheckInConfig{}
	if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
		middleware.LogError(c, "CheckInState.ParseConfig", err)
		return bizerr.ErrInternal
	}
	if config.CycleDays == 0 {
		config.CycleDays = 7 // Default to 7-day cycle
	}

	// Build calendar: show which days are checked in this cycle
	calendar := buildCheckInCalendar(userID, activity.ID, state.Cycle, config.CycleDays, now)

	logger.Infof("[CheckInState] completed: user_id=%d activity_id=%d checked_in=%v streak=%d cycle=%d day=%d",
		userID, activity.ID, checkedInToday, state.Streak, state.Cycle, state.LastDay)

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"has_activity":    true,
		"activity_id":     activity.ID,
		"checked_in":      checkedInToday,
		"streak":          state.Streak,
		"cycle":           state.Cycle,
		"current_day":     state.LastDay,
		"total_days":      state.TotalDays,
		"cycle_days":      config.CycleDays,
		"rewards":         config.Rewards,
		"calendar":        calendar,
	}))
}

// CheckInConfig returns the active check-in activity configuration.
// GET /api/v1/activity/check-in/config
//
// This is a public endpoint (no auth required) that returns the activity's
// cycle length, reset policy, and reward schedule so the client can render
// the check-in UI without the user needing to have participated yet.
func CheckInConfig(c *fiber.Ctx) error {
	db := database.DB()
	if db == nil {
		middleware.LogError(c, "CheckInConfig.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	now := time.Now().UTC()

	var activity model.ActivityDefine
	if err := db.Where("type = ? AND status = 1 AND start_time <= ? AND end_time >= ?",
		"signin", now, now).
		Order("priority DESC, id DESC").
		First(&activity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(bizerr.SuccessResponse(fiber.Map{
				"has_activity": false,
			}))
		}
		middleware.LogError(c, "CheckInConfig.FindActivity", err)
		return bizerr.ErrInternal
	}

	config := model.CheckInConfig{}
	if err := json.Unmarshal([]byte(activity.ConfigJSON), &config); err != nil {
		middleware.LogError(c, "CheckInConfig.ParseConfig", err)
		return bizerr.ErrInternal
	}
	if config.CycleDays == 0 {
		config.CycleDays = 7
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"has_activity": true,
		"activity_id":  activity.ID,
		"name":         activity.Name,
		"start_time":    activity.StartTime,
		"end_time":      activity.EndTime,
		"cycle_days":    config.CycleDays,
		"reset_on_miss": config.ResetOnMiss,
		"rewards":       config.Rewards,
	}))
}

// ListActivities returns all currently active activities.
// GET /api/v1/activity/list
func ListActivities(c *fiber.Ctx) error {
	db := database.DB()
	if db == nil {
		middleware.LogError(c, "ListActivities.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	now := time.Now().UTC()

	var activities []model.ActivityDefine
	if err := db.Where("status = 1 AND start_time <= ? AND end_time >= ?", now, now).
		Order("priority DESC, id ASC").
		Find(&activities).Error; err != nil {
		middleware.LogError(c, "ListActivities.Find", err)
		return bizerr.ErrInternal
	}

	// Build simplified list response (exclude config JSON for lighter payload)
	list := make([]fiber.Map, 0, len(activities))
	for _, act := range activities {
		list = append(list, fiber.Map{
			"id":         act.ID,
			"name":       act.Name,
			"type":       act.Type,
			"start_time": act.StartTime,
			"end_time":   act.EndTime,
		})
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"list": list,
	}))
}

// ActivityState returns the user's state across all active activities.
// GET /api/v1/activity/state
//
// For each active activity, returns whether the user has participated,
// their progress, and the raw state data. This is useful for dashboards
// or composite views that show multiple activity types.
func ActivityState(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.LogError(c, "ActivityState", errors.New("user_id not found in context"))
		return bizerr.ErrUnauthorized
	}

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "ActivityState.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	now := time.Now().UTC()

	// Find all active activities
	var activities []model.ActivityDefine
	if err := db.Where("status = 1 AND start_time <= ? AND end_time >= ?", now, now).
		Order("priority DESC, id ASC").
		Find(&activities).Error; err != nil {
		middleware.LogError(c, "ActivityState.FindActivities", err)
		return bizerr.ErrInternal
	}

	actIDs := make([]int64, len(activities))
	actMap := make(map[int64]model.ActivityDefine)
	for i, act := range activities {
		actIDs[i] = act.ID
		actMap[act.ID] = act
	}

	// Get user's participation records for these activities
	var userActs []model.UserActivity
	if len(actIDs) > 0 {
		if err := db.Where("user_id = ? AND activity_id IN ?", userID, actIDs).Find(&userActs).Error; err != nil {
			middleware.LogError(c, "ActivityState.FindUserActs", err)
			return bizerr.ErrInternal
		}
	}

	// Build response: for each active activity, attach user state if participated
	states := make([]fiber.Map, 0, len(activities))
	for _, act := range activities {
		state := fiber.Map{
			"activity_id": act.ID,
			"type":        act.Type,
			"name":        act.Name,
			"participated": false,
		}
		for _, ua := range userActs {
			if ua.ActivityID == act.ID {
				state["participated"] = true
				state["progress"] = ua.Progress
				state["state_data"] = ua.StateData
				state["updated_at"] = ua.UpdatedAt
				break
			}
		}
		states = append(states, state)
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"states": states,
	}))
}

// ── Internal helpers ──

// findReward finds the reward config for a given cumulative day.
// If the day exceeds the cycle length, it wraps around using modulo
// to pick the reward for the equivalent day within the cycle.
// If no exact day match is found, falls back to the last defined reward.
func findReward(rewards []model.CheckInRewardConfig, totalDays int) *model.CheckInRewardConfig {
	if len(rewards) == 0 {
		return nil
	}
	// Map totalDays to day within cycle (1-indexed).
	// Example: if cycle has 7 rewards and totalDays=8, day becomes 1 (wraps around).
	cycleLen := len(rewards)
	day := ((totalDays - 1) % cycleLen) + 1

	for i := range rewards {
		if rewards[i].Day == day {
			return &rewards[i]
		}
	}
	// Fallback: return last reward if day not found (e.g. sparse day config)
	return &rewards[len(rewards)-1]
}

// buildCheckInCalendar returns which days in the current cycle have been checked in.
// Returns a map of day (1-indexed) -> bool (true = checked in).
//
// Algorithm:
//  1. Fetch the most recent 2*cycleDays check-in records for the user/activity,
//     ordered by creation time descending.
//  2. Walk backwards from the most recent record, assigning calendar positions
//     (day 1 = most recent, day 2 = second most recent, etc.).
//  3. Stop walking backwards when:
//     - We've collected cycleDays entries, OR
//     - The gap between consecutive records exceeds 48 hours (indicating a
//       streak break, i.e. the start of the current cycle).
func buildCheckInCalendar(userID, activityID int64, cycle, cycleDays int, now time.Time) map[int]bool {
	db := database.DB()
	if db == nil {
		return nil
	}

	calendar := make(map[int]bool, cycleDays)

	// Query recent check-in records (fetch extra to cover possible gaps before current cycle)
	var records []model.ActivityRecord
	if err := db.Where("user_id = ? AND activity_id = ? AND action = ?",
		userID, activityID, "checkin").
		Order("created_at DESC").
		Limit(cycleDays * 2).
		Find(&records).Error; err != nil {
		return calendar
	}

	if len(records) == 0 {
		return calendar
	}

	// Walk backwards from most recent to find the current cycle's records.
	// The most recent record is assigned to day 1 of the calendar (most recent check-in),
	// then we go backwards assigning day 2, 3, etc. until we hit a gap or reach cycleDays.
	count := 0
	for _, rec := range records {
		if count >= cycleDays {
			break
		}
		calendar[count+1] = true // 1-indexed days
		count++

		// Check if previous record (next in descending order) is consecutive.
		// A gap > 48h means the streak was broken — this marks the start of the current cycle.
		if count < len(records) {
			prevTime := records[count].CreatedAt.UTC()
			currTime := rec.CreatedAt.UTC()
			diff := currTime.Sub(prevTime)
			if diff > 48*time.Hour {
				// Gap found — stop (streak was broken, start of current cycle reached)
				break
			}
		}
	}

	return calendar
}

// creditCheckInRewardTx credits the check-in reward inside an existing DB transaction.
// Currently only the "bonus" reward type is supported, which increments the user's
// bonus_balance in the user_wallet table. If no wallet row exists, it is created
// atomically via UPSERT (INSERT ... ON DUPLICATE KEY UPDATE).
func creditCheckInRewardTx(tx *gorm.DB, userID int64, activityID int64, reward *model.CheckInRewardConfig) error {
	if reward == nil {
		return nil
	}

	// Only handle bonus reward type for now; log and skip unsupported types
	if reward.RewardType != "bonus" {
		logger.Infof("[CheckIn] creditReward: unsupported reward_type=%s for user_id=%d", reward.RewardType, userID)
		return nil
	}

	// Attempt to update the existing wallet row
	result := tx.Exec("UPDATE user_wallet SET bonus_balance = bonus_balance + ?, updated_at = NOW() WHERE user_id = ?",
		reward.RewardValue, userID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Wallet doesn't exist — create atomically using parameterized upsert query
		return tx.Exec("INSERT INTO user_wallet (user_id, cash_balance, bonus_balance, created_at, updated_at) VALUES (?, 0, ?, NOW(), NOW()) "+
			"ON DUPLICATE KEY UPDATE bonus_balance = bonus_balance + VALUES(bonus_balance)",
			userID, reward.RewardValue).Error
	}
	logger.Infof("[CheckIn] creditReward: user_id=%d activity_id=%d type=bonus value=%.4f",
		userID, activityID, reward.RewardValue)
	return nil
}