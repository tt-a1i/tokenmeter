package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestRunAnalyzeTextFormat(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-time.Hour)
	seedAnalyzeCLISession(t, db, "analyze-cli", event.PlatformClaude, now, "sonnet", 2.5, "internal/foo.go")

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})

	for _, want := range []string{
		"TokenMeter Analysis",
		"Cost",
		"Sessions",
		"Models",
		"Tools (top 5)",
		"Files touched",
		"Activity heatmap",
		"Total:",
		"tokenmeter/main",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("analyze output missing %q:\n%s", want, out)
		}
	}
}

func TestRunAnalyzeJSONFormat(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-time.Hour)
	seedAnalyzeCLISession(t, db, "analyze-json", event.PlatformCodex, now, "gpt-5.5", 3.75, "README.md")

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--json"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})

	var payload struct {
		Range string `json:"range"`
		Cost  struct {
			Total float64 `json:"total"`
		} `json:"cost"`
		Sessions struct {
			Total int `json:"total"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid analyze json: %v\n%s", err, out)
	}
	if payload.Cost.Total != 3.75 || payload.Sessions.Total != 1 {
		t.Fatalf("unexpected json payload: %#v", payload)
	}
}

func TestRunAnalyzeToolErrorsJSONFormat(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-24 * time.Hour)
	seedAnalyzeCLISession(t, db, "tool-errors-json", event.PlatformClaude, now, "sonnet", 2.5, "internal/foo.go")
	for i := 0; i < 3; i++ {
		callID := "tool-errors-fail-" + string(rune('a'+i))
		if _, err := db.InsertToolCallStart(callID, "agent-tool-errors-json", "tool-errors-json", "Bash", "{}", now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("insert fail start: %v", err)
		}
		if err := db.UpdateToolCallEnd(callID, "exit code 1: /Users/admin/code/agmon/file123.go", event.StatusFail, 50, now.Add(time.Duration(i)*time.Hour+time.Second)); err != nil {
			t.Fatalf("insert fail end: %v", err)
		}
	}

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--tool-errors", "--json"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})

	var payload struct {
		ToolErrors struct {
			TopTools []struct {
				Tool string `json:"tool"`
			} `json:"top_tools"`
			Patterns []struct {
				Count int64 `json:"count"`
			} `json:"patterns"`
			Daily []struct {
				Date string `json:"date"`
			} `json:"daily"`
		} `json:"tool_errors"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid tool error json: %v\n%s", err, out)
	}
	if len(payload.ToolErrors.TopTools) == 0 || payload.ToolErrors.TopTools[0].Tool != "Bash" {
		t.Fatalf("unexpected top tools: %+v", payload.ToolErrors.TopTools)
	}
	if len(payload.ToolErrors.Patterns) == 0 || payload.ToolErrors.Patterns[0].Count < 3 {
		t.Fatalf("expected grouped pattern: %+v", payload.ToolErrors.Patterns)
	}
	if len(payload.ToolErrors.Daily) == 0 {
		t.Fatalf("expected daily failure series")
	}
}

func TestRunAnalyzeToolErrorsTextIncludesSections(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-24 * time.Hour)
	seedAnalyzeCLISession(t, db, "tool-errors-text", event.PlatformClaude, now, "sonnet", 2.5, "internal/foo.go")
	for i := 0; i < 3; i++ {
		callID := "tool-errors-text-fail-" + string(rune('a'+i))
		if _, err := db.InsertToolCallStart(callID, "agent-tool-errors-text", "tool-errors-text", "Edit", "{}", now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("insert fail start: %v", err)
		}
		if err := db.UpdateToolCallEnd(callID, "String to replace not found in /tmp/file42.go", event.StatusFail, 50, now.Add(time.Duration(i)*time.Hour+time.Second)); err != nil {
			t.Fatalf("insert fail end: %v", err)
		}
	}

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--tool-errors"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})
	for _, want := range []string{"Top Failing Tools", "Error Pattern Groups", "Daily failure rate", "Edit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("tool error output missing %q:\n%s", want, out)
		}
	}
}

func TestRunAnalyzeFileChurnJSONFormat(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-24 * time.Hour)
	seedAnalyzeCLISession(t, db, "file-churn-json", event.PlatformClaude, now, "sonnet", 2.5, "internal/storage/db.go")
	for i := 0; i < 3; i++ {
		if err := db.InsertFileChange("file-churn-json", "internal/storage/db.go", event.FileEdit, now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("insert file change: %v", err)
		}
	}

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--file-churn", "--limit", "1", "--json"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})

	var payload struct {
		FileChurn struct {
			TopFiles []struct {
				Path string `json:"path"`
			} `json:"top_files"`
			Hotspots []struct {
				Path string `json:"path"`
			} `json:"hotspots"`
			Daily []struct {
				Changes int64 `json:"changes"`
			} `json:"daily"`
		} `json:"file_churn"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid file churn json: %v\n%s", err, out)
	}
	if len(payload.FileChurn.TopFiles) != 1 || payload.FileChurn.TopFiles[0].Path != "internal/storage/db.go" {
		t.Fatalf("unexpected top files: %+v", payload.FileChurn.TopFiles)
	}
	if len(payload.FileChurn.Hotspots) == 0 || payload.FileChurn.Hotspots[0].Path != "internal/storage" {
		t.Fatalf("unexpected hotspots: %+v", payload.FileChurn.Hotspots)
	}
	if len(payload.FileChurn.Daily) == 0 {
		t.Fatalf("expected daily churn series")
	}
}

func TestRunAnalyzeToolErrorsAndFileChurnJSONFormat(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-24 * time.Hour)
	seedAnalyzeCLISession(t, db, "combined-json", event.PlatformClaude, now, "sonnet", 2.5, "cmd/tm/analyze.go")
	for i := 0; i < 3; i++ {
		callID := "combined-fail-" + string(rune('a'+i))
		if _, err := db.InsertToolCallStart(callID, "agent-combined-json", "combined-json", "Bash", "{}", now.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("insert fail start: %v", err)
		}
		if err := db.UpdateToolCallEnd(callID, "exit code 1", event.StatusFail, 50, now.Add(time.Duration(i)*time.Hour+time.Second)); err != nil {
			t.Fatalf("insert fail end: %v", err)
		}
	}

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--tool-errors", "--file-churn", "--json"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid combined json: %v\n%s", err, out)
	}
	if len(payload["tool_errors"]) == 0 || len(payload["file_churn"]) == 0 {
		t.Fatalf("expected both envelopes, got keys: %+v", payload)
	}
}

func TestRunAnalyzeFileChurnTextIncludesSections(t *testing.T) {
	home := t.TempDir()
	db := openHomeDB(t, home)
	now := time.Now().Add(-24 * time.Hour)
	seedAnalyzeCLISession(t, db, "file-churn-text", event.PlatformClaude, now, "sonnet", 2.5, "cmd/tm/analyze.go")

	withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--file-churn"})
	out := captureStdout(t, func() {
		if err := runAnalyze(); err != nil {
			t.Fatalf("runAnalyze: %v", err)
		}
	})
	for _, want := range []string{"Top Changed Files", "File Churn Hotspots", "Daily file changes", "cmd/tm/analyze.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("file churn output missing %q:\n%s", want, out)
		}
	}
}

func seedAnalyzeCLISession(t *testing.T, db interface {
	UpsertSession(string, event.Platform, time.Time) error
	UpdateSessionMeta(string, string, string) error
	UpsertAgent(string, string, string, string, time.Time) error
	InsertToolCallStart(string, string, string, string, string, time.Time) (bool, error)
	UpdateToolCallEnd(string, string, event.ToolCallStatus, int64, time.Time) error
	InsertFileChange(string, string, event.FileChangeType, time.Time) error
	InsertTokenUsage(string, string, int, int, int, int, string, float64, time.Time, string) error
}, sessionID string, platform event.Platform, ts time.Time, model string, cost float64, filePath string) {
	t.Helper()
	if err := db.UpsertSession(sessionID, platform, ts); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := db.UpdateSessionMeta(sessionID, "/repo/tokenmeter", "main"); err != nil {
		t.Fatalf("update meta: %v", err)
	}
	agentID := "agent-" + sessionID
	if err := db.UpsertAgent(agentID, sessionID, "", "main", ts); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	if _, err := db.InsertToolCallStart("call-"+sessionID, agentID, sessionID, "Edit", filePath, ts); err != nil {
		t.Fatalf("insert tool: %v", err)
	}
	if err := db.UpdateToolCallEnd("call-"+sessionID, "ok", event.StatusSuccess, 240, ts.Add(time.Second)); err != nil {
		t.Fatalf("update tool: %v", err)
	}
	if err := db.InsertFileChange(sessionID, filePath, event.FileEdit, ts.Add(2*time.Second)); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	if err := db.InsertTokenUsage(agentID, sessionID, 1000, 250, 0, 0, model, cost, ts.Add(3*time.Second), "token-"+sessionID); err != nil {
		t.Fatalf("insert token: %v", err)
	}
}
