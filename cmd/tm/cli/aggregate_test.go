package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type stubAggregateLoader struct {
	rows []storage.TokenUsageEntry
}

func (s stubAggregateLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return s.rows, nil
}

func TestRunDailyOutputsTableHeader(t *testing.T) {
	var out bytes.Buffer
	loader := stubAggregateLoader{} // returns no rows
	err := cli.RunAggregate(context.Background(), &out, cli.AggregateArgs{
		Shared: cli.Shared{},
		Bucket: cli.BucketDaily,
	}, loader)
	if err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if !strings.Contains(out.String(), "DATE") || !strings.Contains(out.String(), "COST") {
		t.Fatalf("daily table missing headers: %q", out.String())
	}
}
