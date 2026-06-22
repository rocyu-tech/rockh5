package middleware

import (
        "errors"
        "fmt"
        "strings"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/golang-jwt/jwt/v5"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// AuthMiddleware validates JWT tokens from request.
// Uses the full key ring for verification, supporting key rotation with grace period.
// Optionally checks a token blacklist when Redis is available (fail-open).
func AuthMiddleware(secrets []string) fiber.Handler {
        return func(c *fiber.Ctx) error {
                // Extract token from Authorization header first, then fall back to httpOnly cookie
                tokenString := ""
                authHeader := c.Get("Authorization")
                if authHeader != "" {
                        parts := strings.SplitN(authHeader, " ", 2)
                        if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
                                tokenString = parts[1]
                        }
                }
                if tokenString == "" {
                        tokenString = c.Cookies("access_token")
                }
                if tokenString == "" {
                        return bizerr.ErrUnauthorized
                }
                claims := &auth.Claims{}

                token, err := jwt.ParseWithClaims(tokenString, claims, auth.KeyFuncForRing(secrets))
                if err != nil {
                        // Distinguish expired vs invalid tokens
                        if errors.Is(err, jwt.ErrTokenExpired) {
                                logger.Warnf("[AUTH] token expired: request_id=%s ip=%s path=%s", GetRequestID(c), c.IP(), c.Path())
                                return bizerr.ErrTokenExpired
                        }
                        logger.Warnf("[AUTH] token invalid: request_id=%s ip=%s path=%s err=%v", GetRequestID(c), c.IP(), c.Path(), err)
                        return bizerr.ErrInvalidToken
                }
                if !token.Valid {
                        return bizerr.ErrInvalidToken
                }

                // Check token blacklist (fail-open: if Redis is down, allow request)
                rdb := cache.Client()
                if rdb != nil {
                        if auth.IsTokenRevoked(c.Context(), rdb, tokenString) {
                                logger.Warnf("[AUTH] token revoked: request_id=%s ip=%s path=%s user_id=%d", GetRequestID(c), c.IP(), c.Path(), claims.UserID)
                                return bizerr.ErrInvalidToken
                        }
                }

                // Store user info in context
                c.Locals("user_id", claims.UserID)
                c.Locals("device_id", claims.DeviceID)
                c.Locals("claims", claims)
                c.Locals("token_string", tokenString) // Store for potential revocation

                return c.Next()
        }
}

// GetTokenString extracts the raw JWT token string from Fiber context.
// Falls back to the access_token httpOnly cookie if the header is absent.
func GetTokenString(c *fiber.Ctx) string {
        if v := c.Locals("token_string"); v != nil {
                if s, ok := v.(string); ok {
                        return s
                }
        }
        authHeader := c.Get("Authorization")
        if authHeader != "" {
                parts := strings.SplitN(authHeader, " ", 2)
                if len(parts) == 2 {
                        return parts[1]
                }
        }
        // Fallback: httpOnly cookie set by auth handlers
        if token := c.Cookies("access_token"); token != "" {
                return token
        }
        return ""
}

// RevokeCurrentToken revokes the current request's JWT token.
// Useful for logout or password change endpoints.
// The TTL is calculated from the token's remaining lifetime.
func RevokeCurrentToken(c *fiber.Ctx, secrets []string) {
        tokenString := GetTokenString(c)
        rdb := cache.Client()
        if rdb == nil || tokenString == "" {
                return
        }
        // Parse to get remaining TTL
        claims := &auth.Claims{}
        parsedToken, err := jwt.ParseWithClaims(tokenString, claims, auth.KeyFuncForRing(secrets))
        if err != nil || !parsedToken.Valid || claims.ExpiresAt == nil {
                return
        }
        remaining := time.Until(claims.ExpiresAt.Time)
        if remaining <= 0 {
                return
        }
        if err := auth.RevokeToken(c.Context(), rdb, tokenString, remaining); err != nil {
                logger.Warnf("[AUTH] failed to revoke token: %v", err)
        }
}

// GetUserID extracts user ID from context or X-User-ID header (set by Gate proxy).
func GetUserID(c *fiber.Ctx) int64 {
        // Priority 1: from Fiber context (set by local AuthMiddleware)
        if v := c.Locals("user_id"); v != nil {
                if id, ok := v.(int64); ok {
                        return id
                }
        }
        // Priority 2: from X-User-ID header (set by Gate proxy after JWT + HMAC verification)
        if h := c.Get("X-User-ID"); h != "" {
                var id int64
                if _, err := fmt.Sscanf(h, "%d", &id); err == nil {
                        return id
                }
        }
        return 0
}

// GetDeviceID extracts device ID from context or X-Device-ID header.
func GetDeviceID(c *fiber.Ctx) string {
        if d, ok := c.Locals("device_id").(string); ok {
                return d
        }
        // Fallback: from X-Device-ID header (set by Gate proxy)
        if h := c.Get("X-Device-ID"); h != "" {
                return h
        }
        return ""
}