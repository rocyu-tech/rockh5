// Package handler provides HTTP handler functions for the RockGame Shop API.
//
// This file implements wallet, payment channel, recharge, withdraw, and order-related
// handlers for the H5 frontend wallet page.
//
// Endpoints:
//   - GetShopWallet:             user wallet balance and summary
//   - GetPaymentChannels:        available deposit channels
//   - GetWithdrawChannels:       available withdrawal channels
//   - CreateRecharge:            create a deposit order
//   - CreateWithdraw:            create a withdraw order (with frozen balance deduction)
//   - GetOrders:                 paginated order history (recharge/withdraw)
//   - GetPaymentAccounts:        user's saved payment accounts
//   - SavePaymentAccount:        add or update a payment account
//   - SetWithdrawPassword:       set/modify withdraw password
package handler

import (
        "encoding/json"
        "errors"
        "fmt"
        "math"
        "strings"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/pkg/snowflake"
        "golang.org/x/crypto/bcrypt"
        "gorm.io/gorm"

        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
        // withdrawPasswordCost is bcrypt cost for withdraw password hashing.
        withdrawPasswordCost = 10

        // defaultWithdrawFeeRate is the default fee rate (percentage) for withdrawals.
        defaultWithdrawFeeRate = 0.0

        // orderNoPrefix is the prefix for generated order numbers.
        rechargeOrderPrefix = "R"
        withdrawOrderPrefix = "W"
)

// ─── Request / Response types ────────────────────────────────────────────────

// RechargeRequest is the request body for creating a recharge order.
type RechargeRequest struct {
        ChannelID int64   `json:"channel_id"`
        Amount    float64 `json:"amount"`
}

// WithdrawRequest is the request body for creating a withdraw order.
type WithdrawRequest struct {
        ChannelID  int64   `json:"channel_id"`
        Amount     float64 `json:"amount"`
        AccountID  int64   `json:"account_id"`  // use saved payment account
        Account    string  `json:"account"`     // or provide account directly
        AccountName string `json:"account_name"` // account holder name
        WithdrawPwd string `json:"withdraw_pwd"` // withdraw password
}

// SavePaymentAccountRequest is the request body for saving a payment account.
type SavePaymentAccountRequest struct {
        ID          int64  `json:"id"`
        AccountType int8   `json:"account_type"`
        Title       string `json:"title"`
        Account     string `json:"account"`
        Code        string `json:"code"`
        Username    string `json:"username"`
}

// SetWithdrawPasswordRequest is the request body for setting/modifying withdraw password.
type SetWithdrawPasswordRequest struct {
        OldPwd string `json:"old_pwd"`
        NewPwd string `json:"new_pwd"`
}

// ─── Wallet ──────────────────────────────────────────────────────────────────

// GetShopWallet returns the current user's wallet balance and summary stats.
//
// GET /api/v1/shop/wallet
func GetShopWallet(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[GetShopWallet] start: user_id=%d", userID)

        db := MustDB(c, "GetShopWallet")
        if db == nil {
                return nil
        }

        var wallet model.UserWallet
        err := db.Where("user_id = ?", userID).First(&wallet).Error
        if err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        // Auto-create wallet for new users
                        wallet = model.UserWallet{
                                UserID: userID,
                        }
                        if err := db.Create(&wallet).Error; err != nil {
                                middleware.LogError(c, "GetShopWallet.CreateWallet", err)
                                return bizerr.ErrInternal
                        }
                } else {
                        middleware.LogError(c, "GetShopWallet.Query", err)
                        return bizerr.ErrInternal
                }
        }

        // Calculate flow requirements (simplified: 1x recharge amount as flow requirement)
        flowRequired := wallet.TotalRecharge
        flowCompleted := wallet.TotalBet

        logger.Infof("[GetShopWallet] success: user_id=%d balance=%.2f", userID, wallet.CashBalance)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "balance":         fmt.Sprintf("%.2f", wallet.CashBalance),
                "bonus_balance":   fmt.Sprintf("%.2f", wallet.BonusBalance),
                "frozen_balance":  fmt.Sprintf("%.2f", wallet.FrozenBalance),
                "total_recharge":  fmt.Sprintf("%.2f", wallet.TotalRecharge),
                "total_withdraw":  fmt.Sprintf("%.2f", wallet.TotalWithdraw),
                "recharge_count":  0, // TODO: count from recharge_order
                "withdraw_count":  0, // TODO: count from withdraw_order
                "flow_required":   fmt.Sprintf("%.2f", flowRequired),
                "flow_completed":  fmt.Sprintf("%.2f", flowCompleted),
                "currency":        "USD",
        }))
}

