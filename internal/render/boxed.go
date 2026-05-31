package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// RenderAggregate emits a rounded-box table of daily/weekly/monthly
// aggregates, or delegates to the camelCase JSON renderer when opts.JSON
// is set.
func (d defaultRenderer) RenderAggregate(w io.Writer, kind string, rows []AggregateRow, opts Options) error {
	if opts.JSON {
		return d.renderAggregateJSON(w, kind, rows, opts)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	compact := compactLayout(w, opts)

	firstCol := "Date"
	switch kind {
	case "weekly":
		firstCol = "Week"
	case "monthly":
		firstCol = "Month"
	}
	withProjects := opts.Instances && aggregateHasProjects(rows)
	if compact && withProjects {
		t.AppendHeader(table.Row{firstCol, "Project", "Input", "Output", "Cache", "Total", "Cost"})
	} else if compact {
		t.AppendHeader(table.Row{firstCol, "Input", "Output", "Cache", "Total", "Cost"})
	} else if withProjects {
		t.AppendHeader(table.Row{firstCol, "Project", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost"})
	} else {
		t.AppendHeader(table.Row{firstCol, "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost"})
	}

	colorize := func(c text.Color, s string) string {
		if !opts.Color {
			return s
		}
		return c.Sprint(s)
	}

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
		if compact && withProjects {
			t.AppendRow(table.Row{
				r.Bucket,
				r.Project,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		} else if compact {
			t.AppendRow(table.Row{
				r.Bucket,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		} else if withProjects {
			t.AppendRow(table.Row{
				r.Bucket,
				r.Project,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		} else {
			t.AppendRow(table.Row{
				r.Bucket,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		}
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				if compact && withProjects {
					t.AppendRow(table.Row{
						"└─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				} else if compact {
					t.AppendRow(table.Row{
						"└─ " + b.Model,
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				} else if withProjects {
					t.AppendRow(table.Row{
						"└─ " + b.Model, "", "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				} else {
					t.AppendRow(table.Row{
						"└─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				}
			}
		}
		sumIn += r.InputTokens
		sumOut += r.OutputTokens
		sumCC += r.CacheCreateTokens
		sumCR += r.CacheReadTokens
		sumTotal += r.TotalTokens
		sumCost += r.Cost
	}
	t.AppendSeparator()
	if compact && withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	} else if compact {
		t.AppendFooter(table.Row{
			"TOTAL",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	} else if withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	} else {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	}
	t.Render()
	return nil
}

func aggregateHasProjects(rows []AggregateRow) bool {
	for _, r := range rows {
		if r.Project != "" {
			return true
		}
	}
	return false
}

// RenderSessions emits a rounded-box table keyed by session id with project
// path + last-activity columns, or delegates to the JSON renderer when
// opts.JSON is set. Column layout:
//
//	SESSION | PROJECT | MODELS | INPUT | OUTPUT | CACHE CRT. | CACHE READ | TOTAL | COST
//
// TOTAL footer leaves PROJECT blank — it's a per-row identifier, not summable.
func (d defaultRenderer) RenderSessions(w io.Writer, rows []SessionRow, opts Options) error {
	if opts.JSON {
		return d.renderSessionsJSON(w, rows, opts)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	compact := compactLayout(w, opts)
	if compact {
		t.AppendHeader(table.Row{"Session", "Project", "Input", "Output", "Cache", "Total", "Cost"})
	} else {
		t.AppendHeader(table.Row{"Session", "Project", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost"})
	}

	colorize := func(c text.Color, s string) string {
		if !opts.Color {
			return s
		}
		return c.Sprint(s)
	}

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
		if compact {
			t.AppendRow(table.Row{
				r.SessionID,
				r.ProjectPath,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		} else {
			t.AppendRow(table.Row{
				r.SessionID,
				r.ProjectPath,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
			})
		}
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				if compact {
					t.AppendRow(table.Row{
						"└─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				} else {
					t.AppendRow(table.Row{
						"└─ " + b.Model, "", "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost),
					})
				}
			}
		}
		sumIn += r.InputTokens
		sumOut += r.OutputTokens
		sumCC += r.CacheCreateTokens
		sumCR += r.CacheReadTokens
		sumTotal += r.TotalTokens
		sumCost += r.Cost
	}
	t.AppendSeparator()
	if compact {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	} else {
		t.AppendFooter(table.Row{
			"TOTAL", "", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
		})
	}
	t.Render()
	return nil
}

// RenderBlocks emits a rounded-box table of 5-hour session blocks, or
// delegates to the camelCase JSON renderer when opts.JSON is set. Column
// layout:
//
//	PERIOD | MODELS | INPUT | OUTPUT | CACHE CRT. | CACHE READ | TOTAL | COST | STATUS
//
// TOTAL footer leaves STATUS blank (status is per-row, not summable).
func (d defaultRenderer) RenderBlocks(w io.Writer, rows []BlockRow, opts Options) error {
	if opts.JSON {
		return d.renderBlocksJSON(w, rows, opts)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	compact := compactLayout(w, opts)
	withLimit := blocksHaveTokenLimit(rows)
	withProjects := opts.Instances && blockHasProjects(rows)
	progressWidth := 12
	if compact {
		progressWidth = 8
	}
	if compact && withLimit && withProjects {
		t.AppendHeader(table.Row{"Period", "Project", "Input", "Output", "Cache", "Total", "Cost", "Usage%", "Progress", "Status"})
	} else if compact && withLimit {
		t.AppendHeader(table.Row{"Period", "Input", "Output", "Cache", "Total", "Cost", "Usage%", "Progress", "Status"})
	} else if compact && withProjects {
		t.AppendHeader(table.Row{"Period", "Project", "Input", "Output", "Cache", "Total", "Cost", "Status"})
	} else if compact {
		t.AppendHeader(table.Row{"Period", "Input", "Output", "Cache", "Total", "Cost", "Status"})
	} else if withLimit && withProjects {
		t.AppendHeader(table.Row{"Period", "Project", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost", "Usage%", "Progress", "Status"})
	} else if withLimit {
		t.AppendHeader(table.Row{"Period", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost", "Usage%", "Progress", "Status"})
	} else if withProjects {
		t.AppendHeader(table.Row{"Period", "Project", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost", "Status"})
	} else {
		t.AppendHeader(table.Row{"Period", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost", "Status"})
	}

	colorize := func(c text.Color, s string) string {
		if !opts.Color {
			return s
		}
		return c.Sprint(s)
	}

	colorizeStatus := func(s string) string {
		if !opts.Color {
			return s
		}
		switch s {
		case "ACTIVE":
			return text.FgGreen.Sprint(s)
		case "gap":
			return text.Faint.Sprint(s)
		default:
			return text.FgHiBlack.Sprint(s)
		}
	}

	colorizeLimitStatus := func(s string) string {
		if !opts.Color {
			return s
		}
		switch s {
		case "OK":
			return text.FgGreen.Sprint(s)
		case "WARN":
			return text.FgYellow.Sprint(s)
		case "ALERT":
			return text.FgRed.Sprint(s)
		default:
			return s
		}
	}

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
		status := colorizeStatus(r.Status)
		if withLimit && r.TokenLimit > 0 {
			status = colorizeLimitStatus(r.TokenLimitStatus)
		}
		if compact && withLimit && withProjects {
			t.AppendRow(table.Row{
				r.Period,
				r.Project,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				formatUsagePct(r),
				progressBar(r.UsagePct, progressWidth),
				status,
			})
		} else if compact && withLimit {
			t.AppendRow(table.Row{
				r.Period,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				formatUsagePct(r),
				progressBar(r.UsagePct, progressWidth),
				status,
			})
		} else if compact && withProjects {
			t.AppendRow(table.Row{
				r.Period,
				r.Project,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				status,
			})
		} else if compact {
			t.AppendRow(table.Row{
				r.Period,
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens+r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				status,
			})
		} else if withLimit && withProjects {
			t.AppendRow(table.Row{
				r.Period,
				r.Project,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				formatUsagePct(r),
				progressBar(r.UsagePct, progressWidth),
				status,
			})
		} else if withLimit {
			t.AppendRow(table.Row{
				r.Period,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				formatUsagePct(r),
				progressBar(r.UsagePct, progressWidth),
				status,
			})
		} else if withProjects {
			t.AppendRow(table.Row{
				r.Period,
				r.Project,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				status,
			})
		} else {
			t.AppendRow(table.Row{
				r.Period,
				colorize(text.FgCyan, joinList(r.Models)),
				colorize(text.FgYellow, fmtInt(r.InputTokens)),
				colorize(text.FgYellow, fmtInt(r.OutputTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
				colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
				colorize(text.FgYellow, fmtInt(r.TotalTokens)),
				colorize(text.FgRed, fmtCost(r.Cost)),
				status,
			})
		}
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				if compact && withLimit && withProjects {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "", "", "",
					})
				} else if compact && withLimit {
					t.AppendRow(table.Row{
						"  └─ " + b.Model,
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "", "", "",
					})
				} else if compact && withProjects {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "",
					})
				} else if compact {
					t.AppendRow(table.Row{
						"  └─ " + b.Model,
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens + b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "",
					})
				} else if withLimit && withProjects {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "", "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "", "", "",
					})
				} else if withLimit {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "", "", "",
					})
				} else if withProjects {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "", "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "",
					})
				} else {
					t.AppendRow(table.Row{
						"  └─ " + b.Model, "",
						fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
						fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
						fmtInt(b.TotalTokens), fmtCost(b.Cost), "",
					})
				}
			}
		}
		sumIn += r.InputTokens
		sumOut += r.OutputTokens
		sumCC += r.CacheCreateTokens
		sumCR += r.CacheReadTokens
		sumTotal += r.TotalTokens
		sumCost += r.Cost
	}
	t.AppendSeparator()
	if compact && withLimit && withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"", "", "",
		})
	} else if compact && withLimit {
		t.AppendFooter(table.Row{
			"TOTAL",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"", "", "",
		})
	} else if compact && withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"",
		})
	} else if compact {
		t.AppendFooter(table.Row{
			"TOTAL",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC + sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"",
		})
	} else if withLimit && withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"", "", "",
		})
	} else if withLimit {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"", "", "",
		})
	} else if withProjects {
		t.AppendFooter(table.Row{
			"TOTAL", "", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"",
		})
	} else {
		t.AppendFooter(table.Row{
			"TOTAL", "",
			fmtInt(sumIn), fmtInt(sumOut),
			fmtInt(sumCC), fmtInt(sumCR),
			fmtInt(sumTotal), fmtCost(sumCost),
			"",
		})
	}
	t.Render()
	return nil
}

func blocksHaveTokenLimit(rows []BlockRow) bool {
	for _, r := range rows {
		if r.TokenLimit > 0 {
			return true
		}
	}
	return false
}

func blockHasProjects(rows []BlockRow) bool {
	for _, r := range rows {
		if r.Project != "" {
			return true
		}
	}
	return false
}

func formatUsagePct(r BlockRow) string {
	if r.TokenLimit <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", r.UsagePct)
}

func progressBar(usage float64, width int) string {
	if usage < 0 {
		usage = 0
	}
	filled := int(usage*float64(width)/100 + 0.5)
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "] " + fmt.Sprintf("%.0f%%", usage)
}

func fmtInt(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func fmtCost(c float64) string { return fmt.Sprintf("$%.2f", c) }

func joinList(s []string) string {
	if len(s) == 0 {
		return "-"
	}
	return strings.Join(s, ", ")
}
