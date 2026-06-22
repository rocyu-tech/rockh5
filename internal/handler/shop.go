package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/internal/model"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
	"github.com/rocyu-tech/rockgame/pkg/snowflake"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  WALLET / BALANCE
// ──────────────────────────────────────────────

// WalletInfo returns the user's wallet balance and stats.
//
// GET /api/v1/shop/wallet
func WalletInfo(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "WalletInfo")
	if db == nil {
		return bizerr.ErrInternal
	}

	var wallet model.UserWallet
	err := db.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		middleware.LogError(c, "WalletInfo.Query", err)
		return bizerr.ErrInternal
	}

	// No wallet record yet — return zero balances
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(bizerr.SuccessResponse(fiber.Map{
			"balance":         0,
			"bonus_balance":   0,
			"frozen_balance":  0,
			"total_recharge":  0,
			"total_withdraw": 0,
			"recharge_count":  0,
			"withdraw_count":  0,
			"flow_required":   0,
			"flow_completed":  0,
			"currency":        "USD",
		}))
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"balance":         wallet.CashBalance,
		"bonus_balance":   wallet.BonusBalance,
		"frozen_balance":  wallet.FrozenBalance,
		"total_recharge":  wallet.TotalRecharge,
		"total_withdraw": wallet.TotalWithdraw,
		"recharge_count":  wallet.RechargeCount,
		"withdraw_count":  wallet.WithdrawCount,
		"flow_required":   wallet.FlowRequired,
		"flow_completed":  wallet.FlowCompleted,
		"currency":        "USD",
	}))
}

// ──────────────────────────────────────────────
//  PAYMENT / WITHDRAW CHANNELS
// ──────────────────────────────────────────────

// PaymentChannels returns channels available for deposit (charge).
//
// GET /api/v1/shop/payment-channels
func PaymentChannels(c *fiber.Ctx) error {
	db := MustDB(c, "PaymentChannels")
	if db == nil {
		return bizerr.ErrInternal
	}

	var channels []model.PaymentChannel
	err := db.Where("status = 1 AND (channel_type = 0 OR channel_type = 2)").
		Order("sort_order ASC, id ASC").
		Find(&channels).Error
	if err != nil {
		middleware.LogError(c, "PaymentChannels.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]fiber.Map, 0, len(channels))
	for _, ch := range channels {
		list = append(list, fiber.Map{
			"id":            ch.ID,
			"name":          ch.Name,
			"icon":          ch.Icon,
			"type":          ch.Type,
			"sub_title":     ch.SubTitle,
			"min_amount":    ch.MinCharge,
			"max_amount":    ch.MaxCharge,
			"is_hot":        ch.IsHot,
		})
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"list": list,
	}))
}

// WithdrawChannels returns channels available for withdrawal.
//
// GET /api/v1/shop/withdraw-channels
func WithdrawChannels(c *fiber.Ctx) error {
	db := MustDB(c, "WithdrawChannels")
	if db == nil {
		return bizerr.ErrInternal
	}

	var channels []model.PaymentChannel
	err := db.Where("status = 1 AND (channel_type = 1 OR channel_type = 2)").
		Order("sort_order ASC, id ASC").
		Find(&channels).Error
	if err != nil {
		middleware.LogError(c, "WithdrawChannels.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]fiber.Map, 0, len(channels))
	for _, ch := range channels {
		list = append(list, fiber.Map{
			"id":           ch.ID,
			"name":         ch.Name,
			"icon":         ch.Icon,
			"type":         ch.Type,
			"sub_title":    ch.SubTitle,
			"min_amount":   ch.MinWithdraw,
			"max_amount":   ch.MaxWithdraw,
			"daily_limit":  ch.DailyLimit,
			"need_account": isNeedAccount(ch.Type),
		})
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"list": list,
	}))
}

// isNeedAccount determines if a channel type requires the user to provide an account.
func isNeedAccount(channelType string) bool {
	switch channelType {
	case "usdt", "crypto", "bank", "upi", "trc20", "erc20":
		return true
	default:
		return true // default to requiring account for safety
	}
}

