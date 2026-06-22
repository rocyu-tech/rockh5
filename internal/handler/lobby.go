// Package handler provides HTTP handler functions for the RockGame API.
//
// This file contains lobby-related handlers: banners, game categories,
// paginated game listings, lobby configuration, and splash popups.
// All endpoints are designed for the H5 (mobile web) frontend and return
// public-facing data that does not require authentication.
package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// ── Lobby: Banners ──

// LobbyBanner is the banner item returned to the H5 frontend.
// The database column "weight" is aliased to "sort_order" in the response
// to use a more frontend-friendly naming convention.
type LobbyBanner struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	ImageURL  string `json:"image_url"`
	LinkURL   string `json:"link_url"`
	SortOrder int    `json:"sort_order"`
	Status    int8   `json:"status"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

// GetLobbyBanners returns active banners for the H5 lobby.
// Banners are filtered by status=1 and ordered by weight descending
// (higher weight = higher priority), then by ID descending as a tiebreaker.
// Returns an empty array (never null) when no banners exist.
func GetLobbyBanners(c *fiber.Ctx) error {
	logger.Infof("[GetLobbyBanners] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetLobbyBanners.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	var banners []LobbyBanner
	if err := db.Table("banner").
		Select("id, image_url, link_url, weight as sort_order, status, start_time, end_time").
		Where("status = 1").
		Order("weight DESC, id DESC").
		Find(&banners).Error; err != nil {
		middleware.LogError(c, "GetLobbyBanners.Find", err)
		return bizerr.ErrInternal
	}

	// Ensure we return an empty array rather than null when no banners exist.
	if banners == nil {
		banners = []LobbyBanner{}
	}

	logger.Infof("[GetLobbyBanners] success: count=%d", len(banners))
	return c.JSON(bizerr.SuccessResponse(banners))
}

// ── Lobby: Categories ──

// LobbyCategory is the category item returned to the H5 frontend.
// GameCount is computed via a LEFT JOIN + COUNT so that categories with
// zero active games still appear in the list.
type LobbyCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sort_order"`
	GameCount int64  `json:"game_count"`
}

// GetLobbyCategories returns top-level game categories with game counts.
//
// Categories are filtered by lobby_id=0, which represents the top-level
// lobby. Sub-lobbies (lobby_id > 0) may have their own category sets
// and are handled separately.
//
// A LEFT JOIN is used (instead of INNER JOIN) so that categories with
// zero active games still appear in the result, which is important for
// the frontend to render all available tabs even if some are empty.
func GetLobbyCategories(c *fiber.Ctx) error {
	logger.Infof("[GetLobbyCategories] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetLobbyCategories.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// catRow is a local read struct that maps database columns to our
	// internal representation. We use a separate struct to avoid coupling
	// to the ORM model and to control the exact column mapping.
	type catRow struct {
		ID        int64  `gorm:"column:id"`
		Name      string `gorm:"column:name"`
		Icon      string `gorm:"column:icon"`
		SortOrder int    `gorm:"column:sort_order"`
		GameCount int64  `gorm:"column:game_count"`
	}

	var rows []catRow
	if err := db.Table("game_category").
		Select("game_category.id, game_category.name, game_category.icon, game_category.sort_order, COUNT(game_info.id) as game_count").
		// LEFT JOIN ensures categories with no active games still appear with count=0.
		// The "game_info.status = 1" condition is placed inside the JOIN
		// (not in WHERE) so it only filters the joined rows, not the categories.
		Joins("LEFT JOIN game_info ON game_info.category_id = game_category.id AND game_info.status = 1").
		// lobby_id=0 means this is a top-level category not bound to a sub-lobby.
		Where("game_category.lobby_id = 0").
		Group("game_category.id").
		Order("game_category.sort_order ASC").
		Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetLobbyCategories.Find", err)
		return bizerr.ErrInternal
	}

	// Map from the DB row struct to the public-facing response struct.
	// This decouples the internal column names from the JSON API contract.
	list := make([]LobbyCategory, 0, len(rows))
	for _, r := range rows {
		list = append(list, LobbyCategory{
			ID:        r.ID,
			Name:      r.Name,
			Icon:      r.Icon,
			SortOrder: r.SortOrder,
			GameCount: r.GameCount,
		})
	}

	logger.Infof("[GetLobbyCategories] success: count=%d", len(list))
	return c.JSON(bizerr.SuccessResponse(list))
}

// escapeLike escapes special LIKE wildcard characters (%) and (_) in user input
// to prevent them from being interpreted as wildcards. This is necessary when
// user-provided search keywords are used in a LIKE query, as unescaped %
// and _ characters could produce unexpected matches.
//
// Example: the literal string "100%" should match only "100%", not "100" followed
// by any characters.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// ── Lobby: Games ──

// LobbyGame is the game item returned to the H5 frontend.
// Boolean fields (Hot, New) are represented as int8 (0/1) in the
// database and converted to Go bools for the JSON response.
type LobbyGame struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Cover        string `json:"cover"`
	VendorID     int64  `json:"vendor_id"`
	VendorName   string `json:"vendor_name"`
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	Status       int8   `json:"status"`
	Hot          bool   `json:"hot"`
	New          bool   `json:"new"`
	Tag          string `json:"tag"`
}

