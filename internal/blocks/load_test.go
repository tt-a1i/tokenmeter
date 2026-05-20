package blocks_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/event"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func newTestStorageDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestLoadActiveReturnsOnlyActive(t *testing.T) {
	db := newTestStorageDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.UpsertSession("s1", event.PlatformClaude, now); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if err := db.InsertTokenUsage("a1", "s1", 100, 50, 0, 0, "sonnet", 0.01, now.Add(-10*time.Minute), "u1"); err != nil {
		t.Fatalf("InsertTokenUsage: %v", err)
	}
	active, err := blocks.LoadActive(ctx, db, 5*time.Hour, now)
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if active == nil {
		t.Fatalf("expected one active block, got nil")
	}
	if active.BurnRate == nil {
		t.Fatalf("active block must include burn rate")
	}
}
