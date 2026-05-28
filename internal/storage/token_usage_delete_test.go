package storage

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteTokenUsageBySourceIDRollsBackAggregates(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	if err := db.UpsertSession("s-delete", "claude", now); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := db.InsertTokenUsage("agent", "s-delete", 100, 50, 7, 11, "sonnet", 0.01, now, "src-delete"); err != nil {
		t.Fatalf("insert token usage: %v", err)
	}
	assertSessionTotals(t, db, "s-delete", 100, 50, 7, 11, 0.01)
	assertDailyCost(t, db, now, 0.01)

	if err := db.DeleteTokenUsageBySourceID(context.Background(), "src-delete"); err != nil {
		t.Fatalf("delete existing source id: %v", err)
	}
	rows, err := db.ListUsageForBlocks(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows after delete = %d, want 0: %#v", len(rows), rows)
	}
	assertSessionTotals(t, db, "s-delete", 0, 0, 0, 0, 0)
	assertDailyCost(t, db, now, 0)
}

func TestDeleteTokenUsageBySourceIDMissingDoesNotError(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.DeleteTokenUsageBySourceID(context.Background(), "missing-source-id"); err != nil {
		t.Fatalf("delete missing source id: %v", err)
	}
}

func TestDeleteTokenUsageBySourceIDPreservesOtherRows(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	if err := db.UpsertSession("s-preserve", "claude", now); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := db.InsertTokenUsage("agent", "s-preserve", 100, 50, 7, 11, "sonnet", 0.01, now, "src-a"); err != nil {
		t.Fatalf("insert token usage A: %v", err)
	}
	if err := db.InsertTokenUsage("agent", "s-preserve", 30, 9, 3, 5, "sonnet", 0.02, now.Add(time.Minute), "src-b"); err != nil {
		t.Fatalf("insert token usage B: %v", err)
	}
	assertSessionTotals(t, db, "s-preserve", 130, 59, 10, 16, 0.03)
	assertDailyCost(t, db, now, 0.03)

	if err := db.DeleteTokenUsageBySourceID(context.Background(), "src-a"); err != nil {
		t.Fatalf("delete source A: %v", err)
	}
	rows, err := db.ListUsageForBlocks(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	if len(rows) != 1 || rows[0].SourceID != "src-b" {
		t.Fatalf("rows after delete A = %#v, want only src-b", rows)
	}
	assertSessionTotals(t, db, "s-preserve", 30, 9, 3, 5, 0.02)
	assertDailyCost(t, db, now, 0.02)
}

func assertSessionTotals(t *testing.T, db *DB, sessionID string, input, output, cacheCreate, cacheRead int, cost float64) {
	t.Helper()
	var gotInput, gotOutput, gotCacheCreate, gotCacheRead int
	var gotCost float64
	if err := db.db.QueryRow(`
		SELECT total_input_tokens, total_output_tokens, total_cache_creation_tokens,
		       total_cache_read_tokens, total_cost_usd
		FROM sessions WHERE session_id = ?
	`, sessionID).Scan(&gotInput, &gotOutput, &gotCacheCreate, &gotCacheRead, &gotCost); err != nil {
		t.Fatalf("query session totals: %v", err)
	}
	if gotInput != input || gotOutput != output || gotCacheCreate != cacheCreate || gotCacheRead != cacheRead || math.Abs(gotCost-cost) > 1e-9 {
		t.Fatalf("session totals = input %d output %d cacheCreate %d cacheRead %d cost %.12f, want %d/%d/%d/%d/%.12f",
			gotInput, gotOutput, gotCacheCreate, gotCacheRead, gotCost, input, output, cacheCreate, cacheRead, cost)
	}
}

func assertDailyCost(t *testing.T, db *DB, ts time.Time, cost float64) {
	t.Helper()
	var got float64
	if err := db.db.QueryRow(`SELECT COALESCE((SELECT cost_usd FROM daily_cost_cache WHERE day = DATE(?, 'localtime')), 0)`, formatStorageTime(ts)).Scan(&got); err != nil {
		t.Fatalf("query daily cost cache: %v", err)
	}
	if math.Abs(got-cost) > 1e-9 {
		t.Fatalf("daily cost = %.12f, want %.12f", got, cost)
	}
}
