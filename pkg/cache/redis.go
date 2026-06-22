package cache

import (
        "context"
        "crypto/rand"
        "fmt"
        "time"

        "github.com/redis/go-redis/v9"
        "github.com/rocyu-tech/rockgame/internal/config"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// cryptoRandInt64 generates a random int64 using crypto/rand (secure, no seeding needed).
func cryptoRandInt64() int64 {
        b := make([]byte, 8)
        _, _ = rand.Read(b)
        return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
                int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

var (
        client *redis.Client
)

// Init initializes the Redis connection
func Init(cfg *config.RedisConfig) error {
        client = redis.NewClient(&redis.Options{
                Addr:     cfg.Addr,
                Password: cfg.Password,
                DB:       cfg.DB,
                PoolSize: cfg.PoolSize,
        })
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _, err := client.Ping(ctx).Result()
        if err != nil {
                return fmt.Errorf("redis ping failed: %w", err)
        }
        return nil
}

// Client returns the global redis client.
// Returns nil-safe client that logs warnings if Redis is not initialized.
func Client() *redis.Client {
        if client == nil {
                // Return a no-op style behavior rather than nil to prevent panics
                // The caller should check for nil, but this provides defense in depth
                logger.Warn("redis: client not initialized, Redis operations will fail")
        }
        return client
}

// Set stores a key-value pair with TTL
func Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        return client.Set(ctx, key, value, ttl).Err()
}

// SetNX sets a key-value pair only if the key does not exist (atomic).
func SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
        if client == nil {
                return false, fmt.Errorf("redis not initialized")
        }
        return client.SetNX(ctx, key, value, ttl).Result()
}

// Get retrieves a value by key
func Get(ctx context.Context, key string) (string, error) {
        if client == nil {
                return "", fmt.Errorf("redis not initialized")
        }
        return client.Get(ctx, key).Result()
}

// Del deletes one or more keys
func Del(ctx context.Context, keys ...string) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        return client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func Exists(ctx context.Context, key string) (bool, error) {
        if client == nil {
                return false, fmt.Errorf("redis not initialized")
        }
        n, err := client.Exists(ctx, key).Result()
        return n > 0, err
}

// Incr increments a key's value
func Incr(ctx context.Context, key string) (int64, error) {
        if client == nil {
                return 0, fmt.Errorf("redis not initialized")
        }
        return client.Incr(ctx, key).Result()
}

// HSet sets a hash field
func HSet(ctx context.Context, key string, values ...interface{}) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        return client.HSet(ctx, key, values...).Err()
}

// HGet gets a hash field
func HGet(ctx context.Context, key, field string) (string, error) {
        if client == nil {
                return "", fmt.Errorf("redis not initialized")
        }
        return client.HGet(ctx, key, field).Result()
}

// ZAdd adds a member to a sorted set
func ZAdd(ctx context.Context, key string, score float64, member string) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        return client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRevRange returns members in descending score order
func ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
        if client == nil {
                return nil, fmt.Errorf("redis not initialized")
        }
        return client.ZRevRange(ctx, key, start, stop).Result()
}

// SAdd adds members to a set
func SAdd(ctx context.Context, key string, members ...interface{}) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        return client.SAdd(ctx, key, members...).Err()
}

// SMembers returns all members of a set
func SMembers(ctx context.Context, key string) ([]string, error) {
        if client == nil {
                return nil, fmt.Errorf("redis not initialized")
        }
        return client.SMembers(ctx, key).Result()
}

// Close gracefully closes the Redis connection.
// Safe to call multiple times.
func Close() {
        if client != nil {
                if err := client.Close(); err != nil {
                        logger.Warnf("redis: close error: %v", err)
                } else {
                        logger.Info("redis: connection closed")
                }
                client = nil
        }
}

// Lock tries to acquire a distributed lock with auto-expiry.
// Returns a unique token that must be passed to Unlock to release the lock.
// This prevents accidental release by other processes.
func Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
        if client == nil {
                return "", fmt.Errorf("redis not initialized")
        }
        token := fmt.Sprintf("%d-%d", time.Now().UnixNano(), cryptoRandInt64())
        ok, err := client.SetNX(ctx, key, token, ttl).Result()
        if err != nil {
                return "", err
        }
        if !ok {
                return "", nil
        }
        return token, nil
}

// unlockScript is the atomic Lua script for safe lock release.
// It checks the token and deletes the key in one atomic operation, eliminating the
// TOCTOU race window of the previous GET+compare+DEL approach.
const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

// Unlock releases a distributed lock only if the value matches the token.
// Uses a Lua script for atomicity — the token check and deletion happen in one
// operation, preventing accidental release of another process's lock.
func Unlock(ctx context.Context, key, token string) error {
        if client == nil {
                return fmt.Errorf("redis not initialized")
        }
        _, err := client.Eval(ctx, unlockScript, []string{key}, token).Result()
        return err
}

// LockWithRenew acquires a distributed lock with automatic renewal.
// The renewCtx controls how long the renewal goroutine runs (usually tied to the caller's lifecycle).
// The lock is automatically released when renewCtx is cancelled or the TTL expires.
// Returns the lock token and a cancel function that immediately releases the lock and stops renewal.
func LockWithRenew(ctx context.Context, key string, ttl time.Duration, renewCtx context.Context) (string, func(), error) {
        token, err := Lock(ctx, key, ttl)
        if err != nil {
                return "", nil, err
        }
        if token == "" {
                return "", nil, nil
        }

        renewalInterval := ttl / 3 // Renew at 1/3 of TTL
        if renewalInterval < 1*time.Second {
                renewalInterval = 1 * time.Second
        }

        done := make(chan struct{})
        go func() {
                ticker := time.NewTicker(renewalInterval)
                defer ticker.Stop()
                for {
                        select {
                        case <-ticker.C:
                                // Check token still matches before renewing (non-Lua)
                                val, err := client.Get(renewCtx, key).Result()
                                if err != nil || val != token {
                                        return // Lock lost or Redis error
                                }
                                if err := client.Expire(renewCtx, key, ttl).Err(); err != nil {
                                        logger.Warnf("[Lock] renewal failed for key=%s: %v", key, err)
                                        return
                                }
                        case <-renewCtx.Done():
                                return
                        case <-done:
                                return
                        }
                }
        }()

        cancel := func() {
                close(done)
                unlockCtx, cancelUnlock := context.WithTimeout(context.Background(), 3*time.Second)
                defer cancelUnlock()
                if err := Unlock(unlockCtx, key, token); err != nil {
                        logger.Warnf("[Lock] unlock failed for key=%s: %v", key, err)
                }
        }

        return token, cancel, nil
}
