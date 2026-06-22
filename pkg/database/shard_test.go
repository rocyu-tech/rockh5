package database

import "testing"

func TestShardRouter_Route(t *testing.T) {
	router := NewShardRouter(16)

	tests := []struct {
		name   string
		userID int64
		want   string
	}{
		{"user 0", 0, "_00"},
		{"user 1", 1, "_01"},
		{"user 15", 15, "_15"},
		{"user 16 wraps to 0", 16, "_00"},
		{"user 100", 100, "_04"},
		{"user 12345", 12345, "_09"},
		{"negative ID", -1, "_01"}, // absolute value
		{"large ID", 999999999999, "_15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.Route(tt.userID)
			if got != tt.want {
				t.Errorf("Route(%d) = %q, want %q", tt.userID, got, tt.want)
			}
		})
	}
}

func TestShardRouter_TableName(t *testing.T) {
	router := NewShardRouter(16)
	got := router.TableName("user_vip", 12345)
	want := "user_vip_09"
	if got != want {
		t.Errorf("TableName(user_vip, 12345) = %q, want %q", got, want)
	}
}

func TestShardTable(t *testing.T) {
	got := ShardTable("user_vip", 100)
	want := "user_vip_04"
	if got != want {
		t.Errorf("ShardTable(user_vip, 100) = %q, want %q", got, want)
	}
}

func TestShardRouter_IsShardedTable(t *testing.T) {
	router := NewShardRouter(16)

	tests := []struct {
		table string
		want  bool
	}{
		{"user_vip_05", true},
		{"user_vip_00", true},
		{"user_vip_15", true},
		{"users", false},
		{"admin_user", false},
		{"user_vip", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := router.IsShardedTable(tt.table)
			if got != tt.want {
				t.Errorf("IsShardedTable(%q) = %v, want %v", tt.table, got, tt.want)
			}
		})
	}
}

func TestShardRouter_GetBaseTableName(t *testing.T) {
	router := NewShardRouter(16)

	tests := []struct {
		input string
		want  string
	}{
		{"user_vip_05", "user_vip"},
		{"activity_records_00", "activity_records"},
		{"users", "users"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := router.GetBaseTableName(tt.input)
			if got != tt.want {
				t.Errorf("GetBaseTableName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShardRouter_ForEachShard(t *testing.T) {
	router := NewShardRouter(4)
	var tables []string
	err := router.ForEachShard("test", func(tableName string) error {
		tables = append(tables, tableName)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachShard returned error: %v", err)
	}
	if len(tables) != 4 {
		t.Errorf("ForEachShard called fn %d times, want 4", len(tables))
	}
	expected := []string{"test_00", "test_01", "test_02", "test_03"}
	for i, want := range expected {
		if tables[i] != want {
			t.Errorf("tables[%d] = %q, want %q", i, tables[i], want)
		}
	}
}

func TestShardRouter_CustomShardCount(t *testing.T) {
	router := NewShardRouter(8)
	got := router.Route(10) // 10 % 8 = 2
	want := "_02"
	if got != want {
		t.Errorf("Route(10) with 8 shards = %q, want %q", got, want)
	}
}

func TestDailyTableName(t *testing.T) {
	got := DailyTableName("event_log", "20260605")
	want := "event_log_20260605"
	if got != want {
		t.Errorf("DailyTableName() = %q, want %q", got, want)
	}
}

func TestNewShardRouter_Default(t *testing.T) {
	router := NewShardRouter()
	if router.shardCount != DefaultShardCount {
		t.Errorf("default shard count = %d, want %d", router.shardCount, DefaultShardCount)
	}
}