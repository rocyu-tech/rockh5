package proxy

import (
        "context"
        "bytes"
        "crypto/hmac"
        "crypto/sha256"
        "encoding/hex"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "strings"
        "sync"
        "sync/atomic"
        "time"

        "github.com/gofiber/fiber/v2"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/pkg/auth"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// circuitState tracks the health state of a backend.
type circuitState int32

const (
        circuitClosed   circuitState = 0 // healthy, forwarding requests
        circuitOpen     circuitState = 1 // unhealthy, rejecting requests
        circuitHalfOpen circuitState = 2 // probing, allow limited requests
)

// backendCircuit wraps a backendPool with circuit breaker logic.
type backendCircuit struct {
        state              atomic.Int32
        consecutiveFailures atomic.Int64
        lastFailureTime    atomic.Int64
        failureThreshold   int64
        resetTimeout       time.Duration
        halfOpenMax        int64
        halfOpenCount      atomic.Int64
}

func newBackendCircuit(failureThreshold int64, resetTimeout time.Duration) *backendCircuit {
        bc := &backendCircuit{
                failureThreshold: failureThreshold,
                resetTimeout:     resetTimeout,
                halfOpenMax:      3,
        }
        bc.state.Store(int32(circuitClosed))
        return bc
}

func (bc *backendCircuit) allowRequest() bool {
        state := circuitState(bc.state.Load())
        switch state {
        case circuitClosed:
                return true
        case circuitOpen:
                lastFail := time.Unix(0, bc.lastFailureTime.Load())
                if time.Since(lastFail) > bc.resetTimeout {
                        if bc.state.CompareAndSwap(int32(circuitOpen), int32(circuitHalfOpen)) {
                                bc.halfOpenCount.Store(0)
                                logger.Info("[CIRCUIT] transitioning to half-open")
                        }
                        return true
                }
                return false
        case circuitHalfOpen:
                if bc.halfOpenCount.Load() < bc.halfOpenMax {
                        bc.halfOpenCount.Add(1)
                        return true
                }
                return false
        }
        return false
}

func (bc *backendCircuit) recordSuccess() {
        bc.consecutiveFailures.Store(0)
        if circuitState(bc.state.Load()) == circuitHalfOpen {
                bc.state.Store(int32(circuitClosed))
                logger.Info("[CIRCUIT] recovered, transitioning to closed")
        }
}

func (bc *backendCircuit) recordFailure() {
        bc.consecutiveFailures.Add(1)
        bc.lastFailureTime.Store(time.Now().UnixNano())
        if bc.consecutiveFailures.Load() >= bc.failureThreshold {
                if circuitState(bc.state.Load()) != circuitOpen {
                        bc.state.Store(int32(circuitOpen))
                        logger.Warnf("[CIRCUIT] opened after %d consecutive failures", bc.consecutiveFailures.Load())
                }
        }
}

// RouteConfig defines a route entry: path prefix → backend service
type RouteConfig struct {
        Prefix      string `yaml:"prefix"`       // path prefix, e.g. "/api/v1/account"
        Backend     string `yaml:"backend"`      // backend service name, e.g. "account"
        Auth        bool   `yaml:"auth"`         // whether JWT auth is required
        StripPrefix int    `yaml:"strip_prefix"` // number of prefix segments to strip (0 = keep all)
}

// backendPool holds multiple target instances for a single backend service.
// Requests are distributed via round-robin across all registered targets.
type backendPool struct {
        targets []*url.URL
        counter uint64 // atomic counter for round-robin
        seen    map[string]bool // G4: deduplicate addresses within this pool
}

// next returns the next target URL using atomic round-robin.
func (bp *backendPool) next() *url.URL {
        n := uint64(len(bp.targets))
        if n == 0 {
                return nil
        }
        idx := atomic.AddUint64(&bp.counter, 1) % n
        return bp.targets[idx]
}

// Proxy manages the route table and reverse proxies to backend services
type Proxy struct {
        routes    []RouteConfig
        backends  map[string]*backendPool       // backend name → target pool
        circuits  map[string]*backendCircuit    // backend name → circuit breaker
        transport *http.Transport
        routeMu   sync.RWMutex // guards routes for dynamic updates
        mu        sync.RWMutex // guards backends and circuits
}

// New creates a Proxy with the given route table
func New(routes []RouteConfig) *Proxy {
        return &Proxy{
                routes:   routes,
                backends: make(map[string]*backendPool),
                circuits: make(map[string]*backendCircuit),
                transport: &http.Transport{
                        MaxIdleConnsPerHost: 100,
                        IdleConnTimeout:     90 * time.Second,
                },
        }
}

// UpdateRoutes atomically replaces the route table.
// This is called by the etcd route watcher to dynamically update routing
// without restarting the gate service.
func (p *Proxy) UpdateRoutes(routes []RouteConfig) {
        p.routeMu.Lock()
        defer p.routeMu.Unlock()
        p.routes = routes
        logger.Infof("proxy: routes updated, %d routes loaded", len(routes))
}

// GetRoutes returns a copy of the current route table (thread-safe).
func (p *Proxy) GetRoutes() []RouteConfig {
        p.routeMu.RLock()
        defer p.routeMu.RUnlock()
        result := make([]RouteConfig, len(p.routes))
        copy(result, p.routes)
        return result
}

// UniqueBackends extracts deduplicated backend names from the current route table.
// This eliminates the need for a separate backendServices list.
func (p *Proxy) UniqueBackends() []string {
        p.routeMu.RLock()
        defer p.routeMu.RUnlock()
        seen := make(map[string]struct{}, len(p.routes))
        var result []string
        for _, r := range p.routes {
                if _, exists := seen[r.Backend]; !exists {
                        seen[r.Backend] = struct{}{}
                        result = append(result, r.Backend)
                }
        }
        return result
}

// RegisterTarget adds a backend service target address.
// Multiple calls with the same backend name will append to the pool for round-robin.
// G4: Duplicate addresses for the same backend are silently ignored.
func (p *Proxy) RegisterTarget(backend, addr string) {
        u, err := url.Parse(fmt.Sprintf("http://%s", addr))
        if err != nil {
                logger.Warnf("proxy: invalid backend address %s: %v", addr, err)
                return
        }

        p.mu.Lock()
        defer p.mu.Unlock()

        pool, ok := p.backends[backend]
        if !ok {
                pool = &backendPool{seen: make(map[string]bool)}
                p.backends[backend] = pool
        }
        // G4: deduplicate — avoid adding the same address multiple times
        if pool.seen[addr] {
                return
        }
        pool.seen[addr] = true
        pool.targets = append(pool.targets, u)
        logger.Infof("proxy: registered backend [%s] → %s (pool size: %d)", backend, addr, len(pool.targets))
}

// UpdateTargets atomically replaces all targets for a backend.
// Used by etcd watcher to refresh the target pool when nodes go up/down.
func (p *Proxy) UpdateTargets(backend string, addrs []string) {
        p.mu.Lock()
        defer p.mu.Unlock()

        var urls []*url.URL
        for _, addr := range addrs {
                u, err := url.Parse(fmt.Sprintf("http://%s", addr))
                if err != nil {
                        logger.Warnf("proxy: invalid address %s for backend %s: %v", addr, backend, err)
                        continue
                }
                urls = append(urls, u)
        }

        p.backends[backend] = &backendPool{targets: urls, seen: make(map[string]bool, len(urls))}
        // Populate seen set for UpdateTargets dedup tracking
        for _, addr := range addrs {
                p.backends[backend].seen[addr] = true
        }
        // Ensure circuit breaker exists for this backend
        if _, ok := p.circuits[backend]; !ok {
                p.circuits[backend] = newBackendCircuit(10, 30*time.Second)
        }
        logger.Infof("proxy: updated backend [%s] → %d targets: %v", backend, len(urls), addrs)
}

// ResolveRoute finds the matching route config for a request path.
// Uses read-lock to safely access the route table during dynamic updates.
// Returns nil if no route matches.
func (p *Proxy) ResolveRoute(path string) *RouteConfig {
        p.routeMu.RLock()
        defer p.routeMu.RUnlock()
        for i := range p.routes {
                if strings.HasPrefix(path, p.routes[i].Prefix) {
                        return &p.routes[i]
                }
        }
        return nil
}

// Handler returns a Fiber handler that routes and reverse-proxies requests.
// Flow: match route → optional JWT auth check → round-robin pick target → forward
// jwtSecrets is the full key ring for JWT verification (supports key rotation).
// hmacSecret, when set, signs forwarded requests with HMAC-SHA256 so backend
// services can verify the request originated from the Gate proxy.
func (p *Proxy) Handler(jwtSecrets []string, hmacSecret ...string) fiber.Handler {
        hmacKey := ""
        if len(hmacSecret) > 0 {
                hmacKey = hmacSecret[0]
        }
        return func(c *fiber.Ctx) error {
                path := c.Path()
                route := p.ResolveRoute(path)
                if route == nil {
                        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
                                "code":    40401,
                                "message": "no route found",
                        })
                }

                // JWT auth check if this route requires it
                // Use auth.ParseAccessToken directly instead of middleware to avoid
                // c.Next() causing issues outside the Fiber middleware chain
                if route.Auth && len(jwtSecrets) > 0 {
                        authHeader := c.Get("Authorization")
                        claims, err := auth.ParseAccessToken(authHeader, jwtSecrets)
                        if err != nil {
                                if auth.IsTokenExpiredError(err) {
                                        logger.Warnf("[AUTH] token expired: request_id=%s ip=%s path=%s",
                                                getRequestID(c), c.IP(), path)
                                        return bizerr.ErrTokenExpired
                                }
                                logger.Warnf("[AUTH] token invalid: request_id=%s ip=%s path=%s err=%v",
                                        getRequestID(c), c.IP(), path, err)
                                return bizerr.ErrInvalidToken
                        }
                        // Store user info in context for forwarding
                        c.Locals("user_id", claims.UserID)
                        c.Locals("device_id", claims.DeviceID)
                }

                // Resolve backend target (round-robin)
                p.mu.RLock()
                pool, ok := p.backends[route.Backend]
                p.mu.RUnlock()

                if !ok || len(pool.targets) == 0 {
                        logger.Errorf("[PROXY] backend unavailable: request_id=%s backend=%s path=%s ip=%s",
                                getRequestID(c), route.Backend, path, c.IP())
                        return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
                                "code":    50201,
                                "message": fmt.Sprintf("backend %s unavailable", route.Backend),
                        })
                }

                // Circuit breaker check
                circuit := p.getCircuit(route.Backend)
                if circuit != nil && !circuit.allowRequest() {
                        logger.Warnf("[PROXY] circuit open: request_id=%s backend=%s path=%s",
                                getRequestID(c), route.Backend, path)
                        return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
                                "code":    50301,
                                "message": fmt.Sprintf("backend %s circuit open", route.Backend),
                        })
                }

                target := pool.next()

                // Build target URL, optionally stripping prefix
                targetPath := path
                if route.StripPrefix > 0 {
                        segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
                        if len(segments) > route.StripPrefix {
                                targetPath = "/" + strings.Join(segments[route.StripPrefix:], "/")
                        }
                }

                targetURL := *target
                targetURL.Path = targetPath
                if len(c.Request().URI().QueryString()) > 0 {
                        targetURL.RawQuery = string(c.Request().URI().QueryString())
                }

                // Build HTTP request from Fiber context
                forwardCtx, cancelCtx := context.WithTimeout(context.Background(), 30*time.Second)
                defer cancelCtx()

                // Use nil body for GET/HEAD/DELETE to avoid Content-Length: 0 header
                var reqBody io.Reader
                if body := c.Body(); len(body) > 0 {
                        reqBody = bytes.NewReader(body)
                }

                forwardReq, err := http.NewRequestWithContext(
                        forwardCtx,
                        c.Method(),
                        targetURL.String(),
                        reqBody,
                )
                if err != nil {
                        logger.Errorf("[PROXY] create request failed: request_id=%s backend=%s target=%s err=%v",
                                        getRequestID(c), route.Backend, target.Host, err)
                        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                                "code":    50001,
                                "message": "failed to create forward request",
                        })
                }

                // G5: hop-by-hop and dangerous headers (package-level, avoids per-request allocation).
                // Defined below in var hopByHopHeaders.
                c.Request().Header.VisitAll(func(key, value []byte) {
                        headerKey := string(key)
                        if hopByHopHeaders[headerKey] {
                                return
                        }
                        forwardReq.Header.Add(headerKey, string(value))
                })

                // Set proxy-controlled headers (override any client-supplied values)
                forwardReq.Header.Set("X-Forwarded-For", c.IP())
                forwardReq.Header.Set("X-Forwarded-Host", c.Hostname())
                forwardReq.Header.Set("X-Forwarded-Proto", c.Protocol())
                forwardReq.Header.Set("X-Real-Ip", c.IP())
                if reqID, ok := c.Locals("request_id").(string); ok {
                        forwardReq.Header.Set("X-Request-ID", reqID)
                }
                // Pass authenticated user info to backend nodes
                if uid, ok := c.Locals("user_id").(int64); ok {
                        forwardReq.Header.Set("X-User-ID", fmt.Sprintf("%d", uid))
                }
                if did, ok := c.Locals("device_id").(string); ok {
                        forwardReq.Header.Set("X-Device-ID", did)
                }

                // Sign request with HMAC if secret is configured (service-to-service auth)
                // This allows backend services to verify the request originated from Gate.
                if hmacKey != "" {
                        ts := time.Now().Format(time.RFC3339)
                        nonce := fmt.Sprintf("%d", time.Now().UnixNano())
                        bodyStr := string(c.Body())
                        sig := generateHMAC(hmacKey, ts, nonce, bodyStr)
                        forwardReq.Header.Set("X-Timestamp", ts)
                        forwardReq.Header.Set("X-Nonce", nonce)
                        forwardReq.Header.Set("X-Signature", sig)
                }

                // Forward to backend
                resp, err := p.transport.RoundTrip(forwardReq)
                logger.Infof("[PROXY] forward: request_id=%s backend=%s target=%s path=%s method=%s status=%d err=%v",
                        getRequestID(c), route.Backend, targetURL.Host, targetURL.Path, forwardReq.Method, func() int { if resp != nil { return resp.StatusCode }; return 0 }(), err)
                if err != nil {
                        logger.Errorf("[PROXY] forward failed: request_id=%s backend=%s target=%s path=%s err=%v",
                                        getRequestID(c), route.Backend, target.Host, targetURL.Path, err)
                        if circuit != nil {
                                circuit.recordFailure()
                        }
                        return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
                                "code":    50202,
                                "message": "backend request failed",
                        })
                }
                defer resp.Body.Close()

                // Copy response headers (filter out hop-by-hop and internal headers)
                for key, vals := range resp.Header {
                        if responseHopByHopHeaders[key] {
                                continue
                        }
                        for _, v := range vals {
                                c.Set(key, v)
                        }
                }
                c.Status(resp.StatusCode)

                // Copy response body (limit to 10MB to prevent OOM)
                body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
                if err != nil {
                        logger.Errorf("[PROXY] read response body failed: request_id=%s backend=%s target=%s err=%v",
                                        getRequestID(c), route.Backend, target.Host, err)
                        if circuit != nil {
                                circuit.recordFailure()
                        }
                        return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
                                "code":    50203,
                                "message": "failed to read backend response",
                        })
                }

                // Record success/failure in circuit breaker (5xx = failure)
                if circuit != nil {
                        if resp.StatusCode >= 500 {
                                circuit.recordFailure()
                        } else {
                                circuit.recordSuccess()
                        }
                }

                return c.Send(body)
        }
}

