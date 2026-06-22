package grayfilter

// GameOperator is the interface for game list operations.
// Equivalent to C++ GameOperateRule.
type GameOperator interface {
        Op(games []int64, isMatch bool)
}

// ReplaceRule defines a single game replacement rule: FromGameID -> ToGameID.
// Used by ReplaceOperator.
type ReplaceRule struct {
        FromGameID int64
        ToGameID   int64
}

// ReplaceOperator applies game replacement rules.
// If matched: removes FromGameID (old game), keeps ToGameID (new game).
// If not matched: removes ToGameID (new game), keeps FromGameID (old game).
// Equivalent to C++ GameOpByReplace.
//
// Both FromGameID and ToGameID must exist in the list for the rule to take effect.
type ReplaceOperator struct {
        Rules []ReplaceRule
}

func (o *ReplaceOperator) Op(games []int64, isMatch bool) {
        for _, rule := range o.Rules {
                o.applyRule(&games, rule, isMatch)
        }
}

// applyRule applies a single replace rule to the game list.
// Mirrors the C++ GameOpByReplace::Op behavior exactly:
//  - Find both old and new game IDs in the list
//  - If matched, erase the old game ID (showing the new game)
//  - If not matched, erase the new game ID (showing the old game)
func (o *ReplaceOperator) applyRule(games *[]int64, rule ReplaceRule, isMatch bool) {
        oldIdx := indexOf(*games, rule.FromGameID)
        if oldIdx < 0 {
                return
        }
        newIdx := indexOf(*games, rule.ToGameID)
        if newIdx < 0 {
                return
        }

        if isMatch {
                // Remove the old game, keep the new game
                *games = removeAt(*games, oldIdx)
                // After removing oldIdx, newIdx shifts if it was after oldIdx
                if newIdx > oldIdx {
                        newIdx--
                }
        } else {
                // Remove the new game, keep the old game
                *games = removeAt(*games, newIdx)
        }
}

// WhiteListOperator applies whitelist game filtering.
// If matched: keep all games (no removal).
// If not matched: remove all whitelisted games from the list.
// Equivalent to C++ GameOpByWhiteList.
type WhiteListOperator struct {
        GameIDs map[int64]bool
}

func (o *WhiteListOperator) Op(games []int64, isMatch bool) {
        if isMatch {
                return // keep everything
        }
        // Remove all whitelisted games from the list (matching C++ behavior: remove while duplicates exist)
        // Use copy to modify the underlying array in-place so the caller sees the changes
        // even though the slice header is passed by value through the interface.
        for gameID := range o.GameIDs {
                for {
                        idx := indexOf(games, gameID)
                        if idx < 0 {
                                break
                        }
                        // Shift elements left and shorten (in-place, no cap change)
                        n := len(games) - 1
                        copy(games[idx:], games[idx+1:])
                        games[n] = 0
                        games = games[:n]
                }
        }
}

// AddGameID adds a game ID to the whitelist.
func (o *WhiteListOperator) AddGameID(gameID int64) {
        if o.GameIDs == nil {
                o.GameIDs = make(map[int64]bool)
        }
        o.GameIDs[gameID] = true
}

// Size returns the number of whitelisted game IDs.
func (o *WhiteListOperator) Size() int {
        return len(o.GameIDs)
}

// CompositeAndOperator composites multiple GameOperators with AND logic.
// Applies all sub-operators in sequence.
// Equivalent to C++ GameOpCompositeAnd.
type CompositeAndOperator struct {
        Operators []GameOperator
}

func (o *CompositeAndOperator) Op(games []int64, isMatch bool) {
        for _, op := range o.Operators {
                op.Op(games, isMatch)
        }
}

// Add appends a new operator to the composite.
func (o *CompositeAndOperator) Add(op GameOperator) {
        o.Operators = append(o.Operators, op)
}

// Size returns the number of sub-operators.
func (o *CompositeAndOperator) Size() int {
        return len(o.Operators)
}

// --- Helper functions for in-place slice modification ---

// indexOf returns the first index of target in the slice, or -1 if not found.
func indexOf(slice []int64, target int64) int {
        for i, v := range slice {
                if v == target {
                        return i
                }
        }
        return -1
}

// removeAt removes the element at the given index from the slice in-place,
// preserving order. Returns the same slice (shortened by 1).
func removeAt(slice []int64, idx int) []int64 {
        return append(slice[:idx], slice[idx+1:]...)
}
