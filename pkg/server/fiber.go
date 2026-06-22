package server

import (
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
)

// DefaultErrorHandler is a unified Fiber error handler for all RockGame services.
// It properly handles BizError (preserves HTTP status), fiber.Error, and unknown errors.
func DefaultErrorHandler(c *fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError

        if bizE, ok := err.(*bizerr.BizError); ok {
                code = bizE.HTTP
        } else if e, ok := err.(*fiber.Error); ok {
                code = e.Code
        }

        return c.Status(code).JSON(bizerr.ErrorResponse(err))
}

// FiberConfig returns a production-ready Fiber config with:
//   - Immutable: true (prevents fasthttp.Request reuse memory amplification)
//   - IdleTimeout: 20s (reclaims idle keep-alive connections)
//   - BodyLimit: 4MB
//   - DisableStartupMessage: true (no route leak in production logs)
//   - ErrorHandler: unified BizError-aware handler
func FiberConfig(readTimeout, writeTimeout int) fiber.Config {
        return fiber.Config{
                ReadTimeout:          time.Duration(readTimeout) * time.Second,
                WriteTimeout:         time.Duration(writeTimeout) * time.Second,
                IdleTimeout:          20 * time.Second,
                BodyLimit:            4 * 1024 * 1024,
                DisableStartupMessage: true,
                Immutable:            true,
                ErrorHandler:         DefaultErrorHandler,
        }
}