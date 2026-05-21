package collector_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/collector"
)

// TestLoadKimiEntriesParsesSample drives the adapter against the ccusage-
// borrowed fixture in kimi_fixtures/. The fixture contains three wire.jsonl
// lines: a metadata line (skipped), a TurnBegin line (skipped), and a
// StatusUpdate line carrying the token_usage payload. Exactly one
// UsageEntry should come back with the field mapping ccusage's kimi.rs
// defines (input_other → InputTokens, output → OutputTokens,
// input_cache_creation → CacheCreationInputTokens, input_cache_read →
// CacheReadInputTokens).
func TestLoadKimiEntriesParsesSample(t *testing.T) {
	fix, err := filepath.Abs("kimi_fixtures")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	t.Setenv("KIMI_DATA_DIR", fix)

	entries, err := collector.LoadKimiEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKimiEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (entries=%+v)", len(entries), entries)
	}
	e := entries[0]
	if e.Source != "kimi" {
		t.Errorf("Source=%q want %q", e.Source, "kimi")
	}
	if e.SessionID != "session-a" {
		t.Errorf("SessionID=%q want %q", e.SessionID, "session-a")
	}
	if e.Model != "kimi-k2" {
		t.Errorf("Model=%q want %q (should come from config.json)", e.Model, "kimi-k2")
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens=%d want 100 (input_other)", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("OutputTokens=%d want 50", e.OutputTokens)
	}
	if e.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens=%d want 20 (input_cache_creation)", e.CacheCreationInputTokens)
	}
	if e.CacheReadInputTokens != 10 {
		t.Errorf("CacheReadInputTokens=%d want 10 (input_cache_read)", e.CacheReadInputTokens)
	}
	if e.Timestamp.IsZero() {
		t.Errorf("Timestamp not parsed from unix-seconds float")
	}
}

// TestLoadKimiEntriesMissingDirReturnsNil exercises the "user does not have
// Kimi installed" path. The adapter must NOT return an error — the merge
// loop in RunAggregateAllSource treats nil/nil as "this source contributes
// nothing this run" and continues.
func TestLoadKimiEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("KIMI_DATA_DIR", "/nonexistent/path/abcxyz-kimi")
	entries, err := collector.LoadKimiEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing dir, got %d: %+v", len(entries), entries)
	}
}
