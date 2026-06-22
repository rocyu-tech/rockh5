package auth

import (
        "context"
        "fmt"
        "strings"
        "time"

        "github.com/golang-jwt/jwt/v5"
        "github.com/redis/go-redis/v9"
        "golang.org/x/crypto/bcrypt"
)

// tokenBlacklistPrefix is the Redis key prefix for revoked tokens.
const tokenBlacklistPrefix = "token:blacklist:"

// Claims represents JWT claims for access tokens.
// Aligned with middleware.Claims to ensure generate/parse consistency.
type Claims struct {
        UserID   int64  `json:"user_id"`
        DeviceID string `json:"device_id"`
        jwt.RegisteredClaims
}

// RefreshClaims represents JWT claims for refresh tokens.
type RefreshClaims struct {
        UserID int64  `json:"user_id"`
        Type   string `json:"type"` // always "refresh"
        jwt.RegisteredClaims
}

// GenerateToken creates a JWT access token signed with secrets[0] (current key).
// The token header includes a "kid" (key ID) set to "0" so parsers know which key signed it.
// Falls back to single-secret mode when secrets has only one element.
func GenerateToken(secrets []string, userID int64, deviceID string, ttlMinutes int) (string, error) {
        if len(secrets) == 0 {
                return "", fmt.Errorf("no JWT secrets configured")
        }
        now := time.Now()
        claims := Claims{
                UserID:   userID,
                DeviceID: deviceID,
                RegisteredClaims: jwt.RegisteredClaims{
                        ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(ttlMinutes) * time.Minute)),
                        IssuedAt:  jwt.NewNumericDate(now),
                },
        }
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        token.Header["kid"] = "0" // always sign with current key (index 0)
        signed, err := token.SignedString([]byte(secrets[0]))
        if err != nil {
                return "", fmt.Errorf("sign token: %w", err)
        }
        return signed, nil
}

// GenerateRefreshToken creates a JWT refresh token signed with secrets[0] (current key).
func GenerateRefreshToken(secrets []string, userID int64, ttlDays int) (string, error) {
        if len(secrets) == 0 {
                return "", fmt.Errorf("no JWT secrets configured")
        }
        now := time.Now()
        claims := RefreshClaims{
                UserID: userID,
                Type:   "refresh",
                RegisteredClaims: jwt.RegisteredClaims{
                        ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(ttlDays) * 24 * time.Hour)),
                        IssuedAt:  jwt.NewNumericDate(now),
                },
        }
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        token.Header["kid"] = "0"
        signed, err := token.SignedString([]byte(secrets[0]))
        if err != nil {
                return "", fmt.Errorf("sign refresh token: %w", err)
        }
        return signed, nil
}

// KeyFuncForRing creates a jwt.Keyfunc that resolves the signing key.
// If the token has a "kid" header matching a key index, that key is used directly.
// Otherwise, if only one key exists, use it; with multiple keys, kid is required.
func KeyFuncForRing(secrets []string) jwt.Keyfunc {
        return func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                // Try to resolve by kid first
                if kid, ok := token.Header["kid"].(string); ok {
                        idx := 0
                        if _, err := fmt.Sscanf(kid, "%d", &idx); err == nil && idx >= 0 && idx < len(secrets) {
                                return []byte(secrets[idx]), nil
                        }
                }
                // Fallback: if only one key, use it; otherwise return error (kid required for multi-key)
                if len(secrets) == 1 {
                        return []byte(secrets[0]), nil
                }
                return nil, fmt.Errorf("token has no valid kid and multiple secrets configured")
        }
}

// ParseAccessToken parses and validates an access token from an Authorization header.
// Supports key rotation: verifies using the key ring. Tokens signed with any key in the
// ring are accepted (grace period for tokens signed before key rotation).
// This function does NOT call c.Next() and is safe to use outside Fiber middleware chains.
func ParseAccessToken(authHeader string, secrets []string) (*Claims, error) {
        if authHeader == "" {
                return nil, fmt.Errorf("missing authorization header")
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
                return nil, fmt.Errorf("invalid authorization header format")
        }

        tokenString := parts[1]
        claims := &Claims{}

        token, err := jwt.ParseWithClaims(tokenString, claims, KeyFuncForRing(secrets))
        if err != nil {
                return nil, err
        }
        if !token.Valid {
                return nil, fmt.Errorf("invalid token")
        }
        return claims, nil
}

// ErrTokenExpiredStr is the string returned by jwt.ErrTokenExpired.Error()
const ErrTokenExpiredStr = "token is expired"

// IsTokenExpiredError checks if an error from ParseAccessToken indicates token expiration.
func IsTokenExpiredError(err error) bool {
        return err != nil && strings.Contains(err.Error(), ErrTokenExpiredStr)
}

// ParseRefreshToken parses and validates a refresh token, returning claims and the parsed token.
// Supports key rotation via the same key ring mechanism.
func ParseRefreshToken(tokenString string, secrets []string) (*jwt.Token, error) {
        claims := &RefreshClaims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, KeyFuncForRing(secrets))
        if err != nil {
                return nil, fmt.Errorf("parse refresh token: %w", err)
        }
        if !token.Valid {
                return nil, fmt.Errorf("invalid refresh token")
        }
        return token, nil
}

// HashPassword hashes a plain password using bcrypt
func HashPassword(password string) (string, error) {
        hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
                return "", fmt.Errorf("hash password: %w", err)
        }
        return string(hash), nil
}

// CheckPassword compares a plain password with a bcrypt hash
func CheckPassword(password, hash string) bool {
        return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// RevokeToken adds a token to the blacklist in Redis.
// The token is blacklisted until its natural expiry time (from JWT claims).
// If Redis is unavailable, the revocation fails (fail-closed for security).
// The redisClient parameter allows injection without creating a circular import.
func RevokeToken(ctx context.Context, redisClient *redis.Client, tokenString string, ttl time.Duration) error {
        if redisClient == nil {
                return fmt.Errorf("redis not available for token revocation")
        }
        key := tokenBlacklistPrefix + tokenString
        return redisClient.Set(ctx, key, "1", ttl).Err()
}

// IsTokenRevoked checks if a token has been revoked.
// Returns true if the token is in the blacklist.
func IsTokenRevoked(ctx context.Context, redisClient *redis.Client, tokenString string) bool {
        if redisClient == nil {
                return false // If Redis is down, can't verify — let middleware decide
        }
        key := tokenBlacklistPrefix + tokenString
        n, err := redisClient.Exists(ctx, key).Result()
        return err == nil && n > 0
}