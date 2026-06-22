// Package handler (admin_crud.go) implements generic CRUD HTTP handlers for the admin panel.
//
// This file contains admin-facing Create/Update/Delete handlers for the following entities:
//   1. Banners         – lobby banner management (image, link, schedule, status toggle)
//   2. Activities      – promotional activity definitions (type, handler, schedule, status)
//   3. Tasks           – task/mission definitions (target, reward, cycle)
//   4. Games           – game info records (vendor, category, RTP, bet limits, hot/new flags)
//   5. Game Vendors    – game vendor providers (name, logo, status)
//   6. Game Categories – game category tree (parent, lobby, icon, sort)
//   7. Payment Channels – deposit/withdrawal channel configs (regions, limits, status toggle)
//   8. VIP Levels      – VIP level config updates (growth, fee rate, daily bonus)
//   9. Admin Users     – admin account CRUD with role-based access (super/admin/operator/viewer)
//  10. Mail            – mail queue insertion for user notifications
//
// Every write handler follows a consistent pattern:
//   - Parse & validate request
//   - Insert/Update/Delete via GORM on the global database
//   - Fire an async audit log via RecordAuditLog
//   - Publish a cache-invalidation event via notifyCacheInvalidate (non-blocking)
//   - Return a JSON success response with the created/updated entity ID
package handler

import (
        "context"
        "errors"
        "fmt"
        "strconv"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// notifyCacheInvalidate publishes a cache invalidation event via Redis Pub/Sub.
// Called after successful admin CRUD operations so that cached data in other
// services (lobby, game, etc.) is invalidated. Non-blocking — runs in goroutine
// with short timeout. Silently ignores errors (cache is best-effort).
func notifyCacheInvalidate(prefix, action string) {
        go func() {
                ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
                defer cancel()
                // Publish is fire-and-forget; errors are intentionally discarded
                // because cache invalidation should not block or fail the admin operation.
                _ = cache.PublishInvalidate(ctx, cache.CacheInvalidateEvent{
                        Prefix: prefix,
                        Source: "admin-node:" + action,
                })
        }()
}

// lastInsertID is a helper that executes an INSERT and returns the auto-increment ID.
// Uses a transaction to guarantee the same DB connection for LAST_INSERT_ID(),
// preventing race conditions in concurrent admin CRUD operations.
func lastInsertID(db *gorm.DB, table string, data map[string]interface{}) (int64, error) {
        var id int64
        err := db.Transaction(func(tx *gorm.DB) error {
                if err := tx.Table(table).Create(data).Error; err != nil {
                        return err
                }
                return tx.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error
        })
        if err != nil {
                return 0, err
        }
        return id, nil
}

// ============================================================================
// 1. Banner CRUD
// ============================================================================

// CreateBannerRequest defines the request body for creating a banner
type CreateBannerRequest struct {
        LobbyID    int64  `json:"lobby_id"`
        ImageURL   string `json:"image_url"`
        LinkURL    string `json:"link_url"`
        Weight     int    `json:"weight"`
        TargetLang string `json:"target_lang"`
        StartTime  string `json:"start_time"`
        EndTime    string `json:"end_time"`
}

// CreateBanner creates a new banner with the given image URL, link, schedule, and weight.
// The banner is created with status=1 (active) by default.
func CreateBanner(c *fiber.Ctx) error {
        logger.Infof("[CreateBanner] start: image_url=%s, lobby_id=%d", c.FormValue("image_url"), c.FormValue("lobby_id"))

        var req CreateBannerRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateBanner.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.ImageURL == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "image_url is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateBanner.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        bannerID, err := lastInsertID(db, "banner", map[string]interface{}{
                "lobby_id":    req.LobbyID,
                "image_url":   req.ImageURL,
                "link_url":    req.LinkURL,
                "weight":      req.Weight,
                "target_lang": req.TargetLang,
                "status":      1,
                "start_time":  req.StartTime,
                "end_time":    req.EndTime,
                "created_at":  now,
                "updated_at":  now,
        })
        if err != nil {
                middleware.LogError(c, "CreateBanner.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "banner.create", "banner", strconv.FormatInt(bannerID, 10),
                        "create banner: "+req.ImageURL, ip)
        }()

        notifyCacheInvalidate("banner:", "create")
        logger.Infof("[CreateBanner] completed: id=%d", bannerID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": bannerID,
        }))
}

// UpdateBannerRequest defines the request body for updating a banner.
// All fields are pointers so that omitted fields are treated as "no change" (partial update).
type UpdateBannerRequest struct {
        LobbyID    *int64  `json:"lobby_id"`
        ImageURL   *string `json:"image_url"`
        LinkURL    *string `json:"link_url"`
        Weight     *int    `json:"weight"`
        TargetLang *string `json:"target_lang"`
        StartTime  *string `json:"start_time"`
        EndTime    *string `json:"end_time"`
}

// UpdateBanner updates a banner by ID. Only fields present in the request body are modified.
func UpdateBanner(c *fiber.Ctx) error {
        bannerID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || bannerID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateBanner] start: id=%d", bannerID)

        var req UpdateBannerRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateBanner.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateBanner.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check banner exists
        var count int64
        if err := db.Table("banner").Where("id = ?", bannerID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateBanner.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "banner not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.LobbyID != nil {
                updates["lobby_id"] = *req.LobbyID
        }
        if req.ImageURL != nil {
                updates["image_url"] = *req.ImageURL
        }
        if req.LinkURL != nil {
                updates["link_url"] = *req.LinkURL
        }
        if req.Weight != nil {
                updates["weight"] = *req.Weight
        }
        if req.TargetLang != nil {
                updates["target_lang"] = *req.TargetLang
        }
        if req.StartTime != nil {
                updates["start_time"] = *req.StartTime
        }
        if req.EndTime != nil {
                updates["end_time"] = *req.EndTime
        }

        if err := db.Table("banner").Where("id = ?", bannerID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateBanner.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "banner.update", "banner", strconv.FormatInt(bannerID, 10),
                        "update banner", ip)
        }()

        notifyCacheInvalidate("banner:", "update")
        logger.Infof("[UpdateBanner] completed: id=%d", bannerID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": bannerID,
        }))
}

