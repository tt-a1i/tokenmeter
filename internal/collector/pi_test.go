package collector

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPiEntriesParsesSample(t *testing.T) {
	abs, err := filepath.Abs("pi_fixtures/sessions")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("PI_AGENT_DIR", abs)

	entries, err := LoadPiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadPiEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (one assistant message in fixture), got %d", len(entries))
	}
	e := entries[0]
	if e.Source != "pi" {
		t.Errorf("Source = %q, want pi", e.Source)
	}
	// filename stem `prefix_session-a` split on first `_` → `session-a`
	if e.SessionID != "session-a" {
		t.Errorf("SessionID = %q, want session-a", e.SessionID)
	}
	if e.InputTokens != 1500 || e.OutputTokens != 420 {
		t.Errorf("input/output tokens wrong: %+v", e)
	}
	if e.CacheReadInputTokens != 8192 || e.CacheCreationInputTokens != 256 {
		t.Errorf("cache fields wrong: %+v", e)
	}
	if e.CostUSD < 0.053 || e.CostUSD > 0.054 {
		t.Errorf("CostUSD = %v, want ~0.0537 (from usage.cost.total)", e.CostUSD)
	}
	wantTs := time.Date(2026, 4, 15, 10, 30, 0, 0, time.UTC)
	if !e.Timestamp.Equal(wantTs) {
		t.Errorf("Timestamp = %v, want %v", e.Timestamp, wantTs)
	}
}

func TestLoadPiEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("PI_AGENT_DIR", "/nonexistent/path/abcxyz-pi-fixture")

	entries, err := LoadPiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadPiEntriesModelPrefix(t *testing.T) {
	// ccusage prepends "[pi] " to the model name as a display contract so
	// the AllSource summary clearly attributes the row to the pi-agent.
	abs, err := filepath.Abs("pi_fixtures/sessions")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("PI_AGENT_DIR", abs)

	entries, err := LoadPiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadPiEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Model, "[pi] ") {
		t.Errorf("Model = %q, want \"[pi] \" prefix", entries[0].Model)
	}
	if entries[0].Model != "[pi] gpt-5.4" {
		t.Errorf("Model = %q, want \"[pi] gpt-5.4\"", entries[0].Model)
	}
}
