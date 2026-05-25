package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestNormalizeToolErrorPattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   "Exit code 1: gofmt /Users/admin/code/agmon/internal/storage/tool_errors.go:123",
			want: "exit code N: gofmt /users/.../tool_errors.go:N",
		},
		{
			in:   "String to replace not found in /tmp/work/file42.go",
			want: "string to replace not found in /tmp/.../fileN.go",
		},
		{
			in:   "No such file or directory",
			want: "no such file or directory",
		},
	}
	for _, tt := range tests {
		if got := NormalizeToolErrorPattern(tt.in); got != tt.want {
			t.Fatalf("NormalizeToolErrorPattern(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestTopFailingToolsSortsLimitsAndHonorsRange(t *testing.T) {
	db := toolErrorsTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedToolCall(t, db, "bash-ok", "s1", "Bash", "ok", event.StatusSuccess, base)
	for i := 0; i < 3; i++ {
		seedToolCall(t, db, "bash-fail-"+string(rune('a'+i)), "s1", "Bash", "exit code 1: missing file 123", event.StatusFail, base.Add(time.Duration(i)*time.Hour))
	}
	seedToolCall(t, db, "edit-ok", "s2", "Edit", "ok", event.StatusSuccess, base)
	seedToolCall(t, db, "edit-fail", "s2", "Edit", "String to replace not found", event.StatusFail, base.Add(time.Hour))
	seedToolCall(t, db, "old-fail", "s3", "Read", "No such file or directory", event.StatusFail, base.AddDate(0, 0, -2))

	stats, err := db.TopFailingTools(base.Add(-time.Minute), base.AddDate(0, 0, 1), 1)
	if err != nil {
		t.Fatalf("TopFailingTools: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("len=%d want 1", len(stats))
	}
	if stats[0].Tool != "Bash" || stats[0].Total != 4 || stats[0].Failures != 3 {
		t.Fatalf("unexpected top stat: %+v", stats[0])
	}
	if stats[0].TopPattern != "exit code N: missing file N" {
		t.Fatalf("TopPattern=%q", stats[0].TopPattern)
	}
}

func TestErrorPatternGroupsAggregatesAcrossToolsAndThreshold(t *testing.T) {
	db := toolErrorsTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedToolCall(t, db, "p1", "session-a", "Bash", "No such file or directory: /Users/admin/a1.go", event.StatusFail, base)
	seedToolCall(t, db, "p2", "session-b", "Read", "no such file or directory: /Users/admin/a2.go", event.StatusFail, base.Add(time.Hour))
	seedToolCall(t, db, "p3", "session-c", "Bash", "NO SUCH FILE OR DIRECTORY: /Users/admin/a3.go", event.StatusFail, base.Add(2*time.Hour))
	seedToolCall(t, db, "rare", "session-d", "Edit", "String to replace not found", event.StatusFail, base.Add(3*time.Hour))

	patterns, err := db.ErrorPatternGroups(base.Add(-time.Minute), base.AddDate(0, 0, 1), 3)
	if err != nil {
		t.Fatalf("ErrorPatternGroups: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(patterns), patterns)
	}
	got := patterns[0]
	if got.Count != 3 || got.NormalizedKey != "no such file or directory: /users/.../aN.go" {
		t.Fatalf("unexpected pattern: %+v", got)
	}
	if len(got.Tools) != 2 || got.Tools[0] != "Bash" || got.Tools[1] != "Read" {
		t.Fatalf("tools not aggregated/sorted: %+v", got.Tools)
	}
	if len(got.Sessions) != 3 {
		t.Fatalf("sessions=%+v", got.Sessions)
	}
}

func TestDailyFailureRateIncludesGapDaysAndCrossMonth(t *testing.T) {
	db := toolErrorsTestDB(t)
	jan31 := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	feb2 := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	seedToolCall(t, db, "jan-ok", "s1", "Bash", "ok", event.StatusSuccess, jan31)
	seedToolCall(t, db, "jan-fail", "s1", "Bash", "exit code 1", event.StatusFail, jan31.Add(time.Hour))
	seedToolCall(t, db, "feb-fail", "s2", "Edit", "String to replace not found", event.StatusFail, feb2)

	rates, err := db.DailyFailureRate(jan31, time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DailyFailureRate: %v", err)
	}
	if len(rates) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(rates), rates)
	}
	if rates[0].Date != "2026-01-31" || rates[0].Total != 2 || rates[0].Failures != 1 || rates[0].FailureRate != 50 {
		t.Fatalf("jan31 row=%+v", rates[0])
	}
	if rates[1].Date != "2026-02-01" || rates[1].Total != 0 || rates[1].FailureRate != 0 {
		t.Fatalf("gap row=%+v", rates[1])
	}
	if rates[2].Date != "2026-02-02" || rates[2].FailureRate != 100 {
		t.Fatalf("feb2 row=%+v", rates[2])
	}
}

func TestDailyFailureRateEmptyRange(t *testing.T) {
	db := toolErrorsTestDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	rates, err := db.DailyFailureRate(now, now)
	if err != nil {
		t.Fatalf("DailyFailureRate: %v", err)
	}
	if len(rates) != 0 {
		t.Fatalf("empty range len=%d", len(rates))
	}
}

func toolErrorsTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "tool-errors.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedToolCall(t *testing.T, db *DB, callID, sessionID, tool, result string, status event.ToolCallStatus, ts time.Time) {
	t.Helper()
	if err := db.UpsertSession(sessionID, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if _, err := db.InsertToolCallStart(callID, "agent-"+sessionID, sessionID, tool, "{}", ts); err != nil {
		t.Fatalf("InsertToolCallStart: %v", err)
	}
	if err := db.UpdateToolCallEnd(callID, result, status, 100, ts.Add(time.Second)); err != nil {
		t.Fatalf("UpdateToolCallEnd: %v", err)
	}
}