// getCircuit returns the circuit breaker for a backend (thread-safe).
func (p *Proxy) getCircuit(backend string) *backendCircuit {
        p.mu.RLock()
        defer p.mu.RUnlock()
        return p.circuits[backend]
}

// Routes returns the configured route table (for logging/health check, thread-safe).
func (p *Proxy) Routes() []RouteConfig {
        return p.GetRoutes()
}

// BackendInfo returns backend pool info for health check (thread-safe).
func (p *Proxy) BackendInfo() map[string]int {
        p.mu.RLock()
        defer p.mu.RUnlock()
        info := make(map[string]int, len(p.backends))
        for name, pool := range p.backends {
                info[name] = len(pool.targets)
        }
        return info
}

// hopByHopHeaders are headers that should not be forwarded from client to backend.
// G5: package-level to avoid re-creating on every request.
var hopByHopHeaders = map[string]bool{
        "Host":             true,
        "X-Forwarded-Host": true,
        "X-Forwarded-By":   true,
        "X-Real-Ip":        true,
        "X-Original-Url":   true,
        "X-Rewrite-Url":    true,
}

// responseHopByHopHeaders are headers that should not be forwarded from backend to client.
// These are hop-by-hop headers defined in RFC 2616 Section 13.5.1 plus other
// headers that should not be passed through a proxy to the end client.
var responseHopByHopHeaders = map[string]bool{
        "Connection":          true,
        "Keep-Alive":          true,
        "Proxy-Authenticate":  true,
        "Proxy-Authorization": true,
        "Te":                  true,
        "Trailers":            true,
        "Transfer-Encoding":   true,
        "Upgrade":             true,
        "Server":              true, // hide internal backend server identity
}

// getRequestID extracts request_id from Fiber context for logging.
func getRequestID(c *fiber.Ctx) string {
        if v := c.Locals("request_id"); v != nil {
                if s, ok := v.(string); ok {
                        return s
                }
        }
        return ""
}

// generateHMAC creates an HMAC-SHA256 signature for service-to-service authentication.
// Kept as a private helper in the proxy package to avoid circular imports.
func generateHMAC(secret, timestamp, nonce, body string) string {
        payload := fmt.Sprintf("%s|%s|%s", timestamp, nonce, body)
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write([]byte(payload))
        return hex.EncodeToString(mac.Sum(nil))
}
