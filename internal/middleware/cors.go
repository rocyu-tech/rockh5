package middleware

import (
        "strings"

        "github.com/gofiber/fiber/v2"
)

// CORSMiddleware handles Cross-Origin Resource Sharing.
// When allowedOrigins is "*", credentials are disabled for security.
// When specific origins are listed, credentials are enabled.
func CORSMiddleware(allowedOrigins string) fiber.Handler {
        origins := strings.Split(allowedOrigins, ",")
        isWildcard := len(origins) == 1 && origins[0] == "*"

        return func(c *fiber.Ctx) error {
                origin := c.Get("Origin")

                if isWildcard {
                        c.Set("Access-Control-Allow-Origin", "*")
                        // Do NOT set Allow-Credentials with wildcard (CORS spec violation)
                } else {
                        // Check if the request origin is in the allowed list
                        for _, o := range origins {
                                o = strings.TrimSpace(o)
                                if o == origin {
                                        c.Set("Access-Control-Allow-Origin", origin)
                                        c.Set("Vary", "Origin")
                                        c.Set("Access-Control-Allow-Credentials", "true")
                                        break
                                }
                        }
                }

                c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
                c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
                c.Set("Access-Control-Max-Age", "86400")

                if c.Method() == "OPTIONS" {
                        return c.SendStatus(204)
                }
                return c.Next()
        }
}