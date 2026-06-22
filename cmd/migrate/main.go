package main

import (
        "bufio"
        "database/sql"
        "flag"
        "fmt"
        "os"
        "path/filepath"
        "strings"

        _ "github.com/go-sql-driver/mysql"

        "github.com/rocyu-tech/rockgame/internal/config"
)

func main() {
        configPath := flag.String("config", "etc/dev/config.yaml", "config file path")
        flag.Parse()

        // Load config
        cfg, err := config.Load(*configPath)
        if err != nil {
                fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
                os.Exit(1)
        }

        // Connect to MySQL (no database specified first, schema.sql will CREATE DATABASE)
        dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
                cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port)
        db, err := sql.Open("mysql", dsn)
        if err != nil {
                fmt.Fprintf(os.Stderr, "connect mysql failed: %v\n", err)
                os.Exit(1)
        }
        defer db.Close()

        if err := db.Ping(); err != nil {
                fmt.Fprintf(os.Stderr, "ping mysql failed: %v\n", err)
                fmt.Fprintf(os.Stderr, "hint: make sure MySQL is running and credentials are correct\n")
                os.Exit(1)
        }

        // Resolve SQL directory relative to project root
        sqlDir := resolveProjectRoot("sql")

        // Step 1: Execute schema.sql (uses ; as delimiter, multiStatements handles it)
        // Schema creates the database, so we connect without DB first.
        schemaFile := filepath.Join(sqlDir, "schema.sql")
        fmt.Printf("=== Executing schema (%s) ===\n", schemaFile)
        if err := execSQLFile(db, schemaFile); err != nil {
                fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
                os.Exit(1)
        }
        fmt.Println("  OK: schema applied")

        // Reconnect with rockgame database as default.
        // After this, all db.Exec calls operate in rockgame context,
        // so stored procedures and LIKE clauses reference correct tables.
        db.Close()
        dsnWithDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
                cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
        db, err = sql.Open("mysql", dsnWithDB)
        if err != nil {
                fmt.Fprintf(os.Stderr, "reconnect to rockgame failed: %v\n", err)
                os.Exit(1)
        }
        defer db.Close()
        if err := db.Ping(); err != nil {
                fmt.Fprintf(os.Stderr, "ping rockgame failed: %v\n", err)
                os.Exit(1)
        }
        fmt.Printf("  OK: switched to database '%s'\n", cfg.Database.DBName)

        // Step 2: Execute sharding.sql (contains DELIMITER $$ ... $$ blocks)
        // DELIMITER is a mysql client directive, not valid SQL for Go driver.
        // We parse it manually: strip DELIMITER lines, split on custom delimiter,
        // and execute each block.
        shardingFile := filepath.Join(sqlDir, "sharding.sql")
        fmt.Printf("=== Executing sharding (%s) ===\n", shardingFile)
        if err := execShardingFile(db, shardingFile); err != nil {
                fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
                os.Exit(1)
        }
        fmt.Println("  OK: sharding applied")

        // Step 3: Execute seed.sql (uses ; as delimiter)
        seedFile := filepath.Join(sqlDir, "seed.sql")
        fmt.Printf("=== Executing seed (%s) ===\n", seedFile)
        if err := execSQLFile(db, seedFile); err != nil {
                fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
                os.Exit(1)
        }
        fmt.Println("  OK: seed applied")

        // Step 4: Call stored procedure to create hash-sharded tables
        fmt.Println("=== Creating sharded tables (16 shards x 17 tables = 272 tables) ===")
        if _, err := db.Exec("CALL create_sharded_tables()"); err != nil {
                fmt.Fprintf(os.Stderr, "  WARNING: create_sharded_tables failed: %v\n", err)
        } else {
                fmt.Println("  OK: 272 sharded tables created")
        }

        // NOTE: Daily partitioned tables (event_log, click_log) are now in rockgame_log
        // database. See Phase 2 below.

        // Verify table count
        var tableCount int
        db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'rockgame'").Scan(&tableCount)
        fmt.Printf("\n=== Phase 1 complete! Total tables in rockgame: %d ===\n", tableCount)

        // =========================================================================
        // Phase 2: Migrate rockgame_log database
        // =========================================================================
        fmt.Println("\n========== PHASE 2: rockgame_log database ==========")

        // Determine log DB config
        logDBCfg := cfg.LogDB
        if logDBCfg.Host == "" {
                logDBCfg = cfg.Database
                logDBCfg.DBName = cfg.Database.DBName + "_log"
        }
        logDBName := logDBCfg.DBName

        // Connect to MySQL WITHOUT specifying database (same pattern as Phase 1).
        // schema_log.sql contains CREATE DATABASE IF NOT EXISTS rockgame_log.
        logDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
                logDBCfg.User, logDBCfg.Password, logDBCfg.Host, logDBCfg.Port)
        logDB, err := sql.Open("mysql", logDSN)
        if err != nil {
                fmt.Fprintf(os.Stderr, "connect mysql for log db failed: %v\n", err)
                os.Exit(1)
        }
        defer logDB.Close()

        if err := logDB.Ping(); err != nil {
                fmt.Fprintf(os.Stderr, "ping mysql for log db failed: %v\n", err)
                os.Exit(1)
        }

        // Step 1: Execute schema_log.sql (contains CREATE DATABASE rockgame_log)
        schemaLogFile := filepath.Join(sqlDir, "schema_log.sql")
        fmt.Printf("=== Executing log schema (%s) ===\n", schemaLogFile)
        if err := execSQLFile(logDB, schemaLogFile); err != nil {
                fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
                os.Exit(1)
        }
        fmt.Println("  OK: log schema applied")

        // Reconnect with rockgame_log as default database (same pattern as Phase 1)
        logDB.Close()
        logDSNWithDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
                logDBCfg.User, logDBCfg.Password, logDBCfg.Host, logDBCfg.Port, logDBName)
        logDB, err = sql.Open("mysql", logDSNWithDB)
        if err != nil {
                fmt.Fprintf(os.Stderr, "reconnect to %s failed: %v\n", logDBName, err)
                os.Exit(1)
        }
        defer logDB.Close()
        if err := logDB.Ping(); err != nil {
                fmt.Fprintf(os.Stderr, "ping %s failed: %v\n", logDBName, err)
                os.Exit(1)
        }
        fmt.Printf("  OK: switched to database '%s'\n", logDBName)

        // Step 2: Execute sharding_log.sql (DELIMITER blocks with stored procedures)
        shardingLogFile := filepath.Join(sqlDir, "sharding_log.sql")
        fmt.Printf("=== Executing log sharding (%s) ===\n", shardingLogFile)
        if err := execShardingFile(logDB, shardingLogFile); err != nil {
                fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
                os.Exit(1)
        }
        fmt.Println("  OK: log sharding applied")

        // Step 3: Create daily log tables
        fmt.Println("=== Creating daily log tables (next 7 days) ===")
        if _, err := logDB.Exec("CALL create_daily_log_tables(7)"); err != nil {
                fmt.Fprintf(os.Stderr, "  WARNING: create_daily_log_tables failed: %v\n", err)
        } else {
                fmt.Println("  OK: daily log tables created")
        }

        // Verify log table count
        var logTableCount int
        logDB.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?", logDBName).Scan(&logTableCount)
        fmt.Printf("  Total tables in %s: %d\n", logDBName, logTableCount)

        fmt.Printf("\n========== Migration complete! ==========\n")
}

