// Package blocks identifies and aggregates 5-hour session blocks from
// token_usage entries, computing burn rate and projection for the currently
// active window. See docs/superpowers/specs/2026-05-20-ccusage-aligned-refactor-design.md
// §4.2.
package blocks

import "time"

// TokenCounts aggregates token-counter fields for one block.
type TokenCounts struct {
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
}

// Total returns the sum of every counter, mirroring how ccusage displays
// "tokens" in blocks output.
func (t TokenCounts) Total() int64 {
	return t.Input + t.Output + t.CacheCreate + t.CacheRead
}

// BurnRate captures the consumption velocity of an active block.
type BurnRate struct {
	TokensPerMinute float64
	CostPerHour     float64
}

// Projection extrapolates the active block's totals to its scheduled end
// based on the current burn rate.
type Projection struct {
	TotalTokens   int64
	TotalCost     float64
	RemainingTime time.Duration
}

// SessionBlock represents a contiguous activity window bounded by
// SessionDuration (default 5h) of silence.
type SessionBlock struct {
	StartTime time.Time
	EndTime   time.Time
	// ActualEnd is the timestamp of the last entry in the block.
	// Nil for gap blocks.
	ActualEnd  *time.Time
	IsActive   bool
	IsGap      bool
	Tokens     TokenCounts
	Cost       float64
	Models     []string // unique, insertion-ordered
	BurnRate   *BurnRate
	Projection *Projection
	EntryCount int
}
