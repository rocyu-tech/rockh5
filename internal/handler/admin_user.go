// Package handler provides admin HTTP handlers for user management.
// This file contains handlers for listing users, viewing user details,
// wallet info, KYC info, login logs, and banning/unbanning users.
package handler

import (
        "errors"
        "fmt"
        "strconv"
        "strings"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// UserListItem represents a user in the admin user list
type UserListItem struct {
        ID         int64  `json:"id"`
        Email      string `json:"email"`
        Phone      string `json:"phone"`
        Nickname   string `json:"nickname"`
        Avatar     string `json:"avatar"`
        Status     int8   `json:"status"`
        KYCStatus  int8   `json:"kyc_status"`
        VIPLevel   int    `json:"vip_level"`
        Balance    string `json:"balance"`
        CreatedAt  string `json:"created_at"`
}

// GetAdminUsers returns a paginated, filterable list of users for the admin panel.
// Supports filtering by keyword (email/nickname), status, and vip_level.
// When vip_level filter is active, a UNION ALL query across all sharded user_vip tables
// is executed first to collect matching user IDs, which are then used to filter the main query.
func GetAdminUsers(c *fiber.Ctx) error {
        logger.Infof("[GetAdminUsers] start: keyword=%s status=%s vip_level=%s page=%s page_size=%s",
                c.Query("keyword", ""), c.Query("status", ""), c.Query("vip_level", ""),
                c.Query("page", "1"), c.Query("page_size", "20"))

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminUsers.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Parse pagination
        page, pageSize, offset := ParsePagination(c)

        // Parse filters
        keyword := c.Query("keyword", "")
        status := c.Query("status", "")
        vipLevel := c.Query("vip_level", "")

        // Validate vipLevel is a valid integer to prevent SQL injection
        if vipLevel != "" {
                vl, err := strconv.Atoi(vipLevel)
                if err != nil || vl < 0 {
                        middleware.LogWarn(c, "GetAdminUsers.VipLevel", "invalid vip_level: "+vipLevel)
                        return bizerr.New(bizerr.CodeInvalidParams, "vip_level must be a non-negative integer")
                }
        }

        // Build query
        query := db.Table("users")
        if keyword != "" {
                query = query.Where("email LIKE ? OR nickname LIKE ?", "%"+escapeLike(keyword)+"%", "%"+escapeLike(keyword)+"%")
        }
        if status != "" {
                query = query.Where("status = ?", status)
        }

        // Build count query (with same filters)
        countQuery := db.Table("users")
        if keyword != "" {
                countQuery = countQuery.Where("email LIKE ? OR nickname LIKE ?", "%"+escapeLike(keyword)+"%", "%"+escapeLike(keyword)+"%")
        }
        if status != "" {
                countQuery = countQuery.Where("status = ?", status)
        }

        // VIP shard UNION query: user_vip tables are sharded (user_vip_00 .. user_vip_NN).
        // We build a UNION ALL across all shards to find user IDs matching the requested level,
        // then use those IDs to filter both the count and data queries.
        var vipUserIDs []int64
        if vipLevel != "" {
                router := database.NewShardRouter()
                var unionParts []string
                var unionArgs []interface{}
                vl, _ := strconv.Atoi(vipLevel) // already validated above
                for i := 0; i < database.DefaultShardCount; i++ {
                        unionParts = append(unionParts, fmt.Sprintf("SELECT user_id FROM user_vip_%02d WHERE level = ?", i))
                        unionArgs = append(unionArgs, vl)
                }
                unionSQL := strings.Join(unionParts, " UNION ALL ")
                if err := db.Raw(unionSQL, unionArgs...).Scan(&vipUserIDs).Error; err != nil {
                        middleware.LogError(c, "GetAdminUsers.VIPQuery", err)
                        return bizerr.ErrInternal
                }
                if len(vipUserIDs) == 0 {
                        // No users match the VIP level filter
                        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                                List:     []UserListItem{},
                                Total:    0,
                                Page:     page,
                                PageSize: pageSize,
                                HasMore:  false,
                        }))
                }
                countQuery = countQuery.Where("id IN ?", vipUserIDs)
                _ = router // suppress unused warning
        }

        var total int64
        if err := countQuery.Count(&total).Error; err != nil {
                middleware.LogError(c, "GetAdminUsers.Count", err)
                return bizerr.ErrInternal
        }

        // Query users with wallet info (note: user_vip is sharded, VIP level shown from count filter only)
        type userRow struct {
                ID         int64   `gorm:"column:id"`
                Email      string  `gorm:"column:email"`
                Phone      string  `gorm:"column:phone"`
                Nickname   string  `gorm:"column:nickname"`
                Avatar     string  `gorm:"column:avatar"`
                Status     int8    `gorm:"column:status"`
                KYCStatus  int8    `gorm:"column:kyc_status"`
                VIPLevel   int     `gorm:"column:level"`
                Balance    float64 `gorm:"column:cash_balance"`
                CreatedAt  string  `gorm:"column:created_at"`
        }

        var rows []userRow
        query = db.Table("users u").
                Joins("LEFT JOIN user_wallet w ON w.user_id = u.id").
                Select("u.id, u.email, u.phone, u.nickname, u.avatar, u.status, u.kyc_status, 0 as level, COALESCE(w.cash_balance, 0) as cash_balance, u.created_at")

        if keyword != "" {
                query = query.Where("u.email LIKE ? OR u.nickname LIKE ?", "%"+escapeLike(keyword)+"%", "%"+escapeLike(keyword)+"%")
        }
        if status != "" {
                query = query.Where("u.status = ?", status)
        }
        if len(vipUserIDs) > 0 {
                query = query.Where("u.id IN ?", vipUserIDs)
        }

        if err := query.Order("u.id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
                middleware.LogError(c, "GetAdminUsers.Find", err)
                return bizerr.ErrInternal
        }

        list := make([]UserListItem, 0, len(rows))
        for _, r := range rows {
                list = append(list, UserListItem{
                        ID:        r.ID,
                        Email:     r.Email,
                        Phone:     r.Phone,
                        Nickname:  r.Nickname,
                        Avatar:    r.Avatar,
                        Status:    r.Status,
                        KYCStatus: r.KYCStatus,
                        VIPLevel:  r.VIPLevel,
                        Balance:   strconv.FormatFloat(r.Balance, 'f', 4, 64),
                        CreatedAt: r.CreatedAt,
                })
        }

        logger.Infof("[GetAdminUsers] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(list))
        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     list,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}

