// Package bgtask provides a managed goroutine pool for fire-and-forget tasks
// (audit logs, cache invalidation, last-login updates, etc.).
//
// All tasks are tracked by a WaitGroup and protected by panic recovery.
// During graceful shutdown, call Wait() to drain in-flight tasks before
// closing DB/Redis connections.
package bgtask

import (
        "sync"
        "sync/atomic"

        "github.com/rocyu-tech/rockgame/pkg/logger"
)

var (
        wg      sync.WaitGroup
        count   atomic.Int64
        stopped atomic.Bool
)

// Go launches a background goroutine with panic recovery and WaitGroup tracking.
// Safe to call from any goroutine, including HTTP handlers.
//
// Usage:
//
//      bgtask.Go(func() { recordAuditLog(...) })
//      bgtask.Go(func() { notifyCacheInvalidate(...) })
func Go(fn func()) {
        if stopped.Load() {
                logger.Warn("[BG] task rejected: shutdown in progress, running synchronously")
                fn()
                return
        }
        wg.Add(1)
        go func() {
                defer wg.Done()
                defer func() {
                        if r := recover(); r != nil {
                                logger.Errorf("[BACKGROUND_PANIC] %v", r)
                        }
                }()
                count.Add(1)
                defer count.Add(-1)
                fn()
        }()
}

// Wait blocks until all in-flight background tasks complete.
// Call this BEFORE closing DB/Redis connections during graceful shutdown.
// Returns immediately if called more than once (idempotent).
func Wait() {
        if !stopped.CompareAndSwap(false, true) {
                return
        }
        wg.Wait()
}

// Count returns the current number of running background tasks.
// Useful for health checks and diagnostics.
func Count() int64 {
        return count.Load()
}