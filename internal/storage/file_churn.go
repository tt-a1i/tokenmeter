package storage

import (
	"path"
	"sort"
	"strings"
	"time"
)

type FileChurnStats struct {
	Path       string           `json:"path"`
	Changes    int64            `json:"changes"`
	Sessions   int64            `json:"sessions"`
	FirstSeen  time.Time        `json:"first_seen"`
	LastSeen   time.Time        `json:"last_seen"`
	ModeCounts map[string]int64 `json:"mode_counts"`
}

type HotspotStats struct {
	Path    string `json:"path"`
	Changes int64  `json:"changes"`
	Files   int64  `json:"files"`
	TopFile string `json:"top_file"`
}

type DailyChurn struct {
	Date    time.Time `json:"date"`
	Changes int64     `json:"changes"`
}

func (s *DB) TopChurnFiles(from, to time.Time, limit int) ([]FileChurnStats, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT file_path,
		       COUNT(*) AS changes,
		       COUNT(DISTINCT session_id) AS sessions,
		       MIN(timestamp) AS first_seen,
		       MAX(timestamp) AS last_seen
		FROM file_changes
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY file_path
		ORDER BY changes DESC, sessions DESC, file_path ASC
		LIMIT ?
	`, formatQueryTime(from), formatQueryTime(to), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FileChurnStats
	for rows.Next() {
		var row FileChurnStats
		var first, last string
		if err := rows.Scan(&row.Path, &row.Changes, &row.Sessions, &first, &last); err != nil {
			return nil, err
		}
		row.FirstSeen, _ = parseStorageTime(first)
		row.LastSeen, _ = parseStorageTime(last)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		modes, err := s.fileModeCounts(out[i].Path, from, to)
		if err != nil {
			return nil, err
		}
		out[i].ModeCounts = modes
	}
	return out, nil
}

func (s *DB) fileModeCounts(filePath string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.db.Query(`
		SELECT change_type, COUNT(*)
		FROM file_changes
		WHERE file_path = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY change_type
	`, filePath, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var mode string
		var count int64
		if err := rows.Scan(&mode, &count); err != nil {
			return nil, err
		}
		out[mode] = count
	}
	return out, rows.Err()
}

func (s *DB) ChurnHotspots(from, to time.Time, depth int, limit int) ([]HotspotStats, error) {
	if depth <= 0 {
		depth = 2
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT file_path, COUNT(*) AS changes
		FROM file_changes
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY file_path
	`, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type hotspotAcc struct {
		changes    int64
		files      map[string]struct{}
		topFile    string
		topChanges int64
	}
	groups := map[string]*hotspotAcc{}
	for rows.Next() {
		var filePath string
		var changes int64
		if err := rows.Scan(&filePath, &changes); err != nil {
			return nil, err
		}
		key := hotspotPath(filePath, depth)
		g := groups[key]
		if g == nil {
			g = &hotspotAcc{files: map[string]struct{}{}}
			groups[key] = g
		}
		g.changes += changes
		g.files[filePath] = struct{}{}
		base := path.Base(filePath)
		if changes > g.topChanges || (changes == g.topChanges && (g.topFile == "" || base < g.topFile)) {
			g.topFile = base
			g.topChanges = changes
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]HotspotStats, 0, len(groups))
	for key, g := range groups {
		out = append(out, HotspotStats{
			Path:    key,
			Changes: g.changes,
			Files:   int64(len(g.files)),
			TopFile: g.topFile,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Changes == out[j].Changes {
			if out[i].Files == out[j].Files {
				return out[i].Path < out[j].Path
			}
			return out[i].Files > out[j].Files
		}
		return out[i].Changes > out[j].Changes
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *DB) DailyChurnTrend(from, to time.Time) ([]DailyChurn, error) {
	if !to.After(from) {
		return nil, nil
	}
	start := dayStartUTC(from)
	end := dayStartUTC(to)
	if to.After(end) {
		end = end.AddDate(0, 0, 1)
	}
	out := make([]DailyChurn, 0)
	byDay := map[string]int{}
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		out = append(out, DailyChurn{Date: day})
		byDay[key] = len(out) - 1
	}

	rows, err := s.db.Query(`
		SELECT strftime('%Y-%m-%d', timestamp) AS day, COUNT(*)
		FROM file_changes
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY day
	`, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var changes int64
		if err := rows.Scan(&day, &changes); err != nil {
			return nil, err
		}
		if idx, ok := byDay[day]; ok {
			out[idx].Changes = changes
		}
	}
	return out, rows.Err()
}

func hotspotPath(filePath string, depth int) string {
	absolute := strings.HasPrefix(filePath, "/")
	clean := strings.Trim(path.Clean(filePath), "/")
	if clean == "." || clean == "" {
		return clean
	}
	parts := strings.Split(clean, "/")
	if len(parts) == 1 {
		if absolute {
			return "/" + parts[0]
		}
		return parts[0]
	}
	dirCount := len(parts) - 1
	if depth < dirCount {
		dirCount = depth
	}
	prefix := strings.Join(parts[:dirCount], "/")
	if absolute {
		return "/" + prefix
	}
	return prefix
}
