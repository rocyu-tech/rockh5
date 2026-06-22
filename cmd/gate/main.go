package main

import (
        "flag"
        "os"
        "sync"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/gofiber/fiber/v2/middleware/compress"
        "github.com/rocyu-tech/rockgame/internal/config"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "github.com/rocyu-tech/rockgame/pkg/proxy"
        "github.com/rocyu-tech/rockgame/pkg/registry"
        "github.com/rocyu-tech/rockgame/pkg/server"
)

// Build-time variables (set via -ldflags)
var (
        BuildTime string
        GitCommit string
)

func main() {
        configPath := flag.String("config", "etc/dev/config.yaml", "config file path")
        nodeID := flag.Int("node", 0, "gate node instance ID (for etcd ServiceID)")
        port := flag.Int("port", 0, "service port (0 = use config services.ports + nodeID offset)")
        flag.Parse()

        cfg := config.MustLoad(*configPath)

        // Resolve actual port: CLI flag > config + nodeID offset
        actualPort := *port
        if actualPort == 0 {
                actualPort = cfg.Port("gate") + *nodeID
        }
        if actualPort == 0 {
                logger.Fatalf("gate: port not configured, specify -port flag or set services.ports.gate in config")
        }

        logger.InitFromConfig(&logger.LogConfig{
                Level:      cfg.Log.Level,
                Format:     cfg.Log.Format,
                Output:     cfg.Log.Output,
                File:       cfg.Log.File,
                MaxSizeMB:  cfg.Log.MaxSizeMB,
                MaxBackups: cfg.Log.MaxBackups,
                MaxAgeDays: cfg.Log.MaxAgeDays,
                Compress:    cfg.Log.Compress,
                ServiceName: "gate",
                NodeID:      *nodeID,
        })
        defer logger.Sync()

        logger.Infof("Starting Gate [env=%s node=%d] version=%s built=%s", cfg.App.Env, *nodeID, GitCommit, BuildTime)

        // Convert YAML fallback routes to proxy.RouteConfig
        fallbackRoutes := configToProxyRoutes(cfg.Gate.Routes)

        // Build proxy with YAML fallback routes (will be overwritten by etcd if available)
        p := proxy.New(fallbackRoutes)

        // Connect to etcd for service discovery + dynamic routing
        reg, err := registry.NewEtcdRegistry(cfg.Etcd.Addrs)
        if err != nil {
                logger.Fatalf("Gate: failed to connect etcd: %v", err)
        }
        defer reg.Close()

        // ── Phase 1: Load routes from etcd (or keep YAML fallback) ──
        if routes, err := reg.GetRoutes(); err != nil {
                logger.Warnf("Gate: failed to load routes from etcd: %v (using YAML fallback)", err)
        } else if len(routes) > 0 {
                p.UpdateRoutes(etcdToProxyRoutes(routes))
                logger.Infof("Gate: loaded %d routes from etcd (YAML fallback skipped)", len(routes))
        } else {
                logger.Infof("Gate: no routes in etcd, using %d YAML fallback routes", len(fallbackRoutes))
        }

        // ── Phase 2: Derive backend services from route table (no hardcoded list) ──
        backends := p.UniqueBackends()
        logger.Infof("Gate: discovered %d unique backends from routes: %v", len(backends), backends)

        // ── Phase 3: Initial service discovery for all backends ──
        var wg sync.WaitGroup
        for _, svcName := range backends {
                wg.Add(1)
                go func(name string) {
                        defer wg.Done()
                        discoverAndUpdate(p, reg, name)
                }(svcName)
        }
        wg.Wait()

        // ── Phase 4: Watch backend service changes (node up/down) ──
        stopCh := make(chan struct{})
        backendWatchers := make(map[string]struct{})
        for _, svcName := range backends {
                backendWatchers[svcName] = struct{}{}
                go func(name string) {
                        reg.WatchServices(name, func(instances []registry.ServiceInstance) {
                                updateProxyTargets(p, name, instances)
                        }, stopCh)
                }(svcName)
        }

        // ── Phase 5: Watch route changes (dynamic routing hot-reload) ──
        go reg.WatchRoutes(func(entries []registry.RouteEntry) {
                if len(entries) == 0 {
                        logger.Warn("Gate: all routes removed from etcd, keeping current routes")
                        return
                }
                p.UpdateRoutes(etcdToProxyRoutes(entries))

                // Detect new backends added by route changes and start watching them
                newBackends := p.UniqueBackends()
                for _, name := range newBackends {
                        if _, exists := backendWatchers[name]; !exists {
                                backendWatchers[name] = struct{}{}
                                logger.Infof("Gate: new backend [%s] detected from route update, starting discovery + watch", name)
                                go func(n string) {
                                        discoverAndUpdate(p, reg, n)
                                        reg.WatchServices(n, func(instances []registry.ServiceInstance) {
                                                updateProxyTargets(p, n, instances)
                                        }, stopCh)
                                }(name)
                        }
                }
        }, stopCh)

        // Create Fiber app
        app := fiber.New(fiber.Config{
                ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
                WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
                BodyLimit:         4 * 1024 * 1024, // 4MB max request body
                ErrorHandler:      errorHandler,
                DisableStartupMessage: true,
        })

        // Global middleware
        app.Use(middleware.RecoveryMiddleware())
        app.Use(middleware.RequestIDMiddleware())
        app.Use(middleware.AccessLogMiddleware())
        app.Use(compress.New())
        // CORS: allow configuring via CORS_ORIGINS env var (comma-separated).
        // Default to "*" for backward compatibility in dev; production should set this.
        corsOrigins := os.Getenv("CORS_ORIGINS")
        if corsOrigins == "" {
                if cfg.App.Env == "prod" {
                        corsOrigins = "" // Must be explicitly set in production
                } else {
                        corsOrigins = "*"
                }
        }
        if corsOrigins != "" {
                app.Use(middleware.CORSMiddleware(corsOrigins))
                // CSRF origin validation (defense-in-depth alongside SameSite=Lax cookies)
                app.Use(middleware.CSRFMiddleware(corsOrigins))
        }

        // Health check (Gate's own endpoint, not proxied)
        app.Get("/health", func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(fiber.Map{
                        "status":   "ok",
                        "service":  "gate",
                        "nodeID":   *nodeID,
                        "routes":   len(p.Routes()),
                        "backends": p.BackendInfo(),
                        "source":   routeSource(p.Routes(), fallbackRoutes),
                }))
        })

        // All /api/* requests → proxy handler (routing + forwarding)
        app.All("/api/*", p.Handler(cfg.JWT.ActiveSecrets(), cfg.Gate.HMACSecret))

        // Start server
        go func() {
                addr := server.ResolveAddr(cfg.Server.Addr, actualPort)
                logger.Infof("Gate [node=%d] listening on %s, %d routes, etcd=%v",
                        *nodeID, addr, len(p.Routes()), cfg.Etcd.Addrs)
                if err := app.Listen(addr); err != nil {
                        logger.Fatalf("Gate failed: %v", err)
                }
        }()

        // Graceful shutdown
        _ = server.WaitForSignal()

        server.GracefulShutdown("gate", app, 0, server.ShutdownCallbacks{
                BeforeServer: []func() {
                        func() {
                                close(stopCh) // stop all etcd watchers
                        },
                },
        })
}

