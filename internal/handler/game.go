// Package handler provides HTTP handler functions for the RockGame API.
//
// This file contains game-related handlers: launching a specific game and
// listing available game vendors for the H5 frontend.
package handler

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/database"
	"github.com/rocyu-tech/rockgame/pkg/logger"
	"gorm.io/gorm"
)

// ── Game: Launch ──

// LaunchGame returns the game launch URL for a given game ID.
// It validates the game ID parameter, looks up the game in the database,
// checks that the game is currently active (status=1), and returns a
// placeholder launch URL. In production, this URL should be obtained by
// calling the vendor's API with authentication tokens.
func LaunchGame(c *fiber.Ctx) error {
	logger.Infof("[LaunchGame] start: id=%s", c.Params("id"))

	gameID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || gameID <= 0 {
		middleware.LogWarn(c, "LaunchGame.ParseID", "invalid game id")
		return bizerr.ErrInvalidParams
	}

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "LaunchGame.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	// gameRow is a local read-only struct that maps only the columns we need
	// from the game_info table. Using a local struct instead of a model avoids
	// coupling the handler to the full ORM model definition.
	type gameRow struct {
		ID     int64  `gorm:"column:id"`
		GameID string `gorm:"column:game_id"`
		Name   string `gorm:"column:name"`
		Status int8   `gorm:"column:status"`
	}

	var game gameRow
	if err := db.Table("game_info").
		Select("id, game_id, name, status").
		Where("id = ?", gameID).
		First(&game).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.LogWarn(c, "LaunchGame.Find", "game not found")
			return bizerr.ErrNotFound
		}
		middleware.LogError(c, "LaunchGame.Find", err)
		return bizerr.ErrInternal
	}

	// Status check: only status=1 means the game is active and available for play.
	// Any other status (e.g., 0=disabled, 2=maintenance) is treated as unavailable.
	if game.Status != 1 {
		middleware.LogWarn(c, "LaunchGame.Status", "game is not active")
		return bizerr.New(bizerr.CodeGameMaintenance, "game is currently unavailable")
	}

	// Placeholder game URL — integrate with vendor API in production.
	// The actual URL should include a signed session token and be returned
	// from the vendor's launcher API.
	gameURL := "https://play.rockgame.com/launch?game_id=" + game.GameID

	logger.Infof("[LaunchGame] success: game_id=%d, name=%s", game.ID, game.Name)
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"game_url": gameURL,
	}))
}

// ── Game: Vendors (public) ──

// GameVendorPublic is the vendor item returned to the H5 frontend.
// It contains only the fields needed for display; internal fields
// like status or configuration are excluded.
type GameVendorPublic struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Logo string `json:"logo"`
}

// GetGameVendors returns all active game vendors for the H5 frontend.
// Vendors are ordered by ID ascending and filtered to only include
// those with status=1 (active). The result is always a non-nil slice
// to avoid JSON null on the frontend.
func GetGameVendors(c *fiber.Ctx) error {
	logger.Infof("[GetGameVendors] start")

	db := database.DB()
	if db == nil {
		middleware.LogError(c, "GetGameVendors.DB", errors.New("database not initialized"))
		return bizerr.ErrInternal
	}

	var rows []GameVendorPublic
	if err := db.Table("game_vendor").
		Select("id, name, logo").
		Where("status = 1").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		middleware.LogError(c, "GetGameVendors.Find", err)
		return bizerr.ErrInternal
	}

	// Ensure we return an empty array rather than null when no vendors exist.
	// This is important for frontend consumers that expect a JSON array.
	if rows == nil {
		rows = []GameVendorPublic{}
	}

	logger.Infof("[GetGameVendors] success: count=%d", len(rows))
	return c.JSON(bizerr.SuccessResponse(rows))
}