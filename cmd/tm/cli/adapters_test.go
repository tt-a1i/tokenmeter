package cli_test

import (
	"context"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/collector"
)

func TestAdapterLoaderFromFnReturnsLoader(t *testing.T) {
	stub := func(ctx context.Context, opts collector.AdapterOpts) ([]collector.UsageEntry, error) {
		return []collector.UsageEntry{
			{SessionID: "s1", Model: "claude-sonnet-4-6", InputTokens: 100, OutputTokens: 50,
				Timestamp: time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), CostUSD: 0.01},
		}, nil
	}
	loader := cli.AdapterLoaderFromFn(stub)
	got, err := loader.ListUsageForBlocksFiltered(context.Background(), time.Time{}, time.Time{}, "")
	if err != nil {
		t.Fatalf("ListUsageForBlocksFiltered: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "s1" {
		t.Fatalf("loader did not forward entries: %#v", got)
	}
}
