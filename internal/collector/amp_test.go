package collector

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func writeAmpThreadFixture(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	threads := filepath.Join(root, "threads")
	if err := os.MkdirAll(threads, 0o755); err != nil {
		t.Fatalf("mkdir threads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(threads, "thread.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

// G2: When usageLedger.events is absent, fall back to parsing assistant
// messages[].usage rows (mirrors ccusage amp/parser.rs:137-236). Verifies
// that token counters, cache tokens, model, and the usage-level timestamp
// are picked up from messages[].usage.
func TestLoadAmpEntriesFallbackToMessagesUsage(t *testing.T) {
	root := writeAmpThreadFixture(t, `{
		"id":"T-thread-a",
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"inputTokens":10,
				"outputTokens":178,
				"cacheCreationInputTokens":986,
				"cacheReadInputTokens":11372,
				"timestamp":"2026-01-19T11:42:10.652Z"
			}},
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"inputTokens":5,
				"outputTokens":42,
				"cacheCreationInputTokens":0,
				"cacheReadInputTokens":12000,
				"timestamp":"2026-01-19T11:43:00.000Z"
			}}
		]
	}`)
	t.Setenv("AMP_DATA_DIR", root)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries from messages[].usage fallback, got %d", len(entries))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	first := entries[0]
	if first.Source != "amp" || first.SessionID != "T-thread-a" || first.ProjectPath != "Amp" {
		t.Errorf("entry envelope wrong: %+v", first)
	}
	if first.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q, want claude-haiku-4-5-20251001", first.Model)
	}
	if first.InputTokens != 10 || first.OutputTokens != 178 {
		t.Errorf("input/output mismatch: in=%d out=%d", first.InputTokens, first.OutputTokens)
	}
	if first.CacheCreationInputTokens != 986 || first.CacheReadInputTokens != 11372 {
		t.Errorf("cache tokens mismatch: create=%d read=%d", first.CacheCreationInputTokens, first.CacheReadInputTokens)
	}
	wantTs := time.Date(2026, 1, 19, 11, 42, 10, 652000000, time.UTC)
	if !first.Timestamp.Equal(wantTs) {
		t.Errorf("Timestamp = %v, want %v", first.Timestamp, wantTs)
	}
	if entries[1].InputTokens != 5 || entries[1].OutputTokens != 42 || entries[1].CacheReadInputTokens != 12000 {
		t.Errorf("second entry wrong: %+v", entries[1])
	}
}

// G2: usage.timestamp/model take precedence, but message-level timestamp
// and model are valid fallbacks. Mirrors ccusage's
// `non_empty_json_string(usage.get("timestamp")).or_else(message.get(...))`.
func TestLoadAmpEntriesMessagesUsageMessageLevelFallbacks(t *testing.T) {
	root := writeAmpThreadFixture(t, `{
		"id":"T-thread-b",
		"messages":[
			{"role":"assistant","timestamp":"2026-02-01T00:00:00Z","model":"msg-model","usage":{
				"inputTokens":7,
				"outputTokens":11
			}}
		]
	}`)
	t.Setenv("AMP_DATA_DIR", root)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Model != "msg-model" {
		t.Errorf("Model = %q, want msg-model (from message level)", e.Model)
	}
	if !e.Timestamp.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Timestamp = %v, want 2026-02-01T00:00:00Z (from message level)", e.Timestamp)
	}
	if e.InputTokens != 7 || e.OutputTokens != 11 {
		t.Errorf("tokens wrong: %+v", e)
	}
}

// G2: messages[].usage.totalTokens fallback maps total → output_tokens when
// all four buckets are zero. Mirrors ccusage's apply_total_token_fallback in
// the messages path.
func TestLoadAmpEntriesMessagesUsageTotalTokensFallback(t *testing.T) {
	root := writeAmpThreadFixture(t, `{
		"id":"T-thread-c",
		"messages":[
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"totalTokens":345,
				"timestamp":"2026-01-19T11:42:10.652Z"
			}}
		]
	}`)
	t.Setenv("AMP_DATA_DIR", root)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry from totalTokens fallback, got %d", len(entries))
	}
	if entries[0].OutputTokens != 345 {
		t.Errorf("totalTokens=345 must map to OutputTokens=345, got %d", entries[0].OutputTokens)
	}
}

// G2: skip messages[].usage rows where every counter is zero.
func TestLoadAmpEntriesMessagesUsageSkipsAllZero(t *testing.T) {
	root := writeAmpThreadFixture(t, `{
		"id":"T-thread-d",
		"messages":[
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"inputTokens":0,"outputTokens":0,
				"cacheCreationInputTokens":0,"cacheReadInputTokens":0,
				"timestamp":"2026-01-19T11:42:10.652Z"
			}}
		]
	}`)
	t.Setenv("AMP_DATA_DIR", root)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("all-zero usage row must be skipped, got %d entries", len(entries))
	}
}

// G2: when both usageLedger.events and messages[].usage are present and
// the ledger is non-empty, ledger rows win and messages[].usage is ignored.
// Mirrors ccusage's `ledger_events_take_precedence_over_messages_usage`.
func TestLoadAmpEntriesLedgerTakesPrecedenceOverMessages(t *testing.T) {
	root := writeAmpThreadFixture(t, `{
		"id":"thread-precedence",
		"usageLedger":{"events":[{
			"id":"event-a",
			"timestamp":"2026-01-02T00:00:00.000Z",
			"model":"gpt-5",
			"tokens":{"input":1,"output":2}
		}]},
		"messages":[
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"inputTokens":99,"outputTokens":99,
				"timestamp":"2026-01-19T11:42:10.652Z"
			}}
		]
	}`)
	t.Setenv("AMP_DATA_DIR", root)

	entries, err := LoadAmpEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadAmpEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 ledger row (precedence), got %d", len(entries))
	}
	if entries[0].Model != "gpt-5" || entries[0].InputTokens != 1 || entries[0].OutputTokens != 2 {
		t.Errorf("ledger row should win, got %+v", entries[0])
	}
}

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