// ─── Payment Channels ────────────────────────────────────────────────────────

// GetPaymentChannels returns all active payment channels for deposits.
// Channels are sorted by sort_order ascending.
//
// GET /api/v1/shop/payment-channels
func GetPaymentChannels(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[GetPaymentChannels] start: user_id=%d", userID)

        db := MustDB(c, "GetPaymentChannels")
        if db == nil {
                return nil
        }

        var channels []model.PaymentChannel
        if err := db.Where("status = 1").
                Order("sort_order ASC, id ASC").
                Find(&channels).Error; err != nil {
                middleware.LogError(c, "GetPaymentChannels.Find", err)
                return bizerr.ErrInternal
        }

        // Build response with safe fields (exclude config_json)
        list := make([]fiber.Map, 0, len(channels))
        for _, ch := range channels {
                list = append(list, fiber.Map{
                        "id":               ch.ID,
                        "name":             ch.Name,
                        "type":             ch.Type,
                        "min_amount":       ch.MinAmount,
                        "max_amount":       ch.MaxAmount,
                        "supported_regions": ch.SupportedRegions,
                        "sort_order":       ch.SortOrder,
                })
        }

        logger.Infof("[GetPaymentChannels] success: count=%d", len(list))
        return c.JSON(bizerr.SuccessResponse(list))
}

// GetWithdrawChannels returns all active payment channels for withdrawals.
// Withdrawal channels are filtered by type in {usdt, bank}.
//
// GET /api/v1/shop/withdraw-channels
func GetWithdrawChannels(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[GetWithdrawChannels] start: user_id=%d", userID)

        db := MustDB(c, "GetWithdrawChannels")
        if db == nil {
                return nil
        }

        // Withdrawal channels: USDT, bank transfer
        withdrawTypes := []string{"usdt", "bank"}
        var channels []model.PaymentChannel
        if err := db.Where("status = 1 AND type IN ?", withdrawTypes).
                Order("sort_order ASC, id ASC").
                Find(&channels).Error; err != nil {
                middleware.LogError(c, "GetWithdrawChannels.Find", err)
                return bizerr.ErrInternal
        }

        list := make([]fiber.Map, 0, len(channels))
        for _, ch := range channels {
                list = append(list, fiber.Map{
                        "id":               ch.ID,
                        "name":             ch.Name,
                        "type":             ch.Type,
                        "min_amount":       ch.MinAmount,
                        "max_amount":       ch.MaxAmount,
                        "supported_regions": ch.SupportedRegions,
                        "sort_order":       ch.SortOrder,
                })
        }

        logger.Infof("[GetWithdrawChannels] success: count=%d", len(list))
        return c.JSON(bizerr.SuccessResponse(list))
}

// ─── Recharge (Deposit) ─────────────────────────────────────────────────────