// ToggleBannerStatus flips the status of a banner (0->1, 1->0).
// Reads the current status first, then writes the inverted value.
func ToggleBannerStatus(c *fiber.Ctx) error {
        bannerID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || bannerID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleBannerStatus] start: id=%d", bannerID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleBannerStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Read current status
        var banner struct {
                Status int8 `gorm:"column:status"`
        }
        if err := db.Table("banner").Where("id = ?", bannerID).Select("status").Scan(&banner).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "banner not found")
                }
                middleware.LogError(c, "ToggleBannerStatus.Scan", err)
                return bizerr.ErrInternal
        }

        newStatus := int8(1)
        if banner.Status == 1 {
                newStatus = 0
        }

        if err := db.Table("banner").Where("id = ?", bannerID).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "ToggleBannerStatus.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "banner.toggle_status", "banner", strconv.FormatInt(bannerID, 10),
                        fmt.Sprintf("toggle banner status to %d", newStatus), ip)
        }()

        notifyCacheInvalidate("banner:", "toggle")
        logger.Infof("[ToggleBannerStatus] completed: id=%d, new_status=%d", bannerID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     bannerID,
                "status": newStatus,
        }))
}

// DeleteBanner permanently deletes a banner by ID. Returns 404 if the banner does not exist.
func DeleteBanner(c *fiber.Ctx) error {
        bannerID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || bannerID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteBanner] start: id=%d", bannerID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteBanner.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("banner").Where("id = ?", bannerID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteBanner.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "banner not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "banner.delete", "banner", strconv.FormatInt(bannerID, 10),
                        "delete banner", ip)
        }()

        notifyCacheInvalidate("banner:", "delete")
        logger.Infof("[DeleteBanner] completed: id=%d", bannerID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": bannerID,
        }))
}

// ============================================================================
// 2. Activity CRUD
// ============================================================================

// CreateActivityRequest defines the request body for creating an activity
type CreateActivityRequest struct {
        Name        string `json:"name"`
        Type        string `json:"type"`
        HandlerName string `json:"handler_name"`
        Priority    int    `json:"priority"`
        StartTime   string `json:"start_time"`
        EndTime     string `json:"end_time"`
}

// CreateActivity creates a new activity definition with status=1 (active).
func CreateActivity(c *fiber.Ctx) error {
        logger.Infof("[CreateActivity] start")

        var req CreateActivityRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateActivity.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateActivity.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        activityID, err := lastInsertID(db, "activity_define", map[string]interface{}{
                "name":         req.Name,
                "type":         req.Type,
                "handler_name": req.HandlerName,
                "priority":     req.Priority,
                "status":       1,
                "start_time":   req.StartTime,
                "end_time":     req.EndTime,
                "created_at":   now,
                "updated_at":   now,
        })
        if err != nil {
                middleware.LogError(c, "CreateActivity.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "activity.create", "activity", strconv.FormatInt(activityID, 10),
                        "create activity: "+req.Name, ip)
        }()

        notifyCacheInvalidate("activity:", "create")
        logger.Infof("[CreateActivity] completed: id=%d, name=%s", activityID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": activityID,
        }))
}

// UpdateActivityRequest defines the request body for updating an activity.
// All fields are pointers to support partial updates.
type UpdateActivityRequest struct {
        Name        *string `json:"name"`
        Type        *string `json:"type"`
        HandlerName *string `json:"handler_name"`
        Priority    *int    `json:"priority"`
        StartTime   *string `json:"start_time"`
        EndTime     *string `json:"end_time"`
}

// UpdateActivity updates an activity by ID. Only provided fields are modified.
func UpdateActivity(c *fiber.Ctx) error {
        activityID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || activityID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateActivity] start: id=%d", activityID)

        var req UpdateActivityRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateActivity.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateActivity.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check activity exists
        var count int64
        if err := db.Table("activity_define").Where("id = ?", activityID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateActivity.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "activity not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Type != nil {
                updates["type"] = *req.Type
        }
        if req.HandlerName != nil {
                updates["handler_name"] = *req.HandlerName
        }
        if req.Priority != nil {
                updates["priority"] = *req.Priority
        }
        if req.StartTime != nil {
                updates["start_time"] = *req.StartTime
        }
        if req.EndTime != nil {
                updates["end_time"] = *req.EndTime
        }

        if err := db.Table("activity_define").Where("id = ?", activityID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateActivity.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "activity.update", "activity", strconv.FormatInt(activityID, 10),
                        "update activity", ip)
        }()

        notifyCacheInvalidate("activity:", "update")
        logger.Infof("[UpdateActivity] completed: id=%d", activityID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": activityID,
        }))
}

// ChangeActivityStatusRequest defines the request body for changing activity status
type ChangeActivityStatusRequest struct {
        Status string `json:"status"`
}

// ChangeActivityStatus changes the status of an activity to active (1) or inactive (0).
// Accepts both string aliases ("active"/"inactive") and numeric strings ("1"/"0").
func ChangeActivityStatus(c *fiber.Ctx) error {
        activityID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || activityID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ChangeActivityStatus] start: id=%d", activityID)

        var req ChangeActivityStatusRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "ChangeActivityStatus.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        // Map human-readable status values to database integer codes
        var newStatus int8
        switch req.Status {
        case "active", "1":
                newStatus = 1
        case "inactive", "0":
                newStatus = 0
        default:
                return bizerr.New(bizerr.CodeInvalidParams, "status must be 'active', 'inactive', '0', or '1'")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ChangeActivityStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("activity_define").Where("id = ?", activityID).Update("status", newStatus)
        if result.Error != nil {
                middleware.LogError(c, "ChangeActivityStatus.Update", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "activity not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "activity.change_status", "activity", strconv.FormatInt(activityID, 10),
                        "change activity status to "+req.Status, ip)
        }()

        logger.Infof("[ChangeActivityStatus] completed: id=%d, new_status=%d", activityID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     activityID,
                "status": newStatus,
        }))
}

// ============================================================================
// 3. Task CRUD
// ============================================================================

// CreateTaskRequest defines the request body for creating a task
type CreateTaskRequest struct {
        Name        string `json:"name"`
        Type        string `json:"type"`
        Cycle       int    `json:"cycle"`
        TargetKey   string `json:"target_key"`
        TargetValue int    `json:"target_value"`
        RewardType  string `json:"reward_type"`
        RewardValue string `json:"reward_value"`
        SortOrder   int    `json:"sort_order"`
}

