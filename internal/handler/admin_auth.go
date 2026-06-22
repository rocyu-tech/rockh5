// Admin authentication handlers and helpers.
//
// This file implements admin login (with brute-force protection), logout,
// token refresh, admin profile retrieval, role validation, and password
// complexity enforcement.
package handler

import (
        "context"
        "errors"
        "fmt"
        "strings"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/internal/config"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/internal/model"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "gorm.io/gorm"
)

// AdminLoginRequest defines the admin login request body.
type AdminLoginRequest struct {
        Username string `json:"username"`
        Password string `json:"password"`
}

// AdminRefreshRequest defines the admin token refresh request body.
type AdminRefreshRequest struct {
        RefreshToken string `json:"refresh_token"`
}

// validRoles is the set of allowed admin roles.
// Must stay in sync with middleware.adminRoleHierarchy and middleware role constants.
var validRoles = map[string]bool{
        "super_admin": true,
        "admin":       true,
        "operator":    true,
        "finance":     true,
        "support":     true,
        "viewer":      true,
}

const (
        // adminLoginMaxAttempts is the number of failed login attempts before the
        // account is temporarily locked. After lockout, the admin must wait for
        // adminLoginLockoutTTL before trying again.
        adminLoginMaxAttempts = 5
        // adminLoginLockoutTTL defines the lockout window. Both the failed-attempts
        // counter and the lock key share this TTL so the window resets after lockout expires.
        adminLoginLockoutTTL = 15 * time.Minute
)

// adminLoginLockKey returns the Redis key that signals the account is locked.
// Format: admin:login:lock:<username>:<ip>
func adminLoginLockKey(username, ip string) string {
        return fmt.Sprintf("admin:login:lock:%s:%s", username, ip)
}

// adminLoginAttemptsKey returns the Redis key used to count consecutive failed
// login attempts for a given username+IP combination.
// Format: admin:login:attempts:<username>:<ip>
func adminLoginAttemptsKey(username, ip string) string {
        return fmt.Sprintf("admin:login:attempts:%s:%s", username, ip)
}

// checkAdminLoginLocked determines whether the admin account is temporarily
// locked due to too many failed login attempts. The lock is per username+IP to
// avoid blocking legitimate admins from other locations.
//
// Brute-force protection flow:
//  1. On each failed login, the attempt counter is incremented in Redis.
//  2. When attempts reach adminLoginMaxAttempts (5), a lock key is set with
//     adminLoginLockoutTTL (15 min).
//  3. Subsequent login attempts check the lock key first; if present the
//     request is rejected with CodeTooManyRequests.
//  4. On successful login, both the attempt counter and lock key are cleared.
//
// If Redis is unavailable, the check is skipped (fail-open) to preserve
// availability — the password check still guards against unauthorized access.
func checkAdminLoginLocked(c *fiber.Ctx, username string) error {
        rdb := cache.Client()
        if rdb == nil {
                return nil // Redis unavailable — skip lock check (fail-open for availability)
        }
        lockKey := adminLoginLockKey(username, c.IP())
        locked, err := cache.Exists(c.Context(), lockKey)
        if err != nil || !locked {
                return nil
        }
        // Get remaining TTL for the error message
        ttl, err := rdb.TTL(c.Context(), lockKey).Result()
        if err != nil {
                ttl = adminLoginLockoutTTL
        }
        minutes := int(ttl.Minutes()) + 1
        middleware.LogWarn(c, "AdminLogin.Locked", fmt.Sprintf("account locked for %s, %d min remaining", username, minutes))
        return bizerr.New(bizerr.CodeTooManyRequests, fmt.Sprintf("account temporarily locked, try again in %d minutes", minutes))
}

// recordAdminLoginFailure increments the per-username+IP failed-attempt counter
// in Redis. On the first failure the key is created with a TTL so the window
// auto-expires. Once the counter reaches adminLoginMaxAttempts the account is
// locked for adminLoginLockoutTTL.
func recordAdminLoginFailure(c *fiber.Ctx, username string) {
        rdb := cache.Client()
        if rdb == nil {
                return
        }
        ctx := c.Context()
        attemptsKey := adminLoginAttemptsKey(username, c.IP())
        lockKey := adminLoginLockKey(username, c.IP())

        // Increment the failure counter atomically
        attempts, err := cache.Incr(ctx, attemptsKey)
        if err != nil {
                return
        }
        // Set TTL on first attempt — this starts the 15-min sliding window.
        // Subsequent increments within this window reuse the same key.
        if attempts == 1 {
                cache.Set(ctx, attemptsKey, 1, adminLoginLockoutTTL)
        }
        // Lock the account once the threshold is reached.
        // The lock key also expires after adminLoginLockoutTTL so the ban is temporary.
        if attempts >= adminLoginMaxAttempts {
                cache.Set(ctx, lockKey, 1, adminLoginLockoutTTL)
                logger.Warnf("[AdminLogin] account locked: username=%s ip=%s attempts=%d", username, c.IP(), attempts)
        }
}

