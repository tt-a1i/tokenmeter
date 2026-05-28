package collector_test

import (
	"context"
	"math"
	"os"
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

// TestKimiForCodingTimestampSensitivePricing pins ccusage v20's behavior:
// the default model "kimi-for-coding" stays as the visible Model on the
// UsageEntry, but CostUSD is computed by mapping the entry's timestamp to
// the per-period Moonshot price (kimi-k2.5 before cutoff, kimi-k2.6 after).
// Mirrors rust/crates/ccusage/src/adapter/kimi/parser.rs:208-254.
//
// Cutoff is 1_776_698_890_072 ms (Unix epoch milliseconds); timestamps
// strictly less than cutoff use kimi-k2.5, timestamps equal to or greater
// than cutoff use kimi-k2.6.
//
// Token usage in each fixture row: input_other=100, output=50,
// input_cache_creation=20, input_cache_read=10.
//
// Expected costs (matches ccusage test prices_default_kimi_model_by_timestamp):
//
//	k2.5:  100*0.6e-6 + 50*3e-6 + 20*0.75e-6 + 10*0.1e-6 = 0.000226
//	k2.6:  100*0.95e-6 + 50*4e-6 + 20*1.1875e-6 + 10*0.16e-6 = 0.00032035
func TestKimiForCodingTimestampSensitivePricing(t *testing.T) {
	root := t.TempDir()
	// Intentionally do NOT write config.json so the default model
	// "kimi-for-coding" is picked, exercising the timestamp-aware path.
	sessionDir := filepath.Join(root, "sessions", "g", "session-before")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir before: %v", err)
	}
	// 1_776_698_890.071 s → 1_776_698_890_071 ms < cutoff → kimi-k2.5
	beforeLine := `{"timestamp":1776698890.071,"message":{"type":"StatusUpdate","payload":{"token_usage":{"input_other":100,"output":50,"input_cache_creation":20,"input_cache_read":10},"message_id":"msg-before"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "wire.jsonl"), []byte(beforeLine), 0o644); err != nil {
		t.Fatalf("write before: %v", err)
	}
	atDir := filepath.Join(root, "sessions", "g", "session-at")
	if err := os.MkdirAll(atDir, 0o755); err != nil {
		t.Fatalf("mkdir at: %v", err)
	}
	// 1_776_698_890.072 s → 1_776_698_890_072 ms == cutoff → kimi-k2.6
	atLine := `{"timestamp":1776698890.072,"message":{"type":"StatusUpdate","payload":{"token_usage":{"input_other":100,"output":50,"input_cache_creation":20,"input_cache_read":10},"message_id":"msg-at"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(atDir, "wire.jsonl"), []byte(atLine), 0o644); err != nil {
		t.Fatalf("write at: %v", err)
	}

	t.Setenv("KIMI_DATA_DIR", root)
	entries, err := collector.LoadKimiEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKimiEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d (%+v)", len(entries), entries)
	}

	byID := map[string]collector.UsageEntry{}
	for _, e := range entries {
		byID[e.SessionID] = e
	}
	before, ok := byID["session-before"]
	if !ok {
		t.Fatalf("missing session-before entry; got: %+v", entries)
	}
	at, ok := byID["session-at"]
	if !ok {
		t.Fatalf("missing session-at entry; got: %+v", entries)
	}

	// Display model must stay "kimi-for-coding" so the user-facing
	// breakdown does not silently rebrand the row.
	if before.Model != "kimi-for-coding" {
		t.Errorf("before.Model=%q want kimi-for-coding", before.Model)
	}
	if at.Model != "kimi-for-coding" {
		t.Errorf("at.Model=%q want kimi-for-coding", at.Model)
	}

	const beforeWant = 0.000226 // k2.5 prices
	const atWant = 0.00032035   // k2.6 prices
	const epsilon = 1e-9
	if math.Abs(before.CostUSD-beforeWant) > epsilon {
		t.Errorf("before.CostUSD=%.9f want %.9f (kimi-k2.5)", before.CostUSD, beforeWant)
	}
	if math.Abs(at.CostUSD-atWant) > epsilon {
		t.Errorf("at.CostUSD=%.9f want %.9f (kimi-k2.6)", at.CostUSD, atWant)
	}
}

// TestKimiConfigModelOverridesTimestampPricing keeps non-default models
// (set via config.json "model": "...") untouched by the kimi-for-coding
// timestamp branch — cost stays at zero so the AllSource ModeAuto pass
// recomputes via pricing.Resolve on the configured model name.
func TestKimiConfigModelOverridesTimestampPricing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"model":"kimi-k2"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	sessionDir := filepath.Join(root, "sessions", "g", "session-a")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	line := `{"timestamp":1776698890.071,"message":{"type":"StatusUpdate","payload":{"token_usage":{"input_other":100,"output":50,"input_cache_creation":20,"input_cache_read":10},"message_id":"msg-a"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "wire.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatalf("write wire: %v", err)
	}

	t.Setenv("KIMI_DATA_DIR", root)
	entries, err := collector.LoadKimiEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKimiEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Model != "kimi-k2" {
		t.Errorf("Model=%q want kimi-k2 (from config)", e.Model)
	}
	if e.CostUSD != 0 {
		t.Errorf("CostUSD=%v want 0 (non-default model defers to AllSource recompute)", e.CostUSD)
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
