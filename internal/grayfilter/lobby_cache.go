package grayfilter

import (
	"sync"
	"time"

	"github.com/rocyu-tech/rockgame/pkg/logger"
)

const (
	// CacheExpireSec is the local cache TTL (same as C++ default cache_expire_sec = 60).
	CacheExpireSec = 60 * time.Second
)

// ExpireData wraps a cached value with an expiry timestamp.
// Equivalent to C++ ExpireData<T>.
//
// Active() sets the last-active timestamp to now.
// Expire() returns true if the cached data has expired (now - active >= ttl).
type ExpireData[T any] struct {
	Data    T
	Expires time.Time
}

// Active marks the data as freshly loaded at the given time.
func (e *ExpireData[T]) Active(now time.Time) {
	e.Expires = now
}

// Expire returns true if the data has expired based on the given TTL.
// Equivalent to C++ ExpireData<T>::Expire(now, ttl).
func (e *ExpireData[T]) Expire(now time.Time, ttl time.Duration) bool {
	return now.Sub(e.Expires) >= ttl
}

// Redis key constants for lobby cache invalidation.
// These match the C++ Redis key names used in CDbPart::ClearCacheByPhpKey.
const (
	RedisKeyGameCategory       = "gameCategory.json"
	RedisKeyGameHallCfgMap     = "gameHallCfgMap"
	RedisKeyGameCateGameMap    = "gameCate:gameMap"
	RedisKeyCompanyGameMap     = "company:game:map"
	RedisKeyCompanyGameUIDMap  = "company:game_uid:map"
	RedisKeyGameHallCfgMapV1   = "gameHallCfgMapV1"
	RedisKeyGameHallCfgMapV2   = "gameHallCfgMapV2"
	RedisKeyGameHallCategoryV2 = "gameHallCategoryMapV2"
	RedisKeyGameHallCateRecV2  = "gameHallCateRecMapV2"
	RedisKeyGrayTestNew        = "gameHallGameGrayTestNew.json"
	RedisKeyGrayTestOld        = "gameHallGameGrayTest.json"
	RedisKeySidebarLeft        = "game:sidebar:left"
	RedisKeySidebarRight       = "game:sidebar:right"
	RedisKeyMatchTabConfig     = "match_tab_config"
)

// LobbyCache manages fine-grained caching for lobby data.
// Ported from C++ CDbPart cache members and ClearCacheByPhpKey method.
//
// When the PHP/admin backend updates a configuration in Redis, it sends
// a notification with the changed key name. This cache maps those key names
// to the appropriate local caches to invalidate.
//
// All cache operations are protected by sync.RWMutex for thread safety.
type LobbyCache struct {
	mu sync.RWMutex

	// GameHallCache stores the gameCategory.json data.
	// Equivalent to C++ ExpireData<std::string> game_hall_cache.
	GameHallCache ExpireData[string]

	// LanguageGroupCache stores per-language game group configurations.
	// Key: language code. Equivalent to C++ language_group_cache.
	LanguageGroupCache map[string]ExpireData[string]

	// GameGroupCache stores game ID lists by group ID.
	// Key: groupID. Equivalent to C++ game_group_cache.
	GameGroupCache map[int64]ExpireData[[]int64]

	// GameIDCache stores game info by game ID.
	// Key: gameID. Equivalent to C++ game_id_cache.
	GameIDCache map[int64]ExpireData[map[string]interface{}]

	// PlatGameIDCache stores game info by platform:gameID.
	// Key: "plat:gameID". Equivalent to C++ plat_game_id_cache.
	PlatGameIDCache map[string]ExpireData[map[string]interface{}]

	// LangGroupV1Cache stores V1 language group configurations.
	// Key: "lang:platType". Equivalent to C++ language_group_v1_cache.
	LangGroupV1Cache map[string]ExpireData[string]

	// GameGroupDetailV1 stores V1 game group detail JSON.
	// Key: groupID. Equivalent to C++ game_group_detail_v1_cache.
	GameGroupDetailV1 map[int64]ExpireData[string]

	// GameGroupDetailV2 stores V2 game group detail JSON.
	// Key: groupID. Equivalent to C++ game_group_detail_v2_cache.
	GameGroupDetailV2 map[int64]ExpireData[string]

	// SidebarLeftCache stores left sidebar config.
	// Key: config name. Equivalent to C++ m_sidebarLeftCfg_map.
	SidebarLeftCache map[string]string

	// SidebarRightCache stores right sidebar config.
	// Key: config name. Equivalent to C++ m_sidebarRightCfg_map.
	SidebarRightCache map[string]string

	// MatchTabConfigCache stores match tab configuration.
	// Equivalent to C++ match_config_cache.
	MatchTabConfigCache ExpireData[string]
}

var (
	lobbyCacheInst *LobbyCache
	lobbyCacheOnce sync.Once
)