// CreateTask creates a new task definition with status=1 (active).
func CreateTask(c *fiber.Ctx) error {
        logger.Infof("[CreateTask] start")

        var req CreateTaskRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateTask.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateTask.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        taskID, err := lastInsertID(db, "task_define", map[string]interface{}{
                "name":         req.Name,
                "type":         req.Type,
                "cycle":        req.Cycle,
                "target_key":   req.TargetKey,
                "target_value": req.TargetValue,
                "reward_type":  req.RewardType,
                "reward_value": req.RewardValue,
                "sort_order":   req.SortOrder,
                "status":       1,
                "created_at":   now,
                "updated_at":   now,
        })
        if err != nil {
                middleware.LogError(c, "CreateTask.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "task.create", "task", strconv.FormatInt(taskID, 10),
                        "create task: "+req.Name, ip)
        }()

        notifyCacheInvalidate("task:", "create")
        logger.Infof("[CreateTask] completed: id=%d, name=%s", taskID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": taskID,
        }))
}

// UpdateTaskRequest defines the request body for updating a task.
// All fields are pointers to support partial updates.
type UpdateTaskRequest struct {
        Name        *string `json:"name"`
        Type        *string `json:"type"`
        Cycle       *int    `json:"cycle"`
        TargetKey   *string `json:"target_key"`
        TargetValue *int    `json:"target_value"`
        RewardType  *string `json:"reward_type"`
        RewardValue *string `json:"reward_value"`
        SortOrder   *int    `json:"sort_order"`
}

// UpdateTask updates a task by ID. Only provided fields are modified.
func UpdateTask(c *fiber.Ctx) error {
        taskID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || taskID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateTask] start: id=%d", taskID)

        var req UpdateTaskRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateTask.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateTask.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check task exists
        var count int64
        if err := db.Table("task_define").Where("id = ?", taskID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateTask.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "task not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Type != nil {
                updates["type"] = *req.Type
        }
        if req.Cycle != nil {
                updates["cycle"] = *req.Cycle
        }
        if req.TargetKey != nil {
                updates["target_key"] = *req.TargetKey
        }
        if req.TargetValue != nil {
                updates["target_value"] = *req.TargetValue
        }
        if req.RewardType != nil {
                updates["reward_type"] = *req.RewardType
        }
        if req.RewardValue != nil {
                updates["reward_value"] = *req.RewardValue
        }
        if req.SortOrder != nil {
                updates["sort_order"] = *req.SortOrder
        }

        if err := db.Table("task_define").Where("id = ?", taskID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateTask.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "task.update", "task", strconv.FormatInt(taskID, 10),
                        "update task", ip)
        }()

        notifyCacheInvalidate("task:", "update")
        logger.Infof("[UpdateTask] completed: id=%d", taskID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": taskID,
        }))
}

// DeleteTask permanently deletes a task by ID. Returns 404 if the task does not exist.
func DeleteTask(c *fiber.Ctx) error {
        taskID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || taskID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteTask] start: id=%d", taskID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteTask.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("task_define").Where("id = ?", taskID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteTask.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "task not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "task.delete", "task", strconv.FormatInt(taskID, 10),
                        "delete task", ip)
        }()

        notifyCacheInvalidate("task:", "delete")
        logger.Infof("[DeleteTask] completed: id=%d", taskID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": taskID,
        }))
}

// ============================================================================
// 4. Game CRUD (Status, Vendor, Category)
// ============================================================================

// CreateGameRequest defines the request body for creating a game
type CreateGameRequest struct {
        Name       string  `json:"name"`
        GameID     string  `json:"game_id"`
        VendorID   int64   `json:"vendor_id"`
        CategoryID int64   `json:"category_id"`
        Icon       string  `json:"icon"`
        URL        string  `json:"url"`
        RTP        float64 `json:"rtp"`
        BetMin     float64 `json:"bet_min"`
        BetMax     float64 `json:"bet_max"`
        SortOrder  int     `json:"sort_order"`
        Hot        int8    `json:"hot"`
        New        int8    `json:"new"`
}

// CreateGame creates a new game info record with status=1 (active).
// Requires both name and game_id fields.
func CreateGame(c *fiber.Ctx) error {
        logger.Infof("[CreateGame] start")

        var req CreateGameRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateGame.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }
        if req.GameID == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "game_id is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateGame.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        gameID, err := lastInsertID(db, "game_info", map[string]interface{}{
                "name":        req.Name,
                "game_id":     req.GameID,
                "vendor_id":   req.VendorID,
                "category_id": req.CategoryID,
                "icon":        req.Icon,
                "url":         req.URL,
                "rtp":         req.RTP,
                "bet_min":     req.BetMin,
                "bet_max":     req.BetMax,
                "sort_order":  req.SortOrder,
                "hot":         req.Hot,
                "new":         req.New,
                "status":      1,
                "created_at":  now,
                "updated_at":  now,
        })
        if err != nil {
                middleware.LogError(c, "CreateGame.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game.create", "game", strconv.FormatInt(gameID, 10),
                        "create game: "+req.Name, ip)
        }()

        notifyCacheInvalidate("game:", "create")
        logger.Infof("[CreateGame] completed: id=%d, name=%s, game_id=%s", gameID, req.Name, req.GameID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": gameID,
        }))
}

// UpdateGameRequest defines the request body for updating a game.
// All fields are pointers to support partial updates.
type UpdateGameRequest struct {
        Name       *string  `json:"name"`
        GameID     *string  `json:"game_id"`
        VendorID   *int64   `json:"vendor_id"`
        CategoryID *int64   `json:"category_id"`
        Icon       *string  `json:"icon"`
        URL        *string  `json:"url"`
        RTP        *float64 `json:"rtp"`
        BetMin     *float64 `json:"bet_min"`
        BetMax     *float64 `json:"bet_max"`
        SortOrder  *int     `json:"sort_order"`
        Hot        *int8    `json:"hot"`
        New        *int8    `json:"new"`
}

