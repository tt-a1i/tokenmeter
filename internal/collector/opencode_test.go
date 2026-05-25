package collector_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/collector"
	_ "modernc.org/sqlite"
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

func TestLoadOpenCodeEntriesPrefersSQLiteOverDuplicateJSON(t *testing.T) {
	dir := t.TempDir()
	writeOpenCodeJSONMessage(t, dir, "dup", `{
		"id": "msg-dup",
		"sessionID": "json-session",
		"modelID": "json-model",
		"time": {"created": 1767312000000},
		"tokens": {"input": 1, "output": 2},
		"cost": 0.01
	}`)
	writeOpenCodeDB(t, filepath.Join(dir, "opencode.db"), []opencodeDBRow{{
		ID:        "msg-dup",
		SessionID: "db-session",
		Data: `{
			"id": "msg-dup",
			"sessionID": "db-session",
			"modelID": "db-model",
			"time": {"created": 1767312000000},
			"tokens": {"input": 10, "output": 20},
			"cost": 0.20
		}`,
	}})
	t.Setenv("OPENCODE_DATA_DIR", dir)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected duplicate id to collapse to SQLite row, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.SessionID != "db-session" || e.Model != "db-model" || e.InputTokens != 10 || e.OutputTokens != 20 || e.CostUSD != 0.20 {
		t.Fatalf("entry = %+v, want SQLite row values", e)
	}
}

func TestLoadOpenCodeEntriesReadsSQLiteOnly(t *testing.T) {
	dir := t.TempDir()
	writeOpenCodeDB(t, filepath.Join(dir, "opencode.db"), []opencodeDBRow{{
		ID:        "msg-sqlite",
		SessionID: "sqlite-session",
		Data: `{
			"id": "msg-sqlite",
			"sessionID": "sqlite-session",
			"modelID": "sqlite-model",
			"time": {"created": 1767312000000},
			"tokens": {"input": 11, "output": 22, "cache": {"write": 3, "read": 4}},
			"cost": 0.33
		}`,
	}})
	t.Setenv("OPENCODE_DATA_DIR", dir)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 SQLite entry, got %d: %+v", len(entries), entries)
	}
	e := entries[0]
	if e.SessionID != "sqlite-session" || e.Model != "sqlite-model" || e.InputTokens != 11 ||
		e.OutputTokens != 22 || e.CacheCreationInputTokens != 3 || e.CacheReadInputTokens != 4 || e.CostUSD != 0.33 {
		t.Fatalf("entry = %+v, want SQLite values", e)
	}
}

func TestLoadOpenCodeEntriesJSONFallbackWithoutSQLite(t *testing.T) {
	dir := t.TempDir()
	writeOpenCodeJSONMessage(t, dir, "json", `{
		"id": "msg-json",
		"sessionID": "json-session",
		"modelID": "json-model",
		"time": {"created": 1767312000000},
		"tokens": {"input": 7, "output": 8},
		"cost": 0.09
	}`)
	t.Setenv("OPENCODE_DATA_DIR", dir)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 JSON fallback entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].SessionID != "json-session" || entries[0].Model != "json-model" ||
		entries[0].InputTokens != 7 || entries[0].OutputTokens != 8 {
		t.Fatalf("entry = %+v, want JSON fallback values", entries[0])
	}
}

func TestLoadOpenCodeEntriesReadsMultipleChannelDatabases(t *testing.T) {
	dir := t.TempDir()
	writeOpenCodeDB(t, filepath.Join(dir, "opencode-alpha.db"), []opencodeDBRow{{
		ID:        "msg-alpha",
		SessionID: "alpha-session",
		Data: `{
			"id": "msg-alpha",
			"sessionID": "alpha-session",
			"modelID": "alpha-model",
			"time": {"created": 1767312000000},
			"tokens": {"input": 1, "output": 2}
		}`,
	}})
	writeOpenCodeDB(t, filepath.Join(dir, "opencode-beta.db"), []opencodeDBRow{{
		ID:        "msg-beta",
		SessionID: "beta-session",
		Data: `{
			"id": "msg-beta",
			"sessionID": "beta-session",
			"modelID": "beta-model",
			"time": {"created": 1767398400000},
			"tokens": {"input": 3, "output": 4}
		}`,
	}})
	t.Setenv("OPENCODE_DATA_DIR", dir)

	entries, err := collector.LoadOpenCodeEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadOpenCodeEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 channel DB entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].SessionID != "alpha-session" || entries[1].SessionID != "beta-session" {
		t.Fatalf("entries = %+v, want both channel DB rows sorted by timestamp", entries)
	}
}

type opencodeDBRow struct {
	ID        string
	SessionID string
	Data      string
}

func writeOpenCodeDB(t *testing.T, path string, rows []opencodeDBRow) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE message (id TEXT, session_id TEXT, data TEXT)`); err != nil {
		t.Fatalf("create message table: %v", err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO message (id, session_id, data) VALUES (?, ?, ?)`, row.ID, row.SessionID, row.Data); err != nil {
			t.Fatalf("insert message: %v", err)
		}
	}
}

func writeOpenCodeJSONMessage(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "storage", "message", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "message.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write message: %v", err)
	}
}
