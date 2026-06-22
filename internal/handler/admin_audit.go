// Admin audit log handlers and helpers.
//
// This file provides the handler for querying paginated admin audit logs
// (GetAdminAuditLogs) and the fire-and-forget helper (RecordAuditLog) that
// other handlers call asynchronously to record admin actions such as login,
// logout, and data mutations.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/internal/model"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// AuditLogItem represents a single audit log entry returned to the admin UI.
// Fields are selected explicitly to avoid exposing internal columns.
type AuditLogItem struct {
	ID         int64  `json:"id"`
	AdminID    int64  `json:"admin_id"`
	AdminName  string `json:"admin_name"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	CreatedAt  string `json:"created_at"`
}

// GetAdminAuditLogs returns a paginated list of admin audit logs with optional
// filtering by action type and date range.
//
// Query parameters:
//   - page, page_size: standard pagination (defaults applied by ParsePagination)
//   - action: filter by action type (e.g. "login", "logout")
//   - start_date, end_date: inclusive date range filter on created_at
//
// GET /api/v1/admin/audit/logs
func GetAdminAuditLogs(c *fiber.Ctx) error {
	logger.Infof("[GetAdminAuditLogs] start: page=%s page_size=%s action=%s", c.Query("page", "1"), c.Query("page_size", "20"), c.Query("action", ""))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminAuditLogs.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	// Optional filters — only applied when non-empty
	action := c.Query("action", "")
	startDate := c.Query("start_date", "")
	endDate := c.Query("end_date", "")

	// Build count query with the same filters to get total matches
	countQuery := db.Table("admin_audit_log")
	if action != "" {
		countQuery = countQuery.Where("action = ?", action)
	}
	if startDate != "" {
		countQuery = countQuery.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		// Append 23:59:59 to make end_date inclusive of the full day
		countQuery = countQuery.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminAuditLogs.Count", err)
		return bizerr.ErrInternal
	}

	// Build the data query with explicit column selection
	var logs []AuditLogItem
	query := db.Table("admin_audit_log").
		Select("id, admin_id, admin_name, action, target_type, target_id, detail, ip, created_at")

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	// Order by id DESC (newest first) with pagination
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		middleware.LogError(c, "GetAdminAuditLogs.Find", err)
		return bizerr.ErrInternal
	}

	// Ensure an empty array (not null) is returned when no records match
	if logs == nil {
		logs = []AuditLogItem{}
	}

	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     logs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// RecordAuditLog persists an admin action to the admin_audit_log table.
// This is designed to be called via a goroutine (fire-and-forget) so it does
// not block the request handler. If the database is unavailable or the insert
// fails, a warning is logged but the error is not propagated.
//
// If adminName is empty, the function looks up the username from admin_user.
func RecordAuditLog(adminID int64, adminName, action, targetType, targetID, detail, ip string) {
	db := database.DB()
	if db == nil {
		logger.Warnf("[AUDIT] database not available, audit log skipped: admin=%d action=%s", adminID, action)
		return
	}

	// Look up admin name if not provided (e.g. during logout where only ID is available)
	if adminName == "" {
		var admin model.AdminUser
		if err := db.Select("username").First(&admin, adminID).Error; err == nil {
			adminName = admin.Username
		}
	}

	log := model.AdminAuditLog{
		AdminID:    adminID,
		AdminName:  adminName,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
	}

	if err := db.Create(&log).Error; err != nil {
		logger.Warnf("[AUDIT] failed to record audit log: admin=%d action=%s err=%v", adminID, action, err)
	}
}