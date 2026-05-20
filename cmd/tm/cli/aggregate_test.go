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

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestRunAggregateOrderDesc(t *testing.T) {
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"), InputTokens: 100, Model: "claude"},
		{SessionID: "s2", Timestamp: mustTime("2026-05-20T10:00:00Z"), InputTokens: 200, Model: "claude"},
	}}
	var buf bytes.Buffer
	err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Order: "desc"},
		Bucket: cli.BucketDaily,
	}, loader)
	if err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	out := buf.String()
	idx20 := strings.Index(out, "2026-05-20")
	idx19 := strings.Index(out, "2026-05-19")
	if idx20 == -1 || idx19 == -1 || idx20 > idx19 {
		t.Fatalf("expected 2026-05-20 before 2026-05-19, got:\n%s", out)
	}
}

func TestRunAggregateTimezoneBucketing(t *testing.T) {
	// 23:30 UTC = 07:30 next day in Asia/Shanghai
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T23:30:00Z"), InputTokens: 100, Model: "claude"},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Timezone: "Asia/Shanghai"},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if !strings.Contains(buf.String(), "2026-05-20") {
		t.Fatalf("expected 2026-05-20 bucket (Shanghai TZ), got:\n%s", buf.String())
	}
}

func TestRunAggregateModeCalculate(t *testing.T) {
	// CostUSD=0 + InputTokens>0 + Mode=calculate => pricing should recompute non-zero
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			InputTokens: 1_000_000, OutputTokens: 100_000, Model: "claude-opus-4-7", CostUSD: 0},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Mode: "calculate"},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if strings.Contains(buf.String(), "$0.00") {
		t.Fatalf("mode=calculate should recompute cost, got:\n%s", buf.String())
	}
}

func TestRunAggregateBreakdownPropagates(t *testing.T) {
	// Phase A: structural test — Breakdown plumbing lives in aggGroup.perModel.
	// Full render-layer coverage moves to Task 11.
	t.Skip("structural test — coverage moves to render layer in Task 11")
}
