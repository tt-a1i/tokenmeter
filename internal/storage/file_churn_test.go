package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestTopChurnFilesSortsLimitsCountsSessionsModesAndRange(t *testing.T) {
	db := fileChurnTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedFileChange(t, db, "s1", "internal/storage/db.go", event.FileEdit, base)
	seedFileChange(t, db, "s1", "internal/storage/db.go", event.FileEdit, base.Add(time.Hour))
	seedFileChange(t, db, "s2", "internal/storage/db.go", event.FileCreate, base.Add(2*time.Hour))
	seedFileChange(t, db, "s3", "cmd/tm/main.go", event.FileEdit, base.Add(3*time.Hour))
	seedFileChange(t, db, "s3", "cmd/tm/main.go", event.FileChangeType("read"), base.Add(4*time.Hour))
	seedFileChange(t, db, "old", "README.md", event.FileEdit, base.AddDate(0, 0, -2))

	rows, err := db.TopChurnFiles(base.Add(-time.Minute), base.AddDate(0, 0, 1), 1)
	if err != nil {
		t.Fatalf("TopChurnFiles: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len=%d want 1", len(rows))
	}
	got := rows[0]
	if got.Path != "internal/storage/db.go" || got.Changes != 3 || got.Sessions != 2 {
		t.Fatalf("unexpected top file: %+v", got)
	}
	if got.ModeCounts["edit"] != 2 || got.ModeCounts["create"] != 1 {
		t.Fatalf("mode counts not aggregated: %+v", got.ModeCounts)
	}
	if got.FirstSeen.Format("2006-01-02") != "2026-05-10" || got.LastSeen.Format("15:04") != "12:00" {
		t.Fatalf("unexpected first/last seen: %+v", got)
	}
}

func TestTopChurnFilesScopedFiltersByProject(t *testing.T) {
	db := fileChurnTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedFileChangeWithCWD(t, db, "current-1", "/repo/current", "internal/storage/db.go", event.FileEdit, base)
	seedFileChangeWithCWD(t, db, "current-2", "/work/current", "cmd/tm/analyze.go", event.FileEdit, base.Add(time.Hour))
	seedFileChangeWithCWD(t, db, "other", "/repo/other", "README.md", event.FileEdit, base.Add(2*time.Hour))

	rows, err := db.TopChurnFilesScoped(base.Add(-time.Minute), base.AddDate(0, 0, 1), 10, ProjectScope{Project: "current"})
	if err != nil {
		t.Fatalf("TopChurnFilesScoped: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Path == "README.md" {
			t.Fatalf("other project leaked into scoped rows: %+v", rows)
		}
	}
}

func TestChurnHotspotsAndDailyScopedFiltersByProject(t *testing.T) {
	db := fileChurnTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedFileChangeWithCWD(t, db, "current", "/repo/current", "internal/storage/db.go", event.FileEdit, base)
	seedFileChangeWithCWD(t, db, "other", "/repo/other", "docs/site/index.md", event.FileEdit, base.Add(time.Hour))
	scope := ProjectScope{Project: "current"}

	hotspots, err := db.ChurnHotspotsScoped(base.Add(-time.Minute), base.AddDate(0, 0, 1), 2, 10, scope)
	if err != nil {
		t.Fatalf("ChurnHotspotsScoped: %v", err)
	}
	if len(hotspots) != 1 || hotspots[0].Path != "internal/storage" {
		t.Fatalf("unexpected scoped hotspots: %+v", hotspots)
	}
	daily, err := db.DailyChurnTrendScoped(base.Add(-time.Minute), base.AddDate(0, 0, 1), scope)
	if err != nil {
		t.Fatalf("DailyChurnTrendScoped: %v", err)
	}
	if len(daily) != 2 || daily[0].Changes != 1 {
		t.Fatalf("unexpected scoped daily trend: %+v", daily)
	}
}

