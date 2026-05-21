package collector_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/tt-a1i/tokenmeter/internal/collector"
)

// gooseFixtureSchema mirrors ccusage's CREATE TABLE in
// rust/crates/ccusage/src/adapter/goose.rs tests::create_goose_db. Schema
// shape comes from the live Goose CLI binary; ccusage reverse-engineered
// it. Kept verbatim so future schema drift in either project is easy to
// diff against.
const gooseFixtureSchema = `
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    model_config_json TEXT,
    provider_name TEXT,
    created_at TEXT,
    total_tokens INTEGER,
    input_tokens INTEGER,
    output_tokens INTEGER,
    accumulated_total_tokens INTEGER,
    accumulated_input_tokens INTEGER,
    accumulated_output_tokens INTEGER
)
`

// setupGooseFixture mints a temp directory laid out the way GOOSE_PATH_ROOT
// expects ($ROOT/data/sessions/sessions.db) and seeds it with the ccusage
// sample row used by goose.rs's loads_accumulated_tokens_from_goose_sqlite
// test. Returns the root path suitable for t.Setenv.
func setupGooseFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dbDir := filepath.Join(root, "data", "sessions")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dbDir, err)
	}
	dbPath := filepath.Join(dbDir, "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(gooseFixtureSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	// Sample row: accumulated counters set so the adapter must prefer them
	// over the (NULL) per-turn columns. total > input+output by 30 -> the
	// "reasoning remainder" path folds 30 into OutputTokens.
	if _, err := db.Exec(`
INSERT INTO sessions (
    id, model_config_json, provider_name, created_at,
    accumulated_total_tokens, accumulated_input_tokens, accumulated_output_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?)
`,
		"session-a",
		`{"model_name":"claude-sonnet-4-20250514"}`,
		"anthropic",
		"2026-05-01 01:02:03",
		int64(180), int64(100), int64(50),
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return root
}

// TestLoadGooseEntriesParsesSample drives the adapter against a
// programmatically-built fixture DB. ccusage's test vector says
// input=100, output=50, total=180; our adapter folds the 30 reasoning
// remainder into OutputTokens (v1.1 UsageEntry has no reasoning slot),
// so the assertion is output=80 (50 + 30 reasoning).
func TestLoadGooseEntriesParsesSample(t *testing.T) {
	root := setupGooseFixture(t)
	t.Setenv("GOOSE_PATH_ROOT", root)

	entries, err := collector.LoadGooseEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadGooseEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Source != "goose" {
		t.Errorf("Source=%q want %q", e.Source, "goose")
	}
	if e.SessionID != "session-a" {
		t.Errorf("SessionID=%q want session-a", e.SessionID)
	}
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model=%q want claude-sonnet-4-20250514 (from model_config_json.model_name)", e.Model)
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens=%d want 100 (accumulated_input_tokens)", e.InputTokens)
	}
	// 50 base + 30 reasoning remainder (total 180 - 100 - 50) folded in.
	if e.OutputTokens != 80 {
		t.Errorf("OutputTokens=%d want 80 (50 output + 30 reasoning remainder folded in)", e.OutputTokens)
	}
	// "2026-05-01 01:02:03" -> 2026-05-01T01:02:03Z
	if e.Timestamp.Year() != 2026 || e.Timestamp.Month() != 5 || e.Timestamp.Day() != 1 {
		t.Errorf("Timestamp=%v want 2026-05-01 ...", e.Timestamp)
	}
	if e.CostUSD != 0 {
		t.Errorf("CostUSD=%v want 0 (adapter defers cost to ModeAuto recompute)", e.CostUSD)
	}
}

// TestLoadGooseEntriesMissingDirReturnsNil exercises the
// "user does not have Goose installed" path via GOOSE_PATH_ROOT
// pointing at a non-existent directory.
func TestLoadGooseEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("GOOSE_PATH_ROOT", "/nonexistent/path/abcxyz-goose-root")
	entries, err := collector.LoadGooseEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing root should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing root, got %d: %+v", len(entries), entries)
	}
}

// TestLoadGooseEntriesMissingDBFile covers the case where the env-pointed
// root directory exists but data/sessions/sessions.db is missing
// (e.g. user opened Goose but never logged a session, or wiped their DB).
// The adapter must NOT error.
func TestLoadGooseEntriesMissingDBFile(t *testing.T) {
	root := t.TempDir()
	// Intentionally don't create data/sessions/sessions.db.
	t.Setenv("GOOSE_PATH_ROOT", root)
	entries, err := collector.LoadGooseEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing DB file should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing DB file, got %d", len(entries))
	}
}

// TestLoadGooseEntriesPerTurnFallback confirms the row mapper falls back
// from the accumulated_* columns to the per-turn input_tokens /
// output_tokens / total_tokens when accumulated columns are NULL/zero.
// This is the path Goose releases that don't write accumulated_* take.
func TestLoadGooseEntriesPerTurnFallback(t *testing.T) {
	root := t.TempDir()
	dbDir := filepath.Join(root, "data", "sessions")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(dbDir, "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(gooseFixtureSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	// Per-turn columns populated; accumulated columns NULL.
	if _, err := db.Exec(`
INSERT INTO sessions (
    id, model_config_json, provider_name, created_at,
    total_tokens, input_tokens, output_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?)
`,
		"session-b",
		`{"model_name":"gpt-4o-mini"}`,
		"openai",
		"2026-05-02T12:34:56Z",
		int64(0), int64(200), int64(75),
	); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	t.Setenv("GOOSE_PATH_ROOT", root)
	entries, err := collector.LoadGooseEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadGooseEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.InputTokens != 200 || e.OutputTokens != 75 {
		t.Errorf("per-turn fallback: input=%d output=%d want 200/75", e.InputTokens, e.OutputTokens)
	}
	if e.Model != "gpt-4o-mini" {
		t.Errorf("Model=%q want gpt-4o-mini", e.Model)
	}
}
