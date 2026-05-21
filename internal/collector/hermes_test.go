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

// hermesFixtureSchema is the CREATE TABLE statement borrowed verbatim
// from ccusage v20 (rust/crates/ccusage/src/adapter/hermes.rs
// tests::create_state_db). Keep in sync if the upstream schema drifts;
// only an exact field match guarantees the SELECT query in
// gooseSessionQuery succeeds against the fixture.
const hermesFixtureSchema = `
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    model TEXT,
    started_at REAL NOT NULL,
    message_count INTEGER DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_write_tokens INTEGER DEFAULT 0,
    reasoning_tokens INTEGER DEFAULT 0,
    billing_provider TEXT,
    estimated_cost_usd REAL,
    actual_cost_usd REAL
)
`

// setupHermesFixture creates a temp Hermes home with one populated
// state.db (laid out as $HERMES_HOME/state.db).
func setupHermesFixture(t *testing.T) (homeDir string, dbPath string) {
	t.Helper()
	home := t.TempDir()
	db := filepath.Join(home, "state.db")
	conn, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Exec(hermesFixtureSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	// Sample row from ccusage hermes.rs::loads_billable_hermes_sessions_from_state_db
	// (input 1200, output 300, cache_read 50, cache_write 20, reasoning 10,
	//  estimated 0.12, actual 0.34).
	if _, err := conn.Exec(`
INSERT INTO sessions (
    id, source, model, started_at,
    message_count, input_tokens, output_tokens,
    cache_read_tokens, cache_write_tokens, reasoning_tokens,
    billing_provider, estimated_cost_usd, actual_cost_usd
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		"session-1", "cli", "claude-sonnet-4-20250514",
		float64(1750000000.25), // started_at REAL
		int64(42), int64(1200), int64(300),
		int64(50), int64(20), int64(10),
		"anthropic", 0.12, 0.34,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return home, db
}

// TestLoadHermesEntriesParsesSample drives the adapter against the
// ccusage-borrowed fixture. Tokens map per hermes.rs field names;
// reasoning_tokens (10) folds into Output (300 + 10 = 310); cost
// prefers actual_cost_usd (0.34) over estimated_cost_usd (0.12).
func TestLoadHermesEntriesParsesSample(t *testing.T) {
	home, _ := setupHermesFixture(t)
	t.Setenv("HERMES_HOME", home)

	entries, err := collector.LoadHermesEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadHermesEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.Source != "hermes" {
		t.Errorf("Source=%q want hermes", e.Source)
	}
	if e.SessionID != "session-1" {
		t.Errorf("SessionID=%q want session-1", e.SessionID)
	}
	if e.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model=%q want claude-sonnet-4-20250514", e.Model)
	}
	if e.InputTokens != 1200 {
		t.Errorf("InputTokens=%d want 1200", e.InputTokens)
	}
	// 300 base output + 10 reasoning folded in.
	if e.OutputTokens != 310 {
		t.Errorf("OutputTokens=%d want 310 (300 + 10 reasoning folded)", e.OutputTokens)
	}
	if e.CacheReadInputTokens != 50 {
		t.Errorf("CacheReadInputTokens=%d want 50", e.CacheReadInputTokens)
	}
	if e.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens=%d want 20 (cache_write_tokens)", e.CacheCreationInputTokens)
	}
	if e.CostUSD != 0.34 {
		t.Errorf("CostUSD=%v want 0.34 (actual_cost_usd preferred over estimated)", e.CostUSD)
	}
	if e.Timestamp.Year() != 2025 || e.Timestamp.Month() != 6 || e.Timestamp.Day() != 15 {
		t.Errorf("Timestamp=%v want 2025-06-15 (from started_at REAL 1750000000.25)", e.Timestamp)
	}
}

// TestLoadHermesEntriesMissingDirReturnsNil exercises the
// "user does not have Hermes installed" path via HERMES_HOME pointing
// at a non-existent directory.
func TestLoadHermesEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("HERMES_HOME", "/nonexistent/path/abcxyz-hermes-home")
	entries, err := collector.LoadHermesEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing home should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing home, got %d", len(entries))
	}
}

// TestLoadHermesEntriesMissingDBFile covers a real home directory whose
// state.db file is absent — same (nil, nil) contract.
func TestLoadHermesEntriesMissingDBFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)
	entries, err := collector.LoadHermesEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing db file should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing db file, got %d", len(entries))
	}
}

// TestLoadHermesEntriesEstimatedCostFallback verifies cost selection:
// when actual_cost_usd is NULL the adapter must fall back to
// estimated_cost_usd. This is the Hermes-specific quirk distinct from
// every other adapter we ship.
func TestLoadHermesEntriesEstimatedCostFallback(t *testing.T) {
	home := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(home, "state.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(hermesFixtureSchema); err != nil {
		db.Close()
		t.Fatalf("schema: %v", err)
	}
	// actual_cost_usd intentionally NULL; estimated_cost_usd populated.
	if _, err := db.Exec(`
INSERT INTO sessions (
    id, source, model, started_at, input_tokens, output_tokens, estimated_cost_usd
) VALUES (?, ?, ?, ?, ?, ?, ?)
`,
		"session-est", "cli", "gpt-4o", float64(1750000100.5),
		int64(500), int64(200), 0.07,
	); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	t.Setenv("HERMES_HOME", home)
	entries, err := collector.LoadHermesEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadHermesEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].CostUSD != 0.07 {
		t.Errorf("CostUSD=%v want 0.07 (estimated fallback when actual NULL)", entries[0].CostUSD)
	}
}

// TestLoadHermesEntriesUnusedFile exercises path discovery: HERMES_HOME
// is intentionally referenced here so the lint-error symbol _ = filepath
// stays anchored. (Sanity check that os import is live too.)
func TestLoadHermesEntriesPathDiscoveryReadsCommaSeparated(t *testing.T) {
	home1 := t.TempDir()
	home2 := t.TempDir()
	// Only home1 carries a populated DB; home2 is empty. Adapter must
	// still walk both without error and surface home1's row.
	db, err := sql.Open("sqlite", filepath.Join(home1, "state.db"))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(hermesFixtureSchema); err != nil {
		db.Close()
		t.Fatalf("schema: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO sessions (id, source, model, started_at, input_tokens, output_tokens)
VALUES (?, ?, ?, ?, ?, ?)
`,
		"session-multi", "cli", "gemini-1.5-pro", float64(1750000200), int64(100), int64(50),
	); err != nil {
		db.Close()
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	// Sanity: also ensure os/filepath are exercised (avoid unused imports
	// in future refactors).
	_ = os.PathSeparator

	t.Setenv("HERMES_HOME", home1+","+home2)
	entries, err := collector.LoadHermesEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadHermesEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (home1 only), got %d", len(entries))
	}
	if entries[0].SessionID != "session-multi" || entries[0].Model != "gemini-1.5-pro" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}
