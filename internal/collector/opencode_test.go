package collector_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/collector"
)

// TestLoadOpenCodeEntriesParsesSample drives the adapter against the
// ccusage-borrowed fixture in opencode_fixtures/. The first sample
// message lives at storage/message/sample-a/message.json with
// cost=0.02; the adapter must read it through, surface OpenCode's
// stored cost via CostUSD, and translate token names per parser.rs
// (input/output/cache.write/cache.read).
func TestLoadOpenCodeEntriesParsesSample(t *testing.T) {
	fix, err := filepath.Abs("opencode_fixtures")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	t.Setenv("OPENCODE_DATA_DIR", fix)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("expected >= 1 entry, got %d", len(entries))
	}
	// Entries sort ascending by timestamp; sample-a is 1767312000000 ms
	// (2026-01-02 UTC) and sample-b is 1767398400000 (2026-01-03), so
	// sample-a is entries[0].
	e := entries[0]
	if e.Source != "opencode" {
		t.Errorf("Source=%q want %q", e.Source, "opencode")
	}
	if e.SessionID != "session-a" {
		t.Errorf("SessionID=%q want %q", e.SessionID, "session-a")
	}
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model=%q want %q", e.Model, "claude-sonnet-4-20250514")
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens=%d want 100 (tokens.input)", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("OutputTokens=%d want 50 (tokens.output)", e.OutputTokens)
	}
	if e.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens=%d want 20 (tokens.cache.write)", e.CacheCreationInputTokens)
	}
	if e.CacheReadInputTokens != 10 {
		t.Errorf("CacheReadInputTokens=%d want 10 (tokens.cache.read)", e.CacheReadInputTokens)
	}
	if e.CostUSD != 0.02 {
		t.Errorf("CostUSD=%v want 0.02 (positive cost passthrough)", e.CostUSD)
	}
	if e.Timestamp.IsZero() {
		t.Errorf("Timestamp not parsed from time.created milliseconds")
	}
}

// TestLoadOpenCodeEntriesMissingDirReturnsNil exercises the
// "user does not have OpenCode installed" path.
func TestLoadOpenCodeEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("OPENCODE_DATA_DIR", "/nonexistent/path/abcxyz-opencode")
	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing dir, got %d: %+v", len(entries), entries)
	}
}

// TestLoadOpenCodeEntriesNestedStructure verifies the adapter walks
// arbitrarily-nested directories under storage/message/. The fixture
// includes sample-a/message.json AND sample-b/nested/message.json —
// both must surface, and entries must be sorted by timestamp.
func TestLoadOpenCodeEntriesNestedStructure(t *testing.T) {
	fix, err := filepath.Abs("opencode_fixtures")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	t.Setenv("OPENCODE_DATA_DIR", fix)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (sample-a + sample-b/nested), got %d: %+v", len(entries), entries)
	}
	if entries[0].SessionID != "session-a" || entries[1].SessionID != "session-b" {
		t.Errorf("entries not sorted by timestamp; got SessionIDs %q, %q", entries[0].SessionID, entries[1].SessionID)
	}
	// sample-b lives under sample-b/nested/message.json — proves we
	// recurse rather than only reading the immediate children.
	if entries[1].InputTokens != 200 {
		t.Errorf("nested sample-b InputTokens=%d want 200", entries[1].InputTokens)
	}
	if entries[1].CostUSD != 0.05 {
		t.Errorf("nested sample-b CostUSD=%v want 0.05", entries[1].CostUSD)
	}
}
