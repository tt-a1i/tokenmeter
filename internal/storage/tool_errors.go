package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ToolErrorStats struct {
	Tool        string  `json:"tool"`
	Total       int64   `json:"total"`
	Failures    int64   `json:"failures"`
	FailureRate float64 `json:"failure_rate"`
	TopPattern  string  `json:"top_pattern"`
}

type ErrorPattern struct {
	NormalizedKey string   `json:"normalized_key"`
	SampleResult  string   `json:"sample_result"`
	Count         int64    `json:"count"`
	Tools         []string `json:"tools"`
	Sessions      []string `json:"sessions"`
}

type DailyRate struct {
	Date        string  `json:"date"`
	Total       int64   `json:"total"`
	Failures    int64   `json:"failures"`
	FailureRate float64 `json:"failure_rate"`
}

var (
	toolErrorDigitsRE = regexp.MustCompile(`[0-9]+`)
	toolErrorPathRE   = regexp.MustCompile(`/[a-z0-9._-]+(?:/[a-z0-9._-]+)+`)
	toolErrorSpaceRE  = regexp.MustCompile(`\s+`)
)

func NormalizeToolErrorPattern(s string) string {
	s = strings.ToLower(s)
	s = toolErrorDigitsRE.ReplaceAllString(s, "N")
	s = toolErrorPathRE.ReplaceAllStringFunc(s, normalizeToolErrorPath)
	s = toolErrorSpaceRE.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = strings.TrimSpace(s[:80])
	}
	return s
}

func normalizeToolErrorPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) <= 2 {
		return path
	}
	return "/" + parts[0] + "/.../" + parts[len(parts)-1]
}

func (s *DB) TopFailingTools(from, to time.Time, limit int) ([]ToolErrorStats, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT tool_name,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status = 'fail' THEN 1 ELSE 0 END) AS failures
		FROM tool_calls
		WHERE start_time >= ? AND start_time < ?
		GROUP BY tool_name
		HAVING failures > 0
		ORDER BY failures DESC, total DESC, tool_name ASC
		LIMIT ?
	`, formatQueryTime(from), formatQueryTime(to), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ToolErrorStats
	for rows.Next() {
		var stat ToolErrorStats
		if err := rows.Scan(&stat.Tool, &stat.Total, &stat.Failures); err != nil {
			return nil, err
		}
		if stat.Total > 0 {
			stat.FailureRate = float64(stat.Failures) / float64(stat.Total) * 100
		}
		out = append(out, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		top, err := s.topPatternForTool(out[i].Tool, from, to)
		if err != nil {
			return nil, err
		}
		out[i].TopPattern = top
	}
	return out, nil
}

func (s *DB) topPatternForTool(tool string, from, to time.Time) (string, error) {
	rows, err := s.db.Query(`
		SELECT result_summary
		FROM tool_calls
		WHERE tool_name = ? AND status = 'fail' AND start_time >= ? AND start_time < ?
	`, tool, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var result sql.NullString
		if err := rows.Scan(&result); err != nil {
			return "", err
		}
		key := NormalizeToolErrorPattern(result.String)
		if key != "" {
			counts[key]++
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return mostCommonPattern(counts), nil
}

func (s *DB) ErrorPatternGroups(from, to time.Time, minCount int) ([]ErrorPattern, error) {
	if minCount <= 0 {
		minCount = 3
	}
	rows, err := s.db.Query(`
		SELECT tool_name, session_id, result_summary
		FROM tool_calls
		WHERE status = 'fail' AND start_time >= ? AND start_time < ?
	`, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type acc struct {
		key      string
		sample   string
		count    int64
		tools    map[string]struct{}
		sessions []string
		seenSess map[string]struct{}
	}
	groups := map[string]*acc{}
	for rows.Next() {
		var tool, session string
		var result sql.NullString
		if err := rows.Scan(&tool, &session, &result); err != nil {
			return nil, err
		}
		key := NormalizeToolErrorPattern(result.String)
		if key == "" {
			continue
		}
		hash := toolErrorPatternHash(key)
		g := groups[hash]
		if g == nil {
			g = &acc{key: key, sample: result.String, tools: map[string]struct{}{}, seenSess: map[string]struct{}{}}
			groups[hash] = g
		}
		g.count++
		g.tools[tool] = struct{}{}
		if _, ok := g.seenSess[session]; !ok && len(g.sessions) < 5 {
			g.seenSess[session] = struct{}{}
			g.sessions = append(g.sessions, session)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ErrorPattern, 0, len(groups))
	for _, g := range groups {
		if g.count < int64(minCount) {
			continue
		}
		tools := make([]string, 0, len(g.tools))
		for tool := range g.tools {
			tools = append(tools, tool)
		}
		sort.Strings(tools)
		out = append(out, ErrorPattern{
			NormalizedKey: g.key,
			SampleResult:  g.sample,
			Count:         g.count,
			Tools:         tools,
			Sessions:      append([]string(nil), g.sessions...),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].NormalizedKey < out[j].NormalizedKey
		}
		return out[i].Count > out[j].Count
	})
	return out, nil
}

func (s *DB) DailyFailureRate(from, to time.Time) ([]DailyRate, error) {
	if !to.After(from) {
		return nil, nil
	}
	start := dayStartUTC(from)
	end := dayStartUTC(to)
	if to.After(end) {
		end = end.AddDate(0, 0, 1)
	}
	rates := make([]DailyRate, 0)
	byDay := map[string]int{}
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		rates = append(rates, DailyRate{Date: key})
		byDay[key] = len(rates) - 1
	}

	rows, err := s.db.Query(`
		SELECT strftime('%Y-%m-%d', start_time) AS day,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status = 'fail' THEN 1 ELSE 0 END) AS failures
		FROM tool_calls
		WHERE start_time >= ? AND start_time < ?
		GROUP BY day
	`, formatQueryTime(from), formatQueryTime(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var total, failures int64
		if err := rows.Scan(&day, &total, &failures); err != nil {
			return nil, err
		}
		if idx, ok := byDay[day]; ok {
			rates[idx].Total = total
			rates[idx].Failures = failures
			if total > 0 {
				rates[idx].FailureRate = float64(failures) / float64(total) * 100
			}
		}
	}
	return rates, rows.Err()
}

func dayStartUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func mostCommonPattern(counts map[string]int64) string {
	var best string
	var bestCount int64
	for key, count := range counts {
		if count > bestCount || (count == bestCount && (best == "" || key < best)) {
			best = key
			bestCount = count
		}
	}
	return best
}

func toolErrorPatternHash(key string) string {
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}