// ──────────────────────────────────────────────
//  RECHARGE (DEPOSIT)
// ──────────────────────────────────────────────

// CreateRecharge creates a new recharge (deposit) order.
//
// POST /api/v1/shop/recharge
// Body: { "channel_id": number, "amount": number }
func CreateRecharge(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "CreateRecharge")
	if db == nil {
		return bizerr.ErrInternal
	}

	// Parse request
	var req struct {
		ChannelID int64   `json:"channel_id"`
		Amount    float64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	// Validate
	if req.ChannelID <= 0 || req.Amount <= 0 {
		return bizerr.New(30010, "invalid channel_id or amount").WithHTTP(fiber.StatusBadRequest)
	}

	// Lookup channel
	var channel model.PaymentChannel
	if err := db.Where("id = ? AND status = 1", req.ChannelID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(30011, "payment channel not found or disabled").WithHTTP(fiber.StatusNotFound)
		}
		middleware.LogError(c, "CreateRecharge.FindChannel", err)
		return bizerr.ErrInternal
	}

	// Validate amount range
	if req.Amount < channel.MinCharge {
		return bizerr.New(30012, fmt.Sprintf("minimum amount is %.2f", channel.MinCharge)).WithHTTP(fiber.StatusBadRequest)
	}
	if req.Amount > channel.MaxCharge {
		return bizerr.New(30013, fmt.Sprintf("maximum amount is %.2f", channel.MaxCharge)).WithHTTP(fiber.StatusBadRequest)
	}

	// Generate order number
	orderNo := generateOrderNo("R")

	// Create order
	order := model.RechargeOrder{
		OrderNo:       orderNo,
		UserID:        userID,
		ChannelID:     req.ChannelID,
		Amount:        req.Amount,
		Status:        model.ChargeStatusInit,
	}

	// Insert into sharded table
	shardTable := database.ShardTable("recharge_order", userID)
	if err := db.Table(shardTable).Create(&order).Error; err != nil {
		middleware.LogError(c, "CreateRecharge.InsertOrder", err)
		return bizerr.ErrInternal
	}

	// Update sequence count for anti-abuse tracking
	db.Model(&model.UserWallet{}).
		Where("user_id = ?", userID).
		UpdateColumn("recharge_count", gorm.Expr("recharge_count + 1"))

	logger.Infof("[CreateRecharge] order created: user_id=%d order_no=%s amount=%.2f channel=%d",
		userID, orderNo, req.Amount, req.ChannelID)

	// In production, this would call the payment gateway to get pay_url.
	// For now, return the order info.
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"order_no": orderNo,
		"amount":   req.Amount,
		"status":   "pending",
		// "pay_url":  payURL,  // TODO: integrate with payment gateway
	}))
}

// ──────────────────────────────────────────────
//  WITHDRAW
// ──────────────────────────────────────────────

