package cli_test

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
)

// TestAllSourceMergesWithFakeAdapters exercises the v1.1 default
// dispatch end-to-end: AllAdapters' real Load*Entries functions read
// their respective fixtures via env-var overrides, RunAggregateAllSource
// folds them through the in-memory aggregator, and the rendered table
// must surface all three sources' contributions.
//
// Three sources picked to span the v1.1 adapter spectrum:
//   - Amp     (JSON thread file, usageLedger.events)
//   - Copilot (OTEL JSONL spans)
//   - Goose   (SQLite session db)
//
// All three lead through cli.AllAdapters' registered functions, so any
// future regression in the path-discovery / env-override contract of
// these adapters will fail this test rather than waiting for production
// to surface it.
func TestAllSourceMergesWithFakeAdapters(t *testing.T) {
	// Resolve to project-root-relative fixture paths so the test
	// survives a `cd` into cmd/tm/cli.
	ampFix, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "collector", "amp_fixtures"))
	if err != nil {
		t.Fatalf("abs amp fixture: %v", err)
	}
	if _, err := os.Stat(ampFix); err != nil {
		t.Fatalf("amp fixture missing: %v", err)
	}
	copilotFix, err := filepath.Abs(filepath.Join("..", "..", "..", "internal", "collector", "copilot_fixtures", "copilot.jsonl"))
	if err != nil {
		t.Fatalf("abs copilot fixture: %v", err)
	}
	if _, err := os.Stat(copilotFix); err != nil {
		t.Fatalf("copilot fixture missing: %v", err)
	}

	gooseRoot := setupGooseIntegrationFixture(t)

	t.Setenv("AMP_DATA_DIR", ampFix)
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", copilotFix)
	t.Setenv("GOOSE_PATH_ROOT", gooseRoot)
	// Other adapters' env-vars are intentionally left unset; their
	// Load*Entries returns (nil, nil) for missing data and the test
	// must remain green even when the user has those agents installed
	// (any home-dir fallback paths are still searched but treated as
	// not-installed when the path doesn't exist on the test host).
	// To make the test hermetic against developer machines that DO
	// have one of those installs, override every other adapter's env
	// to a path that cannot exist.
	hermeticDir := filepath.Join(t.TempDir(), "no-such-source")
	for _, ev := range []string{
		"OPENCODE_DATA_DIR", "GEMINI_DATA_DIR", "KIMI_DATA_DIR",
		"OPENCLAW_DIR", "HERMES_HOME", "KILO_DATA_DIR",
		"CODEBUFF_DATA_DIR", "PI_AGENT_DIR", "DROID_SESSIONS_DIR",
		"QWEN_DATA_DIR",
	} {
		t.Setenv(ev, hermeticDir)
	}

	// Empty SQLite loader: this is the v1.1 contract — when the
	// Claude/Codex db has nothing in range, the merged view is still
	// driven by the adapters.
	sqliteLoader := stubAggregateLoader{}

	var out bytes.Buffer
	if err := cli.RunAggregateAllSource(
		context.Background(),
		&out,
		cli.AggregateArgs{Shared: cli.Shared{Breakdown: true}, Bucket: cli.BucketDaily},
		sqliteLoader,
		cli.AllAdapters,
	); err != nil {
		t.Fatalf("RunAggregateAllSource: %v", err)
	}

	output := out.String()
	// Sanity: header is present.
	if !strings.Contains(output, "DATE") {
		t.Fatalf("rendered output missing DATE header:\n%s", output)
	}
	// Each source must surface its signature model name via the
	// per-model breakdown rows ("└─ <model>" under each day).
	for _, want := range []string{
		"gpt-5-amp",                // Amp (from threads/thread-abc.json)
		"claude-sonnet-4",          // Copilot OTEL row
		"claude-sonnet-4-20250514", // Goose seed
	} {
		if !strings.Contains(output, want) {
			t.Errorf("merged output missing %q (source contribution silently dropped):\n%s", want, output)
		}
	}
}

// setupGooseIntegrationFixture builds the minimal Goose SQLite layout
// (root/data/sessions/sessions.db) under t.TempDir() and seeds one
// session row. Returns the root path suitable for $GOOSE_PATH_ROOT.
func setupGooseIntegrationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dbDir := filepath.Join(root, "data", "sessions")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("mkdir goose sessions: %v", err)
	}
	dbPath := filepath.Join(dbDir, "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open goose db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
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
)`); err != nil {
		t.Fatalf("create goose schema: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO sessions (id, model_config_json, provider_name, created_at,
    accumulated_total_tokens, accumulated_input_tokens, accumulated_output_tokens)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"integration-session",
		`{"model_name":"claude-sonnet-4-20250514"}`,
		"anthropic",
		"2026-04-15 10:30:00",
		int64(180), int64(100), int64(50),
	); err != nil {
		t.Fatalf("insert goose row: %v", err)
	}
	return root
}
