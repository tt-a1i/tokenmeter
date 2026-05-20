package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"
)

type SessionArgs struct {
	Shared    Shared
	SessionID string // optional; "" lists all
}

func RunSession(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
	entries, err := loader.ListUsageForBlocksFiltered(ctx, time.Time{}, time.Time{}, a.Shared.Project)
	if err != nil {
		return err
	}
	groups := map[string]*aggGroup{}
	for _, e := range entries {
		if a.SessionID != "" && e.SessionID != a.SessionID {
			continue
		}
		g, ok := groups[e.SessionID]
		if !ok {
			g = &aggGroup{seen: map[string]struct{}{}}
			groups[e.SessionID] = g
		}
		g.tokens += e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
		g.cost += e.CostUSD
		if e.Model != "" {
			if _, exists := g.seen[e.Model]; !exists {
				g.models = append(g.models, e.Model)
				g.seen[e.Model] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tMODELS\tTOKENS\tCOST")
	for _, k := range keys {
		g := groups[k]
		fmt.Fprintf(tw, "%s\t%s\t%s\t$%.2f\n", k, joinModels(g.models), fmtInt(g.tokens), g.cost)
	}
	return tw.Flush()
}
