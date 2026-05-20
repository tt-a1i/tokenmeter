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

type Bucket int

const (
	BucketDaily Bucket = iota
	BucketWeekly
	BucketMonthly
)

type AggregateArgs struct {
	Shared Shared
	Bucket Bucket
}

type AggregateLoader interface {
	ListUsageForBlocksFiltered(ctx context.Context, since, until time.Time, workspace string) ([]storage.TokenUsageEntry, error)
}

// pricingMap is loaded once per process; LoadEmbedded panics on malformed
// snapshot, which the test suite would surface immediately.
var pricingMap = pricing.LoadEmbedded()

func RunAggregate(ctx context.Context, w io.Writer, a AggregateArgs, loader AggregateLoader) error {
	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}
	entries, err := loader.ListUsageForBlocksFiltered(ctx, since, until, a.Shared.Project)
	if err != nil {
		return err
	}
	entries = applyPricingMode(entries, pricing.ParseMode(a.Shared.Mode))

	loc := time.UTC
	if a.Shared.Timezone != "" {
		parsed, err := time.LoadLocation(a.Shared.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", a.Shared.Timezone, err)
		}
		loc = parsed
	}

	groups := groupBy(entries, a.Bucket, loc)
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	if a.Shared.Order == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}

	rows := make([]render.AggregateRow, 0, len(keys))
	for _, k := range keys {
		g := groups[k]
		row := render.AggregateRow{
			Bucket:            k,
			Models:            g.models,
			InputTokens:       g.input,
			OutputTokens:      g.output,
			CacheCreateTokens: g.cacheCreate,
			CacheReadTokens:   g.cacheRead,
			TotalTokens:       g.input + g.output + g.cacheCreate + g.cacheRead,
			Cost:              g.cost,
		}
		if a.Shared.Breakdown {
			for model, st := range g.perModel {
				row.Breakdown = append(row.Breakdown, render.ModelBreakdown{
					Model:             model,
					InputTokens:       st.Input,
					OutputTokens:      st.Output,
					CacheCreateTokens: st.CacheCreate,
					CacheReadTokens:   st.CacheRead,
					TotalTokens:       st.Input + st.Output + st.CacheCreate + st.CacheRead,
					Cost:              st.Cost,
				})
			}
			sort.Slice(row.Breakdown, func(i, j int) bool {
				return row.Breakdown[i].Cost > row.Breakdown[j].Cost
			})
		}
		rows = append(rows, row)
	}

	return render.New().RenderAggregate(w, bucketKind(a.Bucket), rows, renderOpts(a.Shared, w))
}

// bucketKind maps the cli Bucket enum to the render package's string kind.
func bucketKind(b Bucket) string {
	switch b {
	case BucketWeekly:
		return "weekly"
	case BucketMonthly:
		return "monthly"
	default:
		return "daily"
	}
}

// modelStats tracks per-model token + cost subtotals inside one aggGroup so
// the render layer can emit Breakdown rows when --breakdown is set.
type modelStats struct {
	Input, Output, CacheCreate, CacheRead int64
	Cost                                  float64
}

type aggGroup struct {
	tokens                                int64
	cost                                  float64
	models                                []string
	seen                                  map[string]struct{}
	input, output, cacheCreate, cacheRead int64
	perModel                              map[string]*modelStats
}

func groupBy(entries []storage.TokenUsageEntry, b Bucket, loc *time.Location) map[string]*aggGroup {
	out := map[string]*aggGroup{}
	for _, e := range entries {
		key := bucketKey(e.Timestamp.In(loc), b)
		g, ok := out[key]
		if !ok {
			g = &aggGroup{
				seen:     map[string]struct{}{},
				perModel: map[string]*modelStats{},
			}
			out[key] = g
		}
		g.tokens += e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
		g.cost += e.CostUSD
		g.input += e.InputTokens
		g.output += e.OutputTokens
		g.cacheCreate += e.CacheCreationInputTokens
		g.cacheRead += e.CacheReadInputTokens
		if e.Model != "" {
			if _, exists := g.seen[e.Model]; !exists {
				g.models = append(g.models, e.Model)
				g.seen[e.Model] = struct{}{}
			}
			ms, ok := g.perModel[e.Model]
			if !ok {
				ms = &modelStats{}
				g.perModel[e.Model] = ms
			}
			ms.Input += e.InputTokens
			ms.Output += e.OutputTokens
			ms.CacheCreate += e.CacheCreationInputTokens
			ms.CacheRead += e.CacheReadInputTokens
			ms.Cost += e.CostUSD
		}
	}
	return out
}

func bucketKey(ts time.Time, b Bucket) string {
	switch b {
	case BucketWeekly:
		_, w := ts.ISOWeek()
		return fmt.Sprintf("%d-W%02d", ts.Year(), w)
	case BucketMonthly:
		return ts.Format("2006-01")
	default:
		return ts.Format("2006-01-02")
	}
}

// parseDateFlag accepts a YYYYMMDD shared-flag value and returns a UTC
// time.Time. An empty string maps to the zero time, meaning "no bound".
func parseDateFlag(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (want YYYYMMDD): %w", s, err)
	}
	return t, nil
}

// ParseDateFlagUntil mirrors parseDateFlag but bumps the result forward by
// 24h so a `--until YYYYMMDD` flag becomes an inclusive closed interval to
// the end of that day, matching ccusage's filter behavior. Empty input
// passes through as the zero time (meaning "no bound"). Exported so the
// shared-flag tests can pin the +24h contract directly.
func ParseDateFlagUntil(s string) (time.Time, error) {
	t, err := parseDateFlag(s)
	if err != nil {
		return time.Time{}, err
	}
	if t.IsZero() {
		return t, nil
	}
	return t.Add(24 * time.Hour), nil
}

// applyPricingMode rewrites each entry's CostUSD according to mode:
//   - ModeDisplay: leaves CostUSD untouched (trust the source row).
//   - ModeCalculate: recomputes from tokens × pricing for the resolved model.
//   - ModeAuto: when CostUSD > 0 keeps it; when it's zero, falls back to
//     calculate so missing cost columns (e.g. Codex) still surface a value.
//
// internal/pricing has no storage import on purpose; the per-entry bridge
// lives here so we don't pull storage into the pricing package.
func applyPricingMode(entries []storage.TokenUsageEntry, mode pricing.Mode) []storage.TokenUsageEntry {
	if mode == pricing.ModeDisplay {
		return entries
	}
	out := make([]storage.TokenUsageEntry, len(entries))
	for i, e := range entries {
		out[i] = e
		if mode == pricing.ModeAuto && e.CostUSD > 0 {
			continue
		}
		if e.Model == "" {
			continue
		}
		p, ok := pricingMap.Resolve(e.Model)
		if !ok {
			continue
		}
		out[i].CostUSD = pricing.CalculateCost(p, pricing.Usage{
			Input:       e.InputTokens,
			Output:      e.OutputTokens,
			CacheCreate: e.CacheCreationInputTokens,
			CacheRead:   e.CacheReadInputTokens,
		}, pricing.SpeedStandard)
	}
	return out
}
