package database

import (
        "fmt"
        "strconv"
        "strings"
)

const DefaultShardCount = 16

// ShardRouter provides table sharding by user_id hash
type ShardRouter struct {
        shardCount int
}

// NewShardRouter creates a new ShardRouter
func NewShardRouter(shardCount ...int) *ShardRouter {
        count := DefaultShardCount
        if len(shardCount) > 0 && shardCount[0] > 0 {
                count = shardCount[0]
        }
        return &ShardRouter{shardCount: count}
}

// Route returns the actual table name suffix for a given user ID.
// Handles negative IDs by using absolute value to prevent negative table indices.
func (r *ShardRouter) Route(userID int64) string {
        id := userID
        if id < 0 {
                id = -id
        }
        index := id % int64(r.shardCount)
        return fmt.Sprintf("_%02d", index)
}

// TableName returns the full sharded table name
// Example: TableName("user_assets", 12345) -> "user_assets_05"
func (r *ShardRouter) TableName(baseTable string, userID int64) string {
        return baseTable + r.Route(userID)
}

// IsShardedTable checks if a table name is sharded
func (r *ShardRouter) IsShardedTable(tableName string) bool {
        parts := strings.Split(tableName, "_")
        if len(parts) < 2 {
                return false
        }
        _, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
        return err == nil
}

// GetBaseTableName extracts the base table name from a sharded table
// Example: "user_assets_05" -> "user_assets"
func (r *ShardRouter) GetBaseTableName(shardedTable string) string {
        parts := strings.Split(shardedTable, "_")
        if len(parts) < 2 {
                return shardedTable
        }
        // Check if last part is a number
        if _, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
                return strings.Join(parts[:len(parts)-1], "_")
        }
        return shardedTable
}

// ForEachShard iterates over all sharded tables, calling fn with each table name
func (r *ShardRouter) ForEachShard(baseTable string, fn func(tableName string) error) error {
        for i := 0; i < r.shardCount; i++ {
                tableName := fmt.Sprintf("%s_%02d", baseTable, i)
                if err := fn(tableName); err != nil {
                        return err
                }
        }
        return nil
}

// global shard router
var defaultRouter = NewShardRouter()

// ShardTable is a convenience function using the default router
func ShardTable(baseTable string, userID int64) string {
        return defaultRouter.TableName(baseTable, userID)
}

// DailyTableName returns the daily-partitioned table name for a date string.
// These tables live in the rockgame_log database.
// Example: DailyTableName("event_log", "20260605") -> "event_log_20260605"
func DailyTableName(baseTable string, date string) string {
        return baseTable + "_" + date
}
