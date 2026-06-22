package main

import (
        "flag"
        "time"

        "github.com/gofiber/fiber/v2"
        "github.com/rocyu-tech/rockgame/pkg/server"
        "github.com/gofiber/fiber/v2/middleware/compress"
        "github.com/rocyu-tech/rockgame/internal/config"
        bizerr "github.com/rocyu-tech/rockgame/internal/errors"
        "github.com/rocyu-tech/rockgame/internal/middleware"
        "github.com/rocyu-tech/rockgame/pkg/logger"
        "github.com/rocyu-tech/rockgame/pkg/registry"
)

const serviceName = "agent"

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

func buildAfterServerCallbacks() []func() {
        return nil
}

func registerRoutes(app *fiber.App) {
        api := app.Group("/api/v1/" + serviceName)

        api.Get("/health", func(c *fiber.Ctx) error {
                return c.JSON(bizerr.SuccessResponse(fiber.Map{"status": "ok", "service": serviceName}))
        })

        authenticated := api.Group("")
        authenticated.Use(middleware.InternalAuthMiddleware(config.C().Gate.HMACSecret))

        authenticated.Get("/info", placeholder("info"))
        authenticated.Get("/subordinates", placeholder("subordinates"))
        authenticated.Get("/commissions", placeholder("commissions"))
        authenticated.Post("/settlement", placeholder("settlement"))
        authenticated.Post("/promo-link", placeholder("promo-link"))
        authenticated.Get("/dashboard", placeholder("dashboard"))
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
