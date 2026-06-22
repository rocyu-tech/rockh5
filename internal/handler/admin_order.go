// Package handler provides admin HTTP handlers for order management.
// This file contains handlers for listing recharge/withdraw orders and
// approving/rejecting withdraw orders. Reject triggers an atomic unfreeze
// of the user's frozen balance within a database transaction.
package handler

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
	"gorm.io/gorm"
)

// RechargeOrderItem represents a recharge order in the admin list
type RechargeOrderItem struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	UserEmail     string  `json:"user_email"`
	UserNickname  string  `json:"user_nickname"`
	OrderNo       string  `json:"order_no"`
	Amount        float64 `json:"amount"`
	AmountUSD     float64 `json:"amount_usd"`
	Currency      string  `json:"currency"`
	ChannelID     int64   `json:"channel_id"`
	ThirdOrderNo  string  `json:"third_order_no"`
	Status        int8    `json:"status"`
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     string  `json:"created_at"`
}

// GetAdminRechargeOrders returns a paginated, filterable list of recharge orders.
// Supports filtering by status, user_id, start_date, and end_date.
// Joins the users table to include email and nickname for each order.
func GetAdminRechargeOrders(c *fiber.Ctx) error {
	logger.Infof("[GetAdminRechargeOrders] start: status=%s user_id=%s start_date=%s end_date=%s page=%s page_size=%s",
		c.Query("status", ""), c.Query("user_id", ""), c.Query("start_date", ""), c.Query("end_date", ""),
		c.Query("page", "1"), c.Query("page_size", "20"))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminRechargeOrders.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	// Filters
	status := c.Query("status", "")
	userID := c.Query("user_id", "")
	startDate := c.Query("start_date", "")
	endDate := c.Query("end_date", "")

	// Build count query
	countQuery := db.Table("recharge_order")
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
		middleware.LogError(c, "GetAdminRechargeOrders.Count", err)
		return bizerr.ErrInternal
	}

	// Query orders with user info
	type orderRow struct {
		ID           int64      `gorm:"column:id"`
		UserID       int64      `gorm:"column:user_id"`
		UserEmail    string     `gorm:"column:email"`
		UserNickname string     `gorm:"column:nickname"`
		OrderNo      string     `gorm:"column:order_no"`
		Amount       float64    `gorm:"column:amount"`
		AmountUSD    float64    `gorm:"column:amount_usd"`
		Currency     string     `gorm:"column:currency"`
		ChannelID    int64      `gorm:"column:channel_id"`
		ThirdOrderNo string     `gorm:"column:third_order_no"`
		Status       int8       `gorm:"column:status"`
		PaidAt       *time.Time `gorm:"column:paid_at"`
		CreatedAt    string     `gorm:"column:created_at"`
	}

	var rows []orderRow
	query := db.Table("recharge_order r").
		Joins("LEFT JOIN users u ON u.id = r.user_id").
		Select("r.id, r.user_id, u.email, u.nickname, r.order_no, r.amount, r.amount_usd, r.currency, r.channel_id, r.third_order_no, r.status, r.paid_at, r.created_at")

	if status != "" {
		query = query.Where("r.status = ?", status)
	}
	if userID != "" {
		query = query.Where("r.user_id = ?", userID)
	}
	if startDate != "" {
		query = query.Where("r.created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("r.created_at <= ?", endDate+" 23:59:59")
	}

	if err := query.Order("r.id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetAdminRechargeOrders.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]RechargeOrderItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, RechargeOrderItem{
			ID:           r.ID,
			UserID:       r.UserID,
			UserEmail:    r.UserEmail,
			UserNickname: r.UserNickname,
			OrderNo:      r.OrderNo,
			Amount:       r.Amount,
			AmountUSD:    r.AmountUSD,
			Currency:     r.Currency,
			ChannelID:    r.ChannelID,
			ThirdOrderNo: r.ThirdOrderNo,
			Status:       r.Status,
			PaidAt:       r.PaidAt,
			CreatedAt:    r.CreatedAt,
		})
	}

	logger.Infof("[GetAdminRechargeOrders] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(list))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// WithdrawOrderItem represents a withdraw order in the admin list
type WithdrawOrderItem struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	UserEmail  string     `json:"user_email"`
	UserNickname string   `json:"user_nickname"`
	OrderNo    string     `json:"order_no"`
	Amount     float64    `json:"amount"`
	Currency   string     `json:"currency"`
	Status     int8       `json:"status"`
	ReviewedBy *int64     `json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	Remark     string     `json:"remark"`
	CreatedAt  string     `json:"created_at"`
}

// GetAdminWithdrawOrders returns a paginated, filterable list of withdraw orders.
// Supports filtering by status, start_date, and end_date.
// Joins the users table to include email and nickname.
func GetAdminWithdrawOrders(c *fiber.Ctx) error {
	logger.Infof("[GetAdminWithdrawOrders] start: status=%s start_date=%s end_date=%s page=%s page_size=%s",
		c.Query("status", ""), c.Query("start_date", ""), c.Query("end_date", ""),
		c.Query("page", "1"), c.Query("page_size", "20"))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminWithdrawOrders.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	// Filters
	status := c.Query("status", "")
	startDate := c.Query("start_date", "")
	endDate := c.Query("end_date", "")

	// Build count query
	countQuery := db.Table("withdraw_order")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	if startDate != "" {
		countQuery = countQuery.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		countQuery = countQuery.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminWithdrawOrders.Count", err)
		return bizerr.ErrInternal
	}

	// Query orders with user info
	type orderRow struct {
		ID           int64      `gorm:"column:id"`
		UserID       int64      `gorm:"column:user_id"`
		UserEmail    string     `gorm:"column:email"`
		UserNickname string     `gorm:"column:nickname"`
		OrderNo      string     `gorm:"column:order_no"`
		Amount       float64    `gorm:"column:amount"`
		Currency     string     `gorm:"column:currency"`
		Status       int8       `gorm:"column:status"`
		ReviewedBy   *int64     `gorm:"column:reviewed_by"`
		ReviewedAt   *time.Time `gorm:"column:reviewed_at"`
		Remark       string     `gorm:"column:remark"`
		CreatedAt    string     `gorm:"column:created_at"`
	}

	var rows []orderRow
	query := db.Table("withdraw_order w").
		Joins("LEFT JOIN users u ON u.id = w.user_id").
		Select("w.id, w.user_id, u.email, u.nickname, w.order_no, w.amount, w.currency, w.status, w.reviewed_by, w.reviewed_at, w.remark, w.created_at")

	if status != "" {
		query = query.Where("w.status = ?", status)
	}
	if startDate != "" {
		query = query.Where("w.created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("w.created_at <= ?", endDate+" 23:59:59")
	}

	if err := query.Order("w.id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetAdminWithdrawOrders.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]WithdrawOrderItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, WithdrawOrderItem{
			ID:           r.ID,
			UserID:       r.UserID,
			UserEmail:    r.UserEmail,
			UserNickname: r.UserNickname,
			OrderNo:      r.OrderNo,
			Amount:       r.Amount,
			Currency:     r.Currency,
			Status:       r.Status,
			ReviewedBy:   r.ReviewedBy,
			ReviewedAt:   r.ReviewedAt,
			Remark:       r.Remark,
			CreatedAt:    r.CreatedAt,
		})
	}

	logger.Infof("[GetAdminWithdrawOrders] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(list))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// ApproveWithdrawRequest for approving withdraw order
type ApproveWithdrawRequest struct{}

// ApproveWithdraw approves a pending withdraw order (status 0 → 1).
// Uses an atomic update pattern: WHERE id=? AND status=0 ensures no double-approve
// even under concurrent requests. If RowsAffected==0, the order was already processed.
// Records an audit log asynchronously on success.
func ApproveWithdraw(c *fiber.Ctx) error {
	orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return bizerr.ErrInvalidParams
	}

	logger.Infof("[ApproveWithdraw] start: order_id=%d", orderID)

	adminID := middleware.GetUserID(c)
	clientIP := c.IP()
	db := database.DB()
	if db == nil {
		middleware.LogError(c, "ApproveWithdraw.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// Check order exists and is pending
	var order struct {
		ID     int64 `gorm:"column:id"`
		Status int8  `gorm:"column:status"`
		UserID int64 `gorm:"column:user_id"`
		Amount float64 `gorm:"column:amount"`
	}
	if err := db.Table("withdraw_order").
		Where("id = ? AND status = 0", orderID).
		Scan(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(bizerr.CodeOrderNotFound, "order not found or not in pending status")
		}
		middleware.LogError(c, "ApproveWithdraw.FindOrder", err)
		return bizerr.ErrInternal
	}

	// Atomic approve: only update if still pending (status=0).
	// This CAS-like pattern prevents double-approve from concurrent admin requests.
	now := time.Now()
	result := db.Table("withdraw_order").
		Where("id = ? AND status = 0", orderID).
		Updates(map[string]interface{}{
			"status":      1,
			"reviewed_by": adminID,
			"reviewed_at": now,
		})
	if result.Error != nil {
		middleware.LogError(c, "ApproveWithdraw.Update", result.Error)
		return bizerr.ErrInternal
	}
	if result.RowsAffected == 0 {
		return bizerr.New(bizerr.CodeOrderNotFound, "order not found or not in pending status")
	}

	// Record audit log
	go func() {
		RecordAuditLog(adminID, "", "order.approve", "withdraw_order", strconv.FormatInt(orderID, 10),
			"approve withdraw order", clientIP)
	}()

	logger.Infof("[ApproveWithdraw] success: order_id=%d user_id=%d amount=%.4f", orderID, order.UserID, order.Amount)
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"order_id": orderID,
		"status":   1,
	}))
}

// RejectWithdrawRequest for rejecting withdraw order
type RejectWithdrawRequest struct {
	Remark string `json:"remark"`
}

// RejectWithdraw rejects a pending withdraw order (status 0 → 3) and unfreezes the user's balance.
// The reject + unfreeze is performed atomically within a single database transaction:
//  1. Reject the order only if still pending (WHERE status=0).
//  2. Unfreeze: transfer amount from frozen_balance back to cash_balance
//     with a safety check (frozen_balance >= amount) to prevent negative values.
// If the unfreeze affects 0 rows (wallet missing or insufficient frozen), a warning is logged
// but the rejection itself still succeeds.
// Records an audit log asynchronously on success.
func RejectWithdraw(c *fiber.Ctx) error {
	orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return bizerr.ErrInvalidParams
	}

	var req RejectWithdrawRequest
	if err := c.BodyParser(&req); err != nil {
		middleware.LogError(c, "RejectWithdraw.BodyParser", err)
		return bizerr.ErrInvalidParams
	}

	logger.Infof("[RejectWithdraw] start: order_id=%d remark=%s", orderID, req.Remark)

	adminID := middleware.GetUserID(c)
	clientIP := c.IP()
	db := database.DB()
	if db == nil {
		middleware.LogError(c, "RejectWithdraw.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// Check order exists and is pending, also fetch user_id and amount for unfreezing
	var order struct {
		ID     int64   `gorm:"column:id"`
		Status int8    `gorm:"column:status"`
		UserID int64   `gorm:"column:user_id"`
		Amount float64 `gorm:"column:amount"`
	}
	if err := db.Table("withdraw_order").
		Where("id = ? AND status = 0", orderID).
		Scan(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(bizerr.CodeOrderNotFound, "order not found or not in pending status")
		}
		middleware.LogError(c, "RejectWithdraw.FindOrder", err)
		return bizerr.ErrInternal
	}

	now := time.Now()

	// Reject + unfreeze frozen_balance in a single transaction to ensure atomicity.
	// If either step fails, both are rolled back.
	err = db.Transaction(func(tx *gorm.DB) error {
		// 1. Reject the order: only update if still pending (status=0)
		result := tx.Table("withdraw_order").
			Where("id = ? AND status = 0", orderID).
			Updates(map[string]interface{}{
				"status":      3,
				"remark":      req.Remark,
				"reviewed_by": adminID,
				"reviewed_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return bizerr.New(bizerr.CodeOrderNotFound, "order not found or not in pending status")
		}

		// 2. Unfreeze: return frozen_balance back to cash_balance.
		// The WHERE frozen_balance >= ? guard prevents negative frozen balance
		// if the wallet state is inconsistent.
		unfreezeResult := tx.Exec(
			"UPDATE user_wallet SET cash_balance = cash_balance + ?, frozen_balance = frozen_balance - ?, updated_at = NOW() WHERE user_id = ? AND frozen_balance >= ?",
			order.Amount, order.Amount, order.UserID, order.Amount,
		)
		if unfreezeResult.Error != nil {
			return unfreezeResult.Error
		}
		if unfreezeResult.RowsAffected == 0 {
			// Wallet not found or insufficient frozen balance — log but don't fail the rejection
			middleware.LogWarn(c, "RejectWithdraw.Unfreeze", fmt.Sprintf("wallet unfreeze affected 0 rows for user_id=%d amount=%.4f", order.UserID, order.Amount))
		}
		return nil
	})
	if err != nil {
		if bizErr, ok := err.(*bizerr.BizError); ok {
			return bizErr
		}
		middleware.LogError(c, "RejectWithdraw.Transaction", err)
		return bizerr.ErrInternal
	}

	// Record audit log
	go func() {
		RecordAuditLog(adminID, "", "order.reject", "withdraw_order", strconv.FormatInt(orderID, 10),
			"reject withdraw order: "+req.Remark, clientIP)
	}()

	logger.Infof("[RejectWithdraw] success: order_id=%d user_id=%d amount=%.4f", orderID, order.UserID, order.Amount)
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"order_id": orderID,
		"status":   3,
	}))
}
