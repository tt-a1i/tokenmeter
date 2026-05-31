package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/projectalias"
	"github.com/tt-a1i/tokenmeter/internal/render"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// BlocksArgs is the resolved input to RunBlocks.
type BlocksArgs struct {
	Shared        Shared
	Platform      string
	SessionLength time.Duration
	Active        bool
	Recent        bool
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
	entries, err := listUsageForBlocks(ctx, loader, since, until, args.Shared.Project, args.Platform)
	if err != nil {
		return err
	}
	entries = applyPricingMode(entries, pricing.ParseMode(args.Shared.Mode))
	all := blocks.Annotate(blocks.Identify(entries, args.SessionLength, args.Now), args.Now)
	blockProjects := map[time.Time]string{}
	if args.Shared.Instances || args.Shared.ProjectAliases != "" {
		aliases, err := projectalias.Load(args.Shared.ProjectAliases)
		if err != nil {
			return err
		}
		blockProjects = projectByBlock(all, entries, aliases)
	}
	if args.Shared.Breakdown {
		all = blocks.PopulatePerModel(all, entries)
	}
	if args.Active {
		all = filterActive(all)
	} else if args.Recent {
		all = filterRecent(all, args.Now)
	}
	tokenLimit, err := resolveTokenLimit(args.Shared.TokenLimit, all)
	if err != nil {
		return err
	}
	limited := blocks.AnnotateWithTokenLimit(all, tokenLimit)
	rows := make([]render.BlockRow, 0, len(all))
	for _, annotated := range limited {
		b := annotated.Block
		row := render.BlockRow{
			ID:                blockID(b),
			Period:            b.StartTime.Format("2006-01-02 15:04"),
			StartTime:         b.StartTime,
			EndTime:           b.EndTime,
			ActualEndTime:     b.ActualEnd,
			IsActive:          b.IsActive,
			IsGap:             b.IsGap,
			EntryCount:        b.EntryCount,
			Project:           blockProjects[b.StartTime],
			Models:            b.Models,
			InputTokens:       b.Tokens.Input,
			OutputTokens:      b.Tokens.Output,
			CacheCreateTokens: b.Tokens.CacheCreate,
			CacheReadTokens:   b.Tokens.CacheRead,
			TotalTokens:       b.Tokens.Total(),
			Cost:              b.Cost,
			Status:            blockStatus(b),
			TokenLimit:        annotated.TokenLimit,
			UsagePct:          annotated.UsagePct,
			TokenLimitStatus:  annotated.TokenLimitStatus,
		}
		if b.BurnRate != nil {
			row.BurnRate = &render.BlockBurnRate{
				TokensPerMinute: b.BurnRate.TokensPerMinute,
				CostPerHour:     b.BurnRate.CostPerHour,
			}
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

func projectByBlock(all []blocks.SessionBlock, entries []storage.TokenUsageEntry, aliases projectalias.Aliases) map[time.Time]string {
	out := map[time.Time]string{}
	for _, b := range all {
		if b.IsGap {
			continue
		}
		for _, e := range entries {
			if e.Timestamp.Before(b.StartTime) || !e.Timestamp.Before(b.EndTime) {
				continue
			}
			out[b.StartTime] = resolveProjectName(aliases, e.CWD)
			break
		}
	}
	return out
}

func resolveTokenLimit(raw string, all []blocks.SessionBlock) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return 0, nil
	}
	if raw == "max" {
		return blocks.MaxTokenLimit(all), nil
	}
	limit, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || limit < 0 {
		return 0, fmt.Errorf("invalid --token-limit %q (want positive integer or max)", raw)
	}
	return limit, nil
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

func filterRecent(in []blocks.SessionBlock, now time.Time) []blocks.SessionBlock {
	cutoff := now.Add(-72 * time.Hour)
	var out []blocks.SessionBlock
	for _, b := range in {
		if b.IsGap {
			continue
		}
		if b.IsActive || !b.EndTime.Before(cutoff) {
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

func blockID(b blocks.SessionBlock) string {
	id := b.StartTime.UTC().Format("2006-01-02T15:04:05.000Z")
	if b.IsGap {
		return "gap-" + id
	}
	return id
}
