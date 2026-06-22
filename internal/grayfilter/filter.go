package grayfilter

import (
        "encoding/json"
        "sync"

        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// FilterPair pairs a user matcher with a game operator.
// Equivalent to std::pair<UserMatchRulePtr, GameOperateRulePtr> in C++.
type FilterPair struct {
        Matcher  UserMatcher
        Operator GameOperator
}

// GameFilter is the gray release filter engine (singleton, thread-safe).
// Ported from C++ GameFilter class.
//
// GameFilter maintains a chain of FilterPair rules. For each user request:
//   1. Iterate through all filter pairs
//   2. Check if the user matches (via UserMatcher)
//   3. Apply the corresponding game operation (via GameOperator)
//   4. Track the match path for debugging/caching (e.g., "tft" = matched, not, matched)
//
// Thread safety is achieved via sync.RWMutex, equivalent to C++ vector::swap pattern.
type GameFilter struct {
        mu      sync.RWMutex
        filters []FilterPair
}

var (
        instance *GameFilter
        once     sync.Once
)

// Instance returns the singleton GameFilter instance.
// Equivalent to GameFilter::Instance() via CSingleton<GameFilter> in C++.
func Instance() *GameFilter {
        once.Do(func() {
                instance = &GameFilter{}
        })
        return instance
}

// FilterByUid applies all filter rules to a game list for a given user.
// Returns the match path string (e.g., "tft" where t=matched, f=not matched).
// This path is used as a cache key for the filtered result.
//
// Equivalent to C++ GameFilter::FilterByUid.
func (f *GameFilter) FilterByUid(games []int64, uid uint32) string {
        f.mu.RLock()
        defer f.mu.RUnlock()

        if len(games) == 0 {
                return ""
        }

        path := make([]byte, 0, len(f.filters))
        for _, pair := range f.filters {
                matched := pair.Matcher.IsMatch(uid)
                pair.Operator.Op(games, matched)
                if matched {
                        path = append(path, 't')
                } else {
                        path = append(path, 'f')
                }
        }
        return string(path)
}

// --- JSON Config Types ---

// grayConfig is the top-level gray config JSON structure stored in Redis.
// Matches the C++ config format:
//
//      {
//        "config": [...],
//        "version_no": "0.20222000 1727683114"
//      }
type grayConfig struct {
        Config    []filterConfigItem `json:"config"`
        VersionNo string             `json:"version_no"`
}

// filterConfigItem represents a single filter rule in the config array.
// Matches the C++ filter config item format:
//
//      {
//        "is_open": 1,
//        "rate": 10,
//        "uid_tails": [{"ge": 20, "lt": 35}],
//        "uid_list": [1001, 1002],
//        "type": 2,             // 1=whitelist(filter), 2=replace
//        "replace_games": [{"from_game_id": 3, "to_game_id": 42}],
//        "single_games": [5, 6, 7]
//      }
type filterConfigItem struct {
        IsOpen       int              `json:"is_open"`
        Rate         int              `json:"rate"`
        UidTails     []uidTailRange   `json:"uid_tails"`
        UidList      []int64          `json:"uid_list"`
        Type         int              `json:"type"`
        ReplaceGames []replaceGameDef `json:"replace_games"`
        SingleGames  []int64          `json:"single_games"`
}

// uidTailRange defines a UID tail number range [ge, lt).
type uidTailRange struct {
        Ge int `json:"ge"`
        Lt int `json:"lt"`
}

// replaceGameDef defines a game replacement rule in the config.
type replaceGameDef struct {
        FromGameID int64 `json:"from_game_id"`
        ToGameID   int64 `json:"to_game_id"`
}

// ReloadFromConfig parses JSON config and atomically replaces the filter chain.
// Uses swap for thread-safe atomic replacement (like C++ m_user_game_filter.swap).
//
// The config JSON must be an array of filter config items.
// For each item with is_open != 0:
//  1. Build CompositeOrMatcher from rate/uid_tails/uid_list (OR logic: any match = match)
//  2. Build CompositeAndOperator based on type (1=whitelist, 2=replace)
//  3. Add FilterPair to new filters slice
//
// Returns error if parsing fails or any rule is invalid.
// Equivalent to C++ GameFilter::ReloadConfByUid.
func (f *GameFilter) ReloadFromConfig(configJSON []byte) error {
        var items []filterConfigItem
        if err := json.Unmarshal(configJSON, &items); err != nil {
                logger.Errorf("grayfilter: failed to parse config JSON: %v", err)
                return err
        }

        newFilters := make([]FilterPair, 0, len(items))

        for idx, item := range items {
                if item.IsOpen == 0 {
                        continue
                }

                // Build user matcher (CompositeOr: any condition matches = user is gray)
                userMatcher := buildUserMatcher(item)
                if userMatcher.Size() == 0 {
                        // I9: log and skip invalid items instead of aborting the entire reload
                        logger.Warnf("grayfilter: filter item[%d] has no valid matcher conditions, skipping", idx)
                        continue
                }

                // Build game operator based on type
                gameOperator := buildGameOperator(item)
                if gameOperator.Size() == 0 {
                        // I9: log and skip invalid items instead of aborting the entire reload
                        logger.Warnf("grayfilter: filter item[%d] has no valid operator rules, skipping", idx)
                        continue
                }

                newFilters = append(newFilters, FilterPair{
                        Matcher:  userMatcher,
                        Operator: gameOperator,
                })
        }

        // Atomic swap (equivalent to C++ m_user_game_filter.swap)
        f.mu.Lock()
        f.filters = newFilters
        f.mu.Unlock()

        logger.Infof("grayfilter: reloaded %d filter rules successfully", len(newFilters))
        return nil
}

// buildUserMatcher constructs a CompositeOrMatcher from the config item.
// Combines rate, uid_tails, and uid_list into an OR matcher.
// At least one condition must be present.
func buildUserMatcher(item filterConfigItem) *CompositeOrMatcher {
        matcher := &CompositeOrMatcher{}

        // Rate-based matching
        if item.Rate > 0 {
                matcher.Add(&PercentMatcher{Percent: uint32(item.Rate)})
        }

        // UID tail range matching
        for _, tail := range item.UidTails {
                matcher.Add(&EndNumMatcher{
                        Ge: uint32(tail.Ge),
                        Lt: uint32(tail.Lt),
                })
        }

        // Explicit UID list matching
        if len(item.UidList) > 0 {
                uids := make([]uint32, 0, len(item.UidList))
                for _, uid := range item.UidList {
                        uids = append(uids, uint32(uid))
                }
                matcher.Add(NewUidListMatcher(uids))
        }

        return matcher
}

// buildGameOperator constructs a CompositeAndOperator from the config item.
// Type 1 = whitelist (filter), Type 2 = replace.
func buildGameOperator(item filterConfigItem) *CompositeAndOperator {
        operator := &CompositeAndOperator{}

        switch item.Type {
        case 1: // Whitelist/filter mode
                if len(item.SingleGames) > 0 {
                        wl := &WhiteListOperator{GameIDs: make(map[int64]bool, len(item.SingleGames))}
                        for _, gid := range item.SingleGames {
                                wl.GameIDs[gid] = true
                        }
                        operator.Add(wl)
                }

        case 2: // Replace mode
                if len(item.ReplaceGames) > 0 {
                        for _, rg := range item.ReplaceGames {
                                operator.Add(&ReplaceOperator{
                                        Rules: []ReplaceRule{
                                                {
                                                        FromGameID: rg.FromGameID,
                                                        ToGameID:   rg.ToGameID,
                                                },
                                        },
                                })
                        }
                }
        }

        return operator
}

// --- Sentinel errors ---

var (
        errNoMatcher = &configError{"no valid user matcher conditions"}
        errNoOperator = &configError{"no valid game operator rules"}
)

type configError struct {
        msg string
}

func (e *configError) Error() string {
        return "grayfilter config error: " + e.msg
}
