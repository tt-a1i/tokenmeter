package collector

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLoadGeminiEntriesParsesSample(t *testing.T) {
	abs, err := filepath.Abs("gemini_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("GEMINI_DATA_DIR", abs)

	entries, err := LoadGeminiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadGeminiEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (one per .jsonl + .json fixture), got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "gemini" {
			t.Errorf("Source = %q, want gemini", e.Source)
		}
		if e.Model == "" {
			t.Errorf("Model not parsed for entry %+v", e)
		}
		if e.InputTokens <= 0 {
			t.Errorf("InputTokens not parsed for entry %+v", e)
		}
	}
}

func TestLoadGeminiEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("GEMINI_DATA_DIR", "/nonexistent/path/abcxyz-gemini-fixture")

	entries, err := LoadGeminiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadGeminiEntriesHandlesBothExtensions(t *testing.T) {
	// Adapter must accept .json and .jsonl from the same directory and
	// skip the .txt distractor sitting next to them.
	abs, err := filepath.Abs("gemini_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("GEMINI_DATA_DIR", abs)

	entries, err := LoadGeminiEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadGeminiEntries: %v", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries from .jsonl + .json, got %d: %+v", len(entries), entries)
	}
	// First entry comes from session-a.jsonl (timestamp 2026-05-17T11:07:32Z).
	jsonl := entries[0]
	if jsonl.SessionID != "session-a" {
		t.Errorf(".jsonl SessionID = %q, want session-a (from cursor record)", jsonl.SessionID)
	}
	if jsonl.Model != "gemini-3-flash-preview" {
		t.Errorf(".jsonl Model = %q", jsonl.Model)
	}
	// normalize_session_input: cached=11526 overlaps input=15327, plus tool=7 ⇒
	//   display_input = (15327 - 11526) + 7 = 3808
	if jsonl.InputTokens != 3808 {
		t.Errorf(".jsonl InputTokens = %d, want 3808 (cached-subtracted + tool)", jsonl.InputTokens)
	}
	if jsonl.CacheReadInputTokens != 11526 {
		t.Errorf(".jsonl CacheReadInputTokens = %d, want 11526", jsonl.CacheReadInputTokens)
	}
	wantJsonlTs := time.Date(2026, 5, 17, 11, 7, 32, 0, time.UTC)
	if !jsonl.Timestamp.Equal(wantJsonlTs) {
		t.Errorf(".jsonl Timestamp = %v, want %v", jsonl.Timestamp, wantJsonlTs)
	}

	// Second entry comes from session-b.json (timestamp 2026-05-17T12:00:30Z).
	js := entries[1]
	if js.SessionID != "session-b" {
		t.Errorf(".json SessionID = %q, want session-b", js.SessionID)
	}
	if js.Model != "gemini-3-flash-preview" {
		t.Errorf(".json Model = %q", js.Model)
	}
	// normalize_session_input: cached=0, so input passes through plus tool=2:
	//   display_input = 1000 + 2 = 1002
	if js.InputTokens != 1002 {
		t.Errorf(".json InputTokens = %d, want 1002 (input + tool, no cache to subtract)", js.InputTokens)
	}
	if js.OutputTokens != 50 {
		t.Errorf(".json OutputTokens = %d, want 50", js.OutputTokens)
	}
}
