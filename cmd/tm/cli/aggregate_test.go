package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestRunDailyJSONEnvelope(t *testing.T) {
	// Boxed-table headers changed when we switched to render.New(); the
	// stable contract is the camelCase JSON envelope. Verify daily wraps
	// rows under "daily" and surfaces a "totals" block.
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			InputTokens: 100, OutputTokens: 50, Model: "claude-opus-4-7", CostUSD: 0.5},
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

func TestRunAggregateUntilInclusive(t *testing.T) {
	// Entry at 23:59:00 on 2026-05-20 should be included when --until=20260520.
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-20T23:59:00Z"),
			InputTokens: 100, Model: "claude-opus-4-7", CostUSD: 0.5},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true, Until: "20260520"},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if !strings.Contains(buf.String(), `"2026-05-20"`) {
		t.Fatalf("expected 2026-05-20 row when --until=20260520, got:\n%s", buf.String())
	}
}

func TestRunAggregateBreakdownPropagates(t *testing.T) {
	// Two entries in the same daily bucket but different models — Breakdown=true
	// must surface them as a modelBreakdowns array under that day's row.
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			InputTokens: 100, Model: "claude-opus-4-7", CostUSD: 1},
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T11:00:00Z"),
			InputTokens: 200, Model: "gpt-5", CostUSD: 2},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true, Breakdown: true},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	out := buf.String()
	// JSON envelope must surface per-model breakdown when --breakdown is set.
	if !strings.Contains(out, "modelBreakdowns") {
		t.Fatalf("expected modelBreakdowns key in --breakdown JSON, got:\n%s", out)
	}
	if !strings.Contains(out, "claude-opus-4-7") || !strings.Contains(out, "gpt-5") {
		t.Fatalf("expected both models in breakdown, got:\n%s", out)
	}
}
