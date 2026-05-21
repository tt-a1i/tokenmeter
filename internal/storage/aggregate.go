// Package storage exposes AggregateUsage as the read-side counterpart to
// ListUsageForBlocksFiltered, but doing SUM / GROUP BY work in SQLite
// rather than pulling every row across the driver boundary.
package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// AggregateBucket selects the group key used by AggregateUsage.
type AggregateBucket int

const (
	BucketDay AggregateBucket = iota
	BucketWeek
	BucketMonth
	BucketSession
)

// AggregateFilter describes the read-side query AggregateUsage runs.
type AggregateFilter struct {
	Since     time.Time
	Until     time.Time
	Project   string // sessions.cwd exact match; "" disables filter
	Bucket    AggregateBucket
	Breakdown bool           // true => add u.model as a secondary GROUP BY column
	Location  *time.Location // nil treated as UTC; affects day/week/month bucket key
}

// AggregateUsageRow is one row returned by AggregateUsage.
type AggregateUsageRow struct {
	Bucket            string   // "2026-05-19" / "2026-W21" / "2026-05" / session_id
	Model             string   // populated only when Breakdown=true
	Models            []string // populated only when Breakdown=false (distinct, sorted)
	InputTokens       int64
	OutputTokens      int64
	CacheCreateTokens int64
	CacheReadTokens   int64
	Cost              float64
	Project           string    // populated only for BucketSession
	LastActivity      time.Time // populated only for BucketSession
}

// AggregateUsage runs a push-down SUM/GROUP BY query against token_usage
// joined to sessions. The returned rows are pre-sorted by Bucket ascending
// (and by Model ascending when Breakdown=true).
func (s *DB) AggregateUsage(ctx context.Context, f AggregateFilter) ([]AggregateUsageRow, error) {
	return nil, fmt.Errorf("AggregateUsage: not implemented yet (Task 2/3/4)")
}

// _ keeps the imports tied to a real reference so this skeleton file
// compiles before Task 2/3/4 land their SQL helpers. Each import will be
// consumed by the real implementation (ctx → QueryContext, strings →
// wheres = append + Join, sort → sort.Slice over the row slice).
var (
	_ = context.Background
	_ = strings.Join
	_ = sort.Slice
)
