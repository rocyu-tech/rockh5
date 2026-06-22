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

	// Initialize database
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatalf("%s: database init failed: %v", serviceName, err)
	}
	logger.Infof("%s: database connected", serviceName)

	// Initialize Redis (optional — degrades gracefully)
	if err := cache.Init(&cfg.Redis); err != nil {
		logger.Warnf("%s: redis init failed: %v (continuing without cache)", serviceName, err)
	} else {
		logger.Infof("%s: redis connected", serviceName)
	}

	// Resolve actual port
	etcdSvcName := serviceName + "-mesh"
	actualPort := *port
	if actualPort == 0 {
		actualPort = cfg.Port(etcdSvcName) + *nodeID
	}
	if actualPort == 0 {
		logger.Fatalf("%s: port not configured", serviceName)
	}
	logger.InitFromConfig(&logger.LogConfig{
		Level:       cfg.Log.Level,
		Format:      cfg.Log.Format,
		Output:      cfg.Log.Output,
		File:        cfg.Log.File,
		MaxSizeMB:   cfg.Log.MaxSizeMB,
		MaxBackups:  cfg.Log.MaxBackups,
		MaxAgeDays:  cfg.Log.MaxAgeDays,
		Compress:    cfg.Log.Compress,
		ServiceName: serviceName,
		NodeID:      *nodeID,
	})
	defer logger.Sync()

	logger.Infof("Starting %s [env=%s node=%d] version=%s built=%s", serviceName, cfg.App.Env, *nodeID, GitCommit, BuildTime)

	// Register with etcd
	var reg *registry.EtcdRegistry
	if regTmp, err := registry.NewEtcdRegistry(cfg.Etcd.Addrs); err != nil {
		logger.Warnf("%s: etcd connect failed: %v", serviceName, err)
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
		BodyLimit:    4 * 1024 * 1024,
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

	_ = server.WaitForSignal()

	server.GracefulShutdown(serviceName, app, 0, server.ShutdownCallbacks{
		BeforeServer: []func(){
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
	return []func(){
		func() {
			logger.Infof("%s: closing database connection", serviceName)
			sqlDB, _ := database.DB().DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
		},
		func() {
			if r := cache.Client(); r != nil {
				r.Close()
			}
		},
	}
}

func registerRoutes(app *fiber.App) {
	api := app.Group("/api/v1/" + serviceName)

	// Health (public)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(bizerr.SuccessResponse(map[string]interface{}{
			"status":  "ok",
			"service": serviceName,
		}))
	})

	// Authenticated routes (HMAC from Gate)
	authenticated := api.Group("")
	authenticated.Use(middleware.InternalAuthMiddleware(config.C().Gate.HMACSecret))

	// ── Wallet ──
	authenticated.Get("/wallet", handler.WalletInfo)

	// ── Channels ──
	authenticated.Get("/payment-channels", handler.PaymentChannels)
	authenticated.Get("/withdraw-channels", handler.WithdrawChannels)

	// ── Recharge (Deposit) ──
	authenticated.Post("/recharge", handler.CreateRecharge)
	authenticated.Post("/recharge/complete", handler.CompleteRecharge)

	// ── Withdraw ──
	authenticated.Post("/withdraw", handler.CreateWithdraw)
	authenticated.Post("/withdraw/complete", handler.CompleteWithdraw)

	// ── Orders ──
	authenticated.Get("/orders", handler.GetOrders)

	// ── Payment Accounts ──
	authenticated.Get("/payment-accounts", handler.GetUserPaymentAccounts)
	authenticated.Post("/payment-accounts", handler.SetUserPaymentAccount)

	// ── Withdraw Password ──
	authenticated.Post("/withdraw-password", handler.SetWithdrawPassword)
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(bizerr.ErrorResponse(err))
}
