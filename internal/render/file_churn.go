package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type FileChurnReport struct {
	ProjectScope string                   `json:"project_scope,omitempty"`
	TopFiles     []storage.FileChurnStats `json:"top_files"`
	Hotspots     []storage.HotspotStats   `json:"hotspots"`
	Daily        []storage.DailyChurn     `json:"daily"`
}

func (d defaultRenderer) RenderFileChurn(w io.Writer, report FileChurnReport, opts Options) error {
	if opts.JSON {
		return d.renderFileChurnJSON(w, report)
	}
	fmt.Fprintln(w, "Top Changed Files")
	renderTopChurnFiles(w, report.TopFiles, opts)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "File Churn Hotspots")
	renderChurnHotspots(w, report.Hotspots)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Daily file changes (last 14 days):")
	renderDailyChurnTrend(w, report.Daily)
	return nil
}

func (d defaultRenderer) renderFileChurnJSON(w io.Writer, report FileChurnReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]FileChurnReport{"file_churn": report})
}

func renderTopChurnFiles(w io.Writer, rows []storage.FileChurnStats, opts Options) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	if opts.Compact {
		t.AppendHeader(table.Row{"Rank", "File", "Changes", "First Seen", "Last Seen"})
		for i, row := range rows {
			t.AppendRow(table.Row{
				i + 1,
				truncateCell(row.Path, 80),
				row.Changes,
				row.FirstSeen.Format("2006-01-02"),
				row.LastSeen.Format("2006-01-02"),
			})
		}
		t.Render()
		return
	}
	t.AppendHeader(table.Row{"Rank", "File", "Changes", "Sessions", "First Seen", "Last Seen", "Mode"})
	for i, row := range rows {
		t.AppendRow(table.Row{
			i + 1,
			truncateCell(row.Path, 80),
			row.Changes,
			row.Sessions,
			row.FirstSeen.Format("2006-01-02"),
			row.LastSeen.Format("2006-01-02"),
			formatModeCounts(row.ModeCounts),
		})
	}
	t.Render()
}

func renderChurnHotspots(w io.Writer, rows []storage.HotspotStats) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Hotspot Path", "Changes", "Files", "Top File"})
	for _, row := range rows {
		t.AppendRow(table.Row{
			truncateCell(row.Path, 80),
			row.Changes,
			row.Files,
			row.TopFile,
		})
	}
	t.Render()
}

func renderDailyChurnTrend(w io.Writer, rows []storage.DailyChurn) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	start := 0
	if len(rows) > 14 {
		start = len(rows) - 14
	}
	maxChanges := int64(0)
	for _, row := range rows[start:] {
		if row.Changes > maxChanges {
			maxChanges = row.Changes
		}
	}
	for _, row := range rows[start:] {
		fmt.Fprintf(w, "%s %s %d\n", row.Date.Format("2006-01-02"), churnBar(row.Changes, maxChanges), row.Changes)
	}
}

func churnBar(value, max int64) string {
	if max <= 0 || value <= 0 {
		return strings.Repeat("░", 10)
	}
	filled := int(value * 10 / max)
	if filled == 0 {
		filled = 1
	}
	if filled > 10 {
		filled = 10
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func formatModeCounts(counts map[string]int64) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] == counts[keys[j]] {
			return keys[i] < keys[j]
		}
		return counts[keys[i]] > counts[keys[j]]
	})
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return strings.Join(parts, " ")
}
