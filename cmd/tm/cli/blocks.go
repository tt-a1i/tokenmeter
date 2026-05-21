package cli

import (
	"context"
	"io"
	"sort"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/render"
)

// BlocksArgs is the resolved input to RunBlocks.
type BlocksArgs struct {
	Shared        Shared
	SessionLength time.Duration
	Active        bool
	Now           time.Time
}

// BlocksLoader is the storage subset RunBlocks needs. Aliased to
// AggregateLoader so `tm blocks` can honor `--since` / `--until` /
// `--project` (P1 / P2-2 from the v1.0 review). Statusline still goes
// through blocks.Reader via NewActiveBlockAdapter, which doesn't need
// workspace filtering.
type BlocksLoader = AggregateLoader

// RunBlocks lists session blocks, optionally only the active one. Output is
// handed off to render.New().RenderBlocks; the boxed table (PERIOD / MODELS
// / INPUT / OUTPUT / CACHE CRT. / CACHE READ / TOTAL / COST / STATUS) and
// the camelCase JSON envelope both live in internal/render.
func RunBlocks(ctx context.Context, out io.Writer, args BlocksArgs, loader BlocksLoader) error {
	since, err := parseDateFlag(args.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(args.Shared.Until)
	if err != nil {
		return err
	}
	entries, err := loader.ListUsageForBlocksFiltered(ctx, since, until, args.Shared.Project)
	if err != nil {
		return err
	}
	entries = applyPricingMode(entries, pricing.ParseMode(args.Shared.Mode))
	all := blocks.Annotate(blocks.Identify(entries, args.SessionLength, args.Now), args.Now)
	if args.Shared.Breakdown {
		all = blocks.PopulatePerModel(all, entries)
	}
	if args.Active {
		all = filterActive(all)
	}
	rows := make([]render.BlockRow, 0, len(all))
	for _, b := range all {
		row := render.BlockRow{
			Period:            b.StartTime.Format("2006-01-02 15:04"),
			Models:            b.Models,
			InputTokens:       b.Tokens.Input,
			OutputTokens:      b.Tokens.Output,
			CacheCreateTokens: b.Tokens.CacheCreate,
			CacheReadTokens:   b.Tokens.CacheRead,
			TotalTokens:       b.Tokens.Total(),
			Cost:              b.Cost,
			Status:            blockStatus(b),
		}
		if b.Projection != nil {
			row.Projection = &render.BlockProjection{
				TotalTokens:   b.Projection.TotalTokens,
				TotalCost:     b.Projection.TotalCost,
				RemainingTime: b.Projection.RemainingTime,
			}
		}
		if args.Shared.Breakdown && len(b.PerModel) > 0 {
			models := make([]string, 0, len(b.PerModel))
			for m := range b.PerModel {
				models = append(models, m)
			}
			sort.Strings(models)
			total := b.Tokens.Total()
			for _, m := range models {
				tc := b.PerModel[m]
				// Per-model cost is approximated by token-share of the
				// block's authoritative total — the source rows are
				// already aggregated, so there is no per-entry cost here.
				// Renderer surfaces the share; the box footer keeps the
				// exact block total.
				share := 0.0
				if total > 0 {
					share = b.Cost * float64(tc.Total()) / float64(total)
				}
				row.Breakdown = append(row.Breakdown, render.ModelBreakdown{
					Model:             m,
					InputTokens:       tc.Input,
					OutputTokens:      tc.Output,
					CacheCreateTokens: tc.CacheCreate,
					CacheReadTokens:   tc.CacheRead,
					TotalTokens:       tc.Total(),
					Cost:              share,
				})
			}
		}
		rows = append(rows, row)
	}
	return render.New().RenderBlocks(out, rows, renderOpts(args.Shared, out))
}

// filterActive keeps only the active 5h window, honoring --active. Lives in
// the cli layer because render is presentation-only.
func filterActive(in []blocks.SessionBlock) []blocks.SessionBlock {
	var out []blocks.SessionBlock
	for _, b := range in {
		if b.IsActive {
			out = append(out, b)
		}
	}
	return out
}

func blockStatus(b blocks.SessionBlock) string {
	switch {
	case b.IsActive:
		return "ACTIVE"
	case b.IsGap:
		return "gap"
	default:
		return "closed"
	}
}