// configToProxyRoutes converts config RouteItem to proxy RouteConfig.
func configToProxyRoutes(items []config.RouteItem) []proxy.RouteConfig {
        routes := make([]proxy.RouteConfig, len(items))
        for i, item := range items {
                routes[i] = proxy.RouteConfig{
                        Prefix:  item.Prefix,
                        Backend: item.Backend,
                        Auth:    item.Auth,
                }
        }
        return routes
}

// etcdToProxyRoutes converts etcd RouteEntry to proxy RouteConfig.
func etcdToProxyRoutes(entries []registry.RouteEntry) []proxy.RouteConfig {
        routes := make([]proxy.RouteConfig, len(entries))
        for i, entry := range entries {
                routes[i] = proxy.RouteConfig{
                        Prefix:  entry.Prefix,
                        Backend: entry.Backend,
                        Auth:    entry.Auth,
                }
        }
        return routes
}

// routeSource checks if routes match YAML fallback (returns "yaml") or etcd ("etcd").
func routeSource(current, fallback []proxy.RouteConfig) string {
        if len(current) != len(fallback) {
                return "etcd"
        }
        for i := range current {
                if current[i].Prefix != fallback[i].Prefix ||
                        current[i].Backend != fallback[i].Backend ||
                        current[i].Auth != fallback[i].Auth {
                        return "etcd"
                }
        }
        return "yaml"
}

// discoverAndUpdate performs initial service discovery and updates proxy targets.
func discoverAndUpdate(p *proxy.Proxy, reg *registry.EtcdRegistry, svcName string) {
        instances, err := reg.DiscoverServices(svcName)
        if err != nil {
                logger.Warnf("Gate: initial discovery of %s failed: %v (will watch for changes)", svcName, err)
                return
        }
        updateProxyTargets(p, svcName, instances)
}

// updateProxyTargets refreshes the proxy target pool for a backend service.
func updateProxyTargets(p *proxy.Proxy, svcName string, instances []registry.ServiceInstance) {
        if len(instances) == 0 {
                p.UpdateTargets(svcName, nil)
                logger.Warnf("Gate: backend [%s] has no instances — cleared target pool", svcName)
                return
        }
        addrs := make([]string, len(instances))
        for i, inst := range instances {
                addrs[i] = inst.Addr
        }
        p.UpdateTargets(svcName, addrs)
        logger.Infof("Gate: updated backend [%s] targets: %v", svcName, addrs)
}

func errorHandler(c *fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError
        if bizE, ok := err.(*bizerr.BizError); ok {
                code = bizE.HTTP
        } else if e, ok := err.(*fiber.Error); ok {
                code = e.Code
        }
        // Log all errors with request context for debugging
        // 4xx = client errors (WARN level), 5xx = server errors (ERROR level)
        if code >= 500 {
                logger.Errorf("[GATE_ERROR] request_id=%s ip=%s method=%s path=%s status=%d err=%v",
                        middleware.GetRequestID(c), c.IP(), c.Method(), c.Path(), code, err)
        } else if code >= 400 {
                logger.Warnf("[GATE_WARN] request_id=%s ip=%s method=%s path=%s status=%d err=%v",
                        middleware.GetRequestID(c), c.IP(), c.Method(), c.Path(), code, err)
        }
        return c.Status(code).JSON(bizerr.ErrorResponse(err))
}
