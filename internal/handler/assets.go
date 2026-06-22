// Package handler provides HTTP handler functions for the RockGame API.
//
// This file contains asset/wallet-related handlers for retrieving the
// authenticated user's account balance and currency information.
package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/internal/middleware"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// GetAccountAssets returns the current user's balance and currency info.
//
// This handler requires authentication (user_id must be present in the
// context, set by the auth middleware). Currently returns a placeholder
// zero balance since the wallet table has not been implemented yet.
// Once the wallet system is built, this should query the user's wallet
// record and return the real balance and currency.
func GetAccountAssets(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		middleware.LogError(c, "GetAccountAssets", errors.New("user_id not found in context"))
		return bizerr.ErrUnauthorized
	}

	logger.Infof("[GetAccountAssets] start: user_id=%d", userID)

	// Placeholder: wallet table not yet implemented.
	// Return zero balance with default currency so the frontend
	// can render the asset panel without special-casing a missing endpoint.
	return c.JSON(bizerr.SuccessResponse(fiber.Map{
		"balance":  "0.00",
		"currency": "USD",
	}))
}