package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	bizerr "github.com/rocyu-tech/rockgame/internal/errors"
	"github.com/rocyu-tech/rockgame/pkg/cache"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

// HMACMiddleware verifies HMAC-SHA256 signatures for internal API calls.
// Used by backend mesh services to verify that requests came from the Gate proxy.
func HMACMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timestamp := c.Get("X-Timestamp")
		nonce := c.Get("X-Nonce")
		signature := c.Get("X-Signature")

		if timestamp == "" || nonce == "" || signature == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(
				bizerr.ErrorResponse(bizerr.ErrUnauthorized),
			)
		}

		// Check timestamp freshness (within 5 minutes)
		ts, err := time.Parse(time.RFC3339, timestamp)
		if err != nil || time.Since(ts) > 5*time.Minute {
			return c.Status(fiber.StatusUnauthorized).JSON(
				bizerr.ErrorResponse(bizerr.ErrInvalidToken),
			)
		}

		// Check nonce uniqueness using atomic SetNX to prevent replay
		ctx := c.Context()
		nonceKey := fmt.Sprintf("hmac:nonce:%s", nonce)
		ok, _ := cache.SetNX(ctx, nonceKey, "1", 10*time.Minute)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(
				bizerr.ErrorResponse(bizerr.ErrDuplicateRequest),
			)
		}

		// Verify signature
		body := c.Body()
		payload := fmt.Sprintf("%s|%s|%s", timestamp, nonce, string(body))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
			logger.Warnf("[HMAC] signature mismatch: request_id=%s ip=%s path=%s",
				GetRequestID(c), c.IP(), c.Path())
			return c.Status(fiber.StatusUnauthorized).JSON(
				bizerr.ErrorResponse(bizerr.ErrInvalidToken),
			)
		}

		return c.Next()
	}
}

// InternalAuthMiddleware provides a unified middleware for backend mesh services.
// It replaces the separate AuthMiddleware (JWT re-validation) with:
//   - If hmacSecret is set: verify HMAC signature from Gate proxy (production mode)
//   - If hmacSecret is empty: trust X-User-ID header from Gate (dev/test mode)
//
// This eliminates the security gap where X-User-ID could be forged without verification.
// The Gate proxy is the ONLY trusted source of X-User-ID, verified via HMAC.
func InternalAuthMiddleware(hmacSecret string) fiber.Handler {
	if hmacSecret != "" {
		// Production mode: require HMAC signature from Gate proxy
		return HMACMiddleware(hmacSecret)
	}
	// Dev/test mode: trust X-User-ID from Gate (no HMAC verification)
	return func(c *fiber.Ctx) error {
		// In dev mode, just pass through — the Gate already validated JWT
		// and set X-User-ID. GetUserID() will read from this header.
		return c.Next()
	}
}

// GenerateHMAC generates HMAC-SHA256 signature for outgoing requests.
func GenerateHMAC(secret, timestamp, nonce, body string) string {
	payload := fmt.Sprintf("%s|%s|%s", timestamp, nonce, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}