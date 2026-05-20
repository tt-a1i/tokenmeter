package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

type SessionArgs struct {
	Shared    Shared
	SessionID string // optional; "" lists all
}

func RunSession(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := parseDateFlag(a.Shared.Until)
	if err != nil {
		return err
	}
	entries, err := loader.ListUsageForBlocksFiltered(ctx, since, until, a.Shared.Project)
	if err != nil {
		return err
	}
	entries = applyPricingMode(entries, pricing.ParseMode(a.Shared.Mode))

	groups := map[string]*aggGroup{}
	for _, e := range entries {
		if a.SessionID != "" && e.SessionID != a.SessionID {
			continue
		}
		g, ok := groups[e.SessionID]
		if !ok {
			g = &aggGroup{
				seen:     map[string]struct{}{},
				perModel: map[string]*modelStats{},
			}
			groups[e.SessionID] = g
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
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	if a.Shared.Order == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tMODELS\tTOKENS\tCOST")
	for _, k := range keys {
		g := groups[k]
		fmt.Fprintf(tw, "%s\t%s\t%s\t$%.2f\n", k, joinModels(g.models), fmtInt(g.tokens), g.cost)
	}
	return tw.Flush()
}
