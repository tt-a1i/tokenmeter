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
	"github.com/tt-a1i/tokenmeter/internal/collector"
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

func (s stubAggregateLoader) ListUsageForBlocksFilteredByPlatform(_ context.Context, _, _ time.Time, _, platform string) ([]storage.TokenUsageEntry, error) {
	if platform == "" {
		return s.rows, nil
	}
	var out []storage.TokenUsageEntry
	for _, row := range s.rows {
		if row.AgentID == platform {
			out = append(out, row)
		}
	}
	return out, nil
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

func (c *capturingLoader) ListUsageForBlocksFilteredByPlatform(_ context.Context, _, _ time.Time, _, _ string) ([]storage.TokenUsageEntry, error) {
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

func TestRunAggregateForwardsPlatformFilter(t *testing.T) {
	loader := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), &bytes.Buffer{}, cli.AggregateArgs{
		Shared:   cli.Shared{},
		Bucket:   cli.BucketDaily,
		Platform: "codex",
	}, loader); err != nil {
		t.Fatal(err)
	}
	if loader.captured.Platform != "codex" {
		t.Fatalf("Platform=%q want codex", loader.captured.Platform)
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

func TestParseSharedProjectAliasFlags(t *testing.T) {
	got, rest, err := cli.ParseShared([]string{"daily", "--instances", "--project-aliases", `{"agmon":["/repo/a"]}`})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if !got.Instances {
		t.Fatal("--instances must be true")
	}
	if got.ProjectAliases == "" {
		t.Fatal("--project-aliases must be captured")
	}
	if len(rest) != 1 || rest[0] != "daily" {
		t.Fatalf("rest=%v want [daily]", rest)
	}
}

func TestRunAggregateProjectAliasesMergeCWDs(t *testing.T) {
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"), CWD: "/repo/agmon", Model: "claude", InputTokens: 100, CostUSD: 1.0},
		{SessionID: "s2", Timestamp: mustTime("2026-05-19T11:00:00Z"), CWD: "/repo/agmon-feature", Model: "claude", InputTokens: 200, CostUSD: 2.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{JSON: true, ProjectAliases: `{"agmon":["/repo/agmon","/repo/agmon-feature"]}`},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	if !strings.Contains(buf.String(), `"inputTokens": 300`) {
		t.Fatalf("alias cwds should merge into one project row:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"project": "agmon"`) {
		t.Fatalf("JSON should include resolved project:\n%s", buf.String())
	}
}

func TestRunAggregateInstancesShowsProjectColumn(t *testing.T) {
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"), CWD: "/repo/agmon", Model: "claude", InputTokens: 100, CostUSD: 1.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Instances: true},
		Bucket: cli.BucketDaily,
	}, loader); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "PROJECT") || !strings.Contains(out, "agmon") {
		t.Fatalf("--instances should show PROJECT column:\n%s", out)
	}
}

func TestRunSessionProjectAliasesNormalizeProjectPath(t *testing.T) {
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "s1", Project: "/repo/agmon-feature", Models: []string{"claude"}, InputTokens: 100},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf, cli.SessionArgs{
		Shared: cli.Shared{ProjectAliases: `{"agmon":["/repo/agmon-feature"]}`},
	}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "agmon") || strings.Contains(out, "/repo/agmon-feature") {
		t.Fatalf("session project path should be normalized by aliases:\n%s", out)
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

// G7: ccusage normalizes --since/--until by stripping '-' before parsing
// (rust/crates/ccusage-cli/src/types.rs:64-66). Accept the dashed form so
// `--since 2026-05-20 --until 2026-05-20` works alongside the canonical
// YYYYMMDD form.
func TestParseDateFlagUntilAcceptsDashedDate(t *testing.T) {
	got, err := cli.ParseDateFlagUntil("2026-05-20")
	if err != nil {
		t.Fatalf("ParseDateFlagUntil(dashed): %v", err)
	}
	want := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("dashed --until 2026-05-20 got %v, want %v", got, want)
	}
}

// G7: --until still accepts YYYYMMDD after normalization (regression
// guard for the dash-stripping change).
func TestParseDateFlagUntilStillAcceptsCompactDate(t *testing.T) {
	got, err := cli.ParseDateFlagUntil("20260520")
	if err != nil {
		t.Fatalf("ParseDateFlagUntil(compact): %v", err)
	}
	want := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("compact --until 20260520 got %v, want %v", got, want)
	}
}