// GetAdminUserDetail returns detailed user info including profile, wallet, KYC, and VIP data.
// Wallet data is non-fatal on error; KYC is non-fatal; VIP is read from the sharded table.
func GetAdminUserDetail(c *fiber.Ctx) error {
        userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || userID <= 0 {
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[GetAdminUserDetail] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminUserDetail.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var user model.User
        if err := db.First(&user, userID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "GetAdminUserDetail.FindUser", err)
                return bizerr.ErrInternal
        }

        // Get wallet info (non-fatal: partial data still returned on error)
        var wallet struct {
                CashBalance   float64 `gorm:"column:cash_balance"`
                BonusBalance  float64 `gorm:"column:bonus_balance"`
                FrozenBalance float64 `gorm:"column:frozen_balance"`
                TotalRecharge float64 `gorm:"column:total_recharge"`
                TotalWithdraw float64 `gorm:"column:total_withdraw"`
                TotalWin      float64 `gorm:"column:total_win"`
                TotalBet      float64 `gorm:"column:total_bet"`
        }
        if err := db.Table("user_wallet").Where("user_id = ?", userID).Scan(&wallet).Error; err != nil {
                middleware.LogError(c, "GetAdminUserDetail.Wallet", err)
        }

        // Get KYC info (non-fatal)
        var kyc struct {
                IDType       int8   `gorm:"column:id_type"`
                RealName     string `gorm:"column:real_name"`
                VerifyStatus int8   `gorm:"column:verify_status"`
                Remark       string `gorm:"column:remark"`
                CreatedAt    string `gorm:"column:created_at"`
        }
        if err := db.Table("user_kyc").Where("user_id = ?", userID).Scan(&kyc).Error; err != nil {
                middleware.LogError(c, "GetAdminUserDetail.KYC", err)
        }

        // Get VIP info from sharded table
        var vip struct {
                Level   int    `gorm:"column:level"`
                Growth  int64  `gorm:"column:growth"`
                Upgrade string `gorm:"column:upgrade_at"`
        }
        vipTable := database.ShardTable("user_vip", userID)
        db.Table(vipTable).Where("user_id = ?", userID).Scan(&vip)

        logger.Infof("[GetAdminUserDetail] success: user_id=%d", userID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "user": fiber.Map{
                        "id":            user.ID,
                        "email":         user.Email,
                        "phone":         user.Phone,
                        "phone_code":    user.PhoneCode,
                        "nickname":      user.Nickname,
                        "avatar":        user.Avatar,
                        "status":        user.Status,
                        "kyc_status":    user.KYCStatus,
                        "kyc_level":     user.KYCLevel,
                        "language":      user.Language,
                        "timezone":      user.Timezone,
                        "last_login_at": user.LastLoginAt,
                        "last_login_ip": user.LastLoginIP,
                        "created_at":    user.CreatedAt,
                        "updated_at":    user.UpdatedAt,
                },
                "wallet": wallet,
                "kyc":    kyc,
                "vip":    vip,
        }))
}

