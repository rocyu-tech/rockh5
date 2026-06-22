package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// SecurityHeadersMiddleware adds production-standard security response headers.
//
// Headers set:
//   - X-Content-Type-Options: nosniff — prevents MIME type sniffing
//   - X-Frame-Options: DENY — prevents clickjacking (iframe embedding)
//   - X-XSS-Protection: 0 — disables legacy XSS filter (modern browsers use CSP;
//     the filter can actually introduce vulnerabilities)
//   - Referrer-Policy: strict-origin-when-cross-origin — only send origin on cross-origin
//   - Permissions-Policy: restricts browser features (camera, microphone, etc.)
//   - Content-Security-Policy: restrictive default policy
//
// Note: HSTS and CSP with specific domains should be configured at the reverse
// proxy level (nginx/Caddy) where TLS termination occurs, but a default CSP is
// set here for defense-in-depth.
func SecurityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "0")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		// Default CSP: restrict script sources to same-origin and 'self',
		// allow images/data from any HTTPS source (needed for game thumbnails, avatars).
		// Production should tighten this per-service.
		c.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")
		return c.Next()
	}
}