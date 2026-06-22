package main

import (
        "flag"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/gofiber/fiber/v2/middleware/compress"
        "github.com/rocyu-tech/rockgame/internal/config"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/handler"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "github.com/rocyu-tech/rockgame/pkg/registry"
        "github.com/rocyu-tech/rockgame/pkg/server"
)

const serviceName = "game-node"

// Build-time variables (set via -ldflags)
var (
        BuildTime string
        GitCommit string
)

func main() {
        configPath := flag.String("config", "etc/dev/config.yaml", "config file path")
        nodeID := flag.Int("node", 0, "node ID for snowflake ID generation and etcd ServiceID")
        port := flag.Int("port", 0, "service port (0 = use config services.ports + nodeID offset)")
        flag.Parse()

        // Load configuration
        cfg := config.MustLoad(*configPath)

        // Resolve actual port: CLI flag > config + nodeID offset
        actualPort := *port
        if actualPort == 0 {
                actualPort = cfg.Port(serviceName) + *nodeID
        }
        if actualPort == 0 {
                logger.Fatalf("%s: port not configured, specify -port flag or set services.ports.%s in config", serviceName, serviceName)
        }

        // Initialize logger
        logger.InitFromConfig(&logger.LogConfig{
                Level:      cfg.Log.Level,
                Format:     cfg.Log.Format,
                Output:     cfg.Log.Output,
                File:       cfg.Log.File,
                MaxSizeMB:  cfg.Log.MaxSizeMB,
                MaxBackups: cfg.Log.MaxBackups,
                MaxAgeDays: cfg.Log.MaxAgeDays,
                Compress:    cfg.Log.Compress,
                ServiceName: serviceName,
                NodeID:      *nodeID,
        })
        defer logger.Sync()

        // Initialize database
        if err := database.Init(&cfg.Database); err != nil {
                logger.Fatalf("%s: failed to connect database: %v", serviceName, err)
        }
        logger.Infof("%s: database connected [%s:%d/%s]", serviceName, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

        logger.Infof("Starting %s [env=%s node=%d] version=%s built=%s", serviceName, cfg.App.Env, *nodeID, GitCommit, BuildTime)

        // Register with etcd for service discovery
        var reg *registry.EtcdRegistry
        if regTmp, err := registry.NewEtcdRegistry(cfg.Etcd.Addrs); err != nil {
                logger.Warnf("%s: etcd connect failed: %v (service still starts)", serviceName, err)
        } else {
                reg = regTmp
                hostIP := registry.ExtractHostIP(cfg.Server.Addr)
                svcAddr := registry.BuildAddr(hostIP, actualPort)
                inst := registry.ServiceInstance{
                        Name:   serviceName,
                        ID:     registry.ServiceID(serviceName, *nodeID),
                        Addr:   svcAddr,
                        Port:   actualPort,
                        NodeID: *nodeID,
                }
                if err := reg.Register(inst); err != nil {
                        logger.Warnf("%s: etcd register failed: %v", serviceName, err)
                }
        }

        // Create Fiber app
        app := fiber.New(fiber.Config{
                ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
                WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
                BodyLimit:    4 * 1024 * 1024, // 4MB max request body
                ErrorHandler: defaultErrorHandler,
        })

        // Global middleware
        app.Use(middleware.RecoveryMiddleware())
        app.Use(middleware.RequestIDMiddleware())
        app.Use(compress.New())

        // Register game routes
        registerGameRoutes(app, cfg)

        // Start server
        go func() {
                addr := server.ResolveAddr(cfg.Server.Addr, actualPort)
                logger.Infof("%s listening on %s", serviceName, addr)
                if err := app.Listen(addr); err != nil {
                        logger.Fatalf("%s failed: %v", serviceName, err)
                }
        }()

        // Block until SIGINT/SIGTERM (SIGHUP triggers config reload automatically)
        _ = server.WaitForSignal()

        server.GracefulShutdown(serviceName, app, 0, server.ShutdownCallbacks{
                BeforeServer: []func() {
                        func() {
                                if reg != nil {
                                        reg.Deregister(serviceName, registry.ServiceID(serviceName, *nodeID))
                                        reg.Close()
                                }
                        },
                },
                AfterServer: buildAfterServerCallbacks(),
        })
}

func buildAfterServerCallbacks() []func() {
        return []func() {
                func() {
                        sqlDB, _ := database.DB().DB()
                        if sqlDB != nil {
                                sqlDB.Close()
                        }
                },
        }
}

func registerGameRoutes(app *fiber.App, cfg *config.Config) {
        api := app.Group("/api/v1/game")

        api.Get("/health", func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(map[string]string{"status": "ok", "service": "game-node"}))
        })

        // Public game routes (no auth required)
        api.Get("/vendors", handler.GetGameVendors)
        api.Get("/launch/:id", handler.LaunchGame)

        // Wallet API — requires HMAC verification from Gate (service-to-service auth).
        // Registered under /api/v1/wallet to match gate route config.
        // The gate validates JWT + signs with HMAC; this middleware verifies the HMAC.
        walletRoot := app.Group("/api/v1/wallet")
        walletRoot.Use(middleware.InternalAuthMiddleware(cfg.Gate.HMACSecret))
        walletRoot.Post("/balance", placeholder("getBalance"))
        walletRoot.Post("/bet", placeholder("processBet"))
        walletRoot.Post("/settle", placeholder("processSettle"))
        walletRoot.Post("/cancel", placeholder("processCancel"))

        // Wallet API under /api/v1/game/wallet (same HMAC protection)
        wallet := api.Group("/wallet")
        wallet.Use(middleware.InternalAuthMiddleware(cfg.Gate.HMACSecret))
        wallet.Post("/balance", placeholder("getBalance"))
        wallet.Post("/bet", placeholder("processBet"))
        wallet.Post("/settle", placeholder("processSettle"))
        wallet.Post("/cancel", placeholder("processCancel"))

        // Game management (authenticated via JWT)
        game := api.Group("/manage")
        game.Use(middleware.AuthMiddleware(cfg.JWT.ActiveSecrets()))
        game.Get("/list", placeholder("getGameList"))
        game.Get("/search", placeholder("searchGames"))
        game.Post("/favorite", placeholder("toggleFavorite"))
        game.Get("/recent", placeholder("recentGames"))
        game.Post("/launch", placeholder("launchGame"))
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError
        if e, ok := err.(*fiber.Error); ok {
                code = e.Code
        }
        return c.Status(code).JSON(bizerr.ErrorResponse(err))
}

func placeholder(action string) fiber.Handler {
        return func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(map[string]string{
                        "action":  action,
                        "message": "not implemented yet",
                }))
        }
}