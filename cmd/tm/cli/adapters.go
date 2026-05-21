// Package cli — adapters.go wires batch-only collector functions into the
// same AggregateLoader interface the SQLite path uses.
package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/collector"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// AdapterLoadFn matches the standard collector.Load<Name>Entries signature.
type AdapterLoadFn func(ctx context.Context, opts collector.AdapterOpts) ([]collector.UsageEntry, error)

// AdapterLoaderFromFn adapts an in-memory batch collector to the
// AggregateLoader interface that RunAggregate / RunSession expect.
func AdapterLoaderFromFn(fn AdapterLoadFn) AggregateLoader {
	return &adapterLoader{fn: fn}
}

type adapterLoader struct {
	fn AdapterLoadFn
}

func (a *adapterLoader) ListUsageForBlocksFiltered(ctx context.Context, since, until time.Time, project string) ([]storage.TokenUsageEntry, error) {
	opts := collector.AdapterOpts{Since: since, Until: until, Project: project}
	entries, err := a.fn(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]storage.TokenUsageEntry, len(entries))
	for i, e := range entries {
		out[i] = storage.TokenUsageEntry{
			SourceID:                 e.SessionID,
			SessionID:                e.SessionID,
			Timestamp:                e.Timestamp,
			Model:                    e.Model,
			InputTokens:              e.InputTokens,
			OutputTokens:             e.OutputTokens,
			CacheCreationInputTokens: e.CacheCreationInputTokens,
			CacheReadInputTokens:     e.CacheReadInputTokens,
			CostUSD:                  e.CostUSD,
		}
	}
	return out, nil
}

// emptyAggregateLoader is the AggregateLoader RunAdapter passes to
// RunAggregateAllSource as the "SQLite baseline" when a per-source
// command (e.g. `tm amp daily`) only wants that source's data — no
// Claude/Codex rows from the local SQLite store. Returns (nil, nil) so
// the merge loop treats SQLite as empty and lets the single registered
// adapter drive the entire rendered view.
type emptyAggregateLoader struct{}

func (emptyAggregateLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return nil, nil
}

// RunAdapter dispatches a per-source adapter command (e.g. `tm amp daily`,
// `tm goose weekly`). It looks up source's loader in AllAdapters and
// runs it as a single-source aggregation through the same in-memory
// render path RunAggregateAllSource uses, so the output is visually
// identical to `tm daily --no-scan=false` filtered to one source.
//
// Returns an error when source is not in AllAdapters; this should be
// unreachable from main.go (router pre-validates source against
// adapterNameSet which mirrors AllAdapters) but is checked defensively
// so a future registry-drift bug surfaces here rather than as a nil
// dereference inside the merge loop.
func RunAdapter(ctx context.Context, w io.Writer, args AggregateArgs, source string) error {
	fn, ok := AllAdapters[source]
	if !ok {
		return fmt.Errorf("unknown adapter source: %s", source)
	}
	return RunAggregateAllSource(ctx, w, args,
		emptyAggregateLoader{},
		map[string]AdapterLoadFn{source: fn},
	)
}
