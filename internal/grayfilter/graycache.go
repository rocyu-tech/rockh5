package grayfilter

import (
        "context"
        "encoding/json"
        "fmt"
        "strconv"
        "strings"
        "sync"
        "time"

        "github.com/rocyu-tech/rockgame/pkg/cache"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

const (
        // grayRedisKey is the Redis key storing the gray release JSON config.
        // Equivalent to C++ "gameHallGameGrayTestNew.json".
        grayRedisKey = "lobby:gray_config"

        // grayVersionTTL is the TTL for checking gray config version changes (24h).
        // Equivalent to C++ gray_expire_sec = 86400.
        grayVersionTTL = 24 * time.Hour

        // grayCacheTTL is the TTL for gray-filtered result caching (60s).
        // Equivalent to C++ cache_expire_sec = 60.
        grayCacheTTL = 60 * time.Second
)

// GrayCache manages gray release configuration and result caching.
// Ported from C++ GrayCacheV2.
//
// GrayCache sits between the lobby handlers and the game data, applying
// gray release filtering rules to game lists based on user UIDs. It:
//   - Periodically checks Redis for config version changes
//   - Triggers GameFilter reload when config changes
//   - Caches filtered results by match path for performance
//
// Thread safety is achieved via sync.RWMutex.
type GrayCache struct {
        mu          sync.RWMutex
        lastVersion string
        loadedAt    time.Time

        // detailCache caches filtered JSON results keyed by "path|groupID".
        // Equivalent to C++ game_group_v2_detail_cache.
        detailCache map[string]*expireStr

        // listCache caches game ID lists keyed by groupID.
        // Equivalent to C++ game_group_v2_list_cache.
        listCache map[int64][]int64

        // gameDataCache caches parsed game JSON data keyed by groupID -> gameID.
        // Equivalent to C++ game_id_v2_cache.
        gameDataCache map[int64]map[int64]json.RawMessage
}

// expireStr wraps a string value with expiry tracking.
// Equivalent to C++ ExpireData<std::string>.
type expireStr struct {
        data    string
        expires time.Time
}

// GrayInstance returns the singleton GrayCache instance.
// Equivalent to C++ GrayCache::Instance() via CSingleton.
func GrayInstance() *GrayCache {
        grayOnce.Do(func() {
                grayInstance = &GrayCache{
                        detailCache:   make(map[string]*expireStr),
                        listCache:     make(map[int64][]int64),
                        gameDataCache: make(map[int64]map[int64]json.RawMessage),
                }
        })
        return grayInstance
}

var (
        grayInstance *GrayCache
        grayOnce     sync.Once
)

// UpdateGray reloads gray config from Redis.
// If force=true, always reload regardless of TTL.
// If force=false, only reload if grayVersionTTL has elapsed.
// Returns true if config was successfully updated.
//
// Equivalent to C++ GrayCacheV2::UpdateGray.
//
// Flow:
//  1. Check TTL if not forced (C++ does not auto-refresh)
//  2. Read JSON from Redis key
//  3. Parse version_no, compare with lastVersion
//  4. If version changed, call GameFilter.ReloadFromConfig
//  5. Clear all caches
//  6. Update lastVersion and loadedAt
func (gc *GrayCache) UpdateGray(force bool) bool {
        now := time.Now()

        if !force {
                // In C++: if (!last_gray_version.Expire(now, gray_expire_sec)) { return false; }
                // The C++ version checks expire to decide whether to refresh.
                // Here we check if enough time has passed since last load.
                if now.Sub(gc.loadedAt) < grayVersionTTL {
                        return false
                }
        }

        gc.mu.Lock()
        defer gc.mu.Unlock()

        gc.loadedAt = now

        // Read config from Redis
        ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()

        redisClient := cache.Client()
        if redisClient == nil {
                logger.Error("grayfilter: redis client not initialized")
                return false
        }

        data, err := redisClient.Get(ctx, grayRedisKey).Result()
        if err != nil {
                logger.Errorf("grayfilter: failed to get gray config from Redis key %s: %v", grayRedisKey, err)
                return false
        }

        if data == "" {
                // Empty config: if we had a version before, clear caches
                if gc.lastVersion != "" {
                        gc.detailCache = make(map[string]*expireStr)
                        gc.listCache = make(map[int64][]int64)
                        gc.gameDataCache = make(map[int64]map[int64]json.RawMessage)
                        gc.lastVersion = ""
                        logger.Info("grayfilter: gray config cleared (empty data received)")
                }
                return true
        }

        // Parse the JSON to extract version_no and config
        var rawConfig map[string]json.RawMessage
        if err := json.Unmarshal([]byte(data), &rawConfig); err != nil {
                logger.Errorf("grayfilter: failed to parse gray config JSON: %v", err)
                return false
        }

        // Extract version_no
        versionNo := ""
        if raw, ok := rawConfig["version_no"]; ok {
                var vn string
                if err := json.Unmarshal(raw, &vn); err == nil {
                        versionNo = vn
                }
        }
        if versionNo == "" {
                logger.Error("grayfilter: gray config missing version_no")
                return false
        }

        // Version unchanged, no need to reload
        if gc.lastVersion == versionNo {
                logger.Debugf("grayfilter: version unchanged: %s", versionNo)
                return true
        }

        // Extract config array
        configRaw, ok := rawConfig["config"]
        if !ok {
                logger.Error("grayfilter: gray config missing 'config' field")
                return false
        }

        // Reload the game filter with new config
        if err := Instance().ReloadFromConfig(configRaw); err != nil {
                logger.Errorf("grayfilter: failed to reload game filter: %v", err)
                return false
        }

        logger.Infof("grayfilter: gray config reloaded, version: %s -> %s", gc.lastVersion, versionNo)

        // Clear all caches on config change
        gc.detailCache = make(map[string]*expireStr)
        gc.listCache = make(map[int64][]int64)
        gc.gameDataCache = make(map[int64]map[int64]json.RawMessage)

        gc.lastVersion = versionNo
        return true
}

// FilterGameList applies gray filtering to a game list for a user.
// Modifies the input slice in-place and returns it.
// Equivalent to C++ GrayCacheV2::FilterGameGroupGray.
func (gc *GrayCache) FilterGameList(games []int64, uid uint32) []int64 {
        gc.UpdateGray(false)
        Instance().FilterByUid(games, uid)
        return games
}

// FilterGameListJSON applies gray filtering to a JSON game array string.
// Parses the JSON, extracts game IDs, applies gray filter, and reassembles.
// Returns the filtered JSON string.
//
// Equivalent to C++ GrayCacheV2::GetLevel2HallGray.
//
// The JSON format is an array of game objects:
//
//      [
//        {"game_info": {"game_id": 42, ...}, ...},
//        ...
//      ]
//
// Results are cached by "path|groupID" key for grayCacheTTL duration.
func (gc *GrayCache) FilterGameListJSON(jsonStr string, uid uint32, groupID int64) string {
        gc.UpdateGray(false)

        // Check list cache first
        gameList := gc.getOrBuildListCache(jsonStr, groupID)
        if gameList == nil {
                return jsonStr
        }

        // Apply gray filter to game ID list
        path := Instance().FilterByUid(gameList, uid)

        // Build cache key: path|groupID
        cacheKey := fmt.Sprintf("%s|%d", path, groupID)

        gc.mu.RLock()
        cached, exists := gc.detailCache[cacheKey]
        gc.mu.RUnlock()

        if exists && !isExpired(cached.expires, grayCacheTTL) {
                return cached.data
        }

        // Reassemble filtered JSON from cached game data
        result := gc.assembleFilteredJSON(gameList, groupID)

        gc.mu.Lock()
        gc.detailCache[cacheKey] = &expireStr{
                data:    result,
                expires: time.Now(),
        }
        gc.mu.Unlock()

        return result
}

// getOrBuildListCache returns the game ID list for a group, building from JSON if needed.
func (gc *GrayCache) getOrBuildListCache(jsonStr string, groupID int64) []int64 {
        gc.mu.RLock()
        cached, exists := gc.listCache[groupID]
        gc.mu.RUnlock()

        if exists {
                // Return a copy to avoid concurrent modification
                result := make([]int64, len(cached))
                copy(result, cached)
                return result
        }

        // Parse JSON and build cache
        var items []json.RawMessage
        if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
                logger.Errorf("grayfilter: failed to parse game list JSON: %v", err)
                return nil
        }

        gameList := make([]int64, 0, len(items))

        gc.mu.Lock()
        defer gc.mu.Unlock()

        // Re-check after acquiring write lock
        if cached, exists := gc.listCache[groupID]; exists {
                result := make([]int64, len(cached))
                copy(result, cached)
                return result
        }

        // Ensure gameDataCache entry exists for this group
        if _, ok := gc.gameDataCache[groupID]; !ok {
                gc.gameDataCache[groupID] = make(map[int64]json.RawMessage)
        }

        for _, raw := range items {
                // Parse each item to extract game_info.game_id
                var item map[string]json.RawMessage
                if err := json.Unmarshal(raw, &item); err != nil {
                        logger.Warnf("grayfilter: skipping malformed game item: %v", err)
                        continue
                }

                gameInfoRaw, ok := item["game_info"]
                if !ok {
                        logger.Debugf("grayfilter: skipping item without game_info field")
                        continue
                }

                var gameInfo map[string]json.RawMessage
                if err := json.Unmarshal(gameInfoRaw, &gameInfo); err != nil {
                        logger.Warnf("grayfilter: skipping malformed game_info: %v", err)
                        continue
                }

                gameIDRaw, ok := gameInfo["game_id"]
                if !ok {
                        continue
                }

                gameIDStr := strings.Trim(string(gameIDRaw), "\" ")
                gameID, err := strconv.ParseInt(gameIDStr, 10, 64)
                if err != nil {
                        logger.Debugf("grayfilter: skipping item with invalid game_id: %q", gameIDStr)
                        continue
                }

                gameList = append(gameList, gameID)
                gc.gameDataCache[groupID][gameID] = raw
        }

        gc.listCache[groupID] = gameList

        logger.Debugf("grayfilter: built game list for group %d: %v", groupID, gameList)

        return gameList
}

// assembleFilteredJSON reassembles a JSON array from the cached game data,
// keeping only the game IDs in the filtered list and preserving order.
func (gc *GrayCache) assembleFilteredJSON(gameList []int64, groupID int64) string {
        gc.mu.RLock()
        gameData := gc.gameDataCache[groupID]
        gc.mu.RUnlock()

        result := make([]json.RawMessage, 0, len(gameList))
        for _, gid := range gameList {
                if raw, ok := gameData[gid]; ok {
                        result = append(result, raw)
                }
        }

        bytes, err := json.Marshal(result)
        if err != nil {
                logger.Errorf("grayfilter: failed to marshal filtered game list: %v", err)
                return "[]"
        }
        return string(bytes)
}

// ClearCaches clears all gray-related caches.
// Called when config version changes.
func (gc *GrayCache) ClearCaches() {
        gc.mu.Lock()
        defer gc.mu.Unlock()
        gc.detailCache = make(map[string]*expireStr)
        gc.listCache = make(map[int64][]int64)
        gc.gameDataCache = make(map[int64]map[int64]json.RawMessage)
}

// isExpired checks if the given expiry time has passed the TTL.
func isExpired(expires time.Time, ttl time.Duration) bool {
        return time.Since(expires) >= ttl
}