// CreateRecharge creates a new recharge (deposit) order.
//
// Flow:
//  1. Validate channel_id and amount (against channel min/max).
//  2. Generate a unique order_no using snowflake.
//  3. Insert a pending recharge_order record.
//  4. Return the order_no so the frontend can redirect to payment.
//
// Note: Actual payment integration (Stripe/PayPal/USDT) is a placeholder.
// The callback from the payment provider will update the order status and credit the wallet.
//
// POST /api/v1/shop/recharge
func CreateRecharge(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        var req RechargeRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateRecharge.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[CreateRecharge] start: user_id=%d channel_id=%d amount=%.2f",
                userID, req.ChannelID, req.Amount)

        // Validate amount
        if req.Amount <= 0 {
                return bizerr.New(bizerr.CodeInvalidParams, "amount must be greater than 0")
        }

        db := MustDB(c, "CreateRecharge")
        if db == nil {
                return nil
        }

        // Validate channel exists and is active
        var channel model.PaymentChannel
        if err := db.Where("id = ? AND status = 1", req.ChannelID).First(&channel).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeInvalidParams, "invalid payment channel")
                }
                middleware.LogError(c, "CreateRecharge.FindChannel", err)
                return bizerr.ErrInternal
        }

        // Validate amount range
        if channel.MinAmount > 0 && req.Amount < channel.MinAmount {
                return bizerr.New(bizerr.CodeInvalidParams,
                        fmt.Sprintf("minimum amount is %.2f", channel.MinAmount))
        }
        if channel.MaxAmount > 0 && req.Amount > channel.MaxAmount {
                return bizerr.New(bizerr.CodeInvalidParams,
                        fmt.Sprintf("maximum amount is %.2f", channel.MaxAmount))
        }

        // Generate order number
        orderNo := generateOrderNo(rechargeOrderPrefix)

        // Create order in sharded table
        shardTable := database.ShardTable("recharge_order", userID)
        result := db.Table(shardTable).Create(map[string]interface{}{
                "user_id":    userID,
                "order_no":   orderNo,
                "amount":     req.Amount,
                "amount_usd": req.Amount, // simplified: assume USD
                "currency":   "USD",
                "channel_id": req.ChannelID,
                "status":     0, // pending
        })
        if result.Error != nil {
                middleware.LogError(c, "CreateRecharge.Insert", result.Error)
                return bizerr.ErrInternal
        }

        logger.Infof("[CreateRecharge] success: user_id=%d order_no=%s amount=%.2f channel_id=%d",
                userID, orderNo, req.Amount, req.ChannelID)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "order_no": orderNo,
                "amount":   req.Amount,
                "status":   "pending",
        }))
}

// ─── Withdraw ───────────────────────────────────────────────────────────────

