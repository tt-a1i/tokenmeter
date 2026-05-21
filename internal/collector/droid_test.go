package collector_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/collector"
)

// TestLoadDroidEntriesParsesSample drives the adapter against the
// ccusage-borrowed fixture in droid_fixtures/. Verifies:
//   - tokenUsage.{input,output,cacheCreation,cacheRead}Tokens mapping
//   - thinkingTokens fold into OutputTokens (50 + 5 = 55)
//   - model name normalization ("Claude-Sonnet-4-[Anthropic]" ->
//     "claude-sonnet-4")
//   - providerLockTimestamp RFC3339 parse
//   - session id derived from "<id>.settings.json" basename
//   - zero-token sentinel file dropped
//   - non-".settings.json" file (decoy.json) ignored by the file matcher
func TestLoadDroidEntriesParsesSample(t *testing.T) {
	fix, err := filepath.Abs("droid_fixtures")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	t.Setenv("DROID_SESSIONS_DIR", fix)

	entries, err := collector.LoadDroidEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadDroidEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (session-a only; zero + decoy filtered), got %d: %+v",
			len(entries), entries)
	}
	e := entries[0]
	if e.Source != "droid" {
		t.Errorf("Source=%q want droid", e.Source)
	}
	if e.SessionID != "session-a" {
		t.Errorf("SessionID=%q want session-a (file basename minus .settings.json)", e.SessionID)
	}
	if e.Model != "claude-sonnet-4" {
		t.Errorf("Model=%q want claude-sonnet-4 (normalize from \"Claude-Sonnet-4-[Anthropic]\")", e.Model)
	}
	if e.InputTokens != 100 {
		t.Errorf("InputTokens=%d want 100", e.InputTokens)
	}
	// 50 base + 5 thinking folded in.
	if e.OutputTokens != 55 {
		t.Errorf("OutputTokens=%d want 55 (50 + 5 thinking folded)", e.OutputTokens)
	}
	if e.CacheCreationInputTokens != 20 {
		t.Errorf("CacheCreationInputTokens=%d want 20", e.CacheCreationInputTokens)
	}
	if e.CacheReadInputTokens != 10 {
		t.Errorf("CacheReadInputTokens=%d want 10", e.CacheReadInputTokens)
	}
	if e.CostUSD != 0 {
		t.Errorf("CostUSD=%v want 0 (adapter defers cost to ModeAuto recompute)", e.CostUSD)
	}
	// providerLockTimestamp "2026-05-01T01:02:03.000Z" -> 2026-05-01 UTC
	if e.Timestamp.Year() != 2026 || e.Timestamp.Month() != 5 || e.Timestamp.Day() != 1 {
		t.Errorf("Timestamp=%v want 2026-05-01 (from providerLockTimestamp)", e.Timestamp)
	}
}

// TestLoadDroidEntriesMissingDirReturnsNil exercises the
// "user does not have Droid installed" path via DROID_SESSIONS_DIR
// pointing at a non-existent directory.
func TestLoadDroidEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("DROID_SESSIONS_DIR", "/nonexistent/path/abcxyz-droid-sessions")
	entries, err := collector.LoadDroidEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing dir, got %d", len(entries))
	}
}

// TestLoadDroidEntriesSuffixFilterIsStrict asserts the file matcher
// requires the full ".settings.json" suffix — a bare ".json" file
// MUST NOT be considered a settings file. The fixture's decoy.json
// was placed for this exact assertion: if it surfaces it would error
// (decoy lacks tokenUsage), so the row count check above also covers
// this. This explicit test pins the behavior so a future "simplify to
// ext check" refactor would fail loudly.
func TestLoadDroidEntriesSuffixFilterIsStrict(t *testing.T) {
	fix, err := filepath.Abs("droid_fixtures")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	t.Setenv("DROID_SESSIONS_DIR", fix)
	entries, err := collector.LoadDroidEntries(context.Background(), collector.AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadDroidEntries: %v", err)
	}
	// 3 files in fixture: session-a.settings.json (kept), zero.settings.json
	// (dropped — all zero tokens), decoy.json (must be ignored by suffix
	// match). Final = 1.
	if len(entries) != 1 {
		t.Fatalf("decoy.json leaked through suffix filter; expected 1 entry, got %d", len(entries))
	}
	if entries[0].SessionID != "session-a" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

// TestLoadDroidEntriesModelNameNormalization exercises a handful of
// edge cases for the model name transform via black-box subtest. Each
// case writes one synthesized settings file and reads back the
// resulting UsageEntry.Model. The adapter contract: Droid stores
// vendor brackets and mixed case verbatim, the adapter normalizes to
// lowercase hyphenated tokens suitable for pricing.Resolve lookup.
func TestLoadDroidEntriesModelNameNormalization(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"brackets-stripped", "Claude-Sonnet-4-[Anthropic]", "claude-sonnet-4"},
		{"custom-and-dots", "custom:Claude-Opus-4.5-Thinking-[Anthropic]-0", "claude-opus-4-5-thinking-0"},
		{"dot-to-dash", "gemini-2.5-pro", "gemini-2-5-pro"},
		{"trim-spaces", "  trailing-spaces  ", "trailing-spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body, _ := jsonEncodeStringField(tc.in)
			content := `{
                "model": ` + body + `,
                "providerLockTimestamp": "2026-05-01T00:00:00.000Z",
                "tokenUsage": {"inputTokens": 1, "outputTokens": 1}
            }`
			if err := os.WriteFile(filepath.Join(dir, "x.settings.json"), []byte(content), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Setenv("DROID_SESSIONS_DIR", dir)
			entries, err := collector.LoadDroidEntries(context.Background(), collector.AdapterOpts{})
			if err != nil {
				t.Fatalf("LoadDroidEntries: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			if entries[0].Model != tc.want {
				t.Errorf("Model=%q want %q for input %q", entries[0].Model, tc.want, tc.in)
			}
		})
	}
}

// jsonEncodeStringField returns a JSON-quoted form of s suitable for
// inline assembly into a JSON object literal. Used only by the
// normalization subtest; saves importing encoding/json into the test
// file for one call.
func jsonEncodeStringField(s string) (string, error) {
	out := []byte{'"'}
	for _, r := range s {
		switch r {
		case '"', '\\':
			out = append(out, '\\', byte(r))
		case '\n':
			out = append(out, '\\', 'n')
		case '\t':
			out = append(out, '\\', 't')
		default:
			out = append(out, []byte(string(r))...)
		}
	}
	out = append(out, '"')
	return string(out), nil
}
