package collector

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLoadAmpEntriesParsesSample(t *testing.T) {
	abs, err := filepath.Abs("amp_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("AMP_DATA_DIR", abs)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (3 ledger events in fixture), got %d", len(entries))
	}

	// Sort by event id (which sorts the same as our fixture's chronological
	// timestamps) so the assertions don't depend on the load loop's
	// per-thread ordering.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	first := entries[0]
	if first.Source != "amp" {
		t.Errorf("Source = %q, want amp", first.Source)
	}
	if first.SessionID != "thread-abc" {
		t.Errorf("SessionID = %q, want thread-abc", first.SessionID)
	}
	if first.Model != "gpt-5-amp" {
		t.Errorf("Model = %q, want gpt-5-amp", first.Model)
	}
	if first.InputTokens != 500 || first.OutputTokens != 250 {
		t.Errorf("token mapping wrong: %+v", first)
	}
	wantTs := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC)
	if !first.Timestamp.Equal(wantTs) {
		t.Errorf("Timestamp = %v, want %v", first.Timestamp, wantTs)
	}
}

func TestLoadAmpEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("AMP_DATA_DIR", "/nonexistent/path/abcxyz-amp-fixture")

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadAmpEntriesJoinsThreadAndLedger(t *testing.T) {
	// Cache token fields live on messages[] but the per-event UsageEntry
	// needs them too. The adapter does an in-memory map from
	// messageId → (cacheCreate, cacheRead) and joins via event.toMessageId.
	// Events with no toMessageId or an unmatched id must surface zeros.
	abs, err := filepath.Abs("amp_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("AMP_DATA_DIR", abs)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// ev-1 → messageId=1 → (1024, 8192)
	if entries[0].CacheCreationInputTokens != 1024 || entries[0].CacheReadInputTokens != 8192 {
		t.Errorf("ev-1 join wrong: CacheCreation=%d CacheRead=%d (want 1024/8192)",
			entries[0].CacheCreationInputTokens, entries[0].CacheReadInputTokens)
	}
	// ev-2 → messageId=3 → (256, 4096)
	if entries[1].CacheCreationInputTokens != 256 || entries[1].CacheReadInputTokens != 4096 {
		t.Errorf("ev-2 join wrong: CacheCreation=%d CacheRead=%d (want 256/4096)",
			entries[1].CacheCreationInputTokens, entries[1].CacheReadInputTokens)
	}
	// ev-3 → no toMessageId → (0, 0)
	if entries[2].CacheCreationInputTokens != 0 || entries[2].CacheReadInputTokens != 0 {
		t.Errorf("ev-3 must surface zero cache (no toMessageId): %+v", entries[2])
	}
}