// CreateWithdraw creates a new withdraw order.
//
// Flow:
//  1. Validate withdraw password if set.
//  2. Validate channel and amount.
//  3. Deduct from cash_balance → frozen_balance (atomic).
//  4. Create withdraw_order in pending status.
//  5. Return the order details.
//
// POST /api/v1/shop/withdraw
func CreateWithdraw(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        var req WithdrawRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "CreateWithdraw.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[CreateWithdraw] start: user_id=%d channel_id=%d amount=%.2f",
                userID, req.ChannelID, req.Amount)

        // Validate amount
        if req.Amount <= 0 {
                return bizerr.New(bizerr.CodeInvalidParams, "amount must be greater than 0")
        }

        db := MustDB(c, "CreateWithdraw")
        if db == nil {
                return nil
        }

        // Check if user has set withdraw password and validate it
        var pwdRecord model.UserWithdrawPassword
        pwdErr := db.Where("user_id = ?", userID).First(&pwdRecord).Error
        if pwdErr == nil && pwdRecord.HasSet == 1 {
                // User has withdraw password set — must verify
                if req.WithdrawPwd == "" {
                        return bizerr.New(bizerr.CodeInvalidParams, "withdraw password is required")
                }
                if err := bcrypt.CompareHashAndPassword(
                        []byte(pwdRecord.PasswordHash), []byte(req.WithdrawPwd)); err != nil {
                        return bizerr.New(bizerr.CodeInvalidPassword, "incorrect withdraw password")
                }
        }

        // Validate channel exists and is active
        var channel model.PaymentChannel
        if err := db.Where("id = ? AND status = 1", req.ChannelID).First(&channel).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.New(bizerr.CodeInvalidParams, "invalid withdraw channel")
                }
                middleware.LogError(c, "CreateWithdraw.FindChannel", err)
                return bizerr.ErrInternal
        }

        // Validate amount range
        if channel.MinAmount > 0 && req.Amount < channel.MinAmount {
                return bizerr.New(bizerr.CodeInvalidParams,
                        fmt.Sprintf("minimum withdrawal amount is %.2f", channel.MinAmount))
        }
        if channel.MaxAmount > 0 && req.Amount > channel.MaxAmount {
                return bizerr.New(bizerr.CodeInvalidParams,
                        fmt.Sprintf("maximum withdrawal amount is %.2f", channel.MaxAmount))
        }

        // Calculate fee
        feeRate := defaultWithdrawFeeRate
        // TODO: look up VIP fee rate from user_vip + vip_level_config
        fee := math.Floor(req.Amount*feeRate*100) / 100 // 2 decimal places
        realAmount := math.Floor((req.Amount-fee)*100) / 100

        // Resolve payment account info
        var bankInfoJSON string
        if req.AccountID > 0 {
                // Use saved payment account
                shardTable := database.ShardTable("user_payment_account", userID)
                var acct model.UserPaymentAccount
                if err := db.Table(shardTable).
                        Where("id = ? AND user_id = ? AND status = 1", req.AccountID, userID).
                        First(&acct).Error; err != nil {
                        return bizerr.New(bizerr.CodeNotFound, "payment account not found")
                }
                bankInfoMap := map[string]interface{}{
                        "account_type": acct.AccountType,
                        "title":        acct.Title,
                        "account":      acct.Account,
                        "code":         acct.Code,
                        "username":     acct.Username,
                }
                bankInfoBytes, _ := json.Marshal(bankInfoMap)
                bankInfoJSON = string(bankInfoBytes)
        } else if req.Account != "" {
                // Direct account info from request
                bankInfoMap := map[string]interface{}{
                        "account_type": channelTypeToAccountType(channel.Type),
                        "account":      req.Account,
                        "username":     req.AccountName,
                }
                if req.Account != "" {
                        bankInfoMap["title"] = channel.Name
                }
                bankInfoBytes, _ := json.Marshal(bankInfoMap)
                bankInfoJSON = string(bankInfoBytes)
        } else {
                return bizerr.New(bizerr.CodeInvalidParams, "account_id or account is required")
        }

        orderNo := generateOrderNo(withdrawOrderPrefix)

        // Atomic: deduct cash_balance → frozen_balance + create order
        err := db.Transaction(func(tx *gorm.DB) error {
                // 1. Freeze balance: cash_balance -= amount, frozen_balance += amount
                freezeResult := tx.Exec(
                        "UPDATE user_wallet SET cash_balance = cash_balance - ?, frozen_balance = frozen_balance + ?, version = version + 1, updated_at = NOW() WHERE user_id = ? AND cash_balance >= ? AND version = ?",
                        req.Amount, req.Amount, userID, req.Amount, 0, // version check simplified
                )
                if freezeResult.Error != nil {
                        return freezeResult.Error
                }
                if freezeResult.RowsAffected == 0 {
                        return bizerr.ErrInsufficientBalance
                }

                // 2. Create withdraw order in sharded table
                shardTable := database.ShardTable("withdraw_order", userID)
                result := tx.Table(shardTable).Create(map[string]interface{}{
                        "user_id":     userID,
                        "order_no":    orderNo,
                        "amount":      req.Amount,
                        "currency":    "USD",
                        "bank_info":   bankInfoJSON,
                        "channel_id":  req.ChannelID,
                        "fee":         fee,
                        "real_amount": realAmount,
                        "status":      0, // pending
                })
                if result.Error != nil {
                        return result.Error
                }

                // 3. Update total_withdraw counter on wallet
                tx.Exec(
                        "UPDATE user_wallet SET total_withdraw = total_withdraw + ? WHERE user_id = ?",
                        req.Amount, userID,
                )

                return nil
        })
        if err != nil {
                if bizErr, ok := err.(*bizerr.BizError); ok {
                        return bizErr
                }
                middleware.LogError(c, "CreateWithdraw.Transaction", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[CreateWithdraw] success: user_id=%d order_no=%s amount=%.2f fee=%.2f real=%.2f",
                userID, orderNo, req.Amount, fee, realAmount)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "order_no":   orderNo,
                "amount":     req.Amount,
                "fee":        fee,
                "real_amount": realAmount,
                "status":     "pending",
        }))
}

// ─── Orders ──────────────────────────────────────────────────────────────────