// CreateWithdraw creates a new withdrawal order.
//
// POST /api/v1/shop/withdraw
// Body: { "channel_id": number, "amount": number, "account": string }
func CreateWithdraw(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "CreateWithdraw")
	if db == nil {
		return bizerr.ErrInternal
	}

	// Parse request
	var req struct {
		ChannelID int64   `json:"channel_id"`
		Amount    float64 `json:"amount"`
		Account   string  `json:"account"`
		AccountName string `json:"account_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	// Validate
	if req.ChannelID <= 0 || req.Amount <= 0 {
		return bizerr.New(30020, "invalid channel_id or amount").WithHTTP(fiber.StatusBadRequest)
	}
	if req.Account == "" {
		return bizerr.New(30021, "withdrawal account is required").WithHTTP(fiber.StatusBadRequest)
	}

	// Lookup channel
	var channel model.PaymentChannel
	if err := db.Where("id = ? AND status = 1", req.ChannelID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.New(30022, "withdraw channel not found or disabled").WithHTTP(fiber.StatusNotFound)
		}
		middleware.LogError(c, "CreateWithdraw.FindChannel", err)
		return bizerr.ErrInternal
	}

	// Validate amount range
	if req.Amount < channel.MinWithdraw {
		return bizerr.New(30023, fmt.Sprintf("minimum withdrawal is %.2f", channel.MinWithdraw)).WithHTTP(fiber.StatusBadRequest)
	}
	if req.Amount > channel.MaxWithdraw {
		return bizerr.New(30024, fmt.Sprintf("maximum withdrawal is %.2f", channel.MaxWithdraw)).WithHTTP(fiber.StatusBadRequest)
	}

	// Get wallet and check balance (with optimistic lock)
	var wallet model.UserWallet
	err := db.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizerr.ErrInsufficientBalance
		}
		middleware.LogError(c, "CreateWithdraw.FindWallet", err)
		return bizerr.ErrInternal
	}

	if wallet.CashBalance < req.Amount {
		return bizerr.ErrInsufficientBalance
	}

	// Check daily limit
	if channel.DailyLimit > 0 {
		today := time.Now().Format("2006-01-02")
		shardTable := database.ShardTable("withdraw_order", userID)
		var todayTotal struct {
			Total float64
		}
		db.Table(shardTable).
			Select("COALESCE(SUM(amount), 0) as total").
			Where("user_id = ? AND status IN (0, 1, 2) AND DATE(created_at) = ?", userID, today).
			Scan(&todayTotal)

		if todayTotal.Total + req.Amount > channel.DailyLimit {
			return bizerr.New(30025, fmt.Sprintf("daily withdrawal limit is %.2f", channel.DailyLimit)).WithHTTP(fiber.StatusBadRequest)
		}
	}

	// Calculate fee (simplified: 0% for now, configurable per VIP later)
	fee := 0.0
	realAmount := req.Amount - fee
	if realAmount < 0 {
		realAmount = 0
	}

	// Generate order number
	orderNo := generateOrderNo("W")

	// Create order
	order := model.WithdrawOrder{
		OrderNo:     orderNo,
		UserID:      userID,
		ChannelID:   req.ChannelID,
		Amount:      req.Amount,
		Fee:         fee,
		RealAmount:  realAmount,
		Account:     req.Account,
		AccountName: req.AccountName,
		Status:      model.WithdrawStatusInit,
	}

	// Insert into sharded table
	shardTable := database.ShardTable("withdraw_order", userID)
	if err := db.Table(shardTable).Create(&order).Error; err != nil {
		middleware.LogError(c, "CreateWithdraw.InsertOrder", err)
		return bizerr.ErrInternal
	}

	// Deduct balance with optimistic lock
	result := db.Model(&model.UserWallet{}).
		Where("user_id = ? AND cash_balance >= ? AND version = ?", userID, req.Amount, wallet.Version).
		Updates(map[string]interface{}{
			"cash_balance":   gorm.Expr("cash_balance - ?", req.Amount),
			"frozen_balance": gorm.Expr("frozen_balance + ?", req.Amount),
			"version":       gorm.Expr("version + 1"),
		})
	if result.RowsAffected == 0 {
		// Rollback order
		db.Table(shardTable).Where("order_no = ?", orderNo).Delete(&model.WithdrawOrder{})
		return bizerr.New(30026, "balance changed during operation, please try again").WithHTTP(fiber.StatusConflict)
	}

	logger.Infof("[CreateWithdraw] order created: user_id=%d order_no=%s amount=%.2f channel=%d",
		userID, orderNo, req.Amount, req.ChannelID)

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"order_no":   orderNo,
		"amount":     req.Amount,
		"fee":        fee,
		"real_amount": realAmount,
		"status":     "pending",
	}))
}

// ──────────────────────────────────────────────
//  ORDER HISTORY
// ──────────────────────────────────────────────

// GetOrders returns paginated order history (both recharge and withdraw).
//
// GET /api/v1/shop/orders?type=recharge&page=1&page_size=20
// GET /api/v1/shop/orders?type=withdraw&page=1&page_size=20
func GetOrders(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "GetOrders")
	if db == nil {
		return bizerr.ErrInternal
	}

	orderType := c.Query("type", "all") // recharge, withdraw, all
	page, pageSize, offset := ParsePagination(c)

	var list []fiber.Map
	var total int64

	if orderType == "withdraw" || orderType == "all" {
		shardTable := database.ShardTable("withdraw_order", userID)
		var orders []model.WithdrawOrder

		countQuery := db.Table(shardTable).Where("user_id = ?", userID)
		if orderType == "all" {
			// nothing extra
		}
		countQuery.Count(&total)

		db.Table(shardTable).
			Where("user_id = ?", userID).
			Order("created_at DESC").
			Offset(offset).
			Limit(pageSize).
			Find(&orders)

		for _, o := range orders {
			list = append(list, fiber.Map{
				"order_no":    o.OrderNo,
				"type":        "withdraw",
				"amount":      o.Amount,
				"fee":         o.Fee,
				"real_amount": o.RealAmount,
				"status":      withdrawStatusText(o.Status),
				"status_code": o.Status,
				"reason":      o.Reason,
				"account":     maskAccount(o.Account),
				"created_at":  o.CreatedAt,
				"finished_at": o.FinishedAt,
			})
		}
	} else {
		// recharge
		shardTable := database.ShardTable("recharge_order", userID)
		var orders []model.RechargeOrder

		db.Table(shardTable).Where("user_id = ?", userID).Count(&total)

		db.Table(shardTable).
			Where("user_id = ?", userID).
			Order("created_at DESC").
			Offset(offset).
			Limit(pageSize).
			Find(&orders)

		for _, o := range orders {
			list = append(list, fiber.Map{
				"order_no":      o.OrderNo,
				"type":          "recharge",
				"amount":        o.Amount,
				"credit_balance": o.CreditBalance,
				"bonus_amount":  o.BonusAmount,
				"status":        chargeStatusText(o.Status),
				"status_code":   o.Status,
				"payment_name":  o.PaymentName,
				"account":       maskAccount(o.Account),
				"created_at":     o.CreatedAt,
				"finished_at":    o.FinishedAt,
			})
		}
	}

	hasMore := int64(page*pageSize) < total

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"list":     list,
		"total":    total,
		"page":     page,
		"page_size": pageSize,
		"has_more": hasMore,
	}))
}

// ──────────────────────────────────────────────
//  USER PAYMENT ACCOUNTS (for withdrawal)
// ──────────────────────────────────────────────

// GetUserPaymentAccounts returns the user's saved payment accounts.
//
// GET /api/v1/shop/payment-accounts
func GetUserPaymentAccounts(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "GetUserPaymentAccounts")
	if db == nil {
		return bizerr.ErrInternal
	}

	shardTable := database.ShardTable("user_payment_account", userID)
	var accounts []model.UserPaymentAccount
	db.Table(shardTable).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&accounts)

	list := make([]fiber.Map, 0, len(accounts))
	for _, a := range accounts {
		list = append(list, fiber.Map{
			"id":           a.ID,
			"account_type": a.AccountType,
			"title":        a.Title,
			"account":      maskAccount(a.Account),
			"code":         a.Code,
			"username":     a.Username,
		})
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"list": list,
	}))
}

// SetUserPaymentAccount creates or updates a payment account for withdrawal.
//
// POST /api/v1/shop/payment-accounts
// Body: { "account_type": number, "title": string, "account": string, "code": string, "username": string }
func SetUserPaymentAccount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "SetUserPaymentAccount")
	if db == nil {
		return bizerr.ErrInternal
	}

	var req struct {
		ID          int64  `json:"id"`
		AccountType int    `json:"account_type"`
		Title       string `json:"title"`
		Account     string `json:"account"`
		Code        string `json:"code"`
		Username    string `json:"username"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	if req.Account == "" {
		return bizerr.New(30030, "account number is required").WithHTTP(fiber.StatusBadRequest)
	}

	shardTable := database.ShardTable("user_payment_account", userID)

	// Update existing
	if req.ID > 0 {
		result := db.Table(shardTable).
			Where("id = ? AND user_id = ?", req.ID, userID).
			Updates(map[string]interface{}{
				"account_type": req.AccountType,
				"title":        req.Title,
				"account":      req.Account,
				"code":         req.Code,
				"username":     req.Username,
				"modify_count": gorm.Expr("modify_count + 1"),
			})
		if result.RowsAffected == 0 {
			return bizerr.New(30031, "payment account not found").WithHTTP(fiber.StatusNotFound)
		}
		return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": req.ID}))
	}

	// Check max accounts (default 5)
	var count int64
	db.Table(shardTable).Where("user_id = ?", userID).Count(&count)
	if count >= 5 {
		return bizerr.New(30032, "maximum 5 payment accounts allowed").WithHTTP(fiber.StatusBadRequest)
	}

	// Create new
	account := model.UserPaymentAccount{
		UserID:      userID,
		AccountType: req.AccountType,
		Title:       req.Title,
		Account:     req.Account,
		Code:        req.Code,
		Username:    req.Username,
	}
	if err := db.Table(shardTable).Create(&account).Error; err != nil {
		middleware.LogError(c, "SetUserPaymentAccount.Create", err)
		return bizerr.ErrInternal
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": account.ID}))
}

// ──────────────────────────────────────────────
//  WITHDRAW PASSWORD
// ──────────────────────────────────────────────

// SetWithdrawPassword sets or changes the withdrawal password.
//
// POST /api/v1/shop/withdraw-password
// Body: { "old_pwd": string, "new_pwd": string }
func SetWithdrawPassword(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		return bizerr.ErrUnauthorized
	}

	db := MustDB(c, "SetWithdrawPassword")
	if db == nil {
		return bizerr.ErrInternal
	}

	var req struct {
		OldPwd string `json:"old_pwd"`
		NewPwd string `json:"new_pwd"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	if req.NewPwd == "" {
		return bizerr.New(30040, "new password is required").WithHTTP(fiber.StatusBadRequest)
	}

	var wallet model.UserWallet
	err := db.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create wallet record with password
			pwdHash := hashWithdrawPwd(req.NewPwd)
			newWallet := model.UserWallet{
				UserID:          userID,
				WithdrawPwdHash: pwdHash,
				WithdrawPwdSet:  true,
			}
			if err := db.Create(&newWallet).Error; err != nil {
				middleware.LogError(c, "SetWithdrawPassword.CreateWallet", err)
				return bizerr.ErrInternal
			}
			return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "ok"}))
		}
		middleware.LogError(c, "SetWithdrawPassword.FindWallet", err)
		return bizerr.ErrInternal
	}

	// Verify old password if already set
	if wallet.WithdrawPwdSet && wallet.WithdrawPwdHash != "" {
		if hashWithdrawPwd(req.OldPwd) != wallet.WithdrawPwdHash {
			return bizerr.New(30041, "incorrect current password").WithHTTP(fiber.StatusUnauthorized)
		}
	}

	pwdHash := hashWithdrawPwd(req.NewPwd)
	db.Model(&wallet).Updates(map[string]interface{}{
		"withdraw_pwd_hash": pwdHash,
		"withdraw_pwd_set":  true,
	})

	return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "ok"}))
}

// ──────────────────────────────────────────────
//  ADMIN CALLBACKS (payment gateway notifications)
// ──────────────────────────────────────────────

// CompleteRecharge handles payment gateway callback for completed deposits.
//
// POST /api/v1/shop/recharge/complete (internal/admin)
// Body: { "order_no": string, "status": number, "amount": number, "third_order_no": string }
func CompleteRecharge(c *fiber.Ctx) error {
	db := MustDB(c, "CompleteRecharge")
	if db == nil {
		return bizerr.ErrInternal
	}

	var req struct {
		OrderNo      string  `json:"order_no"`
		Status       int8    `json:"status"`
		Amount       float64 `json:"amount"`
		ThirdOrderNo string  `json:"third_order_no"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	if req.OrderNo == "" {
		return bizerr.ErrInvalidParams
	}

	// Find order across all shards
	var order model.RechargeOrder
	found := false
	for i := 0; i < database.DefaultShardCount; i++ {
		tableName := fmt.Sprintf("recharge_order_%02d", i)
		err := db.Table(tableName).Where("order_no = ?", req.OrderNo).First(&order).Error
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return bizerr.New(30002, "order not found").WithHTTP(fiber.StatusNotFound)
	}

	// Validate status transition: only INIT -> PAID
	if order.Status != model.ChargeStatusInit {
		return bizerr.New(30042, "order already processed").WithHTTP(fiber.StatusBadRequest)
	}

	now := time.Now()

	if req.Status == model.ChargeStatusPaid {
		// Credit balance
		creditAmount := order.CreditBalance
		if creditAmount <= 0 {
			creditAmount = order.Amount
		}

		// Use transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Update order status
		shardTable := database.ShardTable("recharge_order", order.UserID)
		if err := tx.Table(shardTable).
			Where("order_no = ? AND status = ?", req.OrderNo, model.ChargeStatusInit).
			Updates(map[string]interface{}{
				"status":        model.ChargeStatusPaid,
				"finished_at":   now,
				"third_order_no": req.ThirdOrderNo,
				"amount":        req.Amount,
			}).Error; err != nil {
			tx.Rollback()
			middleware.LogError(c, "CompleteRecharge.UpdateOrder", err)
			return bizerr.ErrInternal
		}

		// Credit wallet
		if result := tx.Model(&model.UserWallet{}).
			Where("user_id = ?", order.UserID).
			Updates(map[string]interface{}{
				"cash_balance":   gorm.Expr("cash_balance + ?", creditAmount),
				"bonus_balance":  gorm.Expr("bonus_balance + ?", order.BonusAmount),
				"total_recharge": gorm.Expr("total_recharge + ?", creditAmount),
				"flow_completed": gorm.Expr("flow_completed + ?", order.BonusFlow),
				"version":       gorm.Expr("version + 1"),
			}); result.Error != nil {
			tx.Rollback()
			middleware.LogError(c, "CompleteRecharge.CreditWallet", result.Error)
			return bizerr.ErrInternal
		}

		tx.Commit()

		logger.Infof("[CompleteRecharge] credited: user_id=%d order=%s amount=%.2f",
			order.UserID, req.OrderNo, creditAmount)

	} else {
		// Mark as failed
		shardTable := database.ShardTable("recharge_order", order.UserID)
		db.Table(shardTable).
			Where("order_no = ?", req.OrderNo).
			Updates(map[string]interface{}{
				"status":      model.ChargeStatusFailed,
				"finished_at": now,
			})
	}

	return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "ok"}))
}