// clearAdminLoginAttempts removes both the attempt counter and the lock key
// on successful login. Uses context.Background() because the caller may invoke
// this in a goroutine where the request context could be cancelled.
func clearAdminLoginAttempts(c *fiber.Ctx, username string) {
        rdb := cache.Client()
        if rdb == nil {
                return
        }
        // Use context.Background() instead of request context — the caller may
        // invoke this in a goroutine or the request context may be cancelled
        // before the Redis command completes.
        ctx := context.Background()
        cache.Del(ctx, adminLoginAttemptsKey(username, c.IP()), adminLoginLockKey(username, c.IP()))
}

// AdminLogin handles admin user authentication.
//
// Flow:
//  1. Parse and validate request body (username + password required).
//  2. Check brute-force lockout (per username+IP).
//  3. Look up admin in DB; verify account is active.
//  4. Verify password hash; record failure or clear counter.
//  5. Fire-and-forget: update last_login_at/last_login_ip.
//  6. Generate JWT access token (480 min) and refresh token (7 days).
//  7. Set httpOnly cookies and return tokens in response body.
//
// POST /api/v1/admin/auth/login
func AdminLogin(c *fiber.Ctx) error {
        logger.Infof("[AdminLogin] start: ip=%s", c.IP())

        var req AdminLoginRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "AdminLogin.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.Username == "" || req.Password == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "username and password are required")
        }

        // Brute-force protection: reject if the account is temporarily locked
        if err := checkAdminLoginLocked(c, req.Username); err != nil {
                return err
        }

        cfg := config.C()
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminLogin.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        // Find admin user by username
        var admin model.AdminUser
        if err := db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "AdminLogin.FindUser", "admin not found: "+req.Username)
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "AdminLogin.FindUser", err)
                return bizerr.ErrInternal
        }

        // Reject disabled accounts
        if admin.Status == model.StatusInactive {
                middleware.LogWarn(c, "AdminLogin.StatusCheck", "admin account disabled")
                return bizerr.ErrAccountDisabled
        }

        // Verify password hash using bcrypt
        if !auth.CheckPassword(req.Password, admin.PasswordHash) {
                middleware.LogWarn(c, "AdminLogin.CheckPassword", "invalid password attempt")
                recordAdminLoginFailure(c, req.Username)
                return bizerr.ErrInvalidPassword
        }

        // Success — clear the failed-attempt counter and lock key
        clearAdminLoginAttempts(c, req.Username)

        // Update last login time and IP asynchronously (fire-and-forget)
        now := time.Now()
        clientIP := c.IP()
        go func() {
                if err := database.DB().Model(&admin).Updates(map[string]interface{}{
                        "last_login_at": now,
                }).Error; err != nil {
                        logger.Warnf("[WARN] AdminLogin.UpdateLastLogin failed: admin_id=%d err=%v", admin.ID, err)
                }
        }()

        // Generate JWT access token (8 hours TTL for admin sessions)
        const adminTTLMinutes = 480
        accessToken, err := auth.GenerateToken(cfg.JWT.ActiveSecrets(), admin.ID, "admin", adminTTLMinutes)
        if err != nil {
                middleware.LogError(c, "AdminLogin.GenerateToken", err)
                return bizerr.ErrInternal
        }

        // Generate refresh token (7 days TTL) for token rotation
        refreshToken, err := auth.GenerateRefreshToken(cfg.JWT.ActiveSecrets(), admin.ID, cfg.JWT.RefreshTTL)
        if err != nil {
                middleware.LogError(c, "AdminLogin.GenerateRefreshToken", err)
                return bizerr.ErrInternal
        }

        // Record audit log asynchronously
        go func() {
                RecordAuditLog(admin.ID, admin.Username, "login", "admin", "", "admin login success", clientIP)
        }()

        // Set httpOnly cookies for XSS/CSRF protection (tokens also returned in body for backward compat)
        setAuthCookies(c, accessToken, refreshToken, adminTTLMinutes*60, cfg.JWT.RefreshTTL*86400)

        // Load menu permissions for this role
        menus := GetRoleMenusByCode(admin.Role)

        logger.Infof("[AdminLogin] success: admin_id=%d username=%s ip=%s menus=%d", admin.ID, admin.Username, clientIP, len(menus))

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "token":         accessToken,
                "refresh_token": refreshToken,
                "token_type":    "Bearer",
                "expires_in":    adminTTLMinutes * 60,
                "admin": fiber.Map{
                        "id":        admin.ID,
                        "username":  admin.Username,
                        "real_name": admin.RealName,
                        "role":      admin.Role,
                        "avatar":    "",
                        "menus":     menus,
                },
        }))
}