// GetOrders returns a paginated list of orders for the current user.
// Supports type filter: "recharge", "withdraw", or "all" (default).
//
// GET /api/v1/shop/orders?type=recharge&page=1&page_size=20
func GetOrders(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        page, pageSize, offset := ParsePagination(c)
        orderType := c.Query("type", "all")

        logger.Infof("[GetOrders] start: user_id=%d type=%s page=%d page_size=%d",
                userID, orderType, page, pageSize)

        db := MustDB(c, "GetOrders")
        if db == nil {
                return nil
        }

        var orders []fiber.Map
        var total int64

        // Build order list from sharded tables
        if orderType == "recharge" || orderType == "all" {
                shardTable := database.ShardTable("recharge_order", userID)
                var count int64
                db.Table(shardTable).Where("user_id = ?", userID).Count(&count)
                total += count

                type rechargeRow struct {
                        ID        int64      `gorm:"column:id"`
                        OrderNo   string     `gorm:"column:order_no"`
                        Amount    float64    `gorm:"column:amount"`
                        ChannelID int64      `gorm:"column:channel_id"`
                        Status    int8       `gorm:"column:status"`
                        CreatedAt time.Time  `gorm:"column:created_at"`
                }
                var rows []rechargeRow
                db.Table(shardTable).Where("user_id = ?", userID).
                        Order("id DESC").Offset(offset).Limit(pageSize).
                        Find(&rows)
                for _, r := range rows {
                        orders = append(orders, fiber.Map{
                                "id":          r.ID,
                                "order_no":    r.OrderNo,
                                "amount":      r.Amount,
                                "channel_id":  r.ChannelID,
                                "type":        "recharge",
                                "status":      orderStatusText(r.Status, "recharge"),
                                "status_code": r.Status,
                                "created_at":  r.CreatedAt.Format("2006-01-02 15:04:05"),
                        })
                }
        }

        if orderType == "withdraw" || orderType == "all" {
                shardTable := database.ShardTable("withdraw_order", userID)
                var count int64
                db.Table(shardTable).Where("user_id = ?", userID).Count(&count)
                total += count

                type withdrawRow struct {
                        ID         int64      `gorm:"column:id"`
                        OrderNo    string     `gorm:"column:order_no"`
                        Amount     float64    `gorm:"column:amount"`
                        ChannelID  int64      `gorm:"column:channel_id"`
                        Fee        float64    `gorm:"column:fee"`
                        RealAmount float64    `gorm:"column:real_amount"`
                        Status     int8       `gorm:"column:status"`
                        CreatedAt  time.Time  `gorm:"column:created_at"`
                }
                var rows []withdrawRow
                db.Table(shardTable).Where("user_id = ?", userID).
                        Order("id DESC").Offset(offset).Limit(pageSize).
                        Find(&rows)
                for _, r := range rows {
                        orders = append(orders, fiber.Map{
                                "id":          r.ID,
                                "order_no":    r.OrderNo,
                                "amount":      r.Amount,
                                "channel_id":  r.ChannelID,
                                "fee":         r.Fee,
                                "real_amount": r.RealAmount,
                                "type":        "withdraw",
                                "status":      orderStatusText(r.Status, "withdraw"),
                                "status_code": r.Status,
                                "created_at":  r.CreatedAt.Format("2006-01-02 15:04:05"),
                        })
                }
        }

        logger.Infof("[GetOrders] success: user_id=%d total=%d returned=%d", userID, total, len(orders))

        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     orders,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}

// ─── Payment Accounts ────────────────────────────────────────────────────────

