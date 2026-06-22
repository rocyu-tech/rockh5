package middleware

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CSRFMiddleware provides defense-in-depth CSRF protection by validating
// the Origin or Referer header on state-changing requests (POST, PUT, DELETE, PATCH)
// against a list of allowed origins.
//
// The primary CSRF protection comes from SameSite=Lax cookies (set by auth handlers),
// which block cross-site POST/PUT/DELETE/PATCH requests at the browser level.
// This middleware adds origin validation as a secondary check for non-browser clients
// or edge cases where SameSite enforcement may vary.
//
// Requests are allowed if:
//   - They are safe methods (GET, HEAD, OPTIONS)
//   - The Origin or Referer matches an allowed origin
//   - No Origin/Referer is present (browser SameSite enforcement applies)
//   - The request carries the access_token cookie (browser-initiated, SameSite enforced)
func CSRFMiddleware(allowedOrigins string) fiber.Handler {
	origins := parseOrigins(allowedOrigins)
	allowAll := len(origins) == 1 && origins[0] == "*"

	return func(c *fiber.Ctx) error {
		method := c.Method()
		// Only check state-changing methods
		if method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
			return c.Next()
		}

		// If all origins are allowed, skip the check
		if allowAll {
			return c.Next()
		}

		origin := c.Get("Origin")
		referer := c.Get("Referer")

		// Extract origin from Referer if Origin is empty
		checkOrigin := origin
		if checkOrigin == "" && referer != "" {
			if u, err := url.Parse(referer); err == nil {
				checkOrigin = u.Scheme + "://" + u.Host
			}
		}

		// If we have an origin to check, validate it
		if checkOrigin != "" {
			for _, allowed := range origins {
				if allowed == checkOrigin {
					return c.Next()
				}
			}
			// Origin present but not in allowed list — reject
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"code":    403,
				"message": "CSRF: origin not allowed",
			})
		}

		// No Origin or Referer present — allow only if the request carries
		// the access_token cookie (indicates browser-initiated with SameSite enforcement).
		// Direct API calls without Origin should use the Authorization header
		// (server-to-server, mobile apps, etc.) and are not subject to CSRF.
		if c.Cookies("access_token") != "" {
			return c.Next()
		}

		// No origin and no auth cookie — this could be a cross-site form POST
		// from an older browser that strips Origin. Reject as a precaution.
		// However, also allow if Authorization header is present (non-browser client).
		if c.Get("Authorization") != "" {
			return c.Next()
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    403,
			"message": "CSRF: missing origin and no authentication",
		})
	}
}

// parseOrigins splits a comma-separated origins string into a trimmed slice.
func parseOrigins(origins string) []string {
	if origins == "" {
		return []string{}
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}