// UpdateGame updates a game by ID. Only provided fields are modified.
func UpdateGame(c *fiber.Ctx) error {
        gameID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || gameID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateGame] start: id=%d", gameID)

        var req UpdateGameRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateGame.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateGame.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("game_info").Where("id = ?", gameID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateGame.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "game not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.GameID != nil {
                updates["game_id"] = *req.GameID
        }
        if req.VendorID != nil {
                updates["vendor_id"] = *req.VendorID
        }
        if req.CategoryID != nil {
                updates["category_id"] = *req.CategoryID
        }
        if req.Icon != nil {
                updates["icon"] = *req.Icon
        }
        if req.URL != nil {
                updates["url"] = *req.URL
        }
        if req.RTP != nil {
                updates["rtp"] = *req.RTP
        }
        if req.BetMin != nil {
                updates["bet_min"] = *req.BetMin
        }
        if req.BetMax != nil {
                updates["bet_max"] = *req.BetMax
        }
        if req.SortOrder != nil {
                updates["sort_order"] = *req.SortOrder
        }
        if req.Hot != nil {
                updates["hot"] = *req.Hot
        }
        if req.New != nil {
                updates["new"] = *req.New
        }

        if err := db.Table("game_info").Where("id = ?", gameID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateGame.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game.update", "game", strconv.FormatInt(gameID, 10),
                        "update game", ip)
        }()

        notifyCacheInvalidate("game:", "update")
        logger.Infof("[UpdateGame] completed: id=%d", gameID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": gameID,
        }))
}

// DeleteGame permanently deletes a game by ID. Returns 404 if the game does not exist.
func DeleteGame(c *fiber.Ctx) error {
        gameID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || gameID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteGame] start: id=%d", gameID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteGame.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("game_info").Where("id = ?", gameID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteGame.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "game not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game.delete", "game", strconv.FormatInt(gameID, 10),
                        "delete game", ip)
        }()

        notifyCacheInvalidate("game:", "delete")
        logger.Infof("[DeleteGame] completed: id=%d", gameID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": gameID,
        }))
}

// ToggleGameStatusRequest defines the request body for toggling game status
type ToggleGameStatusRequest struct {
        Status string `json:"status"`
}

// ToggleGameStatus toggles game status between active/inactive.
// Accepts both string aliases ("active"/"inactive") and numeric strings ("1"/"0").
func ToggleGameStatus(c *fiber.Ctx) error {
        gameID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || gameID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleGameStatus] start: id=%d", gameID)

        var req ToggleGameStatusRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "ToggleGameStatus.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        // Map human-readable status values to database integer codes
        var newStatus int8
        switch req.Status {
        case "active", "1":
                newStatus = 1
        case "inactive", "0":
                newStatus = 0
        default:
                return bizerr.New(bizerr.CodeInvalidParams, "status must be 'active', 'inactive', '0', or '1'")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleGameStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("game_info").Where("id = ?", gameID).Update("status", newStatus)
        if result.Error != nil {
                middleware.LogError(c, "ToggleGameStatus.Update", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeGameNotFound, "game not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game.toggle_status", "game", strconv.FormatInt(gameID, 10),
                        "toggle game status to "+req.Status, ip)
        }()

        notifyCacheInvalidate("game:", "toggle")
        logger.Infof("[ToggleGameStatus] completed: id=%d, new_status=%d", gameID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     gameID,
                "status": newStatus,
        }))
}

// CreateGameVendorRequest defines the request body for creating a game vendor
type CreateGameVendorRequest struct {
        Name   string `json:"name"`
        Logo   string `json:"logo"`
        Status int8   `json:"status"`
}

// CreateGameVendor creates a new game vendor (provider) with the given name, logo, and status.
func CreateGameVendor(c *fiber.Ctx) error {
        logger.Infof("[CreateGameVendor] start")

        var req CreateGameVendorRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateGameVendor.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateGameVendor.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        vendorID, err := lastInsertID(db, "game_vendor", map[string]interface{}{
                "name":       req.Name,
                "logo":       req.Logo,
                "status":     req.Status,
                "created_at": now,
                "updated_at": now,
        })
        if err != nil {
                middleware.LogError(c, "CreateGameVendor.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_vendor.create", "game_vendor", strconv.FormatInt(vendorID, 10),
                        "create game vendor: "+req.Name, ip)
        }()

        notifyCacheInvalidate("game_vendor:", "create")
        logger.Infof("[CreateGameVendor] completed: id=%d, name=%s", vendorID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": vendorID,
        }))
}

// UpdateGameVendorRequest defines the request body for updating a game vendor.
// All fields are pointers to support partial updates.
type UpdateGameVendorRequest struct {
        Name   *string `json:"name"`
        Logo   *string `json:"logo"`
        Status *int8   `json:"status"`
}

// UpdateGameVendor updates a game vendor by ID. Only provided fields are modified.
func UpdateGameVendor(c *fiber.Ctx) error {
        vendorID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || vendorID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateGameVendor] start: id=%d", vendorID)

        var req UpdateGameVendorRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateGameVendor.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateGameVendor.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("game_vendor").Where("id = ?", vendorID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateGameVendor.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "vendor not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Logo != nil {
                updates["logo"] = *req.Logo
        }
        if req.Status != nil {
                updates["status"] = *req.Status
        }

        if err := db.Table("game_vendor").Where("id = ?", vendorID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateGameVendor.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_vendor.update", "game_vendor", strconv.FormatInt(vendorID, 10),
                        "update game vendor", ip)
        }()

        notifyCacheInvalidate("game_vendor:", "update")
        logger.Infof("[UpdateGameVendor] completed: id=%d", vendorID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": vendorID,
        }))
}

// DeleteGameVendor permanently deletes a game vendor by ID. Returns 404 if not found.
func DeleteGameVendor(c *fiber.Ctx) error {
        vendorID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || vendorID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteGameVendor] start: id=%d", vendorID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteGameVendor.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("game_vendor").Where("id = ?", vendorID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteGameVendor.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "vendor not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_vendor.delete", "game_vendor", strconv.FormatInt(vendorID, 10),
                        "delete game vendor", ip)
        }()

        notifyCacheInvalidate("game_vendor:", "delete")
        logger.Infof("[DeleteGameVendor] completed: id=%d", vendorID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": vendorID,
        }))
}

