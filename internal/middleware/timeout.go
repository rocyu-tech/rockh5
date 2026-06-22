package middleware

import (
        "context"
        "time"

        "github.com/gofiber/fiber/v2"
)

// RequestContext returns a context.Context derived from the request with a timeout.
// Use this in handlers that have slow operations (e.g., third-party API calls)
// to ensure they don't exceed a per-operation deadline.
//
// The parent context is cancelled when the client disconnects (via fasthttp),
// and this function adds an additional timeout on top.
//
// Usage:
//
//      ctx, cancel := middleware.RequestContext(c, 5*time.Second)
//      defer cancel()
//      result, err := slowExternalAPICall(ctx, ...)
//
// Note: For most DB/Redis operations, the connection-level ReadTimeout/WriteTimeout
// (configured in server.FiberConfig) already provides adequate per-request timeout.
// This utility is for operations that may exceed those timeouts.
func RequestContext(c *fiber.Ctx, timeout time.Duration) (context.Context, context.CancelFunc) {
        return context.WithTimeout(c.Context(), timeout)
}

// RequestContextWithTimeout is an alias for RequestContext with explicit naming
// to make the intent clear in call sites.
func RequestContextWithTimeout(c *fiber.Ctx, timeout time.Duration) (context.Context, context.CancelFunc) {
        return context.WithTimeout(c.Context(), timeout)
}