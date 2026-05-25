package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/render"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type SessionArgs struct {
	Shared    Shared
	SessionID string // optional; "" lists all
}

// renderOpts builds render.Options from Shared, including the color
// detection that depends on the actual output writer. Shared by RunSession,
// RunBlocks, and RunAggregate.
func renderOpts(s Shared, w io.Writer) render.Options {
	return render.Options{
		JSON:      s.JSON,
		Breakdown: s.Breakdown,
		Color:     render.Resolve(s.JSON, s.NoColor, w),
		Compact:   s.Compact,
	}
}

func RunSession(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
	ul, ok := loader.(AggregateUsageLoader)
	if !ok {
		return fmt.Errorf("session loader %T does not implement AggregateUsage; rebuild against storage v1.0.2", loader)
	}

	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}

	// Note: timezone offset uses the offset at query time, which may
	// mis-bucket historical data that crossed a DST boundary. Known
	// limitation; acceptable for the typical ccusage workflow.
	loc := time.UTC
	if a.Shared.Timezone != "" {
		parsed, err := time.LoadLocation(a.Shared.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", a.Shared.Timezone, err)
		}
		loc = parsed
	}

	mode := pricing.ParseMode(a.Shared.Mode)
	// Same fix as RunAggregate: ModeAuto without --breakdown silently drops
	// per-model zero-cost shares when the bucket SUM > 0. Force storage to
	// return per-(sessionId, model) rows so each can independently trigger
	// the fallback, then fold them back into one row per session for
	// rendering. See convertSessionRows foldForAuto branch.
	needAutoFallback := mode == pricing.ModeAuto && !a.Shared.Breakdown

	filter := storage.AggregateFilter{
		Since:     since,
		Until:     until,
		Project:   a.Shared.Project,
		Bucket:    storage.BucketSession,
		Breakdown: a.Shared.Breakdown || needAutoFallback,
		Location:  loc,
	}
	aggRows, err := ul.AggregateUsage(ctx, filter)
	if err != nil {
		return err
	}

	rows := convertSessionRows(aggRows, a.Shared.Breakdown, mode, a.SessionID, needAutoFallback)

	if a.Shared.Order == "desc" {
		sort.Slice(rows, func(i, j int) bool { return rows[i].SessionID > rows[j].SessionID })
	} else {
		sort.Slice(rows, func(i, j int) bool { return rows[i].SessionID < rows[j].SessionID })
	}

	return render.New().RenderSessions(w, rows, renderOpts(a.Shared, w))
}

// convertSessionRows folds storage rows (one row per (sessionId, model) when
// Breakdown=true, one row per sessionId otherwise) into render rows with
// Breakdown[] populated as needed. Cost mode follows the same rules as
// RunAggregate: Calculate always recomputes; Auto recomputes only when the
// source bucket cost is 0 (Codex zero-cost fallback parity); Display leaves
// cost as SUM-ed by storage. filterID restricts the output to one session
// when SessionArgs.SessionID is set.
//
// foldForAuto mirrors RunAggregate's strategy for ModeAuto without
// --breakdown: per-(sessionId, model) rows come back from storage so each
// independently triggers the zero-cost fallback, then collapse to one
// SessionRow per session (no user-visible Breakdown[]).
func convertSessionRows(in []storage.AggregateUsageRow, breakdown bool, mode pricing.Mode, filterID string, foldForAuto bool) []render.SessionRow {
	if foldForAuto {
		bySession := map[string]*render.SessionRow{}
		order := []string{}
		for _, r := range in {
			if filterID != "" && r.Bucket != filterID {
				continue
			}
			row, ok := bySession[r.Bucket]
			if !ok {
				row = &render.SessionRow{
					SessionID:    r.Bucket,
					ProjectPath:  r.Project,
					LastActivity: r.LastActivity,
				}
				bySession[r.Bucket] = row
				order = append(order, r.Bucket)
			}
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			row.Models = appendUnique(row.Models, r.Model)
			row.InputTokens += r.InputTokens
			row.OutputTokens += r.OutputTokens
			row.CacheCreateTokens += r.CacheCreateTokens
			row.CacheReadTokens += r.CacheReadTokens
			row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
			row.Cost += cost
			if r.LastActivity.After(row.LastActivity) {
				row.LastActivity = r.LastActivity
			}
			// Deliberately NOT populating row.Breakdown.
		}
		out := make([]render.SessionRow, 0, len(order))
		for _, k := range order {
			out = append(out, *bySession[k])
		}
		return out
	}
	if !breakdown {
		out := make([]render.SessionRow, 0, len(in))
		for _, r := range in {
			if filterID != "" && r.Bucket != filterID {
				continue
			}
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			out = append(out, render.SessionRow{
				SessionID:         r.Bucket,
				ProjectPath:       r.Project,
				LastActivity:      r.LastActivity,
				Models:            r.Models,
				InputTokens:       r.InputTokens,
				OutputTokens:      r.OutputTokens,
				CacheCreateTokens: r.CacheCreateTokens,
				CacheReadTokens:   r.CacheReadTokens,
				TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
				Cost:              cost,
			})
		}
		return out
	}
	// Breakdown=true: storage emits one row per (sessionId, model). Fold by
	// sessionId so the render layer sees one SessionRow per session with the
	// per-model rows in Breakdown[].
	bySession := map[string]*render.SessionRow{}
	order := []string{}
	for _, r := range in {
		if filterID != "" && r.Bucket != filterID {
			continue
		}
		row, ok := bySession[r.Bucket]
		if !ok {
			row = &render.SessionRow{
				SessionID:    r.Bucket,
				ProjectPath:  r.Project,
				LastActivity: r.LastActivity,
			}
			bySession[r.Bucket] = row
			order = append(order, r.Bucket)
		}
		cost := r.Cost
		if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
			cost = recalculateCost(r)
		}
		row.Models = appendUnique(row.Models, r.Model)
		row.InputTokens += r.InputTokens
		row.OutputTokens += r.OutputTokens
		row.CacheCreateTokens += r.CacheCreateTokens
		row.CacheReadTokens += r.CacheReadTokens
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
		row.Cost += cost
		if r.LastActivity.After(row.LastActivity) {
			row.LastActivity = r.LastActivity
		}
		row.Breakdown = append(row.Breakdown, render.ModelBreakdown{
			Model:             r.Model,
			InputTokens:       r.InputTokens,
			OutputTokens:      r.OutputTokens,
			CacheCreateTokens: r.CacheCreateTokens,
			CacheReadTokens:   r.CacheReadTokens,
			TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
			Cost:              cost,
		})
	}
	out := make([]render.SessionRow, 0, len(order))
	for _, k := range order {
		out = append(out, *bySession[k])
	}
	return out
}
