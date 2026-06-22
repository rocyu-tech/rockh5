package middleware

import (
        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// RecoveryMiddleware recovers from panics and logs them with request context
func RecoveryMiddleware() fiber.Handler {
        return func(c *fiber.Ctx) error {
                defer func() {
                        if r := recover(); r != nil {
                                logger.Errorf("[PANIC] request_id=%s ip=%s method=%s path=%s panic=%v",
                                        GetRequestID(c), c.IP(), c.Method(), c.Path(), r)
                                c.Status(500).JSON(errors.ErrorResponse(errors.ErrInternal))
                        }
                }()
                return c.Next()
        }
}
