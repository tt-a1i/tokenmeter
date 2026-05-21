package cli

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestWeekKeySQLiteWMatchesSQLite cross-checks weekKeySQLiteW against
// SQLite's actual strftime('%Y-W%W', ts) output for a handful of
// boundary dates. If the Go implementation drifts (or a future SQLite
// release changes %W semantics) this test catches the divergence
// before user-visible weekly bucket asymmetry returns.
//
// Boundary cases worth pinning:
//   - First few days of the year (week 00 before the first Monday)
//   - The year's first Monday (week 01 start)
//   - Mid-year arbitrary date
//   - Year-end (last week of the year)
//   - Leap-year year-end (week 53 edge)
//   - January 1 that falls on a Sunday (special case for daysUntilFirstMonday)
func TestWeekKeySQLiteWMatchesSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	cases := []struct {
		name string
		date string // "YYYY-MM-DD HH:MM:SS"
	}{
		{"2026-01-01-thu-w00", "2026-01-01 00:00:00"},
		{"2026-01-04-sun-w00", "2026-01-04 00:00:00"},
		{"2026-01-05-mon-w01", "2026-01-05 00:00:00"},
		{"2026-05-22-mid", "2026-05-22 00:00:00"},
		{"2025-12-31-year-end", "2025-12-31 00:00:00"},
		{"2024-01-01-leap-mon-w01", "2024-01-01 00:00:00"},
		{"2024-12-31-leap-w53", "2024-12-31 00:00:00"},
		{"2023-01-01-sun-w00", "2023-01-01 00:00:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var want string
			if err := db.QueryRow(`SELECT strftime('%Y-W%W', ?)`, tc.date).Scan(&want); err != nil {
				t.Fatalf("SQLite strftime: %v", err)
			}
			parsed, err := time.Parse("2006-01-02 15:04:05", tc.date)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.date, err)
			}
			got := weekKeySQLiteW(parsed)
			if got != want {
				t.Errorf("date=%s: go=%q sqlite=%q", tc.date, got, want)
			}
		})
	}
}