// CreateGameCategoryRequest defines the request body for creating a game category
type CreateGameCategoryRequest struct {
        ParentID  int64  `json:"parent_id"`
        LobbyID   int64  `json:"lobby_id"`
        Name      string `json:"name"`
        Icon      string `json:"icon"`
        SortOrder int    `json:"sort_order"`
        Status    int8   `json:"status"`
}

// CreateGameCategory creates a new game category, optionally nested under a parent category.
func CreateGameCategory(c *fiber.Ctx) error {
        logger.Infof("[CreateGameCategory] start")

        var req CreateGameCategoryRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateGameCategory.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateGameCategory.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        categoryID, err := lastInsertID(db, "game_category", map[string]interface{}{
                "parent_id":  req.ParentID,
                "lobby_id":   req.LobbyID,
                "name":       req.Name,
                "icon":       req.Icon,
                "sort_order": req.SortOrder,
                "status":     req.Status,
                "created_at": now,
                "updated_at": now,
        })
        if err != nil {
                middleware.LogError(c, "CreateGameCategory.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_category.create", "game_category", strconv.FormatInt(categoryID, 10),
                        "create game category: "+req.Name, ip)
        }()

        notifyCacheInvalidate("game_category:", "create")
        logger.Infof("[CreateGameCategory] completed: id=%d, name=%s", categoryID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": categoryID,
        }))
}

// UpdateGameCategoryRequest defines the request body for updating a game category.
// All fields are pointers to support partial updates.
type UpdateGameCategoryRequest struct {
        ParentID  *int64  `json:"parent_id"`
        LobbyID   *int64  `json:"lobby_id"`
        Name      *string `json:"name"`
        Icon      *string `json:"icon"`
        SortOrder *int    `json:"sort_order"`
        Status    *int8   `json:"status"`
}

// UpdateGameCategory updates a game category by ID. Only provided fields are modified.
func UpdateGameCategory(c *fiber.Ctx) error {
        categoryID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || categoryID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateGameCategory] start: id=%d", categoryID)

        var req UpdateGameCategoryRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateGameCategory.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateGameCategory.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("game_category").Where("id = ?", categoryID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateGameCategory.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "category not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.ParentID != nil {
                updates["parent_id"] = *req.ParentID
        }
        if req.LobbyID != nil {
                updates["lobby_id"] = *req.LobbyID
        }
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Icon != nil {
                updates["icon"] = *req.Icon
        }
        if req.SortOrder != nil {
                updates["sort_order"] = *req.SortOrder
        }
        if req.Status != nil {
                updates["status"] = *req.Status
        }

        if err := db.Table("game_category").Where("id = ?", categoryID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateGameCategory.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_category.update", "game_category", strconv.FormatInt(categoryID, 10),
                        "update game category", ip)
        }()

        notifyCacheInvalidate("game_category:", "update")
        logger.Infof("[UpdateGameCategory] completed: id=%d", categoryID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": categoryID,
        }))
}

// DeleteGameCategory permanently deletes a game category by ID. Returns 404 if not found.
func DeleteGameCategory(c *fiber.Ctx) error {
        categoryID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || categoryID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteGameCategory] start: id=%d", categoryID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteGameCategory.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("game_category").Where("id = ?", categoryID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteGameCategory.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "category not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_category.delete", "game_category", strconv.FormatInt(categoryID, 10),
                        "delete game category", ip)
        }()

        notifyCacheInvalidate("game_category:", "delete")
        logger.Infof("[DeleteGameCategory] completed: id=%d", categoryID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": categoryID,
        }))
}

// ToggleGameCategoryStatus toggles a game category's status between active (1) and inactive (0).
func ToggleGameCategoryStatus(c *fiber.Ctx) error {
        categoryID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || categoryID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleGameCategoryStatus] start: id=%d", categoryID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleGameCategoryStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var category struct {
                Status int8 `gorm:"column:status"`
        }
        if err := db.Table("game_category").Where("id = ?", categoryID).Select("status").Scan(&category).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "category not found")
                }
                middleware.LogError(c, "ToggleGameCategoryStatus.Scan", err)
                return bizerr.ErrInternal
        }

        newStatus := int8(1)
        if category.Status == 1 {
                newStatus = 0
        }

        if err := db.Table("game_category").Where("id = ?", categoryID).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "ToggleGameCategoryStatus.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_category.toggle_status", "game_category", strconv.FormatInt(categoryID, 10),
                        fmt.Sprintf("toggle category status to %d", newStatus), ip)
        }()

        notifyCacheInvalidate("game_category:", "toggle")
        logger.Infof("[ToggleGameCategoryStatus] completed: id=%d, new_status=%d", categoryID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     categoryID,
                "status": newStatus,
        }))
}

// ToggleGameVendorStatus toggles a game vendor's status between active (1) and inactive (0).
func ToggleGameVendorStatus(c *fiber.Ctx) error {
        vendorID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || vendorID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleGameVendorStatus] start: id=%d", vendorID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleGameVendorStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var vendor struct {
                Status int8 `gorm:"column:status"`
        }
        if err := db.Table("game_vendor").Where("id = ?", vendorID).Select("status").Scan(&vendor).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "vendor not found")
                }
                middleware.LogError(c, "ToggleGameVendorStatus.Scan", err)
                return bizerr.ErrInternal
        }

        newStatus := int8(1)
        if vendor.Status == 1 {
                newStatus = 0
        }

        if err := db.Table("game_vendor").Where("id = ?", vendorID).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "ToggleGameVendorStatus.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "game_vendor.toggle_status", "game_vendor", strconv.FormatInt(vendorID, 10),
                        fmt.Sprintf("toggle vendor status to %d", newStatus), ip)
        }()

        notifyCacheInvalidate("game_vendor:", "toggle")
        logger.Infof("[ToggleGameVendorStatus] completed: id=%d, new_status=%d", vendorID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     vendorID,
                "status": newStatus,
        }))
}

// ============================================================================
// 5. Payment Channel CRUD
// ==============================================================================

// CreatePaymentChannelRequest defines the request body for creating a payment channel
type CreatePaymentChannelRequest struct {
        Name             string  `json:"name"`
        Type             string  `json:"type"`
        Config           string  `json:"config"`
        SupportedRegions string  `json:"supported_regions"`
        MinAmount        float64 `json:"min_amount"`
        MaxAmount        float64 `json:"max_amount"`
        SortOrder        int     `json:"sort_order"`
}

