package collector

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLoadCopilotEntriesParsesSample(t *testing.T) {
	fixture, err := filepath.Abs("copilot_fixtures/copilot.jsonl")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", fixture)

	entries, err := LoadCopilotEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadCopilotEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (chat-span + agent-turn-log), got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if e.Source != "copilot" {
			t.Errorf("Source = %q, want copilot", e.Source)
		}
		if e.Model == "" {
			t.Errorf("Model not parsed for entry %+v", e)
		}
	}
}

func TestLoadCopilotEntriesMissingDirReturnsNil(t *testing.T) {
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", "/nonexistent/path/copilot-fixture.jsonl")

	entries, err := LoadCopilotEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("missing path should not error: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestLoadCopilotEnvOverridePointsToFile(t *testing.T) {
	// Env override is a FILE path (not a dir) — different from every other
	// adapter. When the file is valid OTEL JSONL, adapter must parse it.
	fixture, err := filepath.Abs("copilot_fixtures/copilot.jsonl")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	t.Setenv("COPILOT_OTEL_FILE_EXPORTER_PATH", fixture)

	entries, err := LoadCopilotEntries(context.Background(), AdapterOpts{})
	if err != nil {
		t.Fatalf("LoadCopilotEntries: %v", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}

	// First entry: chat span with cached-input subtraction:
	//   input = 19452 - min(19452, 123) = 19329; cache_read = 123.
	chat := entries[0]
	if chat.SessionID != "conv-1" {
		t.Errorf("chat SessionID = %q, want conv-1 (from gen_ai.conversation.id)", chat.SessionID)
	}
	if chat.Model != "claude-sonnet-4" {
		t.Errorf("chat Model = %q, want claude-sonnet-4", chat.Model)
	}
	if chat.InputTokens != 19329 {
		t.Errorf("chat InputTokens = %d, want 19329 (cache-subtracted)", chat.InputTokens)
	}
	if chat.CacheReadInputTokens != 123 {
		t.Errorf("chat CacheReadInputTokens = %d, want 123", chat.CacheReadInputTokens)
	}
	if chat.CacheCreationInputTokens != 25 {
		t.Errorf("chat CacheCreationInputTokens = %d, want 25", chat.CacheCreationInputTokens)
	}
	// Reasoning (128) is folded into output (281) per the Gemini precedent:
	if chat.OutputTokens != 281+128 {
		t.Errorf("chat OutputTokens = %d, want 409 (281 + 128 reasoning fold-in)", chat.OutputTokens)
	}
	// Timestamp from endTime [seconds, nanos] = 1775934264.967317833 →
	// 2026-04-11T19:04:24.967Z
	wantTs := time.Unix(1775934264, 967317833).UTC()
	if !chat.Timestamp.Equal(wantTs) {
		t.Errorf("chat Timestamp = %v, want %v", chat.Timestamp, wantTs)
	}

	// Second entry: agent-turn log row with copilot_chat.session_id and
	// timeUnixNano timestamp.
	agent := entries[1]
	if agent.SessionID != "session-2" {
		t.Errorf("agent SessionID = %q, want session-2", agent.SessionID)
	}
	if agent.Model != "gpt-5" {
		t.Errorf("agent Model = %q, want gpt-5", agent.Model)
	}
	if agent.InputTokens != 500 || agent.OutputTokens != 100 {
		t.Errorf("agent token totals = (%d, %d), want (500, 100)", agent.InputTokens, agent.OutputTokens)
	}
}
