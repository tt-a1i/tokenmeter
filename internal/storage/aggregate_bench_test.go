package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func benchSeed(b *testing.B, db *DB, n int) {
	b.Helper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		sid := fmt.Sprintf("sess-%d", i%500)
		_ = db.UpsertSession(sid, "claude", start)
		ts := start.Add(time.Duration(i) * time.Minute)
		_ = db.InsertTokenUsage("", sid, 1000, 500, 100, 200, "claude", 0.01, ts, fmt.Sprintf("src-%d", i))
	}
}

func benchOpen(b *testing.B) *DB {
	b.Helper()
	path := b.TempDir() + "/bench.db"
	db, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	return db
}

func BenchmarkAggregateUsageDay10k(b *testing.B) {
	db := benchOpen(b)
	benchSeed(b, db, 10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketDay})
	}
}

func BenchmarkAggregateUsageDay100k(b *testing.B) {
	db := benchOpen(b)
	benchSeed(b, db, 100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketDay})
	}
}

func BenchmarkAggregateUsageSession100k(b *testing.B) {
	db := benchOpen(b)
	benchSeed(b, db, 100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.AggregateUsage(context.Background(), AggregateFilter{Bucket: BucketSession})
	}
}

func BenchmarkAggregateUsageDayBreakdown100k(b *testing.B) {
	db := benchOpen(b)
	benchSeed(b, db, 100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.AggregateUsage(context.Background(), AggregateFilter{
			Bucket: BucketDay, Breakdown: true,
		})
	}
}