// execSQLFile reads a SQL file and executes it as a single multi-statement query.
// Works for files that only use standard ; delimiter.
func execSQLFile(db *sql.DB, path string) error {
        content, err := os.ReadFile(path)
        if err != nil {
                return fmt.Errorf("read file: %w", err)
        }
        _, err = db.Exec(string(content))
        return err
}

// execShardingFile parses DELIMITER blocks from sharding.sql and executes them.
//
// The MySQL CLI uses "DELIMITER $$" to change the statement terminator so that
// stored procedures containing ; inside their body can be parsed correctly.
// The Go MySQL driver does not understand DELIMITER (it's not SQL).
//
// Algorithm:
//  1. Read file line by line
//  2. Track current delimiter (default ";")
//  3. Buffer lines until current delimiter is found at end of line
//  4. Execute buffered block as a single statement
func execShardingFile(db *sql.DB, path string) error {
        f, err := os.Open(path)
        if err != nil {
                return fmt.Errorf("open file: %w", err)
        }
        defer f.Close()

        var delimiter = ";"
        var buf strings.Builder

        scanner := bufio.NewScanner(f)
        // Increase buffer size for large SQL blocks
        scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

        lineNum := 0
        for scanner.Scan() {
                lineNum++
                line := strings.TrimSpace(scanner.Text())

                // Empty line or comment — skip only if buffer is empty (preserve inside blocks)
                if buf.Len() == 0 {
                        if line == "" || strings.HasPrefix(line, "--") {
                                continue
                        }
                }

                // Check for DELIMITER directive
                if strings.HasPrefix(strings.ToUpper(line), "DELIMITER ") {
                        // Flush any pending statement first
                        if buf.Len() > 0 {
                                stmt := strings.TrimSpace(buf.String())
                                if stmt != "" {
                                        if _, err := db.Exec(stmt); err != nil {
                                                return fmt.Errorf("line %d: exec failed: %w", lineNum, err)
                                        }
                                }
                                buf.Reset()
                        }
                        newDelimiter := strings.TrimSpace(line[len("DELIMITER "):])
                        if newDelimiter != "" {
                                delimiter = newDelimiter
                        }
                        continue
                }

                buf.WriteString(line)
                buf.WriteString("\n")

                // Check if line ends with current delimiter
                if strings.HasSuffix(line, delimiter) {
                        stmt := strings.TrimSpace(buf.String())
                        // Strip the trailing delimiter from the statement
                        if len(stmt) >= len(delimiter) {
                                stmt = stmt[:len(stmt)-len(delimiter)]
                        }
                        stmt = strings.TrimSpace(stmt)
                        if stmt != "" {
                                if _, err := db.Exec(stmt); err != nil {
                                        return fmt.Errorf("line %d: exec failed: %w", lineNum, err)
                                }
                        }
                        buf.Reset()
                }
        }

        // Flush remaining buffer
        if buf.Len() > 0 {
                stmt := strings.TrimSpace(buf.String())
                if stmt != "" {
                        if _, err := db.Exec(stmt); err != nil {
                                return fmt.Errorf("exec remaining buffer failed: %w", err)
                        }
                }
        }

        return scanner.Err()
}

// resolveProjectRoot finds the project root by looking for go.mod
func resolveProjectRoot(subDir string) string {
        dir, _ := os.Getwd()
        for {
                if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
                        return filepath.Join(dir, subDir)
                }
                parent := filepath.Dir(dir)
                if parent == dir {
                        break
                }
                dir = parent
        }
        // Fallback: try relative to executable
        exe, _ := os.Executable()
        exeDir := filepath.Dir(exe)
        for {
                if _, err := os.Stat(filepath.Join(exeDir, "go.mod")); err == nil {
                        return filepath.Join(exeDir, subDir)
                }
                parent := filepath.Dir(exeDir)
                if parent == exeDir {
                        break
                }
                exeDir = parent
        }
        return subDir
}
