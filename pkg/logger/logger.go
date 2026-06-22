package logger

import (
        "fmt"
        "os"
        "path/filepath"
        "sync"
        "time"

        "go.uber.org/zap"
        "go.uber.org/zap/zapcore"
        "gopkg.in/natefinch/lumberjack.v2"
)

var (
        log    *zap.SugaredLogger
        logMux sync.Mutex
)

// Init initializes the logger.
// G13: logMux protects both log and logMux from concurrent Init() calls.
func Init(level, format, output, file string, opts ...Option) {
        logMux.Lock()
        defer logMux.Unlock()

        var zapLevel zapcore.Level
        switch level {
        case "debug":
                zapLevel = zapcore.DebugLevel
        case "info":
                zapLevel = zapcore.InfoLevel
        case "warn":
                zapLevel = zapcore.WarnLevel
        case "error":
                zapLevel = zapcore.ErrorLevel
        default:
                zapLevel = zapcore.InfoLevel
        }

        var encoder zapcore.Encoder
        encoderConfig := zap.NewProductionEncoderConfig()
        encoderConfig.TimeKey = "time"
        encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
        encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

        if format == "console" {
                encoder = zapcore.NewConsoleEncoder(encoderConfig)
        } else {
                encoder = zapcore.NewJSONEncoder(encoderConfig)
        }

        // Build options with defaults
        cfg := &logOptions{
                maxSizeMB:    100,  // 100MB per file
                maxBackups:   30,   // keep 30 old files
                maxAgeDays:   90,   // keep 90 days
                compress:     true,  // gzip old files
                consoleAlso:  false, // also print to stdout when writing file
        }
        for _, opt := range opts {
                opt(cfg)
        }

        var cores []zapcore.Core

        // Console output (always available)
        if output != "file" || cfg.consoleAlso {
                cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapLevel))
        }

        // File output with daily rotation + size-based rotation
        if output == "file" && file != "" {
                // Ensure log directory exists
                dir := filepath.Dir(file)
                if dir != "" && dir != "." {
                        if err := os.MkdirAll(dir, 0755); err != nil {
                                fmt.Fprintf(os.Stderr, "logger: failed to create log directory %s: %v\n", dir, err)
                        }
                }

                w := newDailyRotatingWriter(file, cfg.serviceName, cfg.nodeID, &dailyRotatingConfig{
                        maxSizeMB:  cfg.maxSizeMB,
                        maxBackups: cfg.maxBackups,
                        maxAgeDays: cfg.maxAgeDays,
                        compress:   cfg.compress,
                })
                cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(w), zapLevel))
        }

        // Build tee core
        var core zapcore.Core
        if len(cores) == 1 {
                core = cores[0]
        } else {
                core = zapcore.NewTee(cores...)
        }

        logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
        log = logger.Sugar()
        // G13: removed nested logMux.Lock/Unlock — caller already holds the lock
}

// --- Daily Rotating Writer ---
// Wraps lumberjack to create a new log file per day (e.g., app.2026-06-08.log).
// When the day changes, the current file is closed and a new one is opened.
// Lumberjack handles size-based rotation within each daily file.

type dailyRotatingConfig struct {
        maxSizeMB  int  // max size per file in MB before lumberjack rotates
        maxBackups int  // max number of old log files to retain
        maxAgeDays int  // max days to retain old log files
        compress   bool // compress rotated files with gzip
}

type dailyRotatingWriter struct {
        baseFile    string               // 原始配置路径 (如 ./logs/app.log)，仅用于提取根目录
        serviceName string               // 服务名 (如 gate, lobby-node)
        nodeID      int                  // 节点 ID (如 0, 1, 2)
        cfg         *dailyRotatingConfig

        mu          sync.Mutex
        currentDate string               // "2006-01-02"
        currentFile *lumberjack.Logger
}

func newDailyRotatingWriter(file string, serviceName string, nodeID int, cfg *dailyRotatingConfig) *dailyRotatingWriter {
        w := &dailyRotatingWriter{
                baseFile:    file,
                serviceName: serviceName,
                nodeID:      nodeID,
                cfg:         cfg,
        }
        w.rotate() // open initial file
        return w
}

// todayFileName returns the actual file path for today.
// 目录结构: {baseDir}/{date}/{serviceName}.{nodeID}.log
// 例如: logs/2026-06-08/gate.0.log
func (w *dailyRotatingWriter) todayFileName(date string) string {
        // 从原始配置路径提取根目录 (如 ./logs/app.log → ./logs)
        baseDir := filepath.Dir(w.baseFile)

        // 服务名默认 app
        svc := w.serviceName
        if svc == "" {
                svc = "app"
        }

        // 构建日期子目录: {baseDir}/{date}/
        dateDir := filepath.Join(baseDir, date)

        // 文件名: {serviceName}.{nodeID}.log
        fileName := fmt.Sprintf("%s.%d.log", svc, w.nodeID)

        return filepath.Join(dateDir, fileName)
}

// rotate closes the current file (if any) and opens a new one for the given date.
func (w *dailyRotatingWriter) rotate() {
        today := time.Now().Format("2006-01-02")
        if w.currentFile != nil && w.currentDate == today {
                return // same day, no rotation needed
        }

        // Close previous day's file
        if w.currentFile != nil {
                _ = w.currentFile.Close()
        }

        filePath := w.todayFileName(today)

        // Ensure date subdirectory exists (e.g. logs/2026-06-08/)
        dateDir := filepath.Dir(filePath)
        if err := os.MkdirAll(dateDir, 0755); err != nil {
                fmt.Fprintf(os.Stderr, "logger: failed to create log directory %s: %v\n", dateDir, err)
        }

        w.currentFile = &lumberjack.Logger{
                Filename:   filePath,
                MaxSize:    w.cfg.maxSizeMB,    // megabytes
                MaxBackups: w.cfg.maxBackups,   // number of backups
                MaxAge:     w.cfg.maxAgeDays,    // days
                Compress:   w.cfg.compress,     // compress rotated files
                LocalTime:  true,               // use local timezone for backup naming
        }
        w.currentDate = today
}

