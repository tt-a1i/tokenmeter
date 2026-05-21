package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// stubAggregateLoader is the shared in-test loader used by aggregate, session,
// and alias tests. It satisfies both halves of the aggregate API:
//
//   - ListUsageForBlocksFiltered returns whatever entries are in .rows.
//   - AggregateUsage returns whatever pre-aggregated rows are in .aggRows.
//
// Tests that exercise the push-down RunAggregate set aggRows; tests that
// still go through the entry-level path (session, aliases) set rows.
type stubAggregateLoader struct {
	rows    []storage.TokenUsageEntry
	aggRows []storage.AggregateUsageRow
}

func (s stubAggregateLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return s.rows, nil
}

func (s stubAggregateLoader) AggregateUsage(_ context.Context, _ storage.AggregateFilter) ([]storage.AggregateUsageRow, error) {
	return s.aggRows, nil
}

// capturingLoader records the AggregateFilter passed by RunAggregate so the
// timezone / until propagation tests can inspect what cli forwarded to
// storage without depending on the actual SQL bucketing.
type capturingLoader struct {
	captured storage.AggregateFilter
	aggRows  []storage.AggregateUsageRow
}

func (c *capturingLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return nil, nil
}

func (c *capturingLoader) AggregateUsage(_ context.Context, f storage.AggregateFilter) ([]storage.AggregateUsageRow, error) {
	c.captured = f
	return c.aggRows, nil
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestRunAggregateUsesPushdownAPI(t *testing.T) {
	// Confirms RunAggregate consumes storage.AggregateUsage rather than
	// re-aggregating raw entries. The JSON envelope must surface the
	// AggregateUsageRow buckets verbatim.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Models: []string{"claude"}, InputTokens: 100, Cost: 1.0},
		{Bucket: "2026-05-20", Models: []string{"claude"}, InputTokens: 200, Cost: 2.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"date": "2026-05-19"`) {
		t.Errorf("expected 2026-05-19 in output:\n%s", out)
	}
	if !strings.Contains(out, `"inputTokens": 100`) {
		t.Errorf("expected inputTokens=100:\n%s", out)
	}
}

func TestRunDailyJSONEnvelope(t *testing.T) {
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Models: []string{"claude-opus-4-7"},
			InputTokens: 100, OutputTokens: 50, Cost: 0.5},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	var got struct {
		Daily []struct {
			Date         string  `json:"date"`
			InputTokens  int64   `json:"inputTokens"`
			OutputTokens int64   `json:"outputTokens"`
			TotalCost    float64 `json:"totalCost"`
		} `json:"daily"`
		Totals struct {
			TotalCost float64 `json:"totalCost"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Daily) != 1 || got.Daily[0].Date != "2026-05-19" {
		t.Fatalf("expected single daily row dated 2026-05-19, got %+v", got.Daily)
	}
	if got.Daily[0].InputTokens != 100 || got.Daily[0].OutputTokens != 50 {
		t.Errorf("token totals wrong: %+v", got.Daily[0])
	}
	if got.Totals.TotalCost < 0.49 {
		t.Errorf("totals.totalCost wrong: %v", got.Totals.TotalCost)
	}
}

func TestRunAggregateOrderDesc(t *testing.T) {
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Models: []string{"claude"}, InputTokens: 100},
		{Bucket: "2026-05-20", Models: []string{"claude"}, InputTokens: 200},
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

func TestRunAggregateTimezonePropagation(t *testing.T) {
	// cli no longer buckets in-process — it forwards Location to storage.
	// Verify the filter argument carries the named timezone.
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Timezone: "Asia/Shanghai"},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatal(err)
	}
	if stub.captured.Location == nil || stub.captured.Location.String() != "Asia/Shanghai" {
		t.Errorf("expected Location=Asia/Shanghai, got %v", stub.captured.Location)
	}
}

func TestRunAggregateModeCalculate(t *testing.T) {
	// AggregateUsageRow with cost=0 + tokens>0 + Mode=calculate => cli's
	// recalculateCost recomputes from pricing.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Models: []string{"claude-opus-4-7"},
			InputTokens: 1_000_000, OutputTokens: 100_000, Cost: 0},
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

func TestRunAggregateModeAutoFallsBackOnZero(t *testing.T) {
	// ModeAuto must fall back to recalculate when bucket cost == 0 (the
	// Codex zero-cost case). Asserts cli parity with v1.0.1 entry-level
	// applyPricingMode semantics.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Models: []string{"claude-opus-4-7"},
			InputTokens: 1_000_000, OutputTokens: 100_000, Cost: 0},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Mode: "auto"},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if strings.Contains(buf.String(), "$0.00") {
		t.Fatalf("mode=auto with zero source cost must fall back to recalc, got:\n%s", buf.String())
	}
}

func TestParseDateFlagUntilClosedInterval(t *testing.T) {
	got, err := cli.ParseDateFlagUntil("20260520")
	if err != nil {
		t.Fatalf("ParseDateFlagUntil: %v", err)
	}
	want := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (+24h to give inclusive close on 2026-05-20)", got, want)
	}
}

func TestParseDateFlagUntilEmpty(t *testing.T) {
	got, err := cli.ParseDateFlagUntil("")
	if err != nil {
		t.Fatalf("ParseDateFlagUntil(\"\"): %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("empty input must stay zero, got %v", got)
	}
}

func TestRunAggregateUntilPropagates(t *testing.T) {
	// --until=20260520 must propagate to filter.Until = 2026-05-21 00:00 UTC
	// so the SQL filter includes entries through end-of-day 2026-05-20.
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Until: "20260520"},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !stub.captured.Until.Equal(want) {
		t.Errorf("filter.Until: got %v, want %v", stub.captured.Until, want)
	}
}

func TestRunAggregateBreakdownPropagates(t *testing.T) {
	// Breakdown=true => storage returns one row per (bucket, model).
	// cli folds them into one render row with Breakdown[] populated.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Model: "claude-opus-4-7", InputTokens: 100, Cost: 1},
		{Bucket: "2026-05-19", Model: "gpt-5", InputTokens: 200, Cost: 2},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true, Breakdown: true},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "modelBreakdowns") {
		t.Fatalf("expected modelBreakdowns key in --breakdown JSON, got:\n%s", out)
	}
	if !strings.Contains(out, "claude-opus-4-7") || !strings.Contains(out, "gpt-5") {
		t.Fatalf("expected both models in breakdown, got:\n%s", out)
	}
}

func TestRunAggregateBreakdownForwardsFilter(t *testing.T) {
	// AggregateFilter.Breakdown must equal Shared.Breakdown so SQL adds the
	// per-model GROUP BY column.
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Breakdown: true},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatal(err)
	}
	if !stub.captured.Breakdown {
		t.Errorf("expected filter.Breakdown=true")
	}
}
