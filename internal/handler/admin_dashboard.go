// Package handler provides admin HTTP handlers for the dashboard.
// This file contains handlers for dashboard statistics, recharge trends, and system health.
package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// GetDashboardStats returns dashboard overview statistics including total users,
// today's new users, today's recharge amount/count, today's withdraw amount,
// active games count, and total platform balance.
// Most queries are non-fatal: partial data is still returned if individual queries fail.
func GetDashboardStats(c *fiber.Ctx) error {
	logger.Infof("[GetDashboardStats] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetDashboardStats.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// H3: use Format instead of Truncate to avoid timezone-dependent truncation bugs
	today := time.Now().Format("2006-01-02")

	var totalUsers int64
	if err := db.Table("users").Count(&totalUsers).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TotalUsers", err)
		return bizerr.ErrInternal
	}

	var todayNewUsers int64
	if err := db.Table("users").Where("DATE(created_at) = ?", today).Count(&todayNewUsers).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TodayNewUsers", err)
		// Non-fatal: continue with zero
	}

	var totalRechargeAmount float64
	if err := db.Table("recharge_order").
		Select("COALESCE(SUM(amount_usd), 0)").
		Where("status = 1 AND DATE(created_at) = ?", today).
		Scan(&totalRechargeAmount).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TodayRecharge", err)
	}

	var todayRechargeCount int64
	if err := db.Table("recharge_order").
		Where("status = 1 AND DATE(created_at) = ?", today).
		Count(&todayRechargeCount).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TodayOrders", err)
	}

	var totalWithdrawAmount float64
	if err := db.Table("withdraw_order").
		Select("COALESCE(SUM(amount), 0)").
		Where("DATE(created_at) = ?", today).
		Scan(&totalWithdrawAmount).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TodayWithdraw", err)
	}

	var activeGames int64
	if err := db.Table("game_info").Where("status = 1").Count(&activeGames).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.ActiveGames", err)
	}

	var totalBalance float64
	if err := db.Table("user_wallet").
		Select("COALESCE(SUM(cash_balance) + SUM(bonus_balance), 0)").
		Scan(&totalBalance).Error; err != nil {
		middleware.LogError(c, "GetDashboardStats.TotalBalance", err)
	}

	logger.Infof("[GetDashboardStats] success: total_users=%d today_new=%d today_recharge=%.2f today_withdraw=%.2f",
		totalUsers, todayNewUsers, totalRechargeAmount, totalWithdrawAmount)
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"total_users":     totalUsers,
		"today_new_users": todayNewUsers,
		"today_recharge":  totalRechargeAmount,
		"today_orders":    todayRechargeCount,
		"today_withdraw":  totalWithdrawAmount,
		"online_users":    0, // placeholder, requires Redis integration
		"active_games":    activeGames,
		"ggr":             0, // placeholder, requires game transaction data
	}))
}

// RechargeTrendItem represents a single day's recharge data for the trend chart
type RechargeTrendItem struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Count  int64   `json:"count"`
}

// GetDashboardTrend returns recharge trend data for the last 7 days.
// H4 optimization: uses a single GROUP BY query instead of 7 separate per-day queries.
// Days with no data are filled with zero values to ensure a complete 7-day series.
func GetDashboardTrend(c *fiber.Ctx) error {
	logger.Infof("[GetDashboardTrend] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetDashboardTrend.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// Calculate date range: last 7 days
	type trendRow struct {
		Date   string  `gorm:"column:d"`
		Amount float64 `gorm:"column:amount"`
		Count  int64   `gorm:"column:cnt"`
	}

	startDate := time.Now().AddDate(0, 0, -6).Format("2006-01-02")

	// H4: Single GROUP BY query fetches all 7 days at once, then we fill gaps in Go.
	// This is much more efficient than 7 individual COUNT queries.
	var rows []trendRow
	if err := db.Table("recharge_order").
		Select("DATE(created_at) as d, COALESCE(SUM(amount_usd), 0) as amount, COUNT(*) as cnt").
		Where("status = 1 AND DATE(created_at) >= ?", startDate).
		Group("DATE(created_at)").
		Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetDashboardTrend.Query", err)
		return bizerr.ErrInternal
	}

	// Build a map for O(1) lookup
	rowMap := make(map[string]*trendRow, len(rows))
	for i := range rows {
		rowMap[rows[i].Date] = &rows[i]
	}

	// Fill all 7 days (including days with zero data)
	items := make([]RechargeTrendItem, 0, 7)
	for i := 6; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if r, ok := rowMap[day]; ok {
			items = append(items, RechargeTrendItem{Date: day, Amount: r.Amount, Count: r.Count})
		} else {
			items = append(items, RechargeTrendItem{Date: day, Amount: 0, Count: 0})
		}
	}

	logger.Infof("[GetDashboardTrend] success: days=%d", len(items))
	return c.JSON(bizerr.SuccessResponse(items))
}

// GetDashboardHealth returns system health information for admin monitoring.
// Currently returns static status for known services; placeholder values
// for services whose health cannot yet be checked from the admin node.
func GetDashboardHealth(c *fiber.Ctx) error {
	return c.JSON(bizerr.SuccessResponse([]fiber.Map{
		{"name": "admin-node", "status": "online", "uptime": "-", "response_time": "< 5ms", "last_check": time.Now().Format("2006-01-02 15:04:05")},
		{"name": "gate", "status": "unknown", "uptime": "-", "response_time": "-", "last_check": time.Now().Format("2006-01-02 15:04:05")},
		{"name": "account-node", "status": "unknown", "uptime": "-", "response_time": "-", "last_check": time.Now().Format("2006-01-02 15:04:05")},
		{"name": "game-node", "status": "unknown", "uptime": "-", "response_time": "-", "last_check": time.Now().Format("2006-01-02 15:04:05")},
		{"name": "database", "status": "online", "uptime": "-", "response_time": "< 10ms", "last_check": time.Now().Format("2006-01-02 15:04:05")},
	}))
}
