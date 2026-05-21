package storage

import (
	"context"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return ts.UTC()
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedRow(t *testing.T, db *DB, sessionID, model string, ts time.Time, input, output, cacheCre, cacheRd int, cost float64) {
	t.Helper()
	if err := db.UpsertSession(sessionID, "claude", ts); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := db.InsertTokenUsage("", sessionID, input, output, cacheCre, cacheRd, model, cost, ts, sessionID+"-"+ts.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert usage: %v", err)
	}
}

func TestAggregateUsageDay(t *testing.T) {
	db := openTestDB(t)
	seedRow(t, db, "s1", "claude", mustTime(t, "2026-05-19T10:00:00Z"), 100, 200, 10, 50, 1.0)
	seedRow(t, db, "s1", "claude", mustTime(t, "2026-05-19T11:00:00Z"), 50, 100, 5, 25, 0.5)
	seedRow(t, db, "s2", "gpt", mustTime(t, "2026-05-20T10:00:00Z"), 300, 400, 0, 0, 2.0)

	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketDay})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Bucket != "2026-05-19" {
		t.Errorf("row 0 bucket: %q", rows[0].Bucket)
	}
	if rows[0].InputTokens != 150 {
		t.Errorf("row 0 input: %d", rows[0].InputTokens)
	}
	if rows[0].OutputTokens != 300 {
		t.Errorf("row 0 output: %d", rows[0].OutputTokens)
	}
	if rows[0].Cost != 1.5 {
		t.Errorf("row 0 cost: %v", rows[0].Cost)
	}
	if len(rows[0].Models) != 1 || rows[0].Models[0] != "claude" {
		t.Errorf("row 0 models: %v", rows[0].Models)
	}
}

func TestAggregateUsageDayBreakdown(t *testing.T) {
	db := openTestDB(t)
	seedRow(t, db, "s1", "claude", mustTime(t, "2026-05-19T10:00:00Z"), 100, 200, 0, 0, 1.0)
	seedRow(t, db, "s1", "gpt", mustTime(t, "2026-05-19T11:00:00Z"), 50, 100, 0, 0, 0.5)

	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{
		Bucket: BucketDay, Breakdown: true,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 breakdown rows, got %d", len(rows))
	}
	if rows[0].Bucket != "2026-05-19" || rows[0].Model != "claude" {
		t.Errorf("row 0 want claude, got %+v", rows[0])
	}
	if rows[1].Model != "gpt" {
		t.Errorf("row 1 want gpt, got %+v", rows[1])
	}
}

func TestAggregateUsageProjectFilter(t *testing.T) {
	db := openTestDB(t)
	seedRow(t, db, "s1", "claude", mustTime(t, "2026-05-19T10:00:00Z"), 100, 0, 0, 0, 1.0)
	seedRow(t, db, "s2", "claude", mustTime(t, "2026-05-19T11:00:00Z"), 200, 0, 0, 0, 2.0)
	// give s1 a cwd; s2 doesn't get one
	if err := db.UpdateSessionMeta("s1", "/code/foo", "main"); err != nil {
		t.Fatalf("update meta: %v", err)
	}

	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{
		Bucket: BucketDay, Project: "/code/foo",
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].InputTokens != 100 {
		t.Errorf("expected only s1 (input=100), got %d", rows[0].InputTokens)
	}
}

func TestAggregateUsageDateRange(t *testing.T) {
	db := openTestDB(t)
	seedRow(t, db, "s1", "x", mustTime(t, "2026-05-18T10:00:00Z"), 10, 0, 0, 0, 0)
	seedRow(t, db, "s2", "x", mustTime(t, "2026-05-19T10:00:00Z"), 20, 0, 0, 0, 0)
	seedRow(t, db, "s3", "x", mustTime(t, "2026-05-20T10:00:00Z"), 30, 0, 0, 0, 0)

	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{
		Bucket: BucketDay,
		Since:  mustTime(t, "2026-05-19T00:00:00Z"),
		Until:  mustTime(t, "2026-05-20T00:00:00Z"),
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 1 || rows[0].Bucket != "2026-05-19" {
		t.Fatalf("expected only 5-19, got %+v", rows)
	}
}

func TestAggregateUsageEmptyRange(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketDay})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
}

func TestAggregateUsageGroupConcatModelOrder(t *testing.T) {
	// SQLite GROUP_CONCAT(DISTINCT) order is unspecified; AggregateUsage
	// must sort Models alphabetically before returning.
	db := openTestDB(t)
	seedRow(t, db, "s1", "zzz", mustTime(t, "2026-05-19T10:00:00Z"), 1, 0, 0, 0, 0)
	seedRow(t, db, "s1", "aaa", mustTime(t, "2026-05-19T11:00:00Z"), 1, 0, 0, 0, 0)
	seedRow(t, db, "s1", "mmm", mustTime(t, "2026-05-19T12:00:00Z"), 1, 0, 0, 0, 0)

	rows, err := db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketDay})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0].Models
	want := []string{"aaa", "mmm", "zzz"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Errorf("models[%d]: got %q want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
