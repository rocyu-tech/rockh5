package database

import (
        "fmt"
        "sync"
        "time"

        "github.com/rocyu-tech/rockgame/internal/config"
        "gorm.io/driver/mysql"
        "gorm.io/gorm"
        "gorm.io/gorm/logger"
)

var (
        db     *gorm.DB
        logDB  *gorm.DB
        once   sync.Once
        logOnce sync.Once
)

// Init initializes the database connection.
// G8: does not modify the input cfg — copies values locally.
func Init(cfg *config.DatabaseConfig) error {
        var err error
        once.Do(func() {
                maxOpen := cfg.MaxOpenConns
                maxIdle := cfg.MaxIdleConns
                maxLife := cfg.MaxLifetime
                if maxOpen == 0 {
                        maxOpen = 50
                }
                if maxIdle == 0 {
                        maxIdle = 10
                }
                if maxLife == 0 {
                        maxLife = 3600
                }

                dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
                        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
                )
                logLevel := logger.Info
                db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
                        Logger: logger.Default.LogMode(logLevel),
                })
                if err != nil {
                        return
                }
                sqlDB, err := db.DB()
                if err != nil {
                        return
                }
                sqlDB.SetMaxOpenConns(maxOpen)
                sqlDB.SetMaxIdleConns(maxIdle)
                sqlDB.SetConnMaxLifetime(time.Duration(maxLife) * time.Second)
        })
        return err
}

// DB returns the global gorm.DB instance
func DB() *gorm.DB {
        return db
}

// InitLogDB initializes the log database connection.
// G8: does not modify the input cfg — copies values locally.
func InitLogDB(cfg *config.DatabaseConfig) error {
        var err error
        logOnce.Do(func() {
                maxOpen := cfg.MaxOpenConns
                maxIdle := cfg.MaxIdleConns
                maxLife := cfg.MaxLifetime
                if maxOpen == 0 {
                        maxOpen = 10
                }
                if maxIdle == 0 {
                        maxIdle = 5
                }
                if maxLife == 0 {
                        maxLife = 3600
                }

                dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
                        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
                )
                logLevel := logger.Info
                logDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
                        Logger: logger.Default.LogMode(logLevel),
                })
                if err != nil {
                        return
                }
                sqlDB, err := logDB.DB()
                if err != nil {
                        return
                }
                sqlDB.SetMaxOpenConns(maxOpen)
                sqlDB.SetMaxIdleConns(maxIdle)
                sqlDB.SetConnMaxLifetime(time.Duration(maxLife) * time.Second)
        })
        return err
}

// LogDB returns the global log gorm.DB instance
func LogDB() *gorm.DB {
        return logDB
}
