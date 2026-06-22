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
        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/database"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "github.com/rocyu-tech/rockgame/pkg/registry"
        "github.com/rocyu-tech/rockgame/pkg/server"
)

const serviceName = "vip"

var (
        BuildTime string
        GitCommit string
)

func main() {
        configPath := flag.String("config", "etc/dev/config.yaml", "config file path")
        nodeID := flag.Int("node", 0, "node ID for etcd ServiceID")
        port := flag.Int("port", 0, "service port (0 = use config services.ports + nodeID offset)")
        flag.Parse()

        cfg := config.MustLoad(*configPath)

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

        logger.Infof("Starting %s [env=%s node=%d] version=%s built=%s", serviceName, cfg.App.Env, *nodeID, GitCommit, BuildTime)

        // Initialize database (required for this service)
        if err := database.Init(&cfg.Database); err != nil {
                logger.Fatalf("%s: database init failed: %v", serviceName, err)
        }

        // Initialize Redis (graceful failure)
        if err := cache.Init(&cfg.Redis); err != nil {
                logger.Warnf("%s: redis init failed: %v (service still starts)", serviceName, err)
        }

        // Register with etcd
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

        registerRoutes(app, cfg)

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

func buildAfterServerCallbacks() []func() {
        return []func() {
                func() {
                        cache.Close()
                },
                func() {
                        sqlDB, _ := database.DB().DB()
                        if sqlDB != nil {
                                sqlDB.Close()
                        }
                },
        }
}

func registerRoutes(app *fiber.App, cfg *config.Config) {
        api := app.Group("/api/v1/" + serviceName)

        api.Get("/health", func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(map[string]interface{}{
                        "status":     "ok",
                        "service":    serviceName,
                        "build_time": BuildTime,
                        "git_commit": GitCommit,
                }))
        })

        authenticated := api.Group("")
        authenticated.Use(middleware.InternalAuthMiddleware(config.C().Gate.HMACSecret))

        authenticated.Get("/levels", handler.VipLevels)
        authenticated.Get("/info", handler.VipInfo)
        authenticated.Get("/benefits", placeholder("benefits"))
        authenticated.Post("/upgrade", placeholder("upgrade"))
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
                return c.Status(fiber.StatusNotImplemented).JSON(bizerr.ErrorResponse(fiber.NewError(
                        fiber.StatusNotImplemented, action+": not implemented yet",
                )))
        }
}