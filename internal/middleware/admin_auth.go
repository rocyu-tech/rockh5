package middleware

import (
        "errors"
        "strings"

        "github.com/gofiber/fiber/v2"
        "github.com/golang-jwt/jwt/v5"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// Admin role constants — must match admin_user.role column values.
const (
        RoleSuper    = "super_admin"
        RoleAdmin    = "admin"
        RoleOperator = "operator"
        RoleFinance  = "finance"
        RoleSupport  = "support"
        RoleViewer   = "viewer"
)

// roleLevel defines the hierarchy: higher number = more privileges.
var roleLevel = map[string]int{
        RoleSuper:    4,
        RoleAdmin:    3,
        RoleOperator: 2,
        RoleFinance:  2,
        RoleSupport:  1,
        RoleViewer:   1,
}

// GetAdminRole extracts the admin role from Fiber context.
// Returns empty string if not set (middleware not applied).
func GetAdminRole(c *fiber.Ctx) string {
        if v := c.Locals("admin_role"); v != nil {
                if s, ok := v.(string); ok {
                        return s
                }
        }
        return ""
}

// AdminAuthMiddleware verifies JWT and loads the admin's role from DB into context.
//
// NOTE: We inline JWT verification here instead of calling AuthMiddleware, because
// AuthMiddleware calls c.Next() which would skip the role-loading step and jump
// straight to the next middleware (RequireRole) before admin_role is set.
func AdminAuthMiddleware(secrets []string) fiber.Handler {
        return func(c *fiber.Ctx) error {
                // Step 1: Extract and verify JWT token (inlined from AuthMiddleware)
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
                        if errors.Is(err, jwt.ErrTokenExpired) {
                                logger.Warnf("[AdminAuth] token expired: request_id=%s ip=%s path=%s", GetRequestID(c), c.IP(), c.Path())
                                return bizerr.ErrTokenExpired
                        }
                        logger.Warnf("[AdminAuth] token invalid: request_id=%s ip=%s path=%s err=%v", GetRequestID(c), c.IP(), c.Path(), err)
                        return bizerr.ErrInvalidToken
                }
                if !token.Valid {
                        return bizerr.ErrInvalidToken
                }

                // Check token blacklist (fail-open: if Redis is down, allow request)
                rdb := cache.Client()
                if rdb != nil {
                        if auth.IsTokenRevoked(c.Context(), rdb, tokenString) {
                                logger.Warnf("[AdminAuth] token revoked: request_id=%s ip=%s path=%s user_id=%d", GetRequestID(c), c.IP(), c.Path(), claims.UserID)
                                return bizerr.ErrInvalidToken
                        }
                }

                // Store user info in context
                c.Locals("user_id", claims.UserID)
                c.Locals("device_id", claims.DeviceID)
                c.Locals("claims", claims)
                c.Locals("token_string", tokenString)

                // Step 2: Load admin role from DB
                adminID := claims.UserID
                if adminID == 0 {
                        return bizerr.ErrUnauthorized
                }

                db := database.DB()
                if db == nil {
                        LogError(c, "AdminAuth.DB", gorm.ErrRecordNotFound)
                        return bizerr.ErrInternal
                }

                var admin model.AdminUser
                if err := db.Select("id, role, status").First(&admin, adminID).Error; err != nil {
                        if errors.Is(err, gorm.ErrRecordNotFound) {
                                LogWarn(c, "AdminAuth.FindAdmin", "admin not found in db")
                                return bizerr.ErrUnauthorized
                        }
                        LogError(c, "AdminAuth.FindAdmin", err)
                        return bizerr.ErrInternal
                }

                if admin.Status != model.StatusActive {
                        LogWarn(c, "AdminAuth.StatusCheck", "admin account is disabled")
                        return bizerr.New(bizerr.CodeAccountDisabled, "admin account is disabled")
                }

                // Step 3: Store role in context, then proceed to next middleware
                c.Locals("admin_role", admin.Role)
                return c.Next()
        }
}

// RequireRole creates a middleware that checks if the admin has the required role level.
// The allowed roles are ordered by privilege — the middleware checks if the admin's role
// level is >= the minimum allowed level.
//
// Usage:
//
//      adminAPI.Use(middleware.RequireRole(middleware.RoleOperator)) // operator and above
//      adminAPI.Use(middleware.RequireRole(middleware.RoleSuper))    // super only
func RequireRole(allowedRoles ...string) fiber.Handler {
        if len(allowedRoles) == 0 {
                return func(c *fiber.Ctx) error { return c.Next() }
        }

        // Find the minimum required level from the allowed roles
        // Any role with level >= minLevel is allowed
        minLevel := int(^uint(0) >> 1) // max int
        for _, r := range allowedRoles {
                if lvl, ok := roleLevel[r]; ok && lvl < minLevel {
                        minLevel = lvl
                }
        }
        // If no valid roles provided, deny all
        if minLevel == int(^uint(0)>>1) {
                logger.Warnf("[RBAC] RequireRole called with unknown roles: %v — denying all access", allowedRoles)
                return func(c *fiber.Ctx) error {
                        return bizerr.ErrForbidden
                }
        }

        return func(c *fiber.Ctx) error {
                currentRole := GetAdminRole(c)
                if currentRole == "" {
                        return bizerr.ErrUnauthorized
                }

                currentLevel, ok := roleLevel[currentRole]
                if !ok {
                        // Unknown role — deny access
                        LogWarn(c, "RBAC.UnknownRole", "admin has unknown role: "+currentRole)
                        return bizerr.ErrForbidden
                }

                if currentLevel < minLevel {
                        LogWarn(c, "RBAC.Insufficient", "role="+currentRole+" requires one of: "+joinRoles(allowedRoles))
                        return bizerr.ErrForbidden
                }

                return c.Next()
        }
}

func joinRoles(roles []string) string {
        result := ""
        for i, r := range roles {
                if i > 0 {
                        result += ", "
                }
                result += r
        }
        return result
}