// UpdateUserStatusRequest for banning/unbanning users
type UpdateUserStatusRequest struct {
        Status int8 `json:"status"`
}

// UpdateUserStatus ban/unban a user by setting status to 0 (banned) or 1 (active).
// Records an audit log asynchronously.
func UpdateUserStatus(c *fiber.Ctx) error {
        userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || userID <= 0 {
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[UpdateUserStatus] start: user_id=%d", userID)

        var req UpdateUserStatusRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "UpdateUserStatus.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Status != 0 && req.Status != 1 {
                return bizerr.New(bizerr.CodeInvalidParams, "status must be 0 or 1")
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "UpdateUserStatus.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Check user exists
        var user model.User
        if err := db.First(&user, userID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "UpdateUserStatus.FindUser", err)
                return bizerr.ErrInternal
        }

        if err := db.Model(&user).Update("status", req.Status).Error; err != nil {
                middleware.LogError(c, "UpdateUserStatus.Update", err)
                return bizerr.ErrInternal
        }

        // Record audit log
        adminID := middleware.GetUserID(c)
        clientIP := c.IP()
        action := "user.ban"
        detail := "ban user"
        if req.Status == 1 {
                action = "user.unban"
                detail = "unban user"
        }
        go func() {
                RecordAuditLog(adminID, "", action, "user", strconv.FormatInt(userID, 10), detail, clientIP)
        }()

        logger.Infof("[UpdateUserStatus] success: user_id=%d new_status=%d", userID, req.Status)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "user_id": userID,
                "status":  req.Status,
        }))
}

// GetAdminUserWallet returns the wallet details for a specific user.
// Includes cash, bonus, and frozen balances as well as lifetime totals.
func GetAdminUserWallet(c *fiber.Ctx) error {
        userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || userID <= 0 {
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[GetAdminUserWallet] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminUserWallet.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var wallet struct {
                ID            int64   `gorm:"column:id"`
                UserID        int64   `gorm:"column:user_id"`
                Currency      string  `gorm:"column:currency"`
                CashBalance   float64 `gorm:"column:cash_balance"`
                BonusBalance  float64 `gorm:"column:bonus_balance"`
                FrozenBalance float64 `gorm:"column:frozen_balance"`
                TotalRecharge float64 `gorm:"column:total_recharge"`
                TotalWithdraw float64 `gorm:"column:total_withdraw"`
                TotalBet      float64 `gorm:"column:total_bet"`
                TotalWin      float64 `gorm:"column:total_win"`
                UpdatedAt     string  `gorm:"column:updated_at"`
        }

        result := db.Table("user_wallet").Where("user_id = ?", userID).Scan(&wallet)
        if result.Error != nil {
                middleware.LogError(c, "GetAdminUserWallet.Scan", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return bizerr.ErrUserNotFound
        }

        logger.Infof("[GetAdminUserWallet] success: user_id=%d", userID)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":              wallet.ID,
                "user_id":         wallet.UserID,
                "currency":        wallet.Currency,
                "balance":         wallet.CashBalance,
                "frozen_balance":  wallet.FrozenBalance,
                "total_recharge":  wallet.TotalRecharge,
                "total_withdraw":  wallet.TotalWithdraw,
                "total_bet":       wallet.TotalBet,
                "total_win":       wallet.TotalWin,
                "updated_at":      wallet.UpdatedAt,
        }))
}

