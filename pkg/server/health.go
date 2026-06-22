package server

import (
        "context"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
)

// HealthCheckHandler returns a readiness endpoint handler that checks
// all configured dependencies (DB, Redis) before returning "ok".
// A service that cannot reach its dependencies should fail readiness
// so that load balancers / Kubernetes stop routing traffic to it.
//
// Usage:
//
//      api.Get("/health", server.HealthCheckHandler("my-service", server.HealthCheckDB))
//      api.Get("/health", server.HealthCheckHandler("my-service", server.HealthCheckDB, server.HealthCheckRedis))
//
// Or use the convenience HealthCheckAll shortcut.
func HealthCheckHandler(serviceName string, checks ...HealthCheck) fiber.Handler {
        return func(c *fiber.Ctx) error {
                result := fiber.Map{
                        "status":  "ok",
                        "service": serviceName,
                }

                for _, check := range checks {
                        start := time.Now()
                        err := check.Check()
                        latency := time.Since(start)
                        if err != nil {
                                result["status"] = "degraded"
                                result[check.Name()+"_error"] = err.Error()
                                result[check.Name()+"_latency_ms"] = latency.Milliseconds()
                        } else {
                                result[check.Name()+"_latency_ms"] = latency.Milliseconds()
                        }
                }

                status := result["status"].(string)
                if status == "degraded" {
                        c.Status(503) // Service Unavailable
                }
                return c.JSON(result)
        }
}

// HealthCheck is a named dependency check function.
type HealthCheck interface {
        // Name returns the dependency name (e.g., "db", "redis")
        Name() string
        // Check returns nil if healthy, error if unhealthy
        Check() error
}

// healthCheckFunc adapts a plain func to the HealthCheck interface.
type healthCheckFunc struct {
        name string
        fn   func() error
}

func (h healthCheckFunc) Name() string    { return h.name }
func (h healthCheckFunc) Check() error    { return h.fn() }

// HealthCheckDB returns a HealthCheck that pings the database.
var HealthCheckDB HealthCheck = healthCheckFunc{name: "db", fn: func() error {
                if database.DB() == nil {
                        return fiber.NewError(fiber.StatusServiceUnavailable, "database not initialized")
                }
                sqlDB, err := database.DB().DB()
                if err != nil {
                        return err
                }
                ctx, cancel := contextWithTimeout(3 * time.Second)
                defer cancel()
                return sqlDB.PingContext(ctx)
        }}

// HealthCheckRedis returns a HealthCheck that pings Redis.
var HealthCheckRedis HealthCheck = healthCheckFunc{
        name: "redis",
        fn: func() error {
                rdb := cache.Client()
                if rdb == nil {
                        return fiber.NewError(fiber.StatusServiceUnavailable, "redis not initialized")
                }
                ctx, cancel := contextWithTimeout(3 * time.Second)
                defer cancel()
                return rdb.Ping(ctx).Err()
        },
}

// HealthCheckAll returns a HealthCheckHandler that checks both DB and Redis.
// Services that don't use Redis should construct their own list.
func HealthCheckAll(serviceName string) fiber.Handler {
        return HealthCheckHandler(serviceName, HealthCheckDB, HealthCheckRedis)
}

// contextWithTimeout creates a context.Context with timeout.
// Defined here to avoid importing "context" in callers that only need this.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
        return context.WithTimeout(context.Background(), d)
}