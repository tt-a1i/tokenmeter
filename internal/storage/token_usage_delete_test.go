package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteTokenUsageBySourceID(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	if err := db.UpsertSession("s-delete", "claude", now); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := db.InsertTokenUsage("agent", "s-delete", 10, 5, 0, 0, "sonnet", 0.01, now, "src-delete"); err != nil {
		t.Fatalf("insert token usage: %v", err)
	}

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
	if err := db.DeleteTokenUsageBySourceID(context.Background(), "missing-source-id"); err != nil {
		t.Fatalf("delete missing source id: %v", err)
	}
}