// GetPaymentAccounts returns the user's saved payment accounts.
//
// GET /api/v1/shop/payment-accounts
func GetPaymentAccounts(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        logger.Infof("[GetPaymentAccounts] start: user_id=%d", userID)

        db := MustDB(c, "GetPaymentAccounts")
        if db == nil {
                return nil
        }

        shardTable := database.ShardTable("user_payment_account", userID)
        var accounts []model.UserPaymentAccount
        if err := db.Table(shardTable).
                Where("user_id = ? AND status = 1", userID).
                Order("is_default DESC, id DESC").
                Find(&accounts).Error; err != nil {
                middleware.LogError(c, "GetPaymentAccounts.Find", err)
                return bizerr.ErrInternal
        }

        list := make([]fiber.Map, 0, len(accounts))
        for _, acct := range accounts {
                list = append(list, fiber.Map{
                        "id":           acct.ID,
                        "account_type": acct.AccountType,
                        "title":        acct.Title,
                        "account":      maskAccount(acct.Account, acct.AccountType),
                        "code":         acct.Code,
                        "username":     acct.Username,
                        "is_default":   acct.IsDefault,
                })
        }

        logger.Infof("[GetPaymentAccounts] success: user_id=%d count=%d", userID, len(list))
        return c.JSON(bizerr.SuccessResponse(list))
}

