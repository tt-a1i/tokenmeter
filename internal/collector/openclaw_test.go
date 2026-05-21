package collector

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOpenClawEntriesParsesSample(t *testing.T) {
	// openclaw_fixtures/ is committed alongside this file and shipped with
	// the package, so the test reads against the same JSONL ccusage's own
	// inline test exercises.
	abs, err := filepath.Abs("openclaw_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("OPENCLAW_DIR", abs)

	entries, err := LoadOpenClawEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenClawEntries: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected ≥ 2 entries from fixture, got %d", len(entries))
	}

	first := entries[0]
	if first.Source != "openclaw" {
		t.Errorf("Source = %q, want openclaw", first.Source)
	}
	if first.SessionID != "abc" {
		t.Errorf("SessionID = %q, want abc (derived from filename stem)", first.SessionID)
	}
	// The model is carried over from the preceding model_change event and
	// wrapped in "[openclaw] " per ccusage's display contract.
	if first.Model != "[openclaw] gpt-5.2" {
		t.Errorf("Model = %q, want [openclaw] gpt-5.2", first.Model)
	}
	if first.InputTokens != 1660 || first.OutputTokens != 55 || first.CacheReadInputTokens != 108928 {
		t.Errorf("token mapping wrong: %+v", first)
	}
	if first.CostUSD < 0.019 || first.CostUSD > 0.021 {
		t.Errorf("CostUSD = %v, want ~0.02", first.CostUSD)
	}
	want := time.UnixMilli(1769753935279).UTC()
	if !first.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", first.Timestamp, want)
	}
}

func TestLoadOpenClawEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("OPENCLAW_DIR", "/nonexistent/path/abcxyz-openclaw-fixture")

	entries, err := LoadOpenClawEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}
