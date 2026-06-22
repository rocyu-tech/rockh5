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
        "github.com/rocyu-tech/rockgame/pkg/snowflake"
)

const serviceName = "shop"

// Build-time variables (set via -ldflags)
var (
        BuildTime string
        GitCommit string
)

func main() {
        configPath := flag.String("config", "etc/dev/config.yaml", "config file path")
        nodeID := flag.Int("node", 0, "node ID")
        port := flag.Int("port", 0, "service port (0 = use config services.ports + nodeID offset)")
        flag.Parse()

        cfg := config.MustLoad(*configPath)

        // Resolve actual port: CLI flag > config + nodeID offset
        etcdSvcName := serviceName + "-mesh"
        actualPort := *port
        if actualPort == 0 {
                actualPort = cfg.Port(etcdSvcName) + *nodeID
        }
        if actualPort == 0 {
                logger.Fatalf("%s: port not configured, specify -port flag or set services.ports.%s in config", serviceName, etcdSvcName)
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
                ServiceName: serviceName,
                NodeID:      *nodeID,
        })
        defer logger.Sync()

        // Initialize database
        if err := database.Init(&cfg.Database); err != nil {
                logger.Fatalf("%s: failed to connect database: %v", serviceName, err)
        }
        logger.Infof("%s: database connected [%s:%d/%s]", serviceName, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

        // Initialize snowflake ID generator
        snowflake.Init(int64(*nodeID))

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
                        Name:   etcdSvcName,
                        ID:     registry.ServiceID(etcdSvcName, *nodeID),
                        Addr:   svcAddr,
                        Port:   actualPort,
                        NodeID: *nodeID,
                }
                if err := reg.Register(inst); err != nil {
                        logger.Warnf("%s: etcd register failed: %v", serviceName, err)
                }
        }


        app := fiber.New(fiber.Config{
                ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
                WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
                BodyLimit:    4 * 1024 * 1024, // 4MB max request body
                ErrorHandler: defaultErrorHandler,
        })

        app.Use(middleware.RecoveryMiddleware())
        app.Use(middleware.RequestIDMiddleware())
        app.Use(compress.New())

        registerRoutes(app)

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
                                        reg.Deregister(etcdSvcName, registry.ServiceID(etcdSvcName, *nodeID))
                                        reg.Close()
                                }
                        },
                },
                AfterServer: buildAfterServerCallbacks(),
        })
}

func registerRoutes(app *fiber.App) {
        api := app.Group("/api/v1/" + serviceName)

        api.Get("/health", func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(map[string]interface{}{
                        "status":  "ok",
                        "service": serviceName,
                }))
        })

        authenticated := api.Group("")
        authenticated.Use(middleware.InternalAuthMiddleware(config.C().Gate.HMACSecret))

        // Wallet
        authenticated.Get("/wallet", handler.GetShopWallet)

        // Payment channels
        authenticated.Get("/payment-channels", handler.GetPaymentChannels)
        authenticated.Get("/withdraw-channels", handler.GetWithdrawChannels)

        // Orders
        authenticated.Post("/recharge", handler.CreateRecharge)
        authenticated.Post("/withdraw", handler.CreateWithdraw)
        authenticated.Get("/orders", handler.GetOrders)

        // Payment accounts
        authenticated.Get("/payment-accounts", handler.GetPaymentAccounts)
        authenticated.Post("/payment-accounts", handler.SavePaymentAccount)

        // Withdraw password
        authenticated.Post("/withdraw-password", handler.SetWithdrawPassword)
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
        code := fiber.StatusInternalServerError
        if e, ok := err.(*fiber.Error); ok {
                code = e.Code
        }
        return c.Status(code).JSON(bizerr.ErrorResponse(err))
}

func buildAfterServerCallbacks() []func() {
        return []func(){
                func() {
                        sqlDB, _ := database.DB().DB()
                        if sqlDB != nil {
                                sqlDB.Close()
                        }
                },
        }
}
