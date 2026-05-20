package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// RenderAggregate emits a rounded-box table of daily/weekly/monthly
// aggregates. JSON delegation lands in Task 9; until then opts.JSON
// surfaces an explicit error rather than silently writing nothing.
func (d defaultRenderer) RenderAggregate(w io.Writer, kind string, rows []AggregateRow, opts Options) error {
	if opts.JSON {
		return fmt.Errorf("render: JSON output for aggregate not implemented yet (Task 9)")
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
