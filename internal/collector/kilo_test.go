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

// kiloFixtureSchema is the CREATE TABLE statement borrowed verbatim
// from ccusage v20 (rust/crates/ccusage/src/adapter/kilo.rs:410
// create_db_message function body). Kilo is "schemaless" — all
// model/token/cost data lives inside JSON in the `data` column.
const kiloFixtureSchema = `CREATE TABLE message (id TEXT, session_id TEXT, data TEXT)`

// setupKiloFixture mints a temp Kilo root containing kilo.db with one
// populated `message` table row. Returns the root directory path
// suitable for $KILO_DATA_DIR.
//
// Default sample JSON matches ccusage's loads_kilo_messages_from_sqlite
// test vector with role/model/tokens/cost fields populated.
func setupKiloFixture(t *testing.T, jsonData string) string {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(root, "kilo.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(kiloFixtureSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, session_id, data) VALUES (?, ?, ?)`,
		"row-1", "session-fallback-a", jsonData); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return root
}

// TestLoadKiloEntriesParsesSample drives the adapter against the
// ccusage-style sample message (role=assistant + tokens + cost).
// Verifies JSON field extraction including the cache.write -> Creation
// and cache.read -> Read remapping and time.created seconds-or-ms
// normalization.
func TestLoadKiloEntriesParsesSample(t *testing.T) {
	root := setupKiloFixture(t, `{
		"role":"assistant",
		"id":"msg-1",
		"session_id":"session-from-json",
		"providerID":"anthropic",
		"modelID":"claude-sonnet-4-20250514",
		"time":{"created":1767312000},
		"tokens":{"input":100,"output":50,"cache":{"read":10,"write":20}},
		"cost":0.02
	}`)
	t.Setenv("KILO_DATA_DIR", root)

	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKiloEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Source != "kilo" {
		t.Errorf("Source=%q want kilo", e.Source)
	}
	// JSON session_id wins over the row's session_id column.
	if e.SessionID != "session-from-json" {
		t.Errorf("SessionID=%q want session-from-json (JSON wins over row column)", e.SessionID)
	}
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model=%q want claude-sonnet-4-20250514", e.Model)
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens=%d want 100", e.InputTokens)
	}
	if e.OutputTokens != 50 {
		t.Errorf("OutputTokens=%d want 50", e.OutputTokens)
	}
	if e.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens=%d want 20 (cache.write)", e.CacheCreationInputTokens)
	}
	if e.CacheReadInputTokens != 10 {
		t.Errorf("CacheReadInputTokens=%d want 10 (cache.read)", e.CacheReadInputTokens)
	}
	if e.CostUSD != 0.02 {
		t.Errorf("CostUSD=%v want 0.02 (passthrough)", e.CostUSD)
	}
	// time.created = 1767312000 (seconds) -> 2026-01-02 00:00:00 UTC
	if e.Timestamp.Year() != 2026 || e.Timestamp.Month() != 1 || e.Timestamp.Day() != 2 {
		t.Errorf("Timestamp=%v want 2026-01-02 (1767312000 seconds normalized)", e.Timestamp)
	}
}

// TestLoadKiloEntriesMissingDirReturnsNil exercises the
// "user does not have Kilo installed" path via KILO_DATA_DIR pointing
// at a non-existent directory.
func TestLoadKiloEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("KILO_DATA_DIR", "/nonexistent/path/abcxyz-kilo")
	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing dir, got %d", len(entries))
	}
}

// TestLoadKiloEntriesMissingDBFile covers an existing root directory
// whose kilo.db file is absent.
func TestLoadKiloEntriesMissingDBFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KILO_DATA_DIR", root)
	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing db file should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing db file, got %d", len(entries))
	}
}

// TestLoadKiloEntriesNonAssistantSkipped verifies the role filter:
// user messages (and any non-"assistant" role) must NOT contribute
// UsageEntries even when they happen to carry a tokens payload.
func TestLoadKiloEntriesNonAssistantSkipped(t *testing.T) {
	root := setupKiloFixture(t, `{
		"role":"user",
		"modelID":"claude-sonnet-4-20250514",
		"time":{"created":1767312000},
		"tokens":{"input":999,"output":999},
		"cost":99.99
	}`)
	t.Setenv("KILO_DATA_DIR", root)
	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKiloEntries: %v", err)
	}
	if entries != nil {
		t.Fatalf("user role should be filtered out, got %d entries: %+v", len(entries), entries)
	}
}

// TestLoadKiloEntriesTimestampMillisFormat confirms the dual-format
// timestamp logic: values >= 1e12 are already milliseconds and pass
// through without the seconds-to-ms multiplication.
func TestLoadKiloEntriesTimestampMillisFormat(t *testing.T) {
	// 1767312000000 ms -> still 2026-01-02 UTC (same wall time as
	// 1767312000 seconds; this is the >1e12 ms-already branch).
	root := setupKiloFixture(t, `{
		"role":"assistant",
		"modelID":"claude-sonnet-4-20250514",
		"time":{"created":1767312000000},
		"tokens":{"input":50,"output":25}
	}`)
	t.Setenv("KILO_DATA_DIR", root)
	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKiloEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Timestamp.Year() != 2026 || entries[0].Timestamp.Month() != 1 || entries[0].Timestamp.Day() != 2 {
		t.Errorf("Timestamp=%v want 2026-01-02 (1767312000000 ms normalized)", entries[0].Timestamp)
	}
}

// TestLoadKiloEntriesSessionIDFallback confirms that when the JSON
// blob omits session_id, the SQLite row's session_id column is used.
func TestLoadKiloEntriesSessionIDFallback(t *testing.T) {
	root := setupKiloFixture(t, `{
		"role":"assistant",
		"modelID":"claude-sonnet-4-20250514",
		"time":{"created":1767312000},
		"tokens":{"input":10,"output":5}
	}`)
	// setupKiloFixture inserts row with session_id column = "session-fallback-a"
	t.Setenv("KILO_DATA_DIR", root)
	entries, err := collector.LoadKiloEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadKiloEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SessionID != "session-fallback-a" {
		t.Errorf("SessionID=%q want session-fallback-a (row column fallback when JSON lacks session_id)", entries[0].SessionID)
	}
	// Compile-time sanity that os/filepath imports stay live across
	// future test edits.
	_ = os.PathSeparator
	_ = filepath.Separator
}
