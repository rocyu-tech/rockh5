package cache

import (
        "context"
        "encoding/json"
        "fmt"
        "math"
        "strings"
        "sync"
        "time"

        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// ── Cache Invalidation via Redis Pub/Sub ──

// CacheNotifyChannel is the Redis Pub/Sub channel for cache invalidation.
const CacheNotifyChannel = "rockgame:cache:invalidate"

// CacheInvalidateEvent represents a cache invalidation notification.
type CacheInvalidateEvent struct {
        Prefix string   `json:"prefix"`
        Keys   []string `json:"keys,omitempty"`
        Source string   `json:"source"`
}

// PublishInvalidate publishes a cache invalidation event to the Pub/Sub channel.
// Non-blocking: uses short-lived context, returns nil if Redis is unavailable.
func PublishInvalidate(ctx context.Context, event CacheInvalidateEvent) error {
        if client == nil {
                return nil
        }
        data, err := json.Marshal(event)
        if err != nil {
                return fmt.Errorf("cache PublishInvalidate marshal: %w", err)
        }
        pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
        defer cancel()
        return client.Publish(pubCtx, CacheNotifyChannel, data).Err()
}

// InvalidateByPrefix clears all cache keys matching the given prefix using SCAN.
func InvalidateByPrefix(ctx context.Context, prefix string) {
        if client == nil {
                return
        }
        var cursor uint64
        var count int
        for {
                keys, nextCursor, err := client.Scan(ctx, cursor, prefix+"*", 100).Result()
                if err != nil {
                        logger.Warnf("cache: InvalidateByPrefix scan error at cursor %d: %v", cursor, err)
                        return
                }
                if len(keys) > 0 {
                        if err := client.Del(ctx, keys...).Err(); err != nil {
                                logger.Warnf("cache: InvalidateByPrefix del error: %v", err)
                        } else {
                                count += len(keys)
                        }
                }
                cursor = nextCursor
                if cursor == 0 {
                        break
                }
        }
        logger.Infof("cache: invalidated %d keys with prefix %s", count, prefix)
}

// InvalidateKeys deletes specific cache keys by their full names.
func InvalidateKeys(ctx context.Context, keys ...string) {
        if client == nil || len(keys) == 0 {
                return
        }
        if err := client.Del(ctx, keys...).Err(); err != nil {
                logger.Warnf("cache: InvalidateKeys error: %v", err)
        } else {
                logger.Infof("cache: invalidated %d specific keys: %v", len(keys), keys)
        }
}

// ── Exponential backoff helper ──

const (
        maxBackoff     = 30 * time.Second
        initialBackoff = 500 * time.Millisecond
        backoffFactor  = 2.0
)

// backoffDuration computes the next sleep duration using exponential backoff with jitter.
func backoffDuration(attempt int) time.Duration {
        // attempt starts at 0 on first failure
        d := float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt))
        if d > float64(maxBackoff) {
                d = float64(maxBackoff)
        }
        // Add ±10% jitter to avoid thundering herd
        jitter := d * 0.1 * (float64(time.Now().UnixNano()%1000)/1000.0 - 0.5)
        return time.Duration(d + jitter)
}

// ── CacheSubscriber ──

// CacheSubscriber handles subscribing to cache invalidation events via Redis Pub/Sub.
type CacheSubscriber struct {
        mu       sync.Mutex
        handlers map[string]func(CacheInvalidateEvent) // prefix → handler
        stopCh   chan struct{}
        wg       sync.WaitGroup // tracks the subscriber goroutine
        running  bool
}

// NewCacheSubscriber creates a new cache subscriber.
func NewCacheSubscriber() *CacheSubscriber {
        return &CacheSubscriber{
                handlers: make(map[string]func(CacheInvalidateEvent)),
                stopCh:   make(chan struct{}),
        }
}

// On registers a handler for cache invalidation events matching the given prefix.
// Use "*" as prefix to handle all events.
func (s *CacheSubscriber) On(prefix string, handler func(CacheInvalidateEvent)) {
        s.mu.Lock()
        defer s.mu.Unlock()
        s.handlers[prefix] = handler
}

// Start begins listening for cache invalidation events in a background goroutine.
// Uses exponential backoff on disconnection (500ms → 1s → 2s → 4s → ... → 30s cap).
// Blocks only briefly to launch the goroutine, then returns immediately.
func (s *CacheSubscriber) Start() {
        if client == nil {
                logger.Warn("cache: subscriber not started — Redis not initialized")
                return
        }

        s.mu.Lock()
        if s.running {
                s.mu.Unlock()
                return
        }
        s.running = true
        s.mu.Unlock()

        s.wg.Add(1)
        go s.loop()
}

// loop is the main subscriber loop with exponential backoff reconnection.
func (s *CacheSubscriber) loop() {
        defer s.wg.Done()

        logger.Infof("cache: subscriber started, listening on channel %s", CacheNotifyChannel)

        var attempt int
        for {
                ch := client.Subscribe(context.Background(), CacheNotifyChannel).Channel()

                // Reset backoff on successful subscription
                attempt = 0

                for {
                        select {
                        case <-s.stopCh:
                                // Close subscriber connection to release resources
                                _ = client.Close()
                                logger.Info("cache: subscriber stopped")
                                return

                        case msg, ok := <-ch:
                                if !ok {
                                        // Channel closed — reconnect with backoff.
                                        // G11: use select on stopCh instead of blocking time.Sleep.
                                        sleep := backoffDuration(attempt)
                                        attempt++
                                        logger.Warnf("cache: subscriber channel closed, reconnecting in %v (attempt %d)...", sleep, attempt)
                                        select {
                                        case <-s.stopCh:
                                                logger.Info("cache: subscriber stopped during reconnect wait")
                                                return
                                        case <-time.After(sleep):
                                                // continue to reconnect
                                        }
                                        break // breaks inner for, continues outer for (re-Subscribe)
                                }

                                s.handleMessage(msg.Payload)
                        }
                }
        }
}

// handleMessage processes a single invalidation event.
// G12: notify registered handlers BEFORE invalidating local caches,
// so handlers can react to the pre-invalidation state if needed.
func (s *CacheSubscriber) handleMessage(payload string) {
        var event CacheInvalidateEvent
        if err := json.Unmarshal([]byte(payload), &event); err != nil {
                logger.Warnf("cache: subscriber unmarshal error: %s", err)
                return
        }

        logger.Infof("cache: received invalidation event: prefix=%s keys=%v source=%s",
                event.Prefix, event.Keys, event.Source)

        // G12: notify registered handlers first (before cache is cleared)
        s.mu.Lock()
        for prefix, handler := range s.handlers {
                if prefix == "*" || strings.HasPrefix(event.Prefix, prefix) {
                        handler(event)
                }
        }
        s.mu.Unlock()

        // Then execute the invalidation
        ctx := context.Background()
        if len(event.Keys) > 0 {
                InvalidateKeys(ctx, event.Keys...)
        } else if event.Prefix != "" {
                InvalidateByPrefix(ctx, event.Prefix)
        }
}

// Stop signals the subscriber to stop and waits for the goroutine to finish.
func (s *CacheSubscriber) Stop() {
        s.mu.Lock()
        if !s.running {
                s.mu.Unlock()
                return
        }
        s.running = false
        close(s.stopCh)
        s.mu.Unlock()

        s.wg.Wait()
}

// Running returns whether the subscriber is active.
func (s *CacheSubscriber) Running() bool {
        s.mu.Lock()
        defer s.mu.Unlock()
        return s.running
}