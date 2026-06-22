package middleware

import (
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/google/uuid"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() fiber.Handler {
        return func(c *fiber.Ctx) error {
                requestID := c.Get("X-Request-ID")
                if requestID == "" {
                        requestID = uuid.New().String()
                }
                c.Set("X-Request-ID", requestID)
                c.Locals("request_id", requestID)
                return c.Next()
        }
}

// AccessLogMiddleware logs request method, path, status, duration, request_id
func AccessLogMiddleware() fiber.Handler {
        return func(c *fiber.Ctx) error {
                start := time.Now()
                err := c.Next()
                latency := time.Since(start)

                logger.Infof("[ACCESS] request_id=%s %s %s %d %v %s",
                        GetRequestID(c),
                        c.Method(),
                        c.Path(),
                        c.Response().StatusCode(),
                        latency,
                        c.IP(),
                )
                return err
        }
}

// GetRequestID extracts request ID from context (empty string if not set)
func GetRequestID(c *fiber.Ctx) string {
        if v := c.Locals("request_id"); v != nil {
                if s, ok := v.(string); ok {
                        return s
                }
        }
        return ""
}

// LogError logs an error with request context (request_id, user_id, ip, path).
// Use this in handlers to ensure all error logs are consistent and traceable.
func LogError(c *fiber.Ctx, operation string, err error) {
        logger.Errorf("[ERROR] request_id=%s user_id=%d ip=%s method=%s path=%s op=%s err=%v",
                GetRequestID(c),
                GetUserID(c),
                c.IP(),
                c.Method(),
                c.Path(),
                operation,
                err,
        )
}

// LogWarn logs a warning with request context.
func LogWarn(c *fiber.Ctx, operation string, msg string) {
        logger.Warnf("[WARN] request_id=%s user_id=%d ip=%s method=%s path=%s op=%s msg=%s",
                GetRequestID(c),
                GetUserID(c),
                c.IP(),
                c.Method(),
                c.Path(),
                operation,
                msg,
        )
}