// CompleteWithdraw handles payment gateway callback for completed withdrawals.
//
// POST /api/v1/shop/withdraw/complete (internal/admin)
// Body: { "order_no": string, "status": number, "reason": string }
func CompleteWithdraw(c *fiber.Ctx) error {
	db := MustDB(c, "CompleteWithdraw")
	if db == nil {
		return bizerr.ErrInternal
	}

	var req struct {
		OrderNo string `json:"order_no"`
		Status  int8   `json:"status"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return bizerr.ErrInvalidParams
	}

	if req.OrderNo == "" {
		return bizerr.ErrInvalidParams
	}

	// Find order across all shards
	var order model.WithdrawOrder
	found := false
	for i := 0; i < database.DefaultShardCount; i++ {
		tableName := fmt.Sprintf("withdraw_order_%02d", i)
		err := db.Table(tableName).Where("order_no = ?", req.OrderNo).First(&order).Error
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return bizerr.New(30002, "order not found").WithHTTP(fiber.StatusNotFound)
	}

	now := time.Now()
	shardTable := database.ShardTable("withdraw_order", order.UserID)

	switch req.Status {
	case model.WithdrawStatusDone:
		// Success: move frozen -> deducted
		if order.Status == model.WithdrawStatusInit {
			tx := db.Begin()
			defer func() {
				if r := recover(); r != nil {
					tx.Rollback()
				}
			}()

			if err := tx.Table(shardTable).
				Where("order_no = ? AND status = ?", req.OrderNo, model.WithdrawStatusInit).
				Updates(map[string]interface{}{
					"status":      model.WithdrawStatusDone,
					"finished_at": now,
					"reason":      req.Reason,
				}).Error; err != nil {
				tx.Rollback()
				middleware.LogError(c, "CompleteWithdraw.UpdateOrder", err)
				return bizerr.ErrInternal
			}

			// Deduct from frozen, update total_withdraw
			if result := tx.Model(&model.UserWallet{}).
				Where("user_id = ?", order.UserID).
				Updates(map[string]interface{}{
					"frozen_balance": gorm.Expr("frozen_balance - ?", order.Amount),
					"total_withdraw": gorm.Expr("total_withdraw + ?", order.Amount),
					"withdraw_count": gorm.Expr("withdraw_count + 1"),
					"version":        gorm.Expr("version + 1"),
				}); result.Error != nil {
				tx.Rollback()
				middleware.LogError(c, "CompleteWithdraw.UpdateWallet", result.Error)
				return bizerr.ErrInternal
			}

			tx.Commit()
		}

	case model.WithdrawStatusRejected:
		// Rejected: refund frozen -> cash
		if order.Status == model.WithdrawStatusInit {
			tx := db.Begin()
			defer func() {
				if r := recover(); r != nil {
					tx.Rollback()
				}
			}()

			if err := tx.Table(shardTable).
				Where("order_no = ? AND status = ?", req.OrderNo, model.WithdrawStatusInit).
				Updates(map[string]interface{}{
					"status":      model.WithdrawStatusRejected,
					"finished_at": now,
					"reason":      req.Reason,
				}).Error; err != nil {
				tx.Rollback()
				middleware.LogError(c, "CompleteWithdraw.RejectOrder", err)
				return bizerr.ErrInternal
			}

			// Refund: frozen -> cash
			if result := tx.Model(&model.UserWallet{}).
				Where("user_id = ?", order.UserID).
				Updates(map[string]interface{}{
					"cash_balance":   gorm.Expr("cash_balance + ?", order.Amount),
					"frozen_balance": gorm.Expr("frozen_balance - ?", order.Amount),
					"version":        gorm.Expr("version + 1"),
				}); result.Error != nil {
				tx.Rollback()
				middleware.LogError(c, "CompleteWithdraw.RefundWallet", result.Error)
				return bizerr.ErrInternal
			}

			tx.Commit()
		}
	}

	logger.Infof("[CompleteWithdraw] processed: user_id=%d order=%s status=%d",
		order.UserID, req.OrderNo, req.Status)

	return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "ok"}))
}

// ──────────────────────────────────────────────
//  HELPERS
// ──────────────────────────────────────────────

// generateOrderNo creates a unique order number with prefix.
// Format: {prefix}{timestamp}{snowflake_id}
func generateOrderNo(prefix string) string {
	id := snowflake.Generate()
	return fmt.Sprintf("%s%s%012d", prefix, time.Now().Format("20060102150405"), id%1000000000000)
}

// chargeStatusText converts charge status code to display text.
func chargeStatusText(status int8) string {
	switch status {
	case model.ChargeStatusInit:
		return "pending"
	case model.ChargeStatusPaid:
		return "success"
	case model.ChargeStatusFailed:
		return "failed"
	case model.ChargeStatusRefunded:
		return "refunded"
	default:
		return "unknown"
	}
}

// withdrawStatusText converts withdraw status code to display text.
func withdrawStatusText(status int8) string {
	switch status {
	case model.WithdrawStatusInit:
		return "pending"
	case model.WithdrawStatusApproved:
		return "approved"
	case model.WithdrawStatusDone:
		return "completed"
	case model.WithdrawStatusRejected:
		return "rejected"
	case model.WithdrawStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// maskAccount masks sensitive account info for display.
// Shows first 4 and last 4 characters.
func maskAccount(account string) string {
	if len(account) <= 8 {
		return account
	}
	return account[:4] + "****" + account[len(account)-4:]
}

// hashWithdrawPwd hashes the withdrawal password with salt.
// Matches C++ implementation: SHA1("&&*AE86*&&" + pwd + "&&*AE86*&&")
func hashWithdrawPwd(pwd string) string {
	salt := "&&*AE86*&&"
	input := salt + pwd + salt
	// Use Go's crypto/sha1
	h := hashSHA1(input)
	return h
}


