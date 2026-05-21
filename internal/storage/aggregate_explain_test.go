package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// explainAggregate runs EXPLAIN QUERY PLAN over a representative
// AggregateUsage query and returns the concatenated detail lines. SQLite's
// EXPLAIN QUERY PLAN emits one row per access path with columns (id, parent,
// notused, detail). We only inspect detail.
func explainAggregate(t *testing.T, db *DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		out.WriteString(detail)
		out.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iter plan: %v", err)
	}
	return out.String()
}

func TestAggregateUsageDayUsesCoveringIndex(t *testing.T) {
	db := openTestDB(t)
	// Seed enough rows that the planner's cost model picks the covering
	// index over a table scan. SQLite's planner uses sqlite_stat tables;
	// with a one-row table it can correctly conclude that "a sequential
	// scan of 1 row is cheaper than an index lookup", which would defeat
	// the assertion below. ~2k rows lets the cost model converge.
	base := mustTime(t, "2026-05-19T00:00:00Z")
	for i := 0; i < 2000; i++ {
		seedRow(t, db, fmt.Sprintf("s%d", i%10), "claude",
			base.Add(time.Duration(i)*time.Minute), 100, 50, 0, 0, 1.0)
	}
	if _, err := db.db.Exec("ANALYZE"); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	// Mirror the SQL shape used by AggregateUsage for BucketDay /
	// non-breakdown / no Project filter — JOIN sessions is skipped, which is
	// what lets the planner pick the covering index.
	//
	// Shape mirrors aggregate.go:73-117 (daily no-JOIN path: needSessions=false).
	// If you change AggregateUsage SQL generation, update this string accordingly.
	q := `SELECT date(u.timestamp) AS bucket,
	             GROUP_CONCAT(DISTINCT u.model) AS model_col,
	             COALESCE(SUM(u.input_tokens), 0),
	             COALESCE(SUM(u.output_tokens), 0),
	             COALESCE(SUM(u.cache_creation_tokens), 0),
	             COALESCE(SUM(u.cache_read_tokens), 0),
	             COALESCE(SUM(u.cost_usd), 0)
	      FROM token_usage u
	      WHERE u.timestamp >= ?
	      GROUP BY date(u.timestamp)
	      ORDER BY 1 ASC`
	plan := explainAggregate(t, db, q, "2026-05-19T00:00:00.000000000Z")
	if !strings.Contains(plan, "idx_token_usage_ts_covering") {
		t.Fatalf("expected idx_token_usage_ts_covering in EXPLAIN, got:\n%s", plan)
	}
}

func TestAggregateUsageSessionUsesCoveringIndex(t *testing.T) {
	db := openTestDB(t)
	// Same rationale as the Day test — seed enough rows to convince the
	// planner that an index access path is cheaper than a scan.
	base := mustTime(t, "2026-05-19T00:00:00Z")
	for i := 0; i < 2000; i++ {
		seedRow(t, db, fmt.Sprintf("s%d", i%50), "claude",
			base.Add(time.Duration(i)*time.Minute), 100, 50, 0, 0, 1.0)
	}
	if _, err := db.db.Exec("ANALYZE"); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	// Shape mirrors aggregate.go:73-117 (session JOIN path: needSessions=true,
	// BucketSession adds MAX(s.cwd) / MAX(u.timestamp) to SELECT).
	// If you change AggregateUsage SQL generation, update this string accordingly.
	q := `SELECT u.session_id AS bucket,
	             GROUP_CONCAT(DISTINCT u.model) AS model_col,
	             COALESCE(SUM(u.input_tokens), 0),
	             COALESCE(SUM(u.output_tokens), 0),
	             COALESCE(SUM(u.cache_creation_tokens), 0),
	             COALESCE(SUM(u.cache_read_tokens), 0),
	             COALESCE(SUM(u.cost_usd), 0),
	             MAX(s.cwd),
	             MAX(u.timestamp)
	      FROM token_usage u
	      JOIN sessions s ON s.session_id = u.session_id
	      GROUP BY u.session_id
	      ORDER BY 1 ASC`
	plan := explainAggregate(t, db, q)
	// Session bucket has no leading-column predicate, so the planner may
	// pick either idx_token_usage_session (compact, 1 column) or the new
	// covering index. Both are acceptable — what matters is that we don't
	// fall back to a table SCAN of token_usage. Assert at least one
	// suitable index is hit and that we never see a raw SCAN of u.
	hasCovering := strings.Contains(plan, "idx_token_usage_ts_covering")
	hasSessionIdx := strings.Contains(plan, "idx_token_usage_session")
	if !hasCovering && !hasSessionIdx {
		t.Fatalf("expected session bucket to use an index, plan:\n%s", plan)
	}
	// A raw table scan looks like "SCAN u\n" or "SCAN token_usage" with no
	// USING INDEX. `SCAN u USING INDEX ...` is an index scan and is the
	// shape SQLite picks for the session bucket today — acceptable because
	// it traverses session_id-sorted rows for GROUP BY without touching
	// the table's main B-tree pages for the join key.
	for _, line := range strings.Split(plan, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "SCAN u") && !strings.Contains(line, "USING INDEX") && !strings.Contains(line, "USING COVERING INDEX") {
			t.Fatalf("session bucket falls back to raw table scan:\n%s", plan)
		}
		if strings.HasPrefix(line, "SCAN token_usage") && !strings.Contains(line, "USING INDEX") && !strings.Contains(line, "USING COVERING INDEX") {
			t.Fatalf("session bucket falls back to raw table scan:\n%s", plan)
		}
	}
}

// TestAggregateUsageDayExplainHumanReadable is a developer-aid test that
// prints the full EXPLAIN QUERY PLAN tree when -v is passed; it never
// fails on its own. Useful when tweaking the covering index column order.
func TestAggregateUsageDayExplainHumanReadable(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 100; i++ {
		seedRow(t, db, fmt.Sprintf("s%d", i%10), "claude",
			mustTime(t, "2026-05-19T10:00:00Z").Add(time.Duration(i)*time.Minute),
			100, 50, 0, 0, 1.0)
	}
	if _, err := db.db.Exec("ANALYZE"); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}
	// NOTE: this is a developer-aid printout. The SQL below INTENTIONALLY
	// retains JOIN sessions to show the "before JOIN elision" plan for
	// comparison. Live AggregateUsage no longer emits this SQL for daily
	// (see aggregate.go needSessions decision) — the production daily path
	// drops the JOIN so the planner picks idx_token_usage_ts_covering.
	plan := explainAggregate(t, db, `SELECT date(u.timestamp), SUM(u.input_tokens)
		FROM token_usage u JOIN sessions s ON s.session_id = u.session_id
		WHERE u.timestamp >= ? GROUP BY 1`, "2026-05-19T00:00:00.000000000Z")
	t.Logf("EXPLAIN QUERY PLAN:\n%s", plan)
}
