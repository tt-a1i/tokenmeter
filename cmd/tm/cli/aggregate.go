package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

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

func RunAggregate(ctx context.Context, w io.Writer, a AggregateArgs, loader AggregateLoader) error {
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
	groups := groupBy(entries, a.Bucket)
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tMODELS\tTOKENS\tCOST")
	for _, k := range keys {
		g := groups[k]
		fmt.Fprintf(tw, "%s\t%s\t%s\t$%.2f\n", k, joinModels(g.models), fmtInt(g.tokens), g.cost)
	}
	return tw.Flush()
}

type aggGroup struct {
	tokens int64
	cost   float64
	models []string
	seen   map[string]struct{}
}

func groupBy(entries []storage.TokenUsageEntry, b Bucket) map[string]*aggGroup {
	out := map[string]*aggGroup{}
	for _, e := range entries {
		key := bucketKey(e.Timestamp, b)
		g, ok := out[key]
		if !ok {
			g = &aggGroup{seen: map[string]struct{}{}}
			out[key] = g
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
