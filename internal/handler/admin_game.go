// Package handler provides admin HTTP handlers for game catalog management.
// This file contains handlers for listing games, vendors, and game categories.
// Categories are returned as a two-level tree (parent → children).
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// GameListItem represents a game in the admin list
type GameListItem struct {
	ID           int64   `json:"id"`
	VendorID     int64   `json:"vendor_id"`
	VendorName   string  `json:"vendor_name"`
	GameID       string  `json:"game_id"`
	Name         string  `json:"name"`
	Icon         string  `json:"icon"`
	URL          string  `json:"url"`
	CategoryID   int64   `json:"category_id"`
	CategoryName string  `json:"category_name"`
	RTP          float64 `json:"rtp"`
	BetMin       float64 `json:"bet_min"`
	BetMax       float64 `json:"bet_max"`
	Status       int8    `json:"status"`
	SortOrder    int     `json:"sort_order"`
	Hot          int8    `json:"hot"`
	New          int8    `json:"new"`
	CreatedAt    string  `json:"created_at"`
}

// GetAdminGames returns a paginated, filterable list of games for the admin panel.
// Supports filtering by status, category_id, vendor_id, and keyword (fuzzy match on name).
// Joins game_vendor and game_category tables to include vendor/category names.
func GetAdminGames(c *fiber.Ctx) error {
	logger.Infof("[GetAdminGames] start: status=%s category_id=%s vendor_id=%s keyword=%s",
		c.Query("status", ""), c.Query("category_id", ""), c.Query("vendor_id", ""), c.Query("keyword", ""))

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminGames.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	page, pageSize, offset := ParsePagination(c)

	// Filters
	status := c.Query("status", "")
	categoryID := c.Query("category_id", "")
	vendorID := c.Query("vendor_id", "")
	keyword := c.Query("keyword", "")

	// Build count query
	countQuery := db.Table("game_info")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	if categoryID != "" {
		countQuery = countQuery.Where("category_id = ?", categoryID)
	}
	if vendorID != "" {
		countQuery = countQuery.Where("vendor_id = ?", vendorID)
	}
	if keyword != "" {
		countQuery = countQuery.Where("name LIKE ?", "%"+escapeLike(keyword)+"%")
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		middleware.LogError(c, "GetAdminGames.Count", err)
		return bizerr.ErrInternal
	}

	// Query games with vendor and category names
	type gameRow struct {
		ID           int64   `gorm:"column:id"`
		VendorID     int64   `gorm:"column:vendor_id"`
		VendorName   string  `gorm:"column:vendor_name"`
		GameID       string  `gorm:"column:game_id"`
		Name         string  `gorm:"column:name"`
		Icon         string  `gorm:"column:icon"`
		URL          string  `gorm:"column:url"`
		CategoryID   int64   `gorm:"column:category_id"`
		CategoryName string  `gorm:"column:cat_name"`
		RTP          float64 `gorm:"column:rtp"`
		BetMin       float64 `gorm:"column:bet_min"`
		BetMax       float64 `gorm:"column:bet_max"`
		Status       int8    `gorm:"column:status"`
		SortOrder    int     `gorm:"column:sort_order"`
		Hot          int8    `gorm:"column:hot"`
		New          int8    `gorm:"column:new"`
		CreatedAt    string  `gorm:"column:created_at"`
	}

	var rows []gameRow
	query := db.Table("game_info g").
		Joins("LEFT JOIN game_vendor v ON v.id = g.vendor_id").
		Joins("LEFT JOIN game_category c ON c.id = g.category_id").
		Select("g.id, g.vendor_id, v.name as vendor_name, g.game_id, g.name, g.icon, g.url, g.category_id, c.name as cat_name, g.rtp, g.bet_min, g.bet_max, g.status, g.sort_order, g.hot, g.new, g.created_at")

	if status != "" {
		query = query.Where("g.status = ?", status)
	}
	if categoryID != "" {
		query = query.Where("g.category_id = ?", categoryID)
	}
	if vendorID != "" {
		query = query.Where("g.vendor_id = ?", vendorID)
	}
	if keyword != "" {
		query = query.Where("g.name LIKE ?", "%"+escapeLike(keyword)+"%")
	}

	if err := query.Order("g.id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetAdminGames.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]GameListItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, GameListItem{
			ID:           r.ID,
			VendorID:     r.VendorID,
			VendorName:   r.VendorName,
			GameID:       r.GameID,
			Name:         r.Name,
			Icon:         r.Icon,
			URL:          r.URL,
			CategoryID:   r.CategoryID,
			CategoryName: r.CategoryName,
			RTP:          r.RTP,
			BetMin:       r.BetMin,
			BetMax:       r.BetMax,
			Status:       r.Status,
			SortOrder:    r.SortOrder,
			Hot:          r.Hot,
			New:          r.New,
			CreatedAt:    r.CreatedAt,
		})
	}

	logger.Infof("[GetAdminGames] success: total=%d page=%d page_size=%d returned=%d", total, page, pageSize, len(list))
	return c.JSON(bizerr.SuccessResponse(&bizerr.PagedData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}))
}