// AdminRefreshToken handles admin token refresh using a valid refresh_token.
// Implements token rotation: both a new access token and a new refresh token
// are issued, and the old refresh token is implicitly discarded.
//
// POST /api/v1/admin/auth/refresh
func AdminRefreshToken(c *fiber.Ctx) error {
        var req AdminRefreshRequest
        if err := c.BodyParser(&req); err != nil {
                middleware.LogError(c, "AdminRefresh.BodyParser", err)
                return bizerr.ErrInvalidParams
        }

        if req.RefreshToken == "" {
                return bizerr.New(bizerr.CodeInvalidParams, "refresh_token is required")
        }

        cfg := config.C()

        // Parse and validate the refresh token
        parsedToken, err := auth.ParseRefreshToken(req.RefreshToken, cfg.JWT.ActiveSecrets())
        if err != nil {
                if strings.Contains(err.Error(), "token is expired") {
                        return bizerr.ErrTokenExpired
                }
                middleware.LogWarn(c, "AdminRefresh.Parse", "invalid refresh token")
                return bizerr.ErrInvalidToken
        }
        claims, ok := parsedToken.Claims.(*auth.RefreshClaims)
        if !ok || claims.Type != "refresh" {
                return bizerr.New(bizerr.CodeInvalidToken, "not a refresh token")
        }

        // Verify admin still exists and is active before issuing new tokens
        db := database.DB()
        if db == nil {
                middleware.LogError(c, "AdminRefresh.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }
        var admin model.AdminUser
        if err := db.First(&admin, claims.UserID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "AdminRefresh.FindAdmin", err)
                return bizerr.ErrInternal
        }
        if admin.Status == model.StatusInactive {
                return bizerr.ErrAccountDisabled
        }

        // Issue a new access token (8h) and a new refresh token (7d) — token rotation
        const adminTTLMinutes = 480
        newAccessToken, err := auth.GenerateToken(cfg.JWT.ActiveSecrets(), admin.ID, "admin", adminTTLMinutes)
        if err != nil {
                middleware.LogError(c, "AdminRefresh.GenerateToken", err)
                return bizerr.ErrInternal
        }
        newRefreshToken, err := auth.GenerateRefreshToken(cfg.JWT.ActiveSecrets(), admin.ID, cfg.JWT.RefreshTTL)
        if err != nil {
                middleware.LogError(c, "AdminRefresh.GenerateRefreshToken", err)
                return bizerr.ErrInternal
        }

        // Set httpOnly cookies for XSS/CSRF protection
        setAuthCookies(c, newAccessToken, newRefreshToken, adminTTLMinutes*60, cfg.JWT.RefreshTTL*86400)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "token":         newAccessToken,
                "refresh_token": newRefreshToken,
                "token_type":    "Bearer",
                "expires_in":    adminTTLMinutes * 60,
        }))
}

