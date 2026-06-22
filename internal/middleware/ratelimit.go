package middleware

import (
        "fmt"
        "strconv"
        "sync"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// memLimiter provides an in-memory sliding-window fallback when Redis is unavailable.
// Uses a per-key sorted list of request timestamps to count requests within the window.
type memLimiter struct {
        mu      sync.Mutex
        buckets map[string]*slidingBucket
}

type slidingBucket struct {
        timestamps []float64 // request timestamps (unix nano as float64 for sorting)
}

var fallbackLimiter = &memLimiter{buckets: make(map[string]*slidingBucket)}

func init() {
        // Periodically clean up stale buckets to prevent unbounded memory growth.
        // The slidingIncr method already prunes expired timestamps per-bucket on
        // each access, but buckets for IPs that stop making requests are never
        // removed from the map. This goroutine reclaims that memory every 5 min.
        go func() {
                ticker := time.NewTicker(5 * time.Minute)
                defer ticker.Stop()
                for range ticker.C {
                        fallbackLimiter.cleanup()
                }
        }()
}

// cleanup removes buckets that have no remaining entries (all timestamps expired).
func (m *memLimiter) cleanup() {
        m.mu.Lock()
        defer m.mu.Unlock()
        window := 2 * time.Minute // conservative window to avoid removing active buckets
        now := float64(time.Now().UnixNano())
        for key, b := range m.buckets {
                stale := true
                for _, ts := range b.timestamps {
                        if ts > now-float64(window.Nanoseconds()) {
                                stale = false
                                break
                        }
                }
                if stale {
                        delete(m.buckets, key)
                }
        }
        if len(m.buckets) > 0 {
                logger.Debugf("ratelimit: fallback bucket cleanup, %d active buckets remain", len(m.buckets))
        }
}

// slidingIncr counts requests in the sliding window and returns the current count.
// Removes expired entries older than now - window.
func (m *memLimiter) slidingIncr(key string, window time.Duration) int64 {
        m.mu.Lock()
        defer m.mu.Unlock()

        now := float64(time.Now().UnixNano())
        windowStart := now - float64(window.Nanoseconds())

        b, ok := m.buckets[key]
        if !ok {
                m.buckets[key] = &slidingBucket{timestamps: []float64{now}}
                return 1
        }

        // Remove expired entries (sliding window)
        valid := b.timestamps[:0]
        for _, ts := range b.timestamps {
                if ts > windowStart {
                        valid = append(valid, ts)
                }
        }
        valid = append(valid, now)
        b.timestamps = valid

        return int64(len(valid))
}

// RateLimitMiddleware implements sliding-window rate limiting using Redis sorted sets.
//
// Algorithm (non-Lua, pure Redis commands):
//   1. ZREMRANGEBYSCORE key -inf (now - window)  — remove expired entries
//   2. ZCARD key                                   — count current window requests
//   3. If under limit: ZADD key now requestID      — record this request
//
// This uses a Redis pipeline (not transaction) for minimal latency.
// The small race window between ZCARD and ZADD is acceptable for rate limiting
// (worst case: a few extra requests slip through, which is preferable to blocking
// legitimate requests or requiring Lua scripts).
//
// Falls back to in-memory sliding window when Redis is unavailable.
func RateLimitMiddleware(maxRequests int, window time.Duration) fiber.Handler {
        return func(c *fiber.Ctx) error {
                userID := GetUserID(c)
                ip := c.IP()

                var key string
                if userID > 0 {
                        key = "rl:" + strconv.FormatInt(userID, 10)
                } else {
                        // NOTE: c.IP() may be spoofed via X-Forwarded-For when no trusted
                        // proxy is configured. This is acceptable for rate limiting unauthenticated
                        // requests: a spoofed IP causes at most false negatives (attacker gets
                        // their own IP rate-limited), never false positives. Authenticated routes
                        // use the secure userID-based key above.
                        key = "rl:ip:" + ip
                }

                ctx := c.Context()
                var count int64

                rdb := cache.Client()
                if rdb != nil {
                        now := float64(time.Now().UnixNano())
                        windowStart := now - float64(window.Nanoseconds())
                        member := fmt.Sprintf("%.0f:%s", now, GetRequestID(c))

                        // Sliding window via sorted set (pipeline, no Lua)
                        pipe := rdb.Pipeline()
                        remCmd := pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%.0f", windowStart))
                        cardCmd := pipe.ZCard(ctx, key)
                        _, err := pipe.Exec(ctx)

                        if err == nil {
                                remCmd.Val() // consume
                                count = cardCmd.Val()

                                if count >= int64(maxRequests) {
                                        c.Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
                                        c.Set("X-RateLimit-Remaining", "0")
                                        return bizerr.ErrTooManyRequests
                                }

                                // Record this request in the window
                                _ = cache.ZAdd(ctx, key, now, member)
                                count++

                                // Set TTL to auto-cleanup when no requests come
                                rdb.Expire(ctx, key, window+time.Second)
                        } else {
                                // Redis error: fall back to in-memory
                                count = fallbackLimiter.slidingIncr(key, window)
                        }
                } else {
                        count = fallbackLimiter.slidingIncr(key, window)
                }

                if count > int64(maxRequests) {
                        c.Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
                        c.Set("X-RateLimit-Remaining", "0")
                        return bizerr.ErrTooManyRequests
                }

                c.Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
                c.Set("X-RateLimit-Remaining", strconv.Itoa(max(int(count)-1, 0)))

                return c.Next()
        }
}