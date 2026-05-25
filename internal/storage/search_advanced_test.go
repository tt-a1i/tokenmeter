package storage

import (
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestSearchAdvancedKeywordOnlyMatchesExistingBehavior(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "keyword-session", event.PlatformClaude, now, 100, 20, 2)
	if _, err := db.InsertToolCallStart("keyword-call", "agent-keyword-session", "keyword-session", "Bash", "run needle command", now); err != nil {
		t.Fatalf("InsertToolCallStart: %v", err)
	}

	hits, err := db.SearchAdvanced(Query{Keywords: []string{"needle"}}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 1 || hits[0].SessionID != "keyword-session" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestSearchAdvancedToolFilter(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "tool-session", event.PlatformClaude, now, 100, 20, 2)
	if _, err := db.InsertToolCallStart("bash-call", "agent-tool-session", "tool-session", "Bash", "needle bash", now); err != nil {
		t.Fatalf("InsertToolCallStart bash: %v", err)
	}
	if _, err := db.InsertToolCallStart("read-call", "agent-tool-session", "tool-session", "Read", "needle read", now.Add(time.Second)); err != nil {
		t.Fatalf("InsertToolCallStart read: %v", err)
	}

	hits, err := db.SearchAdvanced(Query{Keywords: []string{"needle"}, Tool: "Bash"}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 1 || hits[0].Kind != "tool_param" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestSearchAdvancedStatusFilter(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "status-session", event.PlatformClaude, now, 100, 20, 2)
	if _, err := db.InsertToolCallStart("ok-call", "agent-status-session", "status-session", "Bash", "needle ok", now); err != nil {
		t.Fatalf("InsertToolCallStart ok: %v", err)
	}
	if err := db.UpdateToolCallEnd("ok-call", "needle ok result", event.StatusSuccess, 1, now.Add(time.Second)); err != nil {
		t.Fatalf("UpdateToolCallEnd ok: %v", err)
	}
	if _, err := db.InsertToolCallStart("fail-call", "agent-status-session", "status-session", "Bash", "needle fail", now.Add(2*time.Second)); err != nil {
		t.Fatalf("InsertToolCallStart fail: %v", err)
	}
	if err := db.UpdateToolCallEnd("fail-call", "needle fail result", event.StatusFail, 1, now.Add(3*time.Second)); err != nil {
		t.Fatalf("UpdateToolCallEnd fail: %v", err)
	}

	hits, err := db.SearchAdvanced(Query{Keywords: []string{"needle"}, Status: "failed"}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("failed status should include failed params and result, got %+v", hits)
	}
}

func TestSearchAdvancedCompositeFilters(t *testing.T) {
	db := testDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "rich-match", event.PlatformCodex, base, 120000, 1000, 7)
	if _, err := db.InsertToolCallStart("rich-call", "agent-rich-match", "rich-match", "Bash", "exit code from tests", base.Add(time.Hour)); err != nil {
		t.Fatalf("InsertToolCallStart match: %v", err)
	}
	seedSearchAdvancedSession(t, db, "too-cheap", event.PlatformCodex, base, 120000, 1000, 0.5)
	if _, err := db.InsertToolCallStart("cheap-call", "agent-too-cheap", "too-cheap", "Bash", "exit code from tests", base.Add(time.Hour)); err != nil {
		t.Fatalf("InsertToolCallStart cheap: %v", err)
	}
	minCost := 1.0
	since := base.Add(-time.Minute)

	hits, err := db.SearchAdvanced(Query{Keywords: []string{"exit", "code"}, Tool: "Bash", CostMin: &minCost, Since: &since}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 1 || hits[0].SessionID != "rich-match" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestSearchAdvancedEmptyRange(t *testing.T) {
	db := testDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "range-session", event.PlatformClaude, base, 100, 20, 2)
	if _, err := db.InsertToolCallStart("range-call", "agent-range-session", "range-session", "Bash", "needle", base); err != nil {
		t.Fatalf("InsertToolCallStart: %v", err)
	}
	since := base.AddDate(0, 0, 1)
	hits, err := db.SearchAdvanced(Query{Keywords: []string{"needle"}, Since: &since}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits, got %+v", hits)
	}
}

func TestSearchAdvancedToolFilterUsesBindParameters(t *testing.T) {
	db := testDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedSearchAdvancedSession(t, db, "injection-session", event.PlatformClaude, now, 100, 20, 2)
	if _, err := db.InsertToolCallStart("injection-call", "agent-injection-session", "injection-session", "Bash", "needle", now); err != nil {
		t.Fatalf("InsertToolCallStart: %v", err)
	}

	hits, err := db.SearchAdvanced(Query{Keywords: []string{"needle"}, Tool: "Bash' OR 1=1 --"}, 10)
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("SQL injection-like tool filter matched rows: %+v", hits)
	}
}

func seedSearchAdvancedSession(t *testing.T, db *DB, sessionID string, platform event.Platform, ts time.Time, input, output int, cost float64) {
	t.Helper()
	if err := db.UpsertSession(sessionID, platform, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if err := db.UpdateSessionMeta(sessionID, "/repo/search", "main"); err != nil {
		t.Fatalf("UpdateSessionMeta: %v", err)
	}
	if err := db.UpsertAgent("agent-"+sessionID, sessionID, "", "main", ts); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := db.InsertTokenUsage("agent-"+sessionID, sessionID, input, output, 0, 0, "model", cost, ts, "token-"+sessionID); err != nil {
		t.Fatalf("InsertTokenUsage: %v", err)
	}
}