func TestChurnHotspotsDepthAndDistinctFileCount(t *testing.T) {
	db := fileChurnTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		seedFileChange(t, db, "s-storage", "internal/storage/db.go", event.FileEdit, base.Add(time.Duration(i)*time.Hour))
	}
	seedFileChange(t, db, "s-storage-2", "internal/storage/query.go", event.FileEdit, base.Add(4*time.Hour))
	seedFileChange(t, db, "s-web", "internal/web/static/index.html", event.FileEdit, base.Add(5*time.Hour))
	seedFileChange(t, db, "s-cmd", "cmd/tm/main.go", event.FileEdit, base.Add(6*time.Hour))

	depth1, err := db.ChurnHotspots(base.Add(-time.Minute), base.AddDate(0, 0, 1), 1, 10)
	if err != nil {
		t.Fatalf("ChurnHotspots depth1: %v", err)
	}
	if depth1[0].Path != "internal" || depth1[0].Changes != 5 || depth1[0].Files != 3 || depth1[0].TopFile != "db.go" {
		t.Fatalf("unexpected depth1 hotspot: %+v", depth1[0])
	}

	depth2, err := db.ChurnHotspots(base.Add(-time.Minute), base.AddDate(0, 0, 1), 2, 10)
	if err != nil {
		t.Fatalf("ChurnHotspots depth2: %v", err)
	}
	if depth2[0].Path != "internal/storage" || depth2[0].Changes != 4 || depth2[0].Files != 2 {
		t.Fatalf("unexpected depth2 hotspot: %+v", depth2[0])
	}

	depth3, err := db.ChurnHotspots(base.Add(-time.Minute), base.AddDate(0, 0, 1), 3, 10)
	if err != nil {
		t.Fatalf("ChurnHotspots depth3: %v", err)
	}
	if depth3[0].Path != "internal/storage" {
		t.Fatalf("depth3 should not include file names for two-dir paths: %+v", depth3[0])
	}
}

func TestChurnHotspotsPreservesAbsolutePathRoot(t *testing.T) {
	db := fileChurnTestDB(t)
	base := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	seedFileChange(t, db, "s1", "/Users/admin/code/agmon/main.go", event.FileEdit, base)

	rows, err := db.ChurnHotspots(base.Add(-time.Minute), base.AddDate(0, 0, 1), 2, 10)
	if err != nil {
		t.Fatalf("ChurnHotspots: %v", err)
	}
	if rows[0].Path != "/Users/admin" {
		t.Fatalf("absolute hotspot path=%q want /Users/admin", rows[0].Path)
	}
}

func TestDailyChurnTrendIncludesGapDaysAndCrossMonth(t *testing.T) {
	db := fileChurnTestDB(t)
	jan31 := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	feb2 := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	seedFileChange(t, db, "s1", "a.go", event.FileEdit, jan31)
	seedFileChange(t, db, "s1", "b.go", event.FileEdit, jan31.Add(time.Hour))
	seedFileChange(t, db, "s2", "c.go", event.FileEdit, feb2)

	rows, err := db.DailyChurnTrend(jan31, time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DailyChurnTrend: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(rows), rows)
	}
	if rows[0].Date.Format("2006-01-02") != "2026-01-31" || rows[0].Changes != 2 {
		t.Fatalf("jan31 row=%+v", rows[0])
	}
	if rows[1].Date.Format("2006-01-02") != "2026-02-01" || rows[1].Changes != 0 {
		t.Fatalf("gap row=%+v", rows[1])
	}
	if rows[2].Date.Format("2006-01-02") != "2026-02-02" || rows[2].Changes != 1 {
		t.Fatalf("feb2 row=%+v", rows[2])
	}
}

func TestDailyChurnTrendEmptyRange(t *testing.T) {
	db := fileChurnTestDB(t)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	rows, err := db.DailyChurnTrend(now, now)
	if err != nil {
		t.Fatalf("DailyChurnTrend: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty range len=%d", len(rows))
	}
}

func fileChurnTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "file-churn.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedFileChange(t *testing.T, db *DB, sessionID, filePath string, changeType event.FileChangeType, ts time.Time) {
	t.Helper()
	seedFileChangeWithCWD(t, db, sessionID, "", filePath, changeType, ts)
}

func seedFileChangeWithCWD(t *testing.T, db *DB, sessionID, cwd, filePath string, changeType event.FileChangeType, ts time.Time) {
	t.Helper()
	if err := db.UpsertSession(sessionID, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if cwd != "" {
		if err := db.UpdateSessionMeta(sessionID, cwd, "main"); err != nil {
			t.Fatalf("UpdateSessionMeta: %v", err)
		}
	}
	if err := db.InsertFileChange(sessionID, filePath, changeType, ts); err != nil {
		t.Fatalf("InsertFileChange: %v", err)
	}
}