// VendorListItem represents a game vendor
type VendorListItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Logo      string `json:"logo"`
	Status    int8   `json:"status"`
	GameCount int64  `json:"game_count"`
}

// GetAdminVendors returns all game vendors with their associated game counts.
// Used for vendor management in the admin panel and as a filter reference for game listing.
func GetAdminVendors(c *fiber.Ctx) error {
	logger.Infof("[GetAdminVendors] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminVendors.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	type vendorRow struct {
		ID        int64  `gorm:"column:id"`
		Name      string `gorm:"column:name"`
		Logo      string `gorm:"column:logo"`
		Status    int8   `gorm:"column:status"`
		GameCount int64  `gorm:"column:game_count"`
	}

	var rows []vendorRow
	if err := db.Table("game_vendor").
		Select("game_vendor.id, game_vendor.name, game_vendor.logo, game_vendor.status, COUNT(game_info.id) as game_count").
		Joins("LEFT JOIN game_info ON game_info.vendor_id = game_vendor.id").
		Group("game_vendor.id").
		Order("game_vendor.id ASC").
		Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetAdminVendors.Find", err)
		return bizerr.ErrInternal
	}

	list := make([]VendorListItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, VendorListItem{
			ID:        r.ID,
			Name:      r.Name,
			Logo:      r.Logo,
			Status:    r.Status,
			GameCount: r.GameCount,
		})
	}

	logger.Infof("[GetAdminVendors] success: count=%d", len(list))
	return c.JSON(bizerr.SuccessResponse(list))
}

// CategoryListItem represents a game category (supports two-level hierarchy)
type CategoryListItem struct {
	ID        int64              `json:"id"`
	ParentID  int64              `json:"parent_id"`
	LobbyID   int64              `json:"lobby_id"`
	Name      string             `json:"name"`
	Icon      string             `json:"icon"`
	SortOrder int                `json:"sort_order"`
	Status    int8               `json:"status"`
	GameCount int64              `json:"game_count"`
	Children  []CategoryListItem  `json:"children"`
}

// GetAdminCategories returns all game categories as a two-level tree.
// Algorithm:
//  1. Fetch all categories in one query with game counts via LEFT JOIN.
//  2. Build a flat map keyed by ID for O(1) lookups.
//  3. Iterate rows: items with parent_id=0 become roots; others are appended
//     to their parent's Children slice (pointer-based to avoid value-copy issues).
//  4. Dereference root pointers to values for JSON serialization.
func GetAdminCategories(c *fiber.Ctx) error {
	logger.Infof("[GetAdminCategories] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetAdminCategories.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	type catRow struct {
		ID        int64  `gorm:"column:id"`
		ParentID  int64  `gorm:"column:parent_id"`
		LobbyID   int64  `gorm:"column:lobby_id"`
		Name      string `gorm:"column:name"`
		Icon      string `gorm:"column:icon"`
		SortOrder int    `gorm:"column:sort_order"`
		Status    int8   `gorm:"column:status"`
		GameCount int64  `gorm:"column:game_count"`
	}

	var rows []catRow
	if err := db.Table("game_category").
		Select("game_category.id, game_category.parent_id, game_category.lobby_id, game_category.name, game_category.icon, game_category.sort_order, game_category.status, COUNT(game_info.id) as game_count").
		Joins("LEFT JOIN game_info ON game_info.category_id = game_category.id").
		Group("game_category.id").
		Order("game_category.id ASC").
		Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetAdminCategories.Find", err)
		return bizerr.ErrInternal
	}

	// Build flat map first — keyed by category ID for O(1) parent lookup
	itemMap := make(map[int64]*CategoryListItem, len(rows))
	for _, r := range rows {
		item := &CategoryListItem{
			ID:        r.ID,
			ParentID:  r.ParentID,
			LobbyID:   r.LobbyID,
			Name:      r.Name,
			Icon:      r.Icon,
			SortOrder: r.SortOrder,
			Status:    r.Status,
			GameCount: r.GameCount,
			Children:  []CategoryListItem{},
		}
		itemMap[r.ID] = item
	}

	// Build tree: parent_id=0 are roots, others are children.
	// Use pointer list to avoid value copy breaking Children references.
	rootPtrs := make([]*CategoryListItem, 0)
	for _, r := range rows {
		item := itemMap[r.ID]
		if item.ParentID == 0 {
			rootPtrs = append(rootPtrs, item)
		} else if parent, ok := itemMap[item.ParentID]; ok {
			parent.Children = append(parent.Children, *item)
		}
	}

	// Dereference pointers to values for JSON response
	list := make([]CategoryListItem, len(rootPtrs))
	for i, p := range rootPtrs {
		list[i] = *p
	}

	logger.Infof("[GetAdminCategories] success: roots=%d total_items=%d", len(list), len(rows))
	return c.JSON(bizerr.SuccessResponse(list))
}
