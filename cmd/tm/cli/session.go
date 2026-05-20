package cli

import (
	"context"
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
// detection that depends on the actual output writer. Exported package-
// locally so Task 10 (aggregate) and Task 12 (blocks) can drop it in
// verbatim once they swap their tabwriter pipelines.
func renderOpts(s Shared, w io.Writer) render.Options {
	return render.Options{
		JSON:      s.JSON,
		Breakdown: s.Breakdown,
		Color:     render.Resolve(s.JSON, s.NoColor, w),
	}
}

// sessionPathLoader is the opportunistic extension *storage.DB satisfies:
// pull sessionID -> cwd once so SessionRow.ProjectPath can be populated.
// Test stubs that don't implement it simply leave ProjectPath empty.
type sessionPathLoader interface {
	ListSessions() ([]storage.SessionRow, error)
}

func RunSession(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
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

	groups := map[string]*aggGroup{}
	lastActivity := map[string]time.Time{}
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
		if e.Timestamp.After(lastActivity[e.SessionID]) {
			lastActivity[e.SessionID] = e.Timestamp
		}
	}

	// Opportunistically resolve sessionID -> cwd if the loader exposes it
	// (production *storage.DB does; the package-local test stubs do not).
	var cwdByID map[string]string
	if pl, ok := loader.(sessionPathLoader); ok {
		if sessions, err := pl.ListSessions(); err == nil {
			cwdByID = make(map[string]string, len(sessions))
			for _, sr := range sessions {
				cwdByID[sr.SessionID] = sr.CWD
			}
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

	rows := make([]render.SessionRow, 0, len(keys))
	for _, k := range keys {
		g := groups[k]
		row := render.SessionRow{
			SessionID:         k,
			ProjectPath:       cwdByID[k],
			LastActivity:      lastActivity[k],
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

	return render.New().RenderSessions(w, rows, renderOpts(a.Shared, w))
}
