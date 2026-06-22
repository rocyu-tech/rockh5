package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rocyu-tech/rockgame/internal/config"
	"github.com/rocyu-tech/rockgame/pkg/logger"
)

const defaultShutdownTimeout = 15 * time.Second

// ShutdownCallbacks holds optional cleanup functions called during graceful shutdown.
// BeforeServer: runs before HTTP server shutdown (e.g., deregister from etcd, stop watchers).
// AfterServer: runs after HTTP server shutdown (e.g., close Redis, close subscribers).
type ShutdownCallbacks struct {
	BeforeServer []func()
	AfterServer  []func()
}

// WaitForSignal blocks until a termination signal (SIGINT/SIGTERM) or SIGHUP is received.
//   - SIGINT/SIGTERM: returns the signal.
//   - SIGHUP: triggers config.Reload(), then continues waiting.
//
// Register SIGHUP for config hot-reload in every service without duplicating signal code.
// Usage:
//
//	sig := WaitForSignal()
//	// then proceed with shutdown...
func WaitForSignal() os.Signal {
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-hupCh:
			logger.Info("server: received SIGHUP, reloading config...")
			config.Reload()
		case sig := <-quit:
			return sig
		}
	}
}

// GracefulShutdown performs the full graceful shutdown sequence:
//  1. Run BeforeServer callbacks (etcd deregister, stop watchers, etc.)
//  2. Shutdown Fiber app with timeout
//  3. Run AfterServer callbacks (close Redis, stop cache subscriber, etc.)
//
// Returns an error if the Fiber shutdown fails or times out.
func GracefulShutdown(serviceName string, app *fiber.App, timeout time.Duration, callbacks ShutdownCallbacks) error {
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}

	logger.Infof("Shutting down %s...", serviceName)

	// Phase 1: pre-shutdown callbacks
	for _, fn := range callbacks.BeforeServer {
		fn()
	}

	// Phase 2: shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Errorf("%s: shutdown error: %v", serviceName, err)
		return err
	}

	// Phase 3: post-shutdown callbacks
	for _, fn := range callbacks.AfterServer {
		fn()
	}

	logger.Infof("%s stopped", serviceName)
	return nil
}