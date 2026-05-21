// Package storage exposes AggregateUsage as the read-side counterpart to
// ListUsageForBlocksFiltered, but doing SUM / GROUP BY work in SQLite
// rather than pulling every row across the driver boundary.
package storage

import (
	"context"
	"database/sql"
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
	bucketExpr, err := s.bucketExpr(f.Bucket, f.Location)
	if err != nil {
		return nil, err
	}

	var (
		modelCol  string
		groupCols string
		orderCols string
	)
	if f.Breakdown {
		modelCol = "u.model"
		groupCols = bucketExpr + ", u.model"
		orderCols = "1 ASC, u.model ASC"
	} else {
		modelCol = "GROUP_CONCAT(DISTINCT u.model)"
		groupCols = bucketExpr
		orderCols = "1 ASC"
	}

	q := `SELECT ` + bucketExpr + ` AS bucket,
	             ` + modelCol + ` AS model_col,
	             COALESCE(SUM(u.input_tokens), 0),
	             COALESCE(SUM(u.output_tokens), 0),
	             COALESCE(SUM(u.cache_creation_tokens), 0),
	             COALESCE(SUM(u.cache_read_tokens), 0),
	             COALESCE(SUM(u.cost_usd), 0)`
	if f.Bucket == BucketSession {
		q += `, MAX(s.cwd) AS project,
		         MAX(u.timestamp) AS last_activity`
	}
	// Sessions are only needed when filtering by Project or surfacing
	// MAX(s.cwd) for BucketSession. Skipping the dead JOIN unblocks SQLite
	// from picking idx_token_usage_ts_covering for daily/weekly/monthly —
	// the planner otherwise nests via idx_token_usage_session and ignores
	// the covering index entirely. token_usage.session_id has a FK to
	// sessions(session_id) so result rows are unaffected.
	needSessions := f.Project != "" || f.Bucket == BucketSession
	if needSessions {
		q += `
	FROM token_usage u
	JOIN sessions s ON s.session_id = u.session_id`
	} else {
		q += `
	FROM token_usage u`
	}

	var args []any
	var wheres []string
	if !f.Since.IsZero() {
		wheres = append(wheres, "u.timestamp >= ?")
		args = append(args, formatStorageTime(f.Since))
	}
	if !f.Until.IsZero() {
		wheres = append(wheres, "u.timestamp <= ?")
		args = append(args, formatStorageTime(f.Until))
	}
	if f.Project != "" {
		wheres = append(wheres, "s.cwd = ?")
		args = append(args, f.Project)
	}
	if len(wheres) > 0 {
		q += " WHERE " + strings.Join(wheres, " AND ")
	}
	q += " GROUP BY " + groupCols + " ORDER BY " + orderCols

	rowsIter, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregate usage: %w", err)
	}
	defer rowsIter.Close()

	var out []AggregateUsageRow
	for rowsIter.Next() {
		var (
			r          AggregateUsageRow
			modelStr   sql.NullString
			project    sql.NullString
			lastActStr sql.NullString
		)
		dest := []any{
			&r.Bucket, &modelStr,
			&r.InputTokens, &r.OutputTokens,
			&r.CacheCreateTokens, &r.CacheReadTokens, &r.Cost,
		}
		if f.Bucket == BucketSession {
			dest = append(dest, &project, &lastActStr)
		}
		if err := rowsIter.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan aggregate row: %w", err)
		}
		if f.Breakdown {
			r.Model = modelStr.String
		} else {
			r.Models = splitAndSortModels(modelStr.String)
		}
		if f.Bucket == BucketSession {
			r.Project = project.String
			if lastActStr.Valid {
				if ts, ok := parseStorageTime(lastActStr.String); ok {
					r.LastActivity = ts
				}
			}
		}
		out = append(out, r)
	}
	return out, rowsIter.Err()
}

func (s *DB) bucketExpr(bucket AggregateBucket, loc *time.Location) (string, error) {
	tzMod := ""
	if loc != nil && loc != time.UTC {
		_, offset := time.Now().In(loc).Zone()
		// SQLite date modifiers take "+N hours" etc. We pass seconds for precision.
		tzMod = fmt.Sprintf(", '%+d seconds'", offset)
	}
	switch bucket {
	case BucketDay:
		return "date(u.timestamp" + tzMod + ")", nil
	case BucketWeek:
		return "strftime('%Y-W%W', u.timestamp" + tzMod + ")", nil
	case BucketMonth:
		return "strftime('%Y-%m', u.timestamp" + tzMod + ")", nil
	case BucketSession:
		return "u.session_id", nil
	default:
		return "", fmt.Errorf("unknown AggregateBucket %d", bucket)
	}
}

func splitAndSortModels(joined string) []string {
	if joined == "" {
		return nil
	}
	parts := strings.Split(joined, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