// AdminLogout handles admin logout by revoking both the current access token
// and, optionally, the refresh token if provided in the request body.
// Also clears httpOnly auth cookies and records an audit log.
//
// POST /api/v1/admin/auth/logout
func AdminLogout(c *fiber.Ctx) error {
        logger.Infof("[AdminLogout] start: admin_id=%d ip=%s", middleware.GetUserID(c), c.IP())

        cfg := config.C()

        // Revoke the current admin access token (added to Redis blacklist)
        middleware.RevokeCurrentToken(c, cfg.JWT.ActiveSecrets())

        // Optionally revoke the refresh token if provided in request body.
        // This is best-effort: if parsing or revocation fails, we still proceed with logout.
        var req struct {
                RefreshToken string `json:"refresh_token"`
        }
        if len(c.Body()) > 0 {
                if err := c.BodyParser(&req); err == nil && req.RefreshToken != "" {
                        rdb := cache.Client()
                        if rdb != nil {
                                parsedToken, err := auth.ParseRefreshToken(req.RefreshToken, cfg.JWT.ActiveSecrets())
                                if err == nil && parsedToken.Valid {
                                        if claims, ok := parsedToken.Claims.(*auth.RefreshClaims); ok && claims.ExpiresAt != nil {
                                                remaining := time.Until(claims.ExpiresAt.Time)
                                                if remaining > 0 {
                                                        if err := auth.RevokeToken(c.Context(), rdb, req.RefreshToken, remaining); err != nil {
                                                                logger.Warnf("[AdminLogout] failed to revoke refresh token: %v", err)
                                                        }
                                                }
                                        }
                                }
                        }
                }
        }

        // Record audit log asynchronously
        adminID := middleware.GetUserID(c)
        if adminID > 0 {
                go func() {
                        RecordAuditLog(adminID, "", "logout", "admin", "", "admin logout", c.IP())
                }()
        }

        // Clear httpOnly cookies to remove tokens from the client
        c.ClearCookie("access_token", "refresh_token")

        logger.Infof("[AdminLogout] success: admin_id=%d ip=%s", adminID, c.IP())

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "logged_out": true,
        }))
}

// GetAdminInfo returns the profile information for the currently authenticated
// admin user. The admin_id is extracted from the JWT claims via middleware.
//
// GET /api/v1/admin/auth/me
func GetAdminInfo(c *fiber.Ctx) error {
        logger.Infof("[GetAdminInfo] start: admin_id=%d", middleware.GetUserID(c))

        adminID := middleware.GetUserID(c)
        if adminID == 0 {
                middleware.LogError(c, "GetAdminInfo", errors.New("admin_id not found in context"))
                return bizerr.ErrUnauthorized
        }

        db := database.DB()
        if db == nil {
                middleware.LogError(c, "GetAdminInfo.DB", errors.New("database not initialized"))
                return bizerr.ErrInternal
        }

        var admin model.AdminUser
        if err := db.First(&admin, adminID).Error; err != nil {
                if errors.Is(err, gorm.ErrRecordNotFound) {
                        middleware.LogWarn(c, "GetAdminInfo.FindAdmin", "admin not found in db")
                        return bizerr.ErrUserNotFound
                }
                middleware.LogError(c, "GetAdminInfo.FindAdmin", err)
                return bizerr.ErrInternal
        }

        logger.Infof("[GetAdminInfo] success: admin_id=%d username=%s", admin.ID, admin.Username)

        // Load menu permissions for this role
        menus := GetRoleMenusByCode(admin.Role)

        return c.JSON(bizerr.SuccessResponse(fiber.Map{
                "admin_id":     admin.ID,
                "username":     admin.Username,
                "real_name":    admin.RealName,
                "email":        admin.Email,
                "role":         admin.Role,
                "status":       admin.Status,
                "last_login_at": admin.LastLoginAt,
                "created_at":   admin.CreatedAt,
                "menus":        menus,
        }))
}

// IsValidRole checks if a role string is a recognized admin role.
// Used during admin creation/update to validate the role field.
func IsValidRole(role string) bool {
        return validRoles[role]
}

// validateAdminPassword enforces password complexity for admin accounts.
// Rules: minimum 10 characters, at least one uppercase, one lowercase, one digit.
func validateAdminPassword(password string) error {
        if len(password) < 10 {
                return bizerr.New(bizerr.CodeInvalidParams, "password must be at least 10 characters")
        }
        var hasUpper, hasLower, hasDigit bool
        for _, ch := range password {
                switch {
                case 'A' <= ch && ch <= 'Z':
                        hasUpper = true
                case 'a' <= ch && ch <= 'z':
                        hasLower = true
                case '0' <= ch && ch <= '9':
                        hasDigit = true
                }
        }
        if !hasUpper || !hasLower || !hasDigit {
                return bizerr.New(bizerr.CodeInvalidParams, "password must contain at least one uppercase letter, one lowercase letter, and one digit")
        }
        return nil
}