// Package grayfilter implements the GameManage gray release (canary deployment) engine.
// Ported from C++ GameManage server gamefilter module.
//
// Matcher implements user matching strategies: percentage, UID tail range,
// explicit UID list (binary search), and composite OR logic.
package grayfilter

import (
	"sort"
)

// UserMatcher is the interface for all user matching strategies.
// Equivalent to C++ UserMatchRule.
type UserMatcher interface {
	IsMatch(uid uint32) bool
}

// PercentMatcher matches users by percentage: (uid % 100) < percent.
// Equivalent to C++ UserMatchByPercent.
type PercentMatcher struct {
	Percent uint32
}

func (m *PercentMatcher) IsMatch(uid uint32) bool {
	return (uid%100) < m.Percent
}

// EndNumMatcher matches users by UID tail number range: uid%100 in [ge, lt).
// Equivalent to C++ UserMatchByEndNum.
type EndNumMatcher struct {
	Ge uint32
	Lt uint32
}

func (m *EndNumMatcher) IsMatch(uid uint32) bool {
	mod := uid % 100
	return mod >= m.Ge && mod < m.Lt
}

// UidListMatcher matches users in a specific UID set using binary search (O(log n)).
// Equivalent to C++ UserMatchByUidList (which uses std::set).
// The Uids slice is sorted at construction time for efficient lookup.
type UidListMatcher struct {
	Uids []uint32
}

// NewUidListMatcher creates a UidListMatcher with a sorted copy of the given UIDs.
func NewUidListMatcher(uids []uint32) *UidListMatcher {
	sorted := make([]uint32, len(uids))
	copy(sorted, uids)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return &UidListMatcher{Uids: sorted}
}

// IsMatch checks if the given UID exists in the sorted list using binary search.
func (m *UidListMatcher) IsMatch(uid uint32) bool {
	idx := sort.Search(len(m.Uids), func(i int) bool {
		return m.Uids[i] >= uid
	})
	return idx < len(m.Uids) && m.Uids[idx] == uid
}

// CompositeOrMatcher composites multiple UserMatchers with OR logic.
// Returns true if ANY sub-matcher matches.
// Equivalent to C++ UserMatchCompositeOr.
type CompositeOrMatcher struct {
	Matchers []UserMatcher
}

func (m *CompositeOrMatcher) IsMatch(uid uint32) bool {
	for _, sub := range m.Matchers {
		if sub.IsMatch(uid) {
			return true
		}
	}
	return false
}

// Add appends a new matcher to the composite.
func (m *CompositeOrMatcher) Add(matcher UserMatcher) {
	m.Matchers = append(m.Matchers, matcher)
}

// Size returns the number of sub-matchers.
func (m *CompositeOrMatcher) Size() int {
	return len(m.Matchers)
}
