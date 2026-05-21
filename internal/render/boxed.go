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
		return d.renderAggregateJSON(w, kind, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)

	firstCol := "Date"
	switch kind {
	case "weekly":
		firstCol = "Week"
	case "monthly":
		firstCol = "Month"
	}
	t.AppendHeader(table.Row{firstCol, "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost"})

	colorize := func(c text.Color, s string) string {
		if !opts.Color {
			return s
		}
		return c.Sprint(s)
	}

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
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
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				t.AppendRow(table.Row{
					"└─ " + b.Model, "",
					fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
					fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
					fmtInt(b.TotalTokens), fmtCost(b.Cost),
				})
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
	t.AppendFooter(table.Row{
		"TOTAL", "",
		fmtInt(sumIn), fmtInt(sumOut),
		fmtInt(sumCC), fmtInt(sumCR),
		fmtInt(sumTotal), fmtCost(sumCost),
	})
	t.Render()
	return nil
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
		return d.renderSessionsJSON(w, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Session", "Project", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost"})

	colorize := func(c text.Color, s string) string {
		if !opts.Color {
			return s
		}
		return c.Sprint(s)
	}

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
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
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				t.AppendRow(table.Row{
					"└─ " + b.Model, "", "",
					fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
					fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
					fmtInt(b.TotalTokens), fmtCost(b.Cost),
				})
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
	t.AppendFooter(table.Row{
		"TOTAL", "", "",
		fmtInt(sumIn), fmtInt(sumOut),
		fmtInt(sumCC), fmtInt(sumCR),
		fmtInt(sumTotal), fmtCost(sumCost),
	})
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
		return d.renderBlocksJSON(w, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no data in range)")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Period", "Models", "Input", "Output", "Cache Crt.", "Cache Read", "Total", "Cost", "Status"})

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

	var sumIn, sumOut, sumCC, sumCR, sumTotal int64
	var sumCost float64
	for _, r := range rows {
		t.AppendRow(table.Row{
			r.Period,
			colorize(text.FgCyan, joinList(r.Models)),
			colorize(text.FgYellow, fmtInt(r.InputTokens)),
			colorize(text.FgYellow, fmtInt(r.OutputTokens)),
			colorize(text.FgYellow, fmtInt(r.CacheCreateTokens)),
			colorize(text.FgYellow, fmtInt(r.CacheReadTokens)),
			colorize(text.FgYellow, fmtInt(r.TotalTokens)),
			colorize(text.FgRed, fmtCost(r.Cost)),
			colorizeStatus(r.Status),
		})
		if opts.Breakdown {
			for _, b := range r.Breakdown {
				t.AppendRow(table.Row{
					"  └─ " + b.Model, "",
					fmtInt(b.InputTokens), fmtInt(b.OutputTokens),
					fmtInt(b.CacheCreateTokens), fmtInt(b.CacheReadTokens),
					fmtInt(b.TotalTokens), fmtCost(b.Cost), "",
				})
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
	t.AppendFooter(table.Row{
		"TOTAL", "",
		fmtInt(sumIn), fmtInt(sumOut),
		fmtInt(sumCC), fmtInt(sumCR),
		fmtInt(sumTotal), fmtCost(sumCost),
		"",
	})
	t.Render()
	return nil
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