// CreatePaymentChannel creates a new payment channel with status=1 (active).
func CreatePaymentChannel(c *fiber.Ctx) error {
        logger.Infof("[CreatePaymentChannel] start")

        var req CreatePaymentChannelRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreatePaymentChannel.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreatePaymentChannel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        channelID, err := lastInsertID(db, "payment_channel", map[string]interface{}{
                "name":              req.Name,
                "type":              req.Type,
                "config":            req.Config,
                "supported_regions": req.SupportedRegions,
                "min_amount":        req.MinAmount,
                "max_amount":        req.MaxAmount,
                "sort_order":        req.SortOrder,
                "status":            1,
                "created_at":        now,
                "updated_at":        now,
        })
        if err != nil {
                middleware.LogError(c, "CreatePaymentChannel.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "payment_channel.create", "payment_channel", strconv.FormatInt(channelID, 10),
                        "create payment channel: "+req.Name, ip)
        }()

        notifyCacheInvalidate("payment_channel:", "create")
        logger.Infof("[CreatePaymentChannel] completed: id=%d, name=%s", channelID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": channelID,
        }))
}

// UpdatePaymentChannelRequest defines the request body for updating a payment channel.
// All fields are pointers to support partial updates.
type UpdatePaymentChannelRequest struct {
        Name             *string  `json:"name"`
        Type             *string  `json:"type"`
        Config           *string  `json:"config"`
        SupportedRegions *string  `json:"supported_regions"`
        MinAmount        *float64 `json:"min_amount"`
        MaxAmount        *float64 `json:"max_amount"`
        SortOrder        *int     `json:"sort_order"`
}

// UpdatePaymentChannel updates a payment channel by ID. Only provided fields are modified.
func UpdatePaymentChannel(c *fiber.Ctx) error {
        channelID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || channelID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdatePaymentChannel] start: id=%d", channelID)

        var req UpdatePaymentChannelRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdatePaymentChannel.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdatePaymentChannel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("payment_channel").Where("id = ?", channelID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdatePaymentChannel.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "payment channel not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.Type != nil {
                updates["type"] = *req.Type
        }
        if req.Config != nil {
                updates["config"] = *req.Config
        }
        if req.SupportedRegions != nil {
                updates["supported_regions"] = *req.SupportedRegions
        }
        if req.MinAmount != nil {
                updates["min_amount"] = *req.MinAmount
        }
        if req.MaxAmount != nil {
                updates["max_amount"] = *req.MaxAmount
        }
        if req.SortOrder != nil {
                updates["sort_order"] = *req.SortOrder
        }

        if err := db.Table("payment_channel").Where("id = ?", channelID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdatePaymentChannel.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "payment_channel.update", "payment_channel", strconv.FormatInt(channelID, 10),
                        "update payment channel", ip)
        }()

        notifyCacheInvalidate("payment_channel:", "update")
        logger.Infof("[UpdatePaymentChannel] completed: id=%d", channelID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": channelID,
        }))
}

// TogglePaymentChannelStatus flips the status of a payment channel (0->1, 1->0).
// Reads the current status first, then writes the inverted value.
func TogglePaymentChannelStatus(c *fiber.Ctx) error {
        channelID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || channelID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[TogglePaymentChannelStatus] start: id=%d", channelID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "TogglePaymentChannelStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Read current status
        var channel struct {
                Status int8 `gorm:"column:status"`
        }
        if err := db.Table("payment_channel").Where("id = ?", channelID).Select("status").Scan(&channel).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "payment channel not found")
                }
                middleware.LogError(c, "TogglePaymentChannelStatus.Scan", err)
                return bizerr.ErrInternal
        }

        newStatus := int8(1)
        if channel.Status == 1 {
                newStatus = 0
        }

        if err := db.Table("payment_channel").Where("id = ?", channelID).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "TogglePaymentChannelStatus.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "payment_channel.toggle_status", "payment_channel", strconv.FormatInt(channelID, 10),
                        fmt.Sprintf("toggle payment channel status to %d", newStatus), ip)
        }()

        notifyCacheInvalidate("payment_channel:", "toggle")
        logger.Infof("[TogglePaymentChannelStatus] completed: id=%d, new_status=%d", channelID, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     channelID,
                "status": newStatus,
        }))
}

// DeletePaymentChannel permanently deletes a payment channel by ID. Returns 404 if not found.
func DeletePaymentChannel(c *fiber.Ctx) error {
        channelID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || channelID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeletePaymentChannel] start: id=%d", channelID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeletePaymentChannel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        result := db.Table("payment_channel").Where("id = ?", channelID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeletePaymentChannel.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "payment channel not found")
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "payment_channel.delete", "payment_channel", strconv.FormatInt(channelID, 10),
                        "delete payment channel", ip)
        }()

        notifyCacheInvalidate("payment_channel:", "delete")
        logger.Infof("[DeletePaymentChannel] completed: id=%d", channelID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": channelID,
        }))
}

// ============================================================================
// 6. VIP Level CRUD
// ============================================================================

// CreateVIPLevelRequest defines the request body for creating a VIP level.
type CreateVIPLevelRequest struct {
        Name             string  `json:"name"`
        GrowthRequired   int64   `json:"growth_required"`
        WithdrawFeeRate  float64 `json:"withdraw_fee_rate"`
        DailySigninBonus float64 `json:"daily_signin_bonus"`
        Status           int8    `json:"status"`
}

// CreateVIPLevel creates a new VIP level configuration.
// VIP levels define thresholds, fee rates, and bonuses for the VIP tier system.
func CreateVIPLevel(c *fiber.Ctx) error {
        logger.Infof("[CreateVIPLevel] start")

        var req CreateVIPLevelRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateVIPLevel.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Name == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "name is required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateVIPLevel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        levelID, err := lastInsertID(db, "vip_level_config", map[string]interface{}{
                "name":              req.Name,
                "growth_required":   req.GrowthRequired,
                "withdraw_fee_rate":  req.WithdrawFeeRate,
                "daily_signin_bonus": req.DailySigninBonus,
                "status":            req.Status,
                "created_at":        now,
                "updated_at":        now,
        })
        if err != nil {
                middleware.LogError(c, "CreateVIPLevel.Insert", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "vip_level.create", "vip_level_config", strconv.FormatInt(levelID, 10),
                        "create VIP level: "+req.Name, ip)
        }()

        notifyCacheInvalidate("vip_level:", "create")
        logger.Infof("[CreateVIPLevel] completed: id=%d, name=%s", levelID, req.Name)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": levelID,
        }))
}

