package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestRunSearchAdvancedJSONOutputsQueryAndResults(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-time.Minute)
	seedCLISession(t, db, "search-advanced-cli", event.PlatformClaude, "/tmp/tokenmeter", "main", now, 1000, 100, 2)
	if _, err := db.InsertToolCallStart("search-advanced-param", "agent-search-advanced-cli", "search-advanced-cli", "Bash", "exit code from go test", now); err != nil {
		t.Fatalf("insert tool param: %v", err)
	}
	if err := db.UpdateToolCallEnd("search-advanced-param", "exit code output", event.StatusFail, 0, now.Add(time.Second)); err != nil {
		t.Fatalf("update tool result: %v", err)
	}

	withArgs(t, []string{"tokenmeter", "search", "exit code", "tool:Bash", "status:failed", "cost:>1", "--json"})
	out := captureStdout(t, func() {
		if err := runSearch(); err != nil {
			t.Fatalf("runSearch: %v", err)
		}
	})

	var payload struct {
		Query struct {
			Tool   string   `json:"tool"`
			Status string   `json:"status"`
			Terms  []string `json:"keywords"`
		} `json:"query"`
		Results []struct {
			SessionID string `json:"session_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if payload.Query.Tool != "Bash" || payload.Query.Status != "failed" || strings.Join(payload.Query.Terms, " ") != "exit code" {
		t.Fatalf("unexpected parsed query: %+v", payload.Query)
	}
	if len(payload.Results) != 2 || payload.Results[0].SessionID != "search-advanced-cli" {
		t.Fatalf("unexpected results: %+v", payload.Results)
	}
}

func TestRunSearchAdvancedRejectsUnknownFilter(t *testing.T) {
	withArgs(t, []string{"tokenmeter", "search", "foo:bar"})
	err := runSearch()
	if err == nil {
		t.Fatal("expected unknown filter error")
	}
	if !strings.Contains(err.Error(), "unknown filter") || !strings.Contains(err.Error(), "supported: tool, session, status, platform, cost, tokens, since, until") {
		t.Fatalf("unexpected error: %v", err)
	}
}