// G7: RunAggregate must accept --since 2026-05-20 just like 20260520 and
// derive the same lower/upper bounds. This proves the normalization flows
// through the whole shared-flag path, not just the helper.
func TestRunAggregateAcceptsDashedSinceUntil(t *testing.T) {
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Since: "2026-05-20", Until: "2026-05-20"},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatalf("RunAggregate: %v", err)
	}
	wantSince := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if !stub.captured.Since.Equal(wantSince) {
		t.Fatalf("Since = %v, want %v", stub.captured.Since, wantSince)
	}
	if !stub.captured.Until.Equal(wantUntil) {
		t.Fatalf("Until = %v, want %v", stub.captured.Until, wantUntil)
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

func TestRunAggregateModeCalculateBreakdownPrecision(t *testing.T) {
	// SUM-then-reprice path: storage returns per-model aggregated rows
	// (Breakdown=true), cli recalculates each model's cost from its own
	// pricing — no first-model approximation.
	rows := []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Model: "claude-opus-4-7", InputTokens: 1_000_000, Cost: 0},
		{Bucket: "2026-05-19", Model: "gpt-5", InputTokens: 1_000_000, Cost: 0},
	}
	args := cli.AggregateArgs{
		Shared: cli.Shared{Mode: "calculate", Breakdown: true, JSON: true},
		Bucket: cli.BucketDaily,
	}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, args, stubAggregateLoader{aggRows: rows}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"modelName": "claude-opus-4-7"`) {
		t.Errorf("expected claude breakdown:\n%s", out)
	}
	if !strings.Contains(out, `"modelName": "gpt-5"`) {
		t.Errorf("expected gpt breakdown:\n%s", out)
	}
	// Mode=calculate must recompute non-zero costs for both models.
	if strings.Contains(out, `"totalCost": 0`) || strings.Contains(out, `"totalCost":0`) {
		t.Errorf("mode=calculate should recompute non-zero cost:\n%s", out)
	}
}

func TestRunAggregateModeAutoMixedBucket(t *testing.T) {
	// Mixed-model day: Claude entry has cost, Codex entry has cost=0. With
	// only bucket-level Auto fallback, the Codex share is silently dropped
	// because the bucket SUM (= 10) is non-zero. The fix forces cli to
	// pull per-(bucket, model) rows and recompute per row, so totalCost
	// must exceed the Claude-only sum.
	rows := []storage.AggregateUsageRow{
		{Bucket: "2026-05-19", Model: "claude-opus-4-7", InputTokens: 1_000_000, Cost: 10},
		{Bucket: "2026-05-19", Model: "gpt-5-codex", InputTokens: 1_000_000, Cost: 0},
	}
	var buf bytes.Buffer
	if err := cli.RunAggregate(context.Background(), &buf, cli.AggregateArgs{
		Shared: cli.Shared{Mode: "auto", JSON: true},
		Bucket: cli.BucketDaily,
	}, stubAggregateLoader{aggRows: rows}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Daily []struct {
			Date      string  `json:"date"`
			TotalCost float64 `json:"totalCost"`
		} `json:"daily"`
		Totals struct {
			TotalCost float64 `json:"totalCost"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Daily) != 1 {
		t.Fatalf("expected 1 daily row, got %d", len(got.Daily))
	}
	if got.Daily[0].TotalCost <= 10 {
		t.Fatalf("Codex zero-cost share must recompute under Auto; got totalCost=%v (Claude alone was 10)", got.Daily[0].TotalCost)
	}
}

func TestRunAggregateModeAutoForcesBreakdownInFilter(t *testing.T) {
	// Mode=auto without --breakdown still needs per-(bucket, model) rows
	// from storage to trigger the per-row fallback. cli must set
	// filter.Breakdown=true regardless of user choice.
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Mode: "auto"},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatal(err)
	}
	if !stub.captured.Breakdown {
		t.Errorf("filter.Breakdown must be forced true under Mode=auto; got false")
	}
}

func TestRunAggregateModeDisplayKeepsFilterBreakdownFalse(t *testing.T) {
	// Sanity: Mode=display (or anything other than Auto) does NOT touch
	// filter.Breakdown when user didn't ask. Avoids needlessly pulling
	// per-model rows from storage.
	stub := &capturingLoader{}
	if err := cli.RunAggregate(context.Background(), io.Discard, cli.AggregateArgs{
		Shared: cli.Shared{Mode: "display"},
		Bucket: cli.BucketDaily,
	}, stub); err != nil {
		t.Fatal(err)
	}
	if stub.captured.Breakdown {
		t.Errorf("filter.Breakdown must stay false under Mode=display without --breakdown; got true")
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

// TestRunAggregateAllSourceMerges drives RunAggregateAllSource (the
// default daily/weekly/monthly path when --no-scan is not set) with a
// stub SQLite loader plus one registered adapter ("amp"). The two
// sources contribute entries with the same model on the same day, so
// the merged daily row should sum the cost ($0.01 + $0.02 = $0.03).
//
// Note: header asserted as "DATE" — the live render path uses go-pretty
// table.StyleRounded whose default header transform uppercases column
// names. The claude-sonnet-4-6 model token doubles as a merge invariant
// (both sources contribute this model so it must appear in the row).
func TestRunAggregateAllSourceMerges(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	sqliteEntries := []storage.TokenUsageEntry{
		{SourceID: "c1", SessionID: "s-claude", Model: "claude-sonnet-4-6",
			Timestamp: now, InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
	}
	sqliteLoader := stubAggregateLoader{rows: sqliteEntries}
	ampEntries := []collector.UsageEntry{
		{Source: "amp", SessionID: "s-amp", Model: "claude-sonnet-4-6",
			Timestamp: now, InputTokens: 200, OutputTokens: 100, CostUSD: 0.02},
	}
	ampFn := func(_ context.Context, _ collector.AdapterOpts) ([]collector.UsageEntry, error) {
		return ampEntries, nil
	}
	var out bytes.Buffer
	err := cli.RunAggregateAllSource(context.Background(), &out, cli.AggregateArgs{
		Shared: cli.Shared{},
		Bucket: cli.BucketDaily,
	}, sqliteLoader, map[string]cli.AdapterLoadFn{"amp": ampFn})
	if err != nil {
		t.Fatalf("RunAggregateAllSource: %v", err)
	}
	if !strings.Contains(out.String(), "DATE") {
		t.Fatalf("missing DATE header in boxed table: %q", out.String())
	}
	if !strings.Contains(out.String(), "claude-sonnet-4-6") {
		t.Fatalf("merged row missing model: %q", out.String())
	}
	if !strings.Contains(out.String(), "$0.03") {
		t.Fatalf("merged cost not in output (expected $0.01 sqlite + $0.02 amp = $0.03): %q", out.String())
	}
}