// UpdateVIPLevelRequest defines the request body for updating a VIP level.
// All fields are pointers to support partial updates.
type UpdateVIPLevelRequest struct {
        Name             *string  `json:"name"`
        GrowthRequired   *int64   `json:"growth_required"`
        WithdrawFeeRate  *float64 `json:"withdraw_fee_rate"`
        DailySigninBonus *float64 `json:"daily_signin_bonus"`
        Status           *int8    `json:"status"`
}

// UpdateVIPLevel updates a VIP level configuration by ID. Only provided fields are modified.
// VIP levels define thresholds, fee rates, and bonuses for the VIP tier system.
func UpdateVIPLevel(c *fiber.Ctx) error {
        levelID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || levelID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateVIPLevel] start: id=%d", levelID)

        var req UpdateVIPLevelRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateVIPLevel.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateVIPLevel.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("vip_level_config").Where("id = ?", levelID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateVIPLevel.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "VIP level not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.Name != nil {
                updates["name"] = *req.Name
        }
        if req.GrowthRequired != nil {
                updates["growth_required"] = *req.GrowthRequired
        }
        if req.WithdrawFeeRate != nil {
                updates["withdraw_fee_rate"] = *req.WithdrawFeeRate
        }
        if req.DailySigninBonus != nil {
                updates["daily_signin_bonus"] = *req.DailySigninBonus
        }
        if req.Status != nil {
                updates["status"] = *req.Status
        }

        if err := db.Table("vip_level_config").Where("id = ?", levelID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateVIPLevel.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "vip_level.update", "vip_level_config", strconv.FormatInt(levelID, 10),
                        "update VIP level", ip)
        }()

        notifyCacheInvalidate("vip_level:", "update")
        logger.Infof("[UpdateVIPLevel] completed: id=%d", levelID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": levelID,
        }))
}

// ============================================================================
// 7. Admin User CRUD
// ============================================================================

// CreateAdminUserRequest defines the request body for creating an admin user
type CreateAdminUserRequest struct {
        Username string `json:"username"`
        Password string `json:"password"`
        RealName string `json:"real_name"`
        Email    string `json:"email"`
        Role     string `json:"role"`
}

// CreateAdminUser creates a new admin user with hashed password.
// Enforces role validation and prevents non-super admins from creating super accounts.
// Also checks username uniqueness before insertion.
func CreateAdminUser(c *fiber.Ctx) error {
        logger.Infof("[CreateAdminUser] start")

        var req CreateAdminUserRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateAdminUser.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Username == "" || req.Password == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "username and password are required")
        }
        if err := validateAdminPassword(req.Password); err != nil {
                return err
        }
        if !IsValidRole(req.Role) {
                return bizerr.New(bizerr.CodeInvalidParams, "role must be one of: super_admin, admin, operator, finance, support, viewer")
        }
        // Prevent creating "super_admin" role unless the creator is also super_admin
        if req.Role == "super_admin" {
                creatorRole := middleware.GetAdminRole(c)
                if creatorRole != "super_admin" {
                        return bizerr.New(bizerr.CodeForbidden, "only super admins can create super admin accounts")
                }
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateAdminUser.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check username uniqueness
        var count int64
        if err := db.Table("admin_user").Where("username = ?", req.Username).Count(&count).Error; err != nil {
                middleware.LogError(c, "CreateAdminUser.CheckUsername", err)
                return bizerr.ErrInternal
        }
        if count > 0 {
                return bizerr.New(bizerr.CodeUserExists, "username already exists")
        }

        // Hash password before storage — plaintext must never be persisted
        passwordHash, err := auth.HashPassword(req.Password)
        if err != nil {
                middleware.LogError(c, "CreateAdminUser.HashPassword", err)
                return bizerr.ErrInternal
        }

        now := time.Now()
        result := db.Table("admin_user").Create(map[string]interface{}{
                "username":      req.Username,
                "password_hash": passwordHash,
                "real_name":     req.RealName,
                "email":         req.Email,
                "role":          req.Role,
                "status":        1,
                "created_at":    now,
                "updated_at":    now,
        })
        if result.Error != nil {
                middleware.LogError(c, "CreateAdminUser.Insert", result.Error)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "admin_user.create", "admin_user", req.Username,
                        "create admin user: "+req.Username, ip)
        }()

        notifyCacheInvalidate("admin_user:", "create")
        logger.Infof("[CreateAdminUser] completed: username=%s, role=%s", req.Username, req.Role)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "username": req.Username,
        }))
}

// UpdateAdminUserRequest defines the request body for updating an admin user.
// All fields are pointers to support partial updates.
type UpdateAdminUserRequest struct {
        RealName *string `json:"real_name"`
        Email    *string `json:"email"`
        Role     *string `json:"role"`
        Password *string `json:"password"`
}

