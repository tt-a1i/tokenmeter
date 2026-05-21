// Package cli — adapters.go wires batch-only collector functions into the
// same AggregateLoader interface the SQLite path uses.
package cli

import (
	"context"
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