// GetAdminUserKYC returns the KYC (Know Your Customer) verification info for a specific user.
// Returns nil if no KYC record exists. Verify status is mapped to human-readable strings.
func GetAdminUserKYC(c *fiber.Ctx) error {
        userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || userID <= 0 {
                return bizerr.ErrInvalidParams
        }

        logger.Infof("[GetAdminUserKYC] start: user_id=%d", userID)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminUserKYC.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var kyc struct {
                ID         int64   `gorm:"column:id"`
                UserID     int64   `gorm:"column:user_id"`
                RealName   string  `gorm:"column:real_name"`
                IDType     int8    `gorm:"column:id_type"`
                IDNumber   string  `gorm:"column:id_number"`
                IDFrontURL string  `gorm:"column:id_front_url"`
                IDBackURL  string  `gorm:"column:id_back_url"`
                SelfieURL  string  `gorm:"column:selfie_url"`
                Status     int8    `gorm:"column:verify_status"`
                CreatedAt  string  `gorm:"column:created_at"`
                UpdatedAt  string  `gorm:"column:updated_at"`
        }

        result := db.Table("user_kyc").Where("user_id = ?", userID).Scan(&kyc)
        if result.Error != nil {
                middleware.LogError(c, "GetAdminUserKYC.Scan", result.Error)
                return bizerr.ErrInternal
        }
        if result.RowsAffected == 0 {
                return c.JSON(bizerr.SuccessResponse(nil))
        }

        statusStr := "submitted"
        switch kyc.Status {
        case 1:
                statusStr = "approved"
        case 2:
                statusStr = "rejected"
        }

        logger.Infof("[GetAdminUserKYC] success: user_id=%d status=%s", userID, statusStr)
        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "id":            kyc.ID,
                "user_id":       kyc.UserID,
                "real_name":     kyc.RealName,
                "id_type":       kyc.IDType,
                "id_number":     kyc.IDNumber,
                "id_front_url":  kyc.IDFrontURL,
                "id_back_url":   kyc.IDBackURL,
                "selfie_url":    kyc.SelfieURL,
                "status":        statusStr,
                "submitted_at":  kyc.CreatedAt,
                "reviewed_at":   kyc.UpdatedAt,
                "reject_reason": "",
        }))
}

// GetAdminUserLoginLogs returns a paginated list of login history entries for a specific user.
// Supports standard page/page_size query params. Ordered by most recent login first.
func GetAdminUserLoginLogs(c *fiber.Ctx) error {
        userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
        if err != nil || userID <= 0 {
                return bizerr.ErrInvalidParams
        }

        page, pageSize, offset := ParsePagination(c)
        logger.Infof("[GetAdminUserLoginLogs] start: user_id=%d page=%d page_size=%d", userID, page, pageSize)

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminUserLoginLogs.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        type loginLog struct {
                ID       int64  `gorm:"column:id"`
                UserID   int64  `gorm:"column:user_id"`
                IP       string `gorm:"column:ip"`
                Device   string `gorm:"column:device"`
                Location string `gorm:"column:location"`
                LoginAt  string `gorm:"column:created_at"`
        }

        var total int64
        countQ := db.Table("user_login_log").Where("user_id = ?", userID)
        if err := countQ.Count(&total).Error; err != nil {
                middleware.LogError(c, "GetAdminUserLoginLogs.Count", err)
                return bizerr.ErrInternal
        }

        var logs []loginLog
        if err := db.Table("user_login_log").Where("user_id = ?", userID).
                Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
                middleware.LogError(c, "GetAdminUserLoginLogs.Find", err)
                return bizerr.ErrInternal
        }

        if logs == nil {
                logs = []loginLog{}
        }

        items := make([]fiber.Map, 0, len(logs))
        for _, l := range logs {
                items = append(items, fiber.Map{
                        "id":       l.ID,
                        "user_id":  l.UserID,
                        "ip":       l.IP,
                        "device":   l.Device,
                        "location": l.Location,
                        "login_at": l.LoginAt,
                })
        }

        logger.Infof("[GetAdminUserLoginLogs] success: user_id=%d total=%d page=%d returned=%d", userID, total, page, len(items))
        return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
                List:     items,
                Total:    total,
                Page:     page,
                PageSize: pageSize,
                HasMore:  int64(page*pageSize) < total,
        }))
}
