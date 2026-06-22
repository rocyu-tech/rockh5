// Package handler provides admin HTTP handlers for spin wheel management.
//
// admin_spin.go — Spin Wheel Admin Handlers
//
// This file implements admin-facing endpoints for managing the spin wheel feature:
//   - Spin config CRUD: create, update, delete, list spin wheel configurations
//   - Plot config CRUD: manage plot/script configurations that control the amount sequence
//   - Invite config CRUD: manage invite probability settings per VIP level
//   - Order management: list and audit (approve/reject) spin withdrawal orders
//   - Stats: aggregated spin wheel statistics for the admin dashboard
package handler

import (
        "encoding/json"
        "errors"
        "fmt"
        "strconv"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// ── Spin Config Admin Handlers ──

// CreateSpinConfigReq is the request body for creating/updating a spin config.
type CreateSpinConfigReq struct {
        SpinID             string `json:"spin_id"`
        FullGold           int64  `json:"full_gold"`
        FlowMulti          int    `json:"flow_multi"`
        TimeLimitHour      int    `json:"time_limit_hour"`
        AuditUserCnt       int    `json:"audit_usercnt"`
        Rule2InviteTotalLT int    `json:"audit_rule_2_invitetotal_lt"`
        Rule2FlowMulti     int64  `json:"audit_rule_2_flowmutil"`
        Rule3InviteTotalGE int    `json:"audit_rule_3_invtetotal_ge"`
        Rule4Users         int    `json:"audit_rule_4_users"`
        Rule4Labels        string `json:"audit_rule_4_labels"`
        StartTime          int64  `json:"start_time"`
        EndTime            int64  `json:"end_time"`
        UserType           int    `json:"user_type"`
        TagList            string `json:"tag_list"`
        UserList           string `json:"user_list"`
        PlotList           string `json:"plot_list"`
        InviteGroupID      int    `json:"invite_group_id"`
        Priority           int    `json:"priority"`
        BoxGT              int    `json:"box_gt"`
        BoxLE              int    `json:"box_le"`
        Items              []model.SpinItem `json:"items"`
}

// CreateSpinConfig handles POST /admin/spin/configs — creates a new spin config.
func CreateSpinConfig(c *fiber.Ctx) error {
        var req CreateSpinConfigReq
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateSpinConfig.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        logger.Infof("[CreateSpinConfig] start: admin_id=%d spin_id=%s", adminID, req.SpinID)

        if req.SpinID == "" || req.FullGold <= 0 {
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "CreateSpinConfig.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Serialize items to JSON
        itemsJSON, err := json.Marshal(req.Items)
        if err != nil {
                middleware.LogError(c, "CreateSpinConfig.MarshalItems", err)
                return bizerr.ErrInternal
        }

        cfg := model.SpinConfig{
                SpinID:             req.SpinID,
                FullGold:           req.FullGold,
                FlowMulti:          req.FlowMulti,
                TimeLimitHour:      req.TimeLimitHour,
                AuditUserCnt:       req.AuditUserCnt,
                Rule2InviteTotalLT: req.Rule2InviteTotalLT,
                Rule2FlowMulti:     req.Rule2FlowMulti,
                Rule3InviteTotalGE: req.Rule3InviteTotalGE,
                Rule4Users:         req.Rule4Users,
                Rule4Labels:        req.Rule4Labels,
                StartTime:          req.StartTime,
                EndTime:            req.EndTime,
                UserType:           req.UserType,
                TagList:            req.TagList,
                UserList:           req.UserList,
                PlotList:           req.PlotList,
                InviteGroupID:      req.InviteGroupID,
                Priority:           req.Priority,
                BoxGT:              req.BoxGT,
                BoxLE:              req.BoxLE,
                ItemsJSON:          string(itemsJSON),
                Status:             1,
        }

        if err := db.Create(&cfg).Error; err != nil {
                middleware.LogError(c, "CreateSpinConfig.Create", err)
                return bizerr.ErrInternal
        }

        // Reload config cache
        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.create", "spin_config", strconv.FormatInt(cfg.ID, 10),
                "create spin config: "+req.SpinID, c.IP())

        logger.Infof("[CreateSpinConfig] success: id=%d spin_id=%s", cfg.ID, cfg.SpinID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": cfg.ID}))
}

// UpdateSpinConfig handles PUT /admin/spin/configs/:id — updates an existing spin config.
func UpdateSpinConfig(c *fiber.Ctx) error {
        id, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || id <= 0 {
                return bizerr.ErrInvalidParams
        }

        var req CreateSpinConfigReq
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateSpinConfig.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        logger.Infof("[UpdateSpinConfig] start: admin_id=%d id=%d", adminID, id)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateSpinConfig.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var existing model.SpinConfig
        if err := db.First(&existing, id).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrNotFound
                }
                middleware.LogError(c, "UpdateSpinConfig.Find", err)
                return bizerr.ErrInternal
        }

        // Serialize items
        itemsJSON, err := json.Marshal(req.Items)
        if err != nil {
                middleware.LogError(c, "UpdateSpinConfig.MarshalItems", err)
                return bizerr.ErrInternal
        }

        updates := map[string]interface{}{
                "full_gold":              req.FullGold,
                "flow_multi":             req.FlowMulti,
                "time_limit_hour":        req.TimeLimitHour,
                "audit_usercnt":          req.AuditUserCnt,
                "audit_rule_2_invitetotal_lt": req.Rule2InviteTotalLT,
                "audit_rule_2_flowmutil":      req.Rule2FlowMulti,
                "audit_rule_3_invtetotal_ge":  req.Rule3InviteTotalGE,
                "audit_rule_4_users":          req.Rule4Users,
                "audit_rule_4_labels":         req.Rule4Labels,
                "start_time":              req.StartTime,
                "end_time":                req.EndTime,
                "user_type":               req.UserType,
                "tag_list":                req.TagList,
                "user_list":               req.UserList,
                "plot_list":               req.PlotList,
                "invite_group_id":         req.InviteGroupID,
                "priority":                req.Priority,
                "box_gt":                  req.BoxGT,
                "box_le":                  req.BoxLE,
                "items_json":              string(itemsJSON),
        }
        if req.SpinID != "" {
                updates["spin_id"] = req.SpinID
        }

        if err := db.Model(&existing).Updates(updates).Error; err != nil {
                middleware.LogError(c, "UpdateSpinConfig.Update", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.update", "spin_config", strconv.FormatInt(id, 10),
                "update spin config: "+existing.SpinID, c.IP())

        logger.Infof("[UpdateSpinConfig] success: id=%d", id)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": id}))
}

// DeleteSpinConfig handles DELETE /admin/spin/configs/:id — deletes a spin config.
func DeleteSpinConfig(c *fiber.Ctx) error {
        id, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || id <= 0 {
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        logger.Infof("[DeleteSpinConfig] start: admin_id=%d id=%d", adminID, id)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "DeleteSpinConfig.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var existing model.SpinConfig
        if err := db.First(&existing, id).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrNotFound
                }
                middleware.LogError(c, "DeleteSpinConfig.Find", err)
                return bizerr.ErrInternal
        }

        if err := db.Delete(&existing).Error; err != nil {
                middleware.LogError(c, "DeleteSpinConfig.Delete", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.delete", "spin_config", strconv.FormatInt(id, 10),
                "delete spin config: "+existing.SpinID, c.IP())

        logger.Infof("[DeleteSpinConfig] success: id=%d spin_id=%s", id, existing.SpinID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": id}))
}

// ListSpinConfigs handles GET /admin/spin/configs — lists all spin configs with pagination.
func ListSpinConfigs(c *fiber.Ctx) error {
        page, pageSize, offset := ParsePagination(c)

        logger.Infof("[ListSpinConfigs] start: page=%d page_size=%d", page, pageSize)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ListSpinConfigs.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var total int64
        db.Model(&model.SpinConfig{}).Count(&total)

        var configs []model.SpinConfig
        if err := db.Order("id DESC").Offset(offset).Limit(pageSize).Find(&configs).Error; err != nil {
                middleware.LogError(c, "ListSpinConfigs.Find", err)
                return bizerr.ErrInternal
        }

        if configs == nil {
                configs = []model.SpinConfig{}
        }

        logger.Infof("[ListSpinConfigs] success: total=%d returned=%d", total, len(configs))
        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     configs,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}

// GetSpinConfigDetail handles GET /admin/spin/configs/:id — gets a single spin config.
func GetSpinConfigDetail(c *fiber.Ctx) error {
        id, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || id <= 0 {
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetSpinConfigDetail.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var cfg model.SpinConfig
        if err := db.First(&cfg, id).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrNotFound
                }
                middleware.LogError(c, "GetSpinConfigDetail.Find", err)
                return bizerr.ErrInternal
        }

        return c.JSON(bizerr.SuccessResponse(cfg))
}

// ── Plot Config Admin Handlers ──

// CreatePlotConfigReq is the request body for creating/updating a plot config.
type CreatePlotConfigReq struct {
        StepInc int   `json:"step_inc"`
        FreeInc []int `json:"free_inc"`
}

// CreatePlotConfig handles POST /admin/spin/plots — creates a new plot config.
func CreatePlotConfig(c *fiber.Ctx) error {
        var req CreatePlotConfigReq
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreatePlotConfig.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        logger.Infof("[CreatePlotConfig] start: admin_id=%d step_inc=%d len(free_inc)=%d", adminID, req.StepInc, len(req.FreeInc))

        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        freeIncJSON, err := json.Marshal(req.FreeInc)
        if err != nil {
                middleware.LogError(c, "CreatePlotConfig.MarshalFreeInc", err)
                return bizerr.ErrInternal
        }

        plot := model.SpinPlotConfig{
                StepInc: req.StepInc,
                FreeInc: string(freeIncJSON),
                Status:  1,
        }

        if err := db.Create(&plot).Error; err != nil {
                middleware.LogError(c, "CreatePlotConfig.Create", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.create_plot", "spin_plot_config", strconv.FormatInt(int64(plot.ID), 10),
                "create plot config", c.IP())

        logger.Infof("[CreatePlotConfig] success: id=%d", plot.ID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": plot.ID}))
}

// ListPlotConfigs handles GET /admin/spin/plots — lists all plot configs.
func ListPlotConfigs(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        var plots []model.SpinPlotConfig
        if err := db.Order("id DESC").Find(&plots).Error; err != nil {
                middleware.LogError(c, "ListPlotConfigs.Find", err)
                return bizerr.ErrInternal
        }

        if plots == nil {
                plots = []model.SpinPlotConfig{}
        }

        return c.JSON(bizerr.SuccessResponse(plots))
}

// DeletePlotConfig handles DELETE /admin/spin/plots/:id — deletes a plot config.
func DeletePlotConfig(c *fiber.Ctx) error {
        id, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || id <= 0 {
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        if err := db.Delete(&model.SpinPlotConfig{}, id).Error; err != nil {
                middleware.LogError(c, "DeletePlotConfig.Delete", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.delete_plot", "spin_plot_config", strconv.FormatInt(id, 10),
                "delete plot config", c.IP())

        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": id}))
}

// ── Invite Config Admin Handlers ──

// CreateInviteConfigReq is the request body for creating an invite config.
type CreateInviteConfigReq struct {
        GroupID      int   `json:"group_id"`
        VIPLevel     int   `json:"vip"`
        NewCount     int   `json:"new_count"`
        NewRatio     int   `json:"new_ratio"`
        DefaultRatio int   `json:"default_ratio"`
        ReduceRatio  int   `json:"reduce_ratio"`
        BaseRatio    int   `json:"base_ratio"`
        MaxCount     int   `json:"max_count"`
        MaxAmount    int64 `json:"max_amount"`
}

// CreateInviteConfig handles POST /admin/spin/invites — creates an invite config.
func CreateInviteConfig(c *fiber.Ctx) error {
        var req CreateInviteConfigReq
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateInviteConfig.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        logger.Infof("[CreateInviteConfig] start: admin_id=%d group_id=%d vip=%d", adminID, req.GroupID, req.VIPLevel)

        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        ic := model.SpinInviteConfig{
                GroupID:      req.GroupID,
                VIPLevel:     req.VIPLevel,
                NewCount:     req.NewCount,
                NewRatio:     req.NewRatio,
                DefaultRatio: req.DefaultRatio,
                ReduceRatio:  req.ReduceRatio,
                BaseRatio:    req.BaseRatio,
                MaxCount:     req.MaxCount,
                MaxAmount:    req.MaxAmount,
        }

        if err := db.Create(&ic).Error; err != nil {
                middleware.LogError(c, "CreateInviteConfig.Create", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.create_invite", "spin_invite_config", strconv.FormatInt(ic.ID, 10),
                "create invite config", c.IP())

        logger.Infof("[CreateInviteConfig] success: id=%d", ic.ID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": ic.ID}))
}

// ListInviteConfigs handles GET /admin/spin/invites — lists invite configs, optionally filtered by group_id.
func ListInviteConfigs(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        query := db.Model(&model.SpinInviteConfig{})
        if groupID := c.Query("group_id", ""); groupID != "" {
                query = query.Where("group_id = ?", groupID)
        }

        var configs []model.SpinInviteConfig
        if err := query.Order("id DESC").Find(&configs).Error; err != nil {
                middleware.LogError(c, "ListInviteConfigs.Find", err)
                return bizerr.ErrInternal
        }

        if configs == nil {
                configs = []model.SpinInviteConfig{}
        }

        return c.JSON(bizerr.SuccessResponse(configs))
}

// DeleteInviteConfig handles DELETE /admin/spin/invites/:id — deletes an invite config.
func DeleteInviteConfig(c *fiber.Ctx) error {
        id, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || id <= 0 {
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        if err := db.Delete(&model.SpinInviteConfig{}, id).Error; err != nil {
                middleware.LogError(c, "DeleteInviteConfig.Delete", err)
                return bizerr.ErrInternal
        }

        ReloadSpinConfigs()

        go RecordAuditLog(adminID, "", "spin.delete_invite", "spin_invite_config", strconv.FormatInt(id, 10),
                "delete invite config", c.IP())

        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": id}))
}

// ── Spin Withdraw Order Admin Handlers ──

// ListSpinOrders handles GET /admin/spin/orders — lists spin withdrawal orders with filters.
func ListSpinOrders(c *fiber.Ctx) error {
        page, pageSize, offset := ParsePagination(c)

        status := c.Query("status", "")
        userID := c.Query("user_id", "")
        startDate := c.Query("start_date", "")
        endDate := c.Query("end_date", "")

        logger.Infof("[ListSpinOrders] start: status=%s user_id=%s page=%d", status, userID, page)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "ListSpinOrders.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Count query
        countQuery := db.Model(&model.SpinWithdrawOrder{})
        if status != "" {
                countQuery = countQuery.Where("status = ?", status)
        }
        if userID != "" {
                countQuery = countQuery.Where("user_id = ?", userID)
        }
        if startDate != "" {
                countQuery = countQuery.Where("created_at >= ?", startDate)
        }
        if endDate != "" {
                countQuery = countQuery.Where("created_at <= ?", endDate+" 23:59:59")
        }

        var total int64
        if err := countQuery.Count(&total).Error; err != nil {
                middleware.LogError(c, "ListSpinOrders.Count", err)
                return bizerr.ErrInternal
        }

        // Data query
        query := db.Model(&model.SpinWithdrawOrder{})
        if status != "" {
                query = query.Where("status = ?", status)
        }
        if userID != "" {
                query = query.Where("user_id = ?", userID)
        }
        if startDate != "" {
                query = query.Where("created_at >= ?", startDate)
        }
        if endDate != "" {
                query = query.Where("created_at <= ?", endDate+" 23:59:59")
        }

        var orders []model.SpinWithdrawOrder
        if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
                middleware.LogError(c, "ListSpinOrders.Find", err)
                return bizerr.ErrInternal
        }

        if orders == nil {
                orders = []model.SpinWithdrawOrder{}
        }

        logger.Infof("[ListSpinOrders] success: total=%d returned=%d", total, len(orders))
        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     orders,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}

// AuditSpinOrderReq is the request body for manually auditing a spin order.
type AuditSpinOrderReq struct {
        Result int    `json:"result"` // 1=approve, 3=reject
        Reason string `json:"reason"`
}

// AuditSpinOrder handles POST /admin/spin/orders/:id/audit — manually approve or reject a spin order.
// Mirrors C++ __ProcPhpAuditOrderResp + __AuditSpinOrder.
func AuditSpinOrder(c *fiber.Ctx) error {
        orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || orderID <= 0 {
                return bizerr.ErrInvalidParams
        }

        var req AuditSpinOrderReq
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "AuditSpinOrder.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Result != model.SpinOrderApproved && req.Result != model.SpinOrderRejected {
                return bizerr.ErrInvalidParams
        }

        adminID := middleware.GetUserID(c)
        clientIP := c.IP()
        logger.Infof("[AuditSpinOrder] start: admin_id=%d order_id=%d result=%d reason=%s",
                adminID, orderID, req.Result, req.Reason)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AuditSpinOrder.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Load order
        var order model.SpinWithdrawOrder
        if err := db.First(&order, orderID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrSpinOrderNotFound
                }
                middleware.LogError(c, "AuditSpinOrder.Find", err)
                return bizerr.ErrInternal
        }

        if order.Status != model.SpinOrderPending && order.Status != model.SpinOrderDelayed {
                return bizerr.New(bizerr.CodeOrderNotFound, "order already processed")
        }

        // Get admin name
        var admin model.AdminUser
        _ = db.Select("username").First(&admin, adminID).Error
        adminName := admin.Username

        // Build audit data
        auditData := model.SpinAuditData{
                AuditRuleType: 0, // manual audit
        }
        auditJSON, _ := json.Marshal(auditData)

        // Update order
        now := time.Now()
        if err := db.Model(&order).Updates(map[string]interface{}{
                "status":      req.Result,
                "audit_uid":   adminID,
                "audit_name":  adminName,
                "reason":      req.Reason,
                "audit_json":  string(auditJSON),
                "updated_at":  now,
        }).Error; err != nil {
                middleware.LogError(c, "AuditSpinOrder.Update", err)
                return bizerr.ErrInternal
        }

        // Execute audit decision (credit on approve)
        if req.Result == model.SpinOrderApproved {
                db.Exec(
                        "UPDATE user_wallet SET deposit_balance = deposit_balance + ?, updated_at = NOW() WHERE user_id = ?",
                        order.Amount/model.SpinCURRENCY_BASE, order.UserID,
                )
        }

        // Record audit log
        go RecordAuditLog(adminID, adminName, "spin.audit", "spin_withdraw_order",
                strconv.FormatInt(orderID, 10),
                fmt.Sprintf("manual audit: result=%d reason=%s", req.Result, req.Reason), clientIP)

        logger.Infof("[AuditSpinOrder] success: order_id=%d result=%d", orderID, req.Result)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "order_id": orderID,
                "status":   req.Result,
        }))
}

// ── Spin Stats ──

// GetSpinStats handles GET /admin/spin/stats — returns aggregated spin wheel statistics.
func GetSpinStats(c *fiber.Ctx) error {
        logger.Info("[GetSpinStats] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetSpinStats.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        type StatsRow struct {
                TotalUsers    int64 `gorm:"column:total_users"`
                ActiveToday   int64 `gorm:"column:active_today"`
                TotalOrders   int64 `gorm:"column:total_orders"`
                PendingOrders int64 `gorm:"column:pending_orders"`
                ApprovedAmount int64 `gorm:"column:approved_amount"`
                TotalWithdrawn int64 `gorm:"column:total_withdrawn"`
        }

        var stats StatsRow
        today := time.Now().Format("2006-01-02")

        db.Raw(`SELECT
                (SELECT COUNT(*) FROM user_spin_data) AS total_users,
                (SELECT COUNT(*) FROM user_spin_data WHERE updated_at >= ?) AS active_today,
                (SELECT COUNT(*) FROM spin_withdraw_order) AS total_orders,
                (SELECT COUNT(*) FROM spin_withdraw_order WHERE status = 0) AS pending_orders,
                (SELECT COALESCE(SUM(amount), 0) FROM spin_withdraw_order WHERE status = 1) AS approved_amount,
                (SELECT COALESCE(SUM(total_withdraw), 0) FROM user_spin_data) AS total_withdrawn
        `, today).Scan(&stats)

        logger.Infof("[GetSpinStats] success: total_users=%d active_today=%d total_orders=%d",
                stats.TotalUsers, stats.ActiveToday, stats.TotalOrders)

        return c.JSON(bizerr.SuccessResponse(stats))
}

// ── Spin Order Logs ──

// GetSpinOrderLogs handles GET /admin/spin/orders/:id/logs — returns audit logs for a specific order.
func GetSpinOrderLogs(c *fiber.Ctx) error {
        orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || orderID <= 0 {
                return bizerr.ErrInvalidParams
        }

        db := database.DB()
        if db == nil {
                return bizerr.ErrInternal
        }

        var logs []model.SpinOrderLog
        if err := db.Where("order_id = ?", orderID).Order("id ASC").Find(&logs).Error; err != nil {
                middleware.LogError(c, "GetSpinOrderLogs.Find", err)
                return bizerr.ErrInternal
        }

        if logs == nil {
                logs = []model.SpinOrderLog{}
        }

        return c.JSON(bizerr.SuccessResponse(logs))
}