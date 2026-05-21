package collector

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoadCodebuffEntriesParsesSample(t *testing.T) {
	abs, err := filepath.Abs("codebuff_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("CODEBUFF_DATA_DIR", abs)

	entries, err := LoadCodebuffEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadCodebuffEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (one assistant message per fixture chat), got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "codebuff" {
			t.Errorf("Source = %q, want codebuff", e.Source)
		}
		if e.Model == "" {
			t.Errorf("Model not parsed: %+v", e)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	// project-a chat (camelCase keys + role=assistant + credits)
	a := entries[0]
	if !strings.Contains(a.SessionID, "project-a") || !strings.Contains(a.SessionID, "abc") {
		t.Errorf("project-a SessionID = %q, want it to mention project-a and abc", a.SessionID)
	}
	if a.Model != "claude-sonnet-4" {
		t.Errorf("project-a Model = %q, want claude-sonnet-4", a.Model)
	}
	if a.InputTokens != 100 || a.OutputTokens != 50 {
		t.Errorf("project-a token mapping wrong: %+v", a)
	}
	if a.CacheReadInputTokens != 200 || a.CacheCreationInputTokens != 25 {
		t.Errorf("project-a cache mapping wrong: %+v", a)
	}
	if a.CostUSD != 0.0125 {
		t.Errorf("project-a CostUSD = %v, want 0.0125 (from credits)", a.CostUSD)
	}
	wantTsA := time.Date(2026, 4, 15, 10, 0, 30, 0, time.UTC)
	if !a.Timestamp.Equal(wantTsA) {
		t.Errorf("project-a Timestamp = %v, want %v", a.Timestamp, wantTsA)
	}

	// project-b chat (snake_case keys via metadata.codebuff.usage + variant=ai)
	b := entries[1]
	if !strings.Contains(b.SessionID, "project-b") || !strings.Contains(b.SessionID, "def") {
		t.Errorf("project-b SessionID = %q, want it to mention project-b and def", b.SessionID)
	}
	if b.Model != "gpt-5" {
		t.Errorf("project-b Model = %q, want gpt-5", b.Model)
	}
	if b.InputTokens != 300 || b.OutputTokens != 120 {
		t.Errorf("project-b token mapping wrong (prompt_tokens/completion_tokens): %+v", b)
	}
	if b.CacheReadInputTokens != 1024 {
		t.Errorf("project-b CacheReadInputTokens = %d, want 1024", b.CacheReadInputTokens)
	}
}

func TestLoadCodebuffEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("CODEBUFF_DATA_DIR", "/nonexistent/path/abcxyz-codebuff-fixture")

	entries, err := LoadCodebuffEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadCodebuffEntriesMultipleProjects(t *testing.T) {
	// Two projects under one root; both must surface with distinct
	// SessionIDs so the AllSource view can attribute traffic per project.
	abs, err := filepath.Abs("codebuff_fixtures")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("CODEBUFF_DATA_DIR", abs)

	entries, err := LoadCodebuffEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadCodebuffEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	sids := map[string]bool{}
	for _, e := range entries {
		sids[e.SessionID] = true
	}
	if len(sids) != 2 {
		t.Fatalf("expected 2 distinct SessionIDs, got %d: %v", len(sids), sids)
	}
}