// Write implements io.Writer. Checks for date change and rotates if needed.
func (w *dailyRotatingWriter) Write(p []byte) (n int, err error) {
        w.mu.Lock()
        defer w.mu.Unlock()

        // Check if we need to rotate to a new day
        today := time.Now().Format("2006-01-02")
        if w.currentDate != today {
                w.rotate()
        }

        return w.currentFile.Write(p)
}

// Close cleans up the underlying file handle.
func (w *dailyRotatingWriter) Close() error {
        w.mu.Lock()
        defer w.mu.Unlock()
        if w.currentFile != nil {
                return w.currentFile.Close()
        }
        return nil
}

// Sync flushes any buffered log entries
func Sync() {
        logMux.Lock()
        defer logMux.Unlock()
        if log != nil {
                _ = log.Sync()
        }
}

// LogConfig mirrors the config.LogConfig struct for initialization without internal package dependency.
type LogConfig struct {
        Level       string
        Format      string
        Output      string // stdout, file, both
        File        string
        MaxSizeMB   int
        MaxBackups  int
        MaxAgeDays  int
        Compress    bool
        ServiceName string // 服务名，用于日志子目录划分 (e.g. gate, lobby-node)
        NodeID      int    // 节点 ID，用于日志文件名区分多实例 (e.g. 0, 1, 2)
}

// InitFromConfig initializes the logger from a LogConfig struct.
// This is the recommended way to initialize the logger in production.
//
// 日志目录结构: {baseDir}/{date}/{serviceName}.{nodeID}.log
// 例如: ./logs/2026-06-08/gate.0.log
func InitFromConfig(lc *LogConfig) {
        output := lc.Output
        if output == "both" {
                output = "file" // treat "both" as file output, consoleAlso will handle stdout
        }

        Init(
                lc.Level,
                lc.Format,
                output,
                lc.File,
                WithMaxSize(lc.MaxSizeMB),
                WithMaxBackups(lc.MaxBackups),
                WithMaxAge(lc.MaxAgeDays),
                WithCompress(lc.Compress),
                WithConsoleAlso(lc.Output == "both"),
                WithServiceName(lc.ServiceName),
                WithNodeID(lc.NodeID),
        )
}

// --- Functional Options ---

type logOptions struct {
        maxSizeMB   int
        maxBackups  int
        maxAgeDays  int
        compress    bool
        consoleAlso bool
        serviceName string
        nodeID      int
}

type Option func(*logOptions)

// WithMaxSize sets the maximum size in MB of a log file before it gets rotated.
func WithMaxSize(mb int) Option {
        return func(o *logOptions) { o.maxSizeMB = mb }
}

// WithMaxBackups sets the maximum number of old log files to retain.
func WithMaxBackups(n int) Option {
        return func(o *logOptions) { o.maxBackups = n }
}

// WithMaxAge sets the maximum number of days to retain old log files.
func WithMaxAge(d int) Option {
        return func(o *logOptions) { o.maxAgeDays = d }
}

// WithCompress enables or disables gzip compression of rotated files.
func WithCompress(c bool) Option {
        return func(o *logOptions) { o.compress = c }
}

// WithConsoleAlso makes the logger also write to stdout when output is file.
func WithConsoleAlso(b bool) Option {
        return func(o *logOptions) { o.consoleAlso = b }
}

// WithServiceName sets the service name for log directory separation.
func WithServiceName(s string) Option {
        return func(o *logOptions) { o.serviceName = s }
}

// WithNodeID sets the node ID for log filename differentiation.
func WithNodeID(id int) Option {
        return func(o *logOptions) { o.nodeID = id }
}

// --- Public logging functions (unchanged API) ---

// Info logs info level message
func Info(args ...interface{}) {
        if log != nil {
                log.Info(args...)
        }
}

// Infof logs formatted info message
func Infof(template string, args ...interface{}) {
        if log != nil {
                log.Infof(template, args...)
        }
}

// Error logs error level message
func Error(args ...interface{}) {
        if log != nil {
                log.Error(args...)
        }
}

// Errorf logs formatted error message
func Errorf(template string, args ...interface{}) {
        if log != nil {
                log.Errorf(template, args...)
        }
}

// Warn logs warning level message
func Warn(args ...interface{}) {
        if log != nil {
                log.Warn(args...)
        }
}

// Warnf logs formatted warning message
func Warnf(template string, args ...interface{}) {
        if log != nil {
                log.Warnf(template, args...)
        }
}

// Debug logs debug level message
func Debug(args ...interface{}) {
        if log != nil {
                log.Debug(args...)
        }
}

// Debugf logs formatted debug message
func Debugf(template string, args ...interface{}) {
        if log != nil {
                log.Debugf(template, args...)
        }
}

// Fatalf logs formatted fatal level message and exits
func Fatalf(template string, args ...interface{}) {
        if log != nil {
                log.Fatalf(template, args...)
        }
}

// Fatal logs fatal level message and exits
func Fatal(args ...interface{}) {
        if log != nil {
                log.Fatal(args...)
        }
}
