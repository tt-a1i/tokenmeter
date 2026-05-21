package collector

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoadQwenEntriesParsesSample(t *testing.T) {
	abs, err := filepath.Abs("qwen_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("QWEN_DATA_DIR", abs)

	entries, err := LoadQwenEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadQwenEntries: %v", err)
	}
	// Two assistant rows in sample.jsonl + zero from the off-depth
	// extra.jsonl distractor = 2 entries.
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	first := entries[0]
	if first.Source != "qwen" {
		t.Errorf("Source = %q, want qwen", first.Source)
	}
	if first.SessionID != "sess-a" {
		t.Errorf("SessionID = %q, want sess-a (from sessionId field)", first.SessionID)
	}
	if first.Model != "qwen3-coder" {
		t.Errorf("Model = %q, want qwen3-coder", first.Model)
	}
	if first.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500 (promptTokenCount)", first.InputTokens)
	}
	if first.OutputTokens < 420 {
		t.Errorf("OutputTokens = %d, want ≥ 420 (candidatesTokenCount; thoughts may fold in)", first.OutputTokens)
	}
	if first.CacheReadInputTokens != 8192 {
		t.Errorf("CacheReadInputTokens = %d, want 8192 (cachedContentTokenCount)", first.CacheReadInputTokens)
	}
	wantTs := time.Date(2026, 4, 15, 9, 30, 15, 500000000, time.UTC)
	if !first.Timestamp.Equal(wantTs) {
		t.Errorf("Timestamp = %v, want %v", first.Timestamp, wantTs)
	}
	if first.ProjectPath != "project-a" {
		t.Errorf("ProjectPath = %q, want project-a (extracted from path)", first.ProjectPath)
	}

	// Second entry: totalTokenCount fallback row (only total is set; the
	// adapter folds the total into output_tokens so the row contributes).
	second := entries[1]
	if second.OutputTokens != 555 {
		t.Errorf("totalTokenCount fallback row OutputTokens = %d, want 555", second.OutputTokens)
	}
	// Session id falls back to project-stem when sessionId is missing.
	if !strings.Contains(second.SessionID, "project-a") {
		t.Errorf("fallback SessionID = %q, want it to mention project-a", second.SessionID)
	}
}

func TestLoadQwenEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("QWEN_DATA_DIR", "/nonexistent/path/abcxyz-qwen-fixture")

	entries, err := LoadQwenEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadQwenEntriesWrongDepthSkipped(t *testing.T) {
	// extra.jsonl sits at `projects/<project>/extra.jsonl` (depth 2 from
	// projects) instead of the canonical `projects/<project>/chats/<file>.jsonl`
	// (depth 3). The adapter must skip it entirely; no entry should
	// surface from its huge fake token counts.
	abs, err := filepath.Abs("qwen_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("QWEN_DATA_DIR", abs)

	entries, err := LoadQwenEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadQwenEntries: %v", err)
	}
	for _, e := range entries {
		if e.SessionID == "WRONG-DEPTH" {
			t.Fatalf("off-depth extra.jsonl must be filtered out, but its WRONG-DEPTH entry leaked: %+v", e)
		}
		// Sanity: 99999999 input/output is the off-depth signature.
		if e.InputTokens == 99999999 || e.OutputTokens == 99999999 {
			t.Fatalf("off-depth row leaked through depth filter: %+v", e)
		}
	}
}