// GetLobbyGames returns a paginated, filterable game list for the H5 lobby.
//
// Supported query parameters:
//   - category_id: filter by game category
//   - vendor_id:   filter by game vendor
//   - keyword:     search by game name (LIKE match, wildcard chars are escaped)
//   - page, page_size: pagination (handled by ParsePagination)
//
// The handler performs two separate queries: one for the total count
// (with the same filters but no JOIN) and one for the actual data rows.
// The count query avoids JOINs for better performance since we only
// need COUNT(*) from game_info.
func GetLobbyGames(c *fiber.Ctx) error {
	logger.Infof("[GetLobbyGames] start: category_id=%s, vendor_id=%s, keyword=%s",
		c.Query("category_id", ""), c.Query("vendor_id", ""), c.Query("keyword", ""))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetLobbyGames.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	categoryID := c.Query("category_id", "")
	vendorID := c.Query("vendor_id", "")
	keyword := c.Query("keyword", "")

	// Build the main data query: only return active games (status=1).
	// LEFT JOINs are used for vendor and category so that games without
	// a matching vendor or category still appear (with empty name fields).
	query := db.Table("game_info g").
		Joins("LEFT JOIN game_vendor v ON v.id = g.vendor_id").
		Joins("LEFT JOIN game_category c ON c.id = g.category_id").
		Select("g.id, g.name, g.icon as cover, g.vendor_id, v.name as vendor_name, g.category_id, c.name as category_name, g.status, g.`hot`, g.`new`, g.tag").
		Where("g.status = 1")

	// Apply optional filters. Each filter is additive — all provided
	// filters are combined with AND logic.
	if categoryID != "" {
		query = query.Where("g.category_id = ?", categoryID)
	}
	if vendorID != "" {
		query = query.Where("g.vendor_id = ?", vendorID)
	}
	if keyword != "" {
		// escapeLike prevents user input from injecting LIKE wildcards.
		query = query.Where("g.name LIKE ?", "%"+escapeLike(keyword)+"%")
	}

	// Count query: runs against game_info only (no JOINs) for performance.
	// The same filter conditions must be applied to ensure the count
	// matches the actual data rows.
	var total int64
	countQuery := db.Table("game_info g").Where("g.status = 1")
	if categoryID != "" {
		countQuery = countQuery.Where("g.category_id = ?", categoryID)
	}
	if vendorID != "" {
		countQuery = countQuery.Where("g.vendor_id = ?", vendorID)
	}
	if keyword != "" {
		countQuery = countQuery.Where("g.name LIKE ?", "%"+escapeLike(keyword)+"%")
	}
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetLobbyGames.Count", err)
		return bizerr.ErrInternal
	}

	// gameRow is a local struct that mirrors the DB column types.
	// We use int8 for Hot/New in the DB row and convert to bool
	// when building the public LobbyGame response.
	type gameRow struct {
		ID           int64  `gorm:"column:id"`
		Name         string `gorm:"column:name"`
		Cover        string `gorm:"column:cover"`
		VendorID     int64  `gorm:"column:vendor_id"`
		VendorName   string `gorm:"column:vendor_name"`
		CategoryID   int64  `gorm:"column:category_id"`
		CategoryName string `gorm:"column:category_name"`
		Status       int8   `gorm:"column:status"`
		Hot          int8   `gorm:"column:hot"`
		New          int8   `gorm:"column:new"`
		Tag          string `gorm:"column:tag"`
	}

	var rows []gameRow
	// Ordering: sort_order ASC gives control over featured placement,
	// then id DESC ensures newest games appear first within the same sort_order.
	if err := query.Order("g.sort_order ASC, g.id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetLobbyGames.Find", err)
		return bizerr.ErrInternal
	}

	// Map DB rows to the public response struct, converting int8 flags to bools.
	list := make([]LobbyGame, 0, len(rows))
	for _, r := range rows {
		list = append(list, LobbyGame{
			ID:           r.ID,
			Name:         r.Name,
			Cover:        r.Cover,
			VendorID:     r.VendorID,
			VendorName:   r.VendorName,
			CategoryID:   r.CategoryID,
			CategoryName: r.CategoryName,
			Status:       r.Status,
			Hot:          r.Hot == 1,
			New:          r.New == 1,
			Tag:          r.Tag,
		})
	}

	logger.Infof("[GetLobbyGames] success: total=%d, page=%d, page_size=%d, returned=%d",
		total, page, pageSize, len(list))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// ── Lobby: Config ──

// GetLobbyConfig returns lobby configuration (customer service URL, download links, etc.).
// Currently returns a static default config with empty URLs and maintenance=false.
// In the future, this should be loaded from a database table or config service
// to allow runtime updates without redeployment.
func GetLobbyConfig(c *fiber.Ctx) error {
	logger.Infof("[GetLobbyConfig] start")

	// Return default config — can be loaded from DB/config later
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"customer_service_url": "",
		"download_url_android": "",
		"download_url_ios":     "",
		"telegram_url":         "",
		"maintenance":          false,
	}))
}

// ── Lobby: Splash ──

// GetLobbySplash returns splash popup configuration.
// Currently returns nil (no splash popup). When splash popups are implemented,
// this should return the active splash content (image, link, display rules)
// based on the user's locale and VIP level.
func GetLobbySplash(c *fiber.Ctx) error {
	logger.Infof("[GetLobbySplash] start")

	// No splash popup configured yet
	return c.JSON(bizerr.SuccessResponse(nil))
}