// UpdateAdminUser updates an admin user by ID. Only provided fields are modified.
// Role changes to "super_admin" are restricted to super admins. Password changes are re-hashed.
func UpdateAdminUser(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[UpdateAdminUser] start: id=%d", targetID)

        var req UpdateAdminUserRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateAdminUser.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateAdminUser.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var count int64
        if err := db.Table("admin_user").Where("id = ?", targetID).Count(&count).Error; err != nil {
                middleware.LogError(c, "UpdateAdminUser.Count", err)
                return bizerr.ErrInternal
        }
        if count == 0 {
                return bizerr.New(bizerr.CodeNotFound, "admin user not found")
        }

        updates := map[string]interface{}{"updated_at": time.Now()}
        if req.RealName != nil {
                updates["real_name"] = *req.RealName
        }
        if req.Email != nil {
                updates["email"] = *req.Email
        }
        if req.Role != nil {
                if !IsValidRole(*req.Role) {
                        return bizerr.New(bizerr.CodeInvalidParams, "role must be one of: super_admin, admin, operator, finance, support, viewer")
                }
                // Only super can assign super role
                if *req.Role == "super_admin" {
                        creatorRole := middleware.GetAdminRole(c)
                        if creatorRole != "super_admin" {
                                return bizerr.New(bizerr.CodeForbidden, "only super admins can assign super role")
                        }
                }
                updates["role"] = *req.Role
        }
        if req.Password != nil && *req.Password != "" {
                if err := validateAdminPassword(*req.Password); err != nil {
                        return err
                }
                // Hash password before storage — plaintext must never be persisted
                passwordHash, err := auth.HashPassword(*req.Password)
                if err != nil {
                        middleware.LogError(c, "UpdateAdminUser.HashPassword", err)
                        return bizerr.ErrInternal
                }
                updates["password_hash"] = passwordHash
        }

        if err := db.Table("admin_user").Where("id = ?", targetID).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateAdminUser.Update", err)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "admin_user.update", "admin_user", strconv.FormatInt(targetID, 10),
                        "update admin user", ip)
        }()

        notifyCacheInvalidate("admin_user:", "update")
        logger.Infof("[UpdateAdminUser] completed: id=%d", targetID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": targetID,
        }))
}

// ToggleAdminUserStatus flips the status of an admin user (0->1, 1->0).
// Prevents an admin from toggling their own status to avoid self-lockout.
func ToggleAdminUserStatus(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[ToggleAdminUserStatus] start: id=%d", targetID)

        // Prevent toggling self — avoids accidental self-lockout
        currentAdminID := middleware.GetUserID(c)
        if currentAdminID == targetID {
                return bizerr.New(bizerr.CodeForbidden, "cannot toggle your own status")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ToggleAdminUserStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Read current status
        var adminUser struct {
                Username string `gorm:"column:username"`
                Status   int8   `gorm:"column:status"`
        }
        if err := db.Table("admin_user").Where("id = ?", targetID).Select("username, status").Scan(&adminUser).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "admin user not found")
                }
                middleware.LogError(c, "ToggleAdminUserStatus.Scan", err)
                return bizerr.ErrInternal
        }

        newStatus := int8(1)
        if adminUser.Status == 1 {
                newStatus = 0
        }

        if err := db.Table("admin_user").Where("id = ?", targetID).Update("status", newStatus).Error; err != nil {
                middleware.LogError(c, "ToggleAdminUserStatus.Update", err)
                return bizerr.ErrInternal
        }

        ip := c.IP()
        go func() {
                RecordAuditLog(currentAdminID, "", "admin_user.toggle_status", "admin_user", strconv.FormatInt(targetID, 10),
                        fmt.Sprintf("toggle admin user status to %d: %s", newStatus, adminUser.Username), ip)
        }()

        notifyCacheInvalidate("admin_user:", "toggle")
        logger.Infof("[ToggleAdminUserStatus] completed: id=%d, username=%s, new_status=%d", targetID, adminUser.Username, newStatus)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":     targetID,
                "status": newStatus,
        }))
}

// DeleteAdminUser permanently deletes an admin user by ID.
// Prevents self-deletion to avoid losing all admin access.
// Captures the username before deletion for the audit log.
func DeleteAdminUser(c *fiber.Ctx) error {
        targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || targetID <= 0 {
                return bizerr.ErrInvalidParams
        }
        logger.Infof("[DeleteAdminUser] start: id=%d", targetID)

        // Prevent deleting self — avoids losing all admin access
        currentAdminID := middleware.GetUserID(c)
        if currentAdminID == targetID {
                return bizerr.New(bizerr.CodeForbidden, "cannot delete yourself")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteAdminUser.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Get username for audit log before deletion
        var adminUser struct {
                Username string `gorm:"column:username"`
        }
        if err := db.Table("admin_user").Where("id = ?", targetID).Select("username").Scan(&adminUser).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeNotFound, "admin user not found")
                }
                middleware.LogError(c, "DeleteAdminUser.Scan", err)
                return bizerr.ErrInternal
        }

        result := db.Table("admin_user").Where("id = ?", targetID).Delete(nil)
        if result.Error != nil {
                middleware.LogError(c, "DeleteAdminUser.Delete", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.New(bizerr.CodeNotFound, "admin user not found")
        }

        ip := c.IP()
        go func() {
                RecordAuditLog(currentAdminID, "", "admin_user.delete", "admin_user", strconv.FormatInt(targetID, 10),
                        "delete admin user: "+adminUser.Username, ip)
        }()

        notifyCacheInvalidate("admin_user:", "delete")
        logger.Infof("[DeleteAdminUser] completed: id=%d, username=%s", targetID, adminUser.Username)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id": targetID,
        }))
}

// ============================================================================
// 8. Mail Send
// ============================================================================

// SendMailRequest defines the request body for sending mail to a user
type SendMailRequest struct {
        UserID  int64  `json:"user_id"`
        Subject string `json:"subject"`
        Content string `json:"content"`
        Type    string `json:"type"`
}

// SendMail inserts a mail record into the mail_queue table with status=0 (pending).
// The actual mail delivery is handled asynchronously by a worker consuming from the queue.
func SendMail(c *fiber.Ctx) error {
        logger.Infof("[SendMail] start: user_id=%s", c.FormValue("user_id"))

        var req SendMailRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "SendMail.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Subject == "" || req.Content == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "subject and content are required")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "SendMail.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        now := time.Now()
        // Insert with status=0 (pending) — a background worker will pick this up for delivery
        result := db.Table("mail_queue").Create(map[string]interface{}{
                "user_id":    req.UserID,
                "subject":    req.Subject,
                "content":    req.Content,
                "type":       req.Type,
                "status":     0, // pending
                "created_at": now,
        })
        if result.Error != nil {
                middleware.LogError(c, "SendMail.Insert", result.Error)
                return bizerr.ErrInternal
        }

        adminID := middleware.GetUserID(c)
        ip := c.IP()
        go func() {
                RecordAuditLog(adminID, "", "mail.send", "mail_queue", strconv.FormatInt(req.UserID, 10),
                        "send mail: "+req.Subject, ip)
        }()

        logger.Infof("[SendMail] completed: user_id=%d, subject=%s", req.UserID, req.Subject)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "subject": req.Subject,
                "status":  "queued",
        }))
}