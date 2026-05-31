package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func captureStdoutErr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	prev := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w

	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- data
	}()

	runErr := fn()
	_ = w.Close()
	out := string(<-done)
	os.Stdout = prev
	return out, runErr
}

func seedParityDB(t *testing.T, home string) {
	t.Helper()
	setTestHome(t, home)
	db, err := storage.Open(storage.DefaultDBPath())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ts := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	seedCLISession(t, db, "claude-parity", event.PlatformClaude, "/repo/claude", "main", ts, 100, 10, 1.25)
	seedCLISession(t, db, "codex-parity", event.PlatformCodex, "/repo/codex", "main", ts.Add(time.Hour), 200, 20, 2.50)
}

func decodeDailyTotals(t *testing.T, body string) struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	TotalCost    float64 `json:"totalCost"`
} {
	t.Helper()
	var got struct {
		Daily  []json.RawMessage `json:"daily"`
		Totals struct {
			InputTokens  int64   `json:"inputTokens"`
			OutputTokens int64   `json:"outputTokens"`
			TotalCost    float64 `json:"totalCost"`
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal daily JSON: %v\n%s", err, body)
	}
	return got.Totals
}

func TestRunCLIDispatchGlobalJSONBeforeDaily(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TOKENMETER_OFFLINE", "1")
	seedParityDB(t, home)

	out, err := captureStdoutErr(t, func() error {
		return runCLIDispatch(normalizeTopLevelArgs([]string{"--json", "daily"}))
	})
	if err != nil {
		t.Fatalf("runCLIDispatch: %v", err)
	}
	totals := decodeDailyTotals(t, out)
	if totals.InputTokens != 300 || totals.OutputTokens != 30 {
		t.Fatalf("global --json daily totals mismatch: %+v\n%s", totals, out)
	}
}

func TestRunCLIDispatchGlobalSinceBeforeDailyFiltersFuture(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TOKENMETER_OFFLINE", "1")
	seedParityDB(t, home)

	out, err := captureStdoutErr(t, func() error {
		return runCLIDispatch(normalizeTopLevelArgs([]string{"-s", "29990101", "daily", "--json"}))
	})
	if err != nil {
		t.Fatalf("runCLIDispatch: %v", err)
	}
	totals := decodeDailyTotals(t, out)
	if totals.InputTokens != 0 || totals.OutputTokens != 0 || totals.TotalCost != 0 {
		t.Fatalf("future -s daily should be empty, got %+v\n%s", totals, out)
	}
}

func TestRunCLIDispatchSourceDailyJSONFiltersPlatform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TOKENMETER_OFFLINE", "1")
	seedParityDB(t, home)

	claudeOut, err := captureStdoutErr(t, func() error {
		return runCLIDispatch([]string{"claude", "daily", "--json"})
	})
	if err != nil {
		t.Fatalf("claude daily: %v", err)
	}
	if totals := decodeDailyTotals(t, claudeOut); totals.InputTokens != 100 || totals.OutputTokens != 10 {
		t.Fatalf("claude daily totals mismatch: %+v\n%s", totals, claudeOut)
	}

	codexOut, err := captureStdoutErr(t, func() error {
		return runCLIDispatch([]string{"codex", "daily", "--json"})
	})
	if err != nil {
		t.Fatalf("codex daily: %v", err)
	}
	if totals := decodeDailyTotals(t, codexOut); totals.InputTokens != 200 || totals.OutputTokens != 20 {
		t.Fatalf("codex daily totals mismatch: %+v\n%s", totals, codexOut)
	}
}

func TestRunCLIDispatchDailyJQFiltersOutput(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}
	home := t.TempDir()
	t.Setenv("TOKENMETER_OFFLINE", "1")
	seedParityDB(t, home)

	out, err := captureStdoutErr(t, func() error {
		return runCLIDispatch([]string{"daily", "--no-scan", "--jq", ".totals.inputTokens"})
	})
	if err != nil {
		t.Fatalf("daily --jq: %v", err)
	}
	if out != "300\n" {
		t.Fatalf("jq-filtered output = %q, want 300\\n", out)
	}
}