// SavePaymentAccount adds or updates a payment account for the user.
//
// POST /api/v1/shop/payment-accounts
func SavePaymentAccount(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        var req SavePaymentAccountRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "SavePaymentAccount.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[SavePaymentAccount] start: user_id=%d account_type=%d title=%s id=%d",
                userID, req.AccountType, req.Title, req.ID)

        // Validate required fields
        if req.Title == "" || req.Account == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "title and account are required")
        }
        if req.AccountType < 1 || req.AccountType > 3 {
                return bizerr.New(bizerr.CodeInvalidParams, "invalid account_type")
        }

        db := MustDB(c, "SavePaymentAccount")
        if db == nil {
                return nil
        }

        shardTable := database.ShardTable("user_payment_account", userID)

        if req.ID > 0 {
                // Update existing
                result := db.Table(shardTable).
                        Where("id = ? AND user_id = ?", req.ID, userID).
                        Updates(map[string]interface{}{
                                "account_type": req.AccountType,
                                "title":        req.Title,
                                "account":      req.Account,
                                "code":         req.Code,
                                "username":     req.Username,
                                "updated_at":   time.Now(),
                        })
                if result.Error != nil {
                        middleware.LogError(c, "SavePaymentAccount.Update", result.Error)
                        return bizerr.ErrInternal
                }
                if result.RowsAffected == 0 {
                        return bizerr.New(bizerr.CodeNotFound, "payment account not found")
                }

                logger.Infof("[SavePaymentAccount] updated: user_id=%d id=%d", userID, req.ID)
                return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": req.ID}))
        }

        // Create new — limit to 5 accounts per user
        var count int64
        db.Table(shardTable).Where("user_id = ? AND status = 1", userID).Count(&count)
        if count >= 5 {
                return bizerr.New(bizerr.CodeInvalidParams, "maximum 5 payment accounts allowed")
        }

        // If this is the first account, set as default
        isDefault := int8(0)
        if count == 0 {
                isDefault = 1
        }

        var newAcct model.UserPaymentAccount
        newAcct.UserID = userID
        newAcct.AccountType = req.AccountType
        newAcct.Title = req.Title
        newAcct.Account = req.Account
        newAcct.Code = req.Code
        newAcct.Username = req.Username
        newAcct.IsDefault = isDefault
        newAcct.Status = 1

        if err := db.Table(shardTable).Create(&newAcct).Error; err != nil {
                middleware.LogError(c, "SavePaymentAccount.Create", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[SavePaymentAccount] created: user_id=%d id=%d", userID, newAcct.ID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"id": newAcct.ID}))
}

// ─── Withdraw Password ──────────────────────────────────────────────────────

// SetWithdrawPassword sets or modifies the user's withdraw password.
// If old_pwd is not provided and no password exists, this is a first-time set.
// If old_pwd is provided, it must match the existing password.
//
// POST /api/v1/shop/withdraw-password
func SetWithdrawPassword(c *fiber.Ctx) error {
        userID := middleware.GetUserID(c)
        if userID == 0 {
                return bizerr.ErrUnauthorized
        }

        var req SetWithdrawPasswordRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "SetWithdrawPassword.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[SetWithdrawPassword] start: user_id=%d", userID)

        // Validate new password length
        if len(req.NewPwd) < 6 {
                return bizerr.New(bizerr.CodeInvalidParams, "password must be at least 6 characters")
        }

        db := MustDB(c, "SetWithdrawPassword")
        if db == nil {
                return nil
        }

        var pwdRecord model.UserWithdrawPassword
        err := db.Where("user_id = ?", userID).First(&pwdRecord).Error

        if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
                middleware.LogError(c, "SetWithdrawPassword.Query", err)
                return bizerr.ErrInternal
        }

        // Hash the new password
        hashed, hashErr := bcrypt.GenerateFromPassword([]byte(req.NewPwd), withdrawPasswordCost)
        if hashErr != nil {
                middleware.LogError(c, "SetWithdrawPassword.Hash", hashErr)
                return bizerr.ErrInternal
        }

        if errors.Is(err, gorm.ErrRecordNotFound) {
                // First-time set — require old_pwd to be empty
                if req.OldPwd != "" {
                        return bizerr.New(bizerr.CodeInvalidParams, "no existing password, do not provide old_pwd")
                }

                pwdRecord = model.UserWithdrawPassword{
                        UserID:       userID,
                        PasswordHash: string(hashed),
                        HasSet:       1,
                }
                if err := db.Create(&pwdRecord).Error; err != nil {
                        middleware.LogError(c, "SetWithdrawPassword.Create", err)
                        return bizerr.ErrInternal
                }

                logger.Infof("[SetWithdrawPassword] created: user_id=%d", userID)
                return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "set"}))
        }

        // Modify existing password — verify old password
        if pwdRecord.HasSet != 1 {
                // Record exists but password not set yet
                return bizerr.New(bizerr.CodeInvalidParams, "no existing password, do not provide old_pwd")
        }

        if req.OldPwd == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "old_pwd is required to change password")
        }

        if err := bcrypt.CompareHashAndPassword(
                []byte(pwdRecord.PasswordHash), []byte(req.OldPwd)); err != nil {
                return bizerr.New(bizerr.CodeInvalidPassword, "incorrect old password")
        }

        // Update password
        if err := db.Model(&pwdRecord).Updates(map[string]interface{}{
                "password_hash": string(hashed),
                "updated_at":    time.Now(),
        }).Error; err != nil {
                middleware.LogError(c, "SetWithdrawPassword.Update", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[SetWithdrawPassword] updated: user_id=%d", userID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{"result": "updated"}))
}

// ─── Helper functions ───────────────────────────────────────────────────────

// generateOrderNo creates a unique order number: prefix + timestamp + snowflake ID.
func generateOrderNo(prefix string) string {
        id := snowflake.NextID()
        now := time.Now()
        return fmt.Sprintf("%s%s%012d", prefix, now.Format("20060102150405"), id%1000000000000)
}

// orderStatusText converts a numeric order status to a human-readable string.
func orderStatusText(status int8, orderType string) string {
        if orderType == "recharge" {
                switch status {
                case 0:
                        return "pending"
                case 1:
                        return "paid"
                case 2:
                        return "failed"
                case 3:
                        return "refunded"
                default:
                        return "unknown"
                }
        }
        // withdraw
        switch status {
        case 0:
                return "pending"
        case 1:
                return "approved"
        case 2:
                return "completed"
        case 3:
                return "rejected"
        case 4:
                return "cancelled"
        default:
                return "unknown"
        }
}

// maskAccount partially masks sensitive account info for display.
func maskAccount(account string, accountType int8) string {
        if len(account) <= 4 {
                return account
        }
        switch accountType {
        case 2: // USDT address — show first 6 and last 4
                if len(account) > 10 {
                        return account[:6] + "..." + account[len(account)-4:]
                }
        case 3: // PayPal email
                parts := strings.Split(account, "@")
                if len(parts) == 2 && len(parts[0]) > 2 {
                        return parts[0][:2] + "***@" + parts[1]
                }
        }
        // Bank account: show last 4
        return "****" + account[len(account)-4:]
}

// channelTypeToAccountType maps a payment channel type to an account type.
func channelTypeToAccountType(channelType string) int8 {
        switch strings.ToLower(channelType) {
        case "usdt":
                return 2
        case "paypal":
                return 3
        default:
                return 1 // bank
        }
}
