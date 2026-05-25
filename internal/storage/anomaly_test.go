package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestDailyCostSpikeThresholds(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		todayCost float64
		want      bool
	}{
		{name: "one point five times does not trigger", todayCost: 15, want: false},
		{name: "two times triggers", todayCost: 20, want: true},
		{name: "two point five times triggers", todayCost: 25, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := anomalyTestDB(t)
			insertDailyCosts(t, db, base, 7, 10)
			insertTokenCost(t, db, "today", tt.todayCost, base)

			got, err := DailyCostSpike(db, base, 7, 2.0)
			if err != nil {
				t.Fatalf("DailyCostSpike: %v", err)
			}
			if (got != nil) != tt.want {
				t.Fatalf("trigger = %v, want %v; spike=%#v", got != nil, tt.want, got)
			}
			if got != nil && (got.TodayCost != tt.todayCost || got.BaselineCost != 10 || got.Ratio != tt.todayCost/10) {
				t.Fatalf("unexpected spike payload: %#v", got)
			}
		})
	}
}

func TestDailyCostSpikeRequiresFullLookback(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for _, historyDays := range []int{0, 1} {
		db := anomalyTestDB(t)
		insertDailyCosts(t, db, base, historyDays, 10)
		insertTokenCost(t, db, "today", 100, base)

		got, err := DailyCostSpike(db, base, 7, 2.0)
		if err != nil {
			t.Fatalf("DailyCostSpike history=%d: %v", historyDays, err)
		}
		if got != nil {
			t.Fatalf("history=%d should not trigger with insufficient lookback: %#v", historyDays, got)
		}
	}
}

func TestHourlyToolFailureRegressionTriggersForDoubledFailureRate(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	db := anomalyTestDB(t)
	insertToolCalls(t, db, "Bash", now.Add(-24*time.Hour), 100, 10)
	insertToolCalls(t, db, "Bash", now.Add(-30*time.Minute), 20, 10)

	got, err := HourlyToolFailureRegression(db, now, 7, 10, 2.0)
	if err != nil {
		t.Fatalf("HourlyToolFailureRegression: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("regressions = %#v, want one", got)
	}
	if got[0].Tool != "Bash" || got[0].RecentFailures != 10 || got[0].RecentFailureRate != 50 || got[0].BaselineFailureRate != 10 || got[0].Ratio != 5 {
		t.Fatalf("unexpected regression: %#v", got[0])
	}
}

func TestHourlyToolFailureRegressionHonorsMinFailures(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	db := anomalyTestDB(t)
	insertToolCalls(t, db, "Bash", now.Add(-24*time.Hour), 100, 10)
	insertToolCalls(t, db, "Bash", now.Add(-30*time.Minute), 18, 9)

	got, err := HourlyToolFailureRegression(db, now, 7, 10, 2.0)
	if err != nil {
		t.Fatalf("HourlyToolFailureRegression: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("regression should honor min failures, got %#v", got)
	}
}

func TestHourlyToolFailureRegressionMultipleTools(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	db := anomalyTestDB(t)
	insertToolCalls(t, db, "Bash", now.Add(-24*time.Hour), 100, 10)
	insertToolCalls(t, db, "Edit", now.Add(-24*time.Hour), 100, 5)
	insertToolCalls(t, db, "Bash", now.Add(-30*time.Minute), 20, 10)
	insertToolCalls(t, db, "Edit", now.Add(-20*time.Minute), 20, 10)

	got, err := HourlyToolFailureRegression(db, now, 7, 10, 2.0)
	if err != nil {
		t.Fatalf("HourlyToolFailureRegression: %v", err)
	}
	if len(got) != 2 || got[0].Tool != "Edit" || got[1].Tool != "Bash" {
		t.Fatalf("regressions should sort by ratio then failures, got %#v", got)
	}
}

func TestHourlyToolFailureRegressionRequiresLookbackData(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	db := anomalyTestDB(t)
	insertToolCalls(t, db, "Bash", now.Add(-30*time.Minute), 20, 10)

	got, err := HourlyToolFailureRegression(db, now, 7, 10, 2.0)
	if err != nil {
		t.Fatalf("HourlyToolFailureRegression: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("lookback data missing should not trigger, got %#v", got)
	}
}

func anomalyTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "anomaly.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertDailyCosts(t *testing.T, db *DB, today time.Time, days int, cost float64) {
	t.Helper()
	for i := 1; i <= days; i++ {
		insertTokenCost(t, db, "history-"+time.Duration(i).String(), cost, today.AddDate(0, 0, -i))
	}
}

func insertTokenCost(t *testing.T, db *DB, session string, cost float64, ts time.Time) {
	t.Helper()
	if err := db.UpsertSession(session, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if err := db.InsertTokenUsage("agent-"+session, session, 1, 1, 0, 0, "sonnet", cost, ts, session+"-token"); err != nil {
		t.Fatalf("InsertTokenUsage: %v", err)
	}
}

func insertToolCalls(t *testing.T, db *DB, tool string, ts time.Time, total, failures int) {
	t.Helper()
	session := tool + "-" + ts.Format("20060102150405")
	if err := db.UpsertSession(session, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	for i := 0; i < total; i++ {
		callID := session + "-" + time.Duration(i).String()
		start := ts.Add(time.Duration(i) * time.Millisecond)
		if _, err := db.InsertToolCallStart(callID, "agent-"+session, session, tool, "{}", start); err != nil {
			t.Fatalf("InsertToolCallStart: %v", err)
		}
		status := event.StatusSuccess
		if i < failures {
			status = event.StatusFail
		}
		if err := db.UpdateToolCallEnd(callID, "result", status, 1, start.Add(time.Millisecond)); err != nil {
			t.Fatalf("UpdateToolCallEnd: %v", err)
		}
	}
}
