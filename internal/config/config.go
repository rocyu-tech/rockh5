package config

import (
        "fmt"
        "sync"
        "sync/atomic"

        "github.com/fsnotify/fsnotify"
        "github.com/spf13/viper"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// Config is the global application configuration
type Config struct {
        App      AppConfig      `mapstructure:"app"`
        Server   ServerConfig   `mapstructure:"server"`
        Services ServicesConfig `mapstructure:"services"`
        Database DatabaseConfig `mapstructure:"database"`
        LogDB    DatabaseConfig `mapstructure:"log_db"`
        Redis    RedisConfig    `mapstructure:"redis"`
        Etcd     EtcdConfig     `mapstructure:"etcd"`
        Log      LogConfig      `mapstructure:"log"`
        JWT      JWTConfig      `mapstructure:"jwt"`
        Gate     GateConfig     `mapstructure:"gate"`
}

type AppConfig struct {
        Name     string `mapstructure:"name"`
        Env      string `mapstructure:"env"`     // dev, test, prod
        Version  string `mapstructure:"version"`
        Region   string `mapstructure:"region"`  // deployment region
        Language string `mapstructure:"language"` // default language
}

type ServerConfig struct {
        Addr         string `mapstructure:"addr"`          // service listen address, e.g. ":8080"
        ReadTimeout  int    `mapstructure:"read_timeout"`   // seconds
        WriteTimeout int    `mapstructure:"write_timeout"`  // seconds
        Mode         string `mapstructure:"mode"`           // debug, release
}

// ServicesConfig holds per-service configuration (ports, etc.)
type ServicesConfig struct {
        Ports map[string]int `mapstructure:"ports"` // per-service default ports, keyed by service name (e.g. "gate": 8080)
}

type DatabaseConfig struct {
        Host         string `mapstructure:"host"`
        Port         int    `mapstructure:"port"`
        User         string `mapstructure:"user"`
        Password     string `mapstructure:"password"`
        DBName       string `mapstructure:"db_name"`
        MaxOpenConns int    `mapstructure:"max_open_conns"`
        MaxIdleConns int    `mapstructure:"max_idle_conns"`
        MaxLifetime  int    `mapstructure:"max_lifetime"` // seconds
        ShardCount   int    `mapstructure:"shard_count"`   // hash shard count, default 16
}

type RedisConfig struct {
        Addr     string `mapstructure:"addr"`
        Password string `mapstructure:"password"`
        DB       int    `mapstructure:"db"`
        PoolSize int    `mapstructure:"pool_size"`
}

type EtcdConfig struct {
        Addrs       []string `mapstructure:"addrs"`       // etcd endpoints, e.g. ["127.0.0.1:2379"]
        DialTimeout int      `mapstructure:"dial_timeout"` // connection timeout in seconds
}

type LogConfig struct {
        Level       string `mapstructure:"level"`        // debug, info, warn, error
        Format      string `mapstructure:"format"`       // json, console
        Output      string `mapstructure:"output"`       // stdout, file, both
        File        string `mapstructure:"file"`         // log file path (e.g., /var/log/rockgame/app.log)
        MaxSizeMB   int    `mapstructure:"max_size_mb"`  // max size per log file in MB, default 100
        MaxBackups  int    `mapstructure:"max_backups"`   // max number of old log files, default 30
        MaxAgeDays  int    `mapstructure:"max_age_days"`  // max days to retain old log files, default 90
        Compress    bool   `mapstructure:"compress"`      // gzip rotated files, default true
}

type JWTConfig struct {
        Secret     string   `mapstructure:"secret"`     // primary signing secret (backward compat)
        Secrets   []string `mapstructure:"secrets"`    // key ring for rotation: [current, previous, ...]
        AccessTTL  int      `mapstructure:"access_ttl"`  // minutes, default 15
        RefreshTTL int      `mapstructure:"refresh_ttl"` // days, default 7
        SessionTTL int      `mapstructure:"session_ttl"` // minutes, default 5 (game session)
        Issuer     string   `mapstructure:"issuer"`
}

// ActiveSecrets returns the ordered key ring for JWT signing/verification.
// If Secrets is configured, returns it directly (rotation mode).
// Otherwise, wraps the legacy single Secret into a one-element slice (backward compat).
// secrets[0] = current key (used for signing), secrets[1:] = old keys (grace period).
func (c *JWTConfig) ActiveSecrets() []string {
        if len(c.Secrets) > 0 {
                return c.Secrets
        }
        if c.Secret != "" {
                return []string{c.Secret}
        }
        return nil
}

// GateConfig holds gate-specific configuration including static route definitions.
// These routes serve as fallback defaults when etcd dynamic routing is unavailable.
type GateConfig struct {
        Routes     []RouteItem `mapstructure:"routes"`      // static route definitions (fallback)
        HMACSecret string      `mapstructure:"hmac_secret"` // HMAC-SHA256 secret for internal service-to-service auth
}

// RouteItem defines a single route in config (same structure as proxy.RouteConfig).
// Used in config YAML and maps to etcd RouteEntry.
type RouteItem struct {
        Prefix  string `mapstructure:"prefix"`  // path prefix, e.g. "/api/v1/account"
        Backend string `mapstructure:"backend"` // backend service name, e.g. "account-node"
        Auth    bool   `mapstructure:"auth"`    // whether JWT auth is required
}

// ── ReloadCallback is called after a successful config reload. ──
// The callback receives the new config. It must not block.
type ReloadCallback func(oldCfg, newCfg *Config)

var (
        globalConfig atomic.Pointer[Config] // atomic config pointer for lock-free reads
        loadOnce     sync.Once
        configPath   string // stored config file path for hot-reload
        callbacks    []ReloadCallback
        callbacksMu  sync.RWMutex
)

// Load reads configuration from file and environment variables
func Load(configFilePath string) (*Config, error) {
        viper.SetConfigFile(configFilePath)
        viper.SetConfigType("yaml")

        // Environment variable override
        viper.AutomaticEnv()
        viper.SetEnvPrefix("ROCKGAME")

        if err := viper.ReadInConfig(); err != nil {
                return nil, fmt.Errorf("failed to read config: %w", err)
        }

        var cfg Config
        if err := viper.Unmarshal(&cfg); err != nil {
                return nil, fmt.Errorf("failed to unmarshal config: %w", err)
        }

        applyDefaults(&cfg)

        return &cfg, nil
}

// applyDefaults sets sensible defaults for zero-value config fields.
// G7: operates on a copy of the passed pointer to avoid surprising callers.
func applyDefaults(cfg *Config) {
        if cfg.Database.ShardCount == 0 {
                cfg.Database.ShardCount = 16
        }
        if cfg.JWT.AccessTTL == 0 {
                cfg.JWT.AccessTTL = 15
        }
        if cfg.JWT.RefreshTTL == 0 {
                cfg.JWT.RefreshTTL = 7
        }
        if cfg.JWT.SessionTTL == 0 {
                cfg.JWT.SessionTTL = 5
        }
        if len(cfg.Etcd.Addrs) == 0 {
                cfg.Etcd.Addrs = []string{"127.0.0.1:2379"}
        }
        if cfg.Etcd.DialTimeout == 0 {
                cfg.Etcd.DialTimeout = 5
        }

        // Set defaults for log database (inherits from main database if not specified)
        // G7: only set if LogDB.Host is empty (don't overwrite caller's explicit LogDB config)
        if cfg.LogDB.Host == "" && cfg.Database.Host != "" {
                cfg.LogDB.Host = cfg.Database.Host
                cfg.LogDB.Port = cfg.Database.Port
                cfg.LogDB.User = cfg.Database.User
                cfg.LogDB.Password = cfg.Database.Password
                cfg.LogDB.DBName = cfg.Database.DBName + "_log"
                cfg.LogDB.MaxOpenConns = 10
                cfg.LogDB.MaxIdleConns = 5
                cfg.LogDB.MaxLifetime = cfg.Database.MaxLifetime
                if cfg.LogDB.MaxLifetime == 0 {
                        cfg.LogDB.MaxLifetime = 3600
                }
        }
}

// MustLoad loads config or panics (thread-safe, uses sync.Once).
// Also enables file watching for hot-reload (fsnotify via Viper).
func MustLoad(path string) *Config {
        loadOnce.Do(func() {
                configPath = path
                cfg, err := Load(path)
                if err != nil {
                        panic(err)
                }
                globalConfig.Store(cfg)

                // Enable Viper file watching (fsnotify) for automatic hot-reload.
                // SIGHUP is handled separately in WatchSIGHUP.
                viper.OnConfigChange(func(e fsnotify.Event) {
                        logger.Infof("config: file change detected: %s (op=%s), reloading...", e.Name, e.Op.String())
                        doReload("fsnotify")
                })
                viper.WatchConfig()
                logger.Info("config: file watching enabled (fsnotify)")
        })
        return globalConfig.Load()
}

// C returns the current global configuration (lock-free, always safe).
// Panics only if MustLoad was never called.
func C() *Config {
        cfg := globalConfig.Load()
        if cfg == nil {
                panic("config not loaded, call Load or MustLoad first")
        }
        return cfg
}

// Reload forces a config reload from disk. Called by SIGHUP handler or manually.
func Reload() {
        doReload("manual")
}

// doReload is the internal reload logic shared by fsnotify and SIGHUP.
func doReload(source string) {
        newCfg, err := Load(configPath)
        if err != nil {
                logger.Errorf("config: reload failed (%s): %v — keeping current config", source, err)
                return
        }

        oldCfg := globalConfig.Load()
        globalConfig.Store(newCfg)
        logger.Infof("config: reloaded successfully (%s)", source)

        // Notify registered callbacks (non-blocking reads)
        callbacksMu.RLock()
        cbs := make([]ReloadCallback, len(callbacks))
        copy(cbs, callbacks)
        callbacksMu.RUnlock()

        for _, cb := range cbs {
                cb(oldCfg, newCfg)
        }
}

// OnReload registers a callback to be invoked after a successful config reload.
// Callbacks receive the old and new config for diff comparison.
// Must be called after MustLoad. Thread-safe.
func OnReload(cb ReloadCallback) {
        callbacksMu.Lock()
        defer callbacksMu.Unlock()
        callbacks = append(callbacks, cb)
}

// Port returns the configured default port for the given service name.
// Returns 0 if the service is not configured.
func (c *Config) Port(serviceName string) int {
        if c.Services.Ports == nil {
                return 0
        }
        return c.Services.Ports[serviceName]
}