package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type ToolErrorReport struct {
	TopTools []storage.ToolErrorStats `json:"top_tools"`
	Patterns []storage.ErrorPattern   `json:"patterns"`
	Daily    []storage.DailyRate      `json:"daily"`
}

func (d defaultRenderer) RenderToolErrors(w io.Writer, report ToolErrorReport, opts Options) error {
	if opts.JSON {
		return d.renderToolErrorsJSON(w, report)
	}
	fmt.Fprintln(w, "Top Failing Tools")
	renderTopFailingTools(w, report.TopTools)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Error Pattern Groups")
	renderErrorPatternGroups(w, report.Patterns)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Daily failure rate (last 14 days):")
	renderDailyFailureRate(w, report.Daily)
	return nil
}

func (d defaultRenderer) renderToolErrorsJSON(w io.Writer, report ToolErrorReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]ToolErrorReport{"tool_errors": report})
}

func renderTopFailingTools(w io.Writer, rows []storage.ToolErrorStats) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Rank", "Tool", "Total Calls", "Failures", "Failure Rate", "Top Error Pattern"})
	for i, row := range rows {
		t.AppendRow(table.Row{
			i + 1,
			row.Tool,
			row.Total,
			row.Failures,
			fmt.Sprintf("%.1f%%", row.FailureRate),
			truncateCell(row.TopPattern, 80),
		})
	}
	t.Render()
}

func renderErrorPatternGroups(w io.Writer, rows []storage.ErrorPattern) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Pattern", "Count", "Tools", "Sample Sessions"})
	for _, row := range rows {
		t.AppendRow(table.Row{
			truncateCell(row.NormalizedKey, 80),
			row.Count,
			strings.Join(row.Tools, ", "),
			strings.Join(row.Sessions, ", "),
		})
	}
	t.Render()
}

func renderDailyFailureRate(w io.Writer, rows []storage.DailyRate) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "  none")
		return
	}
	start := 0
	if len(rows) > 14 {
		start = len(rows) - 14
	}
	for _, row := range rows[start:] {
		fmt.Fprintf(w, "%s %s %.1f%%\n", row.Date, failureRateBar(row.FailureRate), row.FailureRate)
	}
}

func failureRateBar(rate float64) string {
	if rate < 0 {
		rate = 0
	}
	if rate > 100 {
		rate = 100
	}
	filled := int(rate / 10)
	if rate > 0 && filled == 0 {
		filled = 1
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

func truncateCell(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