// LobbyCacheInstance returns the singleton LobbyCache instance.
func LobbyCacheInstance() *LobbyCache {
	lobbyCacheOnce.Do(func() {
		lobbyCacheInst = &LobbyCache{
			LanguageGroupCache: make(map[string]ExpireData[string]),
			GameGroupCache:     make(map[int64]ExpireData[[]int64]),
			GameIDCache:        make(map[int64]ExpireData[map[string]interface{}]),
			PlatGameIDCache:    make(map[string]ExpireData[map[string]interface{}]),
			LangGroupV1Cache:   make(map[string]ExpireData[string]),
			GameGroupDetailV1:  make(map[int64]ExpireData[string]),
			GameGroupDetailV2:  make(map[int64]ExpireData[string]),
			SidebarLeftCache:   make(map[string]string),
			SidebarRightCache:  make(map[string]string),
		}
	})
	return lobbyCacheInst
}

// ClearCacheByKey implements fine-grained cache invalidation.
// Ported from C++ CDbPart::ClearCacheByPhpKey.
//
// When PHP/admin backend updates config in Redis, it calls this with the
// changed key name. The method maps old C++ Redis keys to their corresponding
// Go cache structures and invalidates them.
//
// Key mappings (C++ Redis key -> Go cache):
//   - "gameCategory.json"           -> expire GameHallCache immediately
//   - "gameHallCfgMap"              -> clear all LanguageGroupCache
//   - "gameCate:gameMap"            -> clear game caches + trigger gray update
//   - "company:game:map"            -> clear game caches + trigger gray update
//   - "company:game_uid:map"        -> clear game caches + trigger gray update
//   - "gameHallCfgMapV1"            -> clear V1/V2 caches + trigger gray update
//   - "gameHallCfgMapV2"            -> clear V1/V2 caches + trigger gray update
//   - "gameHallCategoryMapV2"       -> clear V1/V2 caches + trigger gray update
//   - "gameHallCateRecMapV2"        -> clear V1/V2 caches + trigger gray update
//   - "gameHallGameGrayTest.json"   -> trigger gray update
//   - "gameHallGameGrayTestNew.json" -> trigger gray update
//   - "game:sidebar:left"           -> clear sidebar left + reload
//   - "game:sidebar:right"          -> clear sidebar right + reload
//   - "match_tab_config"            -> expire match tab cache + reload
//
// Returns true if a gray update was triggered.
func (lc *LobbyCache) ClearCacheByKey(name string) bool {
	logger.Debugf("grayfilter: ClearCacheByKey called: %s", name)

	lc.mu.Lock()
	defer lc.mu.Unlock()

	needUpdateGray := false

	switch name {
	case RedisKeyGameCategory:
		// Set data to expire immediately (equivalent to C++ game_hall_cache.Active(0))
		lc.GameHallCache.Active(time.Time{})

	case RedisKeyGameHallCfgMap:
		// Clear all per-language game group configs
		lc.LanguageGroupCache = make(map[string]ExpireData[string])

	case RedisKeyGameCateGameMap, RedisKeyCompanyGameMap, RedisKeyCompanyGameUIDMap:
		// Clear game group cache and game ID cache
		lc.GameGroupCache = make(map[int64]ExpireData[[]int64])
		lc.GameIDCache = make(map[int64]ExpireData[map[string]interface{}])
		needUpdateGray = true

	case RedisKeyGameHallCfgMapV1, RedisKeyGameHallCfgMapV2,
		RedisKeyGameHallCategoryV2, RedisKeyGameHallCateRecV2:
		// Clear all V1/V2 caches
		lc.LangGroupV1Cache = make(map[string]ExpireData[string])
		lc.GameGroupDetailV1 = make(map[int64]ExpireData[string])
		lc.GameGroupDetailV2 = make(map[int64]ExpireData[string])
		needUpdateGray = true

	case RedisKeyGrayTestOld, RedisKeyGrayTestNew:
		// Gray config changed, trigger update
		needUpdateGray = true

	case RedisKeySidebarLeft:
		// Clear left sidebar config
		lc.SidebarLeftCache = make(map[string]string)

	case RedisKeySidebarRight:
		// Clear right sidebar config
		lc.SidebarRightCache = make(map[string]string)

	case RedisKeyMatchTabConfig:
		// Expire match tab config immediately
		lc.MatchTabConfigCache.Active(time.Time{})
	}

	if needUpdateGray {
		// Trigger async gray update (equivalent to C++ state |= 0x1)
		go func() {
			if ok := GrayInstance().UpdateGray(true); ok {
				logger.Info("grayfilter: gray config updated via cache invalidation")
			}
		}()
	}

	return needUpdateGray
}

// IsExpired checks if a given expiry time has passed the default TTL.
func IsExpired(expires time.Time) bool {
	return time.Since(expires) >= CacheExpireSec
}
