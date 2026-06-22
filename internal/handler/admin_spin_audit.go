// Package handler provides HTTP request handlers for the RockGame platform.
//
// admin_spin_audit.go — Admin Spin Withdraw Audit Handler
//
// Provides endpoints for admin users to manage spin-withdraw orders:
//   - List pending orders (paginated, filterable by status)
//   - Approve or reject a specific order
//
// These endpoints are registered under the admin auth middleware
// (JWT + RBAC), not the regular HMAC middleware.
package handler

import (
        "encoding/json"
        "errors"
        "fmt"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// ============================================================================
// AdminSpinWithdrawList — GET /api/v1/admin/spin-withdraw/orders
//
// Lists spin-withdraw orders with pagination and optional status filter.
// Query params: page, page_size, status (optional, 0=pending, 1=auto-pass, 2=manual-pass, 3=rejected, 4=paid)
//
// This is equivalent to the C++ PHP backend's order list query,
// but implemented natively in the Go admin service.
// ============================================================================

func AdminSpinWithdrawList(c *fiber.Ctx) error {
        logger.Infof("[AdminSpinWithdrawList] start")

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminSpinWithdrawList.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        page, pageSize, offset := ParsePagination(c)

        // Optional status filter
        statusFilter := c.Query("status", "")

        query := db.Table("spin_withdraw_order")
        if statusFilter != "" {
                query = query.Where("status = ?", statusFilter)
        }

        var total int64
        query.Count(&total)

        var orders []model.SpinWithdrawOrder
        query.Order("created_at DESC").
                Offset(offset).
                Limit(pageSize).
                Find(&orders)

        if orders == nil {
                orders = []model.SpinWithdrawOrder{}
        }

        logger.Infof("[AdminSpinWithdrawList] success: total=%d page=%d", total, page)

        return c.JSON(bizerr.SuccessResponse(bizerr.PagedData{
                List:     orders,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(offset+pageSize) < total,
        }))
}

// ============================================================================
// AdminSpinWithdrawAudit — POST /api/v1/admin/spin-withdraw/audit
//
// Allows an admin to approve or reject a pending spin-withdraw order.
//
// Request body:
//   {
//     "order_id": 123,
//     "result": 1,        // 1=approve, 3=reject
//     "reason": "..."     // required when rejecting
//   }
//
// On approve: credits the amount to the user's bonus_balance immediately.
// On reject: marks the order as rejected (no wallet change).
//
// This is the Go equivalent of C++ __ProcPhpAuditOrderResp + __AuditSpinOrder.
// ============================================================================

func AdminSpinWithdrawAudit(c *fiber.Ctx) error {
        adminUID := middleware.GetUserID(c)
        if adminUID == 0 {
                middleware.LogError(c, "AdminSpinWithdrawAudit", errors.New("admin_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        var req struct {
                OrderID int64  `json:"order_id"`
                Result   int8   `json:"result"`   // 1=approve, 3=reject
                Reason   string `json:"reason"`
        }
        if err := c.BodyParser(&req); err != nil || req.OrderID <= 0 {
                middleware.LogWarn(c, "AdminSpinWithdrawAudit.ParseBody", "invalid request body")
                return bizerr.ErrInvalidParams
        }

        if req.Result != 1 && req.Result != 3 {
                middleware.LogWarn(c, "AdminSpinWithdrawAudit.InvalidResult", "result must be 1 (approve) or 3 (reject)")
                return bizerr.ErrInvalidParams
        }

        if req.Result == 3 && req.Reason == "" {
                middleware.LogWarn(c, "AdminSpinWithdrawAudit.NoReason", "reason is required for rejection")
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[AdminSpinWithdrawAudit] start: admin=%d order_id=%d result=%d", adminUID, req.OrderID, req.Result)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminSpinWithdrawAudit.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // 1. Load order
        var order model.SpinWithdrawOrder
        if err := db.Where("id = ?", req.OrderID).First(&order).Error; err != nil {
                middleware.LogError(c, "AdminSpinWithdrawAudit.FindOrder", err)
                return bizerr.ErrSpinOrderNotFound
        }

        // 2. Validate order status (only pending orders can be audited)
        if order.Status != model.SpinOrderStatusInit {
                middleware.LogWarn(c, "AdminSpinWithdrawAudit.WrongStatus",
                        fmt.Sprintf("order %d status=%d, expected %d", req.OrderID, order.Status, model.SpinOrderStatusInit))
                return bizerr.ErrInvalidParams
        }

        // 3. Get admin name
        var adminName string
        var adminRow struct {
                Username string `gorm:"column:username"`
        }
        if err := db.Table("admin_user").Select("username").
                Where("id = ?", adminUID).First(&adminRow).Error; err == nil {
                adminName = adminRow.Username
        }

        // 4. Parse existing audit detail
        auditDetail := &model.SpinAuditDetail{}
        if order.AuditDetail != "" {
                _ = json.Unmarshal([]byte(order.AuditDetail), auditDetail)
        }

        // 5. Execute audit
        if req.Result == 1 {
                // Approve
                approveSpinOrder(db, order.ID, order.UserID, order.Amount, order.FlowRequired,
                        model.SpinOrderStatusManualPass, adminName, auditDetail)
                logger.Infof("[AdminSpinWithdrawAudit] approved: admin=%d order=%d amount=%.4f",
                        adminUID, order.ID, order.Amount)
        } else {
                // Reject
                rejectSpinOrder(db, order.ID, adminUID, adminName, req.Reason, auditDetail)
                logger.Infof("[AdminSpinWithdrawAudit] rejected: admin=%d order=%d reason=%s",
                        adminUID, order.ID, req.Reason)
        }

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "result": 0,
        }))
}

// AdminSpinInviteConfigList — GET /api/v1/admin/spin-withdraw/invite-config
//
// Lists all spin invite configurations for admin management.
func AdminSpinInviteConfigList(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminSpinInviteConfigList.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var configs []model.SpinInviteConfig
        db.Order("group_id ASC, vip_level ASC").Find(&configs)
        if configs == nil {
                configs = []model.SpinInviteConfig{}
        }

        return c.JSON(bizerr.SuccessResponse(configs))
}

// AdminSpinInviteConfigCreate — POST /api/v1/admin/spin-withdraw/invite-config
//
// Creates a new spin invite configuration entry.
func AdminSpinInviteConfigCreate(c *fiber.Ctx) error {
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminSpinInviteConfigCreate.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var cfg model.SpinInviteConfig
        if err := c.BodyParser(&cfg); err != nil {
                middleware.LogWarn(c, "AdminSpinInviteConfigCreate.ParseBody", "invalid request body")
                return bizerr.ErrInvalidParams
        }

        if err := db.Create(&cfg).Error; err != nil {
                middleware.LogError(c, "AdminSpinInviteConfigCreate.Create", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[AdminSpinInviteConfigCreate] created: id=%d group=%d vip=%d", cfg.ID, cfg.GroupID, cfg.VipLevel)
        return c.JSON(bizerr.SuccessResponse(cfg))
}