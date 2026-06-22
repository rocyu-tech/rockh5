package middleware

import (
        "context"
        "strconv"
        "strings"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// adminRoleHierarchy maps admin roles to permission levels (higher = more access).
var adminRoleHierarchy = map[string]int{
        "super_admin": 100,
        "admin":       80,
        "operator":    50,
        "finance":     40,
        "support":     20,
        "viewer":      10,
}

// routeRBAC maps route path prefixes to the minimum role level required.
// The longest matching prefix wins.
var routeRBAC = []struct {
        prefix string
        level  int
}{
        {"/api/v1/admin/system/admins", 100},
        {"/api/v1/admin/system/payment", 100},
        {"/api/v1/admin/system/vip", 50},
        {"/api/v1/admin/orders", 40},
        {"/api/v1/admin/reports", 40},
        {"/api/v1/admin/games", 50},
        {"/api/v1/admin/banners", 50},
        {"/api/v1/admin/activities", 50},
        {"/api/v1/admin/operation", 50},
        {"/api/v1/admin/upload", 50},
        {"/api/v1/admin/users", 20},
        {"/api/v1/admin/audit-logs", 20},
}

const adminRoleCachePrefix = "admin:role:"
const adminRoleCacheTTL = 5 * time.Minute

// RequireAdminRole is a middleware that enforces role-based access control
// on admin API routes. It reads the admin user's role from Redis cache (or DB fallback)
// and checks against the routeRBAC table. Must be placed after AuthMiddleware.
func RequireAdminRole() fiber.Handler {
        return func(c *fiber.Ctx) error {
                userID := GetUserID(c)
                if userID == 0 {
                        return bizerr.ErrUnauthorized
                }

                role := getAdminRole(c, userID)
                if role == "" {
                        logger.Warnf("[RBAC] admin role not found for user_id=%d path=%s request_id=%s",
                                userID, c.Path(), GetRequestID(c))
                        return bizerr.ErrForbidden
                }

                userLevel, ok := adminRoleHierarchy[role]
                if !ok {
                        logger.Warnf("[RBAC] unknown role %q for user_id=%d: request_id=%s", role, userID, GetRequestID(c))
                        return bizerr.ErrForbidden
                }

                // Find the longest matching route prefix
                path := c.Path()
                requiredLevel := 10 // default: viewer
                for _, entry := range routeRBAC {
                        if strings.HasPrefix(path, entry.prefix) && len(entry.prefix) > len(findBestPrefix(path, requiredLevel)) {
                                requiredLevel = entry.level
                        }
                }

                if userLevel < requiredLevel {
                        logger.Warnf("[RBAC] denied: user_id=%d role=%s(level=%d) required=%d path=%s request_id=%s",
                                userID, role, userLevel, requiredLevel, path, GetRequestID(c))
                        return bizerr.ErrForbidden
                }

                c.Locals("admin_role", role)
                return c.Next()
        }
}

// findBestPrefix returns the longest routeRBAC prefix matching the path at the given level.
func findBestPrefix(path string, level int) string {
        best := ""
        for _, entry := range routeRBAC {
                if strings.HasPrefix(path, entry.prefix) && entry.level == level && len(entry.prefix) > len(best) {
                        best = entry.prefix
                }
        }
        return best
}

// getAdminRole retrieves the admin user's role from Redis cache,
// falling back to database lookup. Returns empty string if not an admin.
func getAdminRole(c *fiber.Ctx, userID int64) string {
        cacheKey := adminRoleCachePrefix + strconv.FormatInt(userID, 10)

        // Try Redis cache first
        rdb := cache.Client()
        if rdb != nil {
                cached, err := rdb.Get(c.Context(), cacheKey).Result()
                if err == nil && cached != "" {
                        return cached
                }
        }

        // DB fallback
        if adminRoleLookup != nil {
                role := adminRoleLookup(c.Context(), userID)
                if role != "" && rdb != nil {
                        // Cache for 5 minutes
                        _ = rdb.Set(c.Context(), cacheKey, role, adminRoleCacheTTL).Err()
                }
                return role
        }

        return ""
}

// AdminRoleLookupFunc is the function signature for looking up an admin's role.
type AdminRoleLookupFunc func(ctx context.Context, userID int64) string

// adminRoleLookup is set during app initialization to provide DB access
// without creating an import cycle.
var adminRoleLookup AdminRoleLookupFunc

// SetAdminRoleLookup sets the function used by RequireAdminRole to look up
// admin roles from the database. Must be called once during app startup.
func SetAdminRoleLookup(fn AdminRoleLookupFunc) {
        adminRoleLookup = fn
}

// InvalidateAdminRoleCache removes the cached role for a specific admin user.
// Call this after role changes (e.g., admin update, status toggle).
func InvalidateAdminRoleCache(ctx context.Context, userID int64) {
        rdb := cache.Client()
        if rdb != nil {
                _ = rdb.Del(ctx, adminRoleCachePrefix+strconv.FormatInt(userID, 10)).Err()
        }
}