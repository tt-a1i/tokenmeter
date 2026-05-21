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

func TestRunSessionListMode(t *testing.T) {
	var out bytes.Buffer
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "abc", Models: []string{"sonnet"}, InputTokens: 10, OutputTokens: 5, Cost: 0.001,
			LastActivity: time.Now()},
	}}
	if err := cli.RunSession(context.Background(), &out, cli.SessionArgs{Shared: cli.Shared{}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if !strings.Contains(out.String(), "SESSION") {
		t.Fatalf("expected SESSION header in output: %q", out.String())
	}
}

func TestRunSessionOrderDesc(t *testing.T) {
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "aaa", Models: []string{"claude"}, LastActivity: mustTime("2026-05-19T10:00:00Z")},
		{Bucket: "zzz", Models: []string{"claude"}, LastActivity: mustTime("2026-05-19T11:00:00Z")},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf, cli.SessionArgs{Shared: cli.Shared{Order: "desc"}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	out := buf.String()
	idxZ := strings.Index(out, "zzz")
	idxA := strings.Index(out, "aaa")
	if idxZ == -1 || idxA == -1 || idxZ > idxA {
		t.Fatalf("expected zzz before aaa, got:\n%s", out)
	}
}

func TestRunSessionJSONSchema(t *testing.T) {
	// JSON envelope is the stable contract — assert via Unmarshal rather
	// than peeking at boxed-table characters.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "abc", Models: []string{"claude-opus-4-7"},
			InputTokens: 300, OutputTokens: 125, Cost: 0.3,
			LastActivity: mustTime("2026-05-19T11:30:00Z")},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf,
		cli.SessionArgs{Shared: cli.Shared{JSON: true}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	var got struct {
		Sessions []struct {
			SessionID    string    `json:"sessionId"`
			LastActivity time.Time `json:"lastActivity"`
			InputTokens  int64     `json:"inputTokens"`
			OutputTokens int64     `json:"outputTokens"`
			TotalCost    float64   `json:"totalCost"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Sessions) != 1 || got.Sessions[0].SessionID != "abc" {
		t.Fatalf("expected one session abc, got %+v", got.Sessions)
	}
	if got.Sessions[0].InputTokens != 300 || got.Sessions[0].TotalCost < 0.29 {
		t.Errorf("aggregated tokens/cost wrong: %+v", got.Sessions[0])
	}
	want := mustTime("2026-05-19T11:30:00Z")
	if !got.Sessions[0].LastActivity.Equal(want) {
		t.Errorf("lastActivity: got %v, want %v", got.Sessions[0].LastActivity, want)
	}
}

func TestRunSessionProjectPathInJSON(t *testing.T) {
	// SessionBucket carries Project + LastActivity directly from SQL.
	// Verify cli passes both through to the render layer.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{
			Bucket: "abc-123", Models: []string{"claude"},
			InputTokens: 100, Cost: 1.0,
			Project:      "/code/foo",
			LastActivity: mustTime("2026-05-19T11:30:00Z"),
		},
	}}
	args := cli.SessionArgs{Shared: cli.Shared{JSON: true}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf, args, loader); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"projectPath": "/code/foo"`) {
		t.Errorf("expected projectPath in output:\n%s", out)
	}
	if !strings.Contains(out, `"sessionId": "abc-123"`) {
		t.Errorf("expected sessionId in output:\n%s", out)
	}
}

func TestRunSessionFiltersBySessionID(t *testing.T) {
	// SessionArgs.SessionID restricts the displayed buckets even though
	// the loader returned more.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "abc", Models: []string{"claude"}, InputTokens: 1, LastActivity: mustTime("2026-05-19T10:00:00Z")},
		{Bucket: "xyz", Models: []string{"claude"}, InputTokens: 2, LastActivity: mustTime("2026-05-19T11:00:00Z")},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf,
		cli.SessionArgs{Shared: cli.Shared{JSON: true}, SessionID: "xyz"}, loader); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"sessionId": "xyz"`) {
		t.Errorf("expected xyz in output:\n%s", out)
	}
	if strings.Contains(out, `"sessionId": "abc"`) {
		t.Errorf("filtered-out session abc must not appear:\n%s", out)
	}
}

func TestRunSessionModeCalculate(t *testing.T) {
	// AggregateUsageRow with cost=0 + tokens>0 + Mode=calculate =>
	// cli's recalculateCost recomputes from pricing.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "s1", Models: []string{"claude-opus-4-7"},
			InputTokens: 1_000_000, OutputTokens: 100_000, Cost: 0,
			LastActivity: mustTime("2026-05-19T10:00:00Z")},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf, cli.SessionArgs{Shared: cli.Shared{Mode: "calculate"}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if strings.Contains(buf.String(), "$0.00") {
		t.Fatalf("mode=calculate should recompute cost, got:\n%s", buf.String())
	}
}

func TestRunSessionModeAutoFallsBackOnZero(t *testing.T) {
	// Codex-style zero-cost row + Mode=auto must trigger the recalc
	// fallback, matching RunAggregate parity.
	loader := stubAggregateLoader{aggRows: []storage.AggregateUsageRow{
		{Bucket: "s1", Models: []string{"claude-opus-4-7"},
			InputTokens: 1_000_000, OutputTokens: 100_000, Cost: 0,
			LastActivity: mustTime("2026-05-19T10:00:00Z")},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf,
		cli.SessionArgs{Shared: cli.Shared{Mode: "auto"}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if strings.Contains(buf.String(), "$0.00") {
		t.Fatalf("mode=auto with zero source cost must fall back to recalc, got:\n%s", buf.String())
	}
}

func TestRunSessionModeAutoMixedSession(t *testing.T) {
	// Same parity bug as RunAggregate: a session with mixed models, one of
	// which has cost=0, must recompute the zero-cost share under Auto.
	rows := []storage.AggregateUsageRow{
		{Bucket: "s1", Model: "claude-opus-4-7", InputTokens: 1_000_000, Cost: 10,
			LastActivity: mustTime("2026-05-19T10:00:00Z")},
		{Bucket: "s1", Model: "gpt-5-codex", InputTokens: 1_000_000, Cost: 0,
			LastActivity: mustTime("2026-05-19T11:00:00Z")},
	}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf,
		cli.SessionArgs{Shared: cli.Shared{Mode: "auto", JSON: true}},
		stubAggregateLoader{aggRows: rows}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Sessions []struct {
			SessionID string  `json:"sessionId"`
			TotalCost float64 `json:"totalCost"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Sessions) != 1 || got.Sessions[0].SessionID != "s1" {
		t.Fatalf("expected 1 session s1, got %+v", got.Sessions)
	}
	if got.Sessions[0].TotalCost <= 10 {
		t.Fatalf("Codex zero-cost share must recompute under Auto; got totalCost=%v (Claude alone was 10)", got.Sessions[0].TotalCost)
	}
}

func TestRunSessionModeAutoForcesBreakdownInFilter(t *testing.T) {
	stub := &capturingLoader{}
	if err := cli.RunSession(context.Background(), &bytes.Buffer{},
		cli.SessionArgs{Shared: cli.Shared{Mode: "auto"}}, stub); err != nil {
		t.Fatal(err)
	}
	if !stub.captured.Breakdown {
		t.Errorf("filter.Breakdown must be forced true under Mode=auto; got false")
	}
}

func TestRunSessionForwardsFilterToBucketSession(t *testing.T) {
	// RunSession must always tell storage to GROUP BY session_id, even
	// when --breakdown is on (per-model rows are folded back inside cli).
	stub := &capturingLoader{}
	if err := cli.RunSession(context.Background(), &bytes.Buffer{},
		cli.SessionArgs{Shared: cli.Shared{Breakdown: true}}, stub); err != nil {
		t.Fatal(err)
	}
	if stub.captured.Bucket != storage.BucketSession {
		t.Errorf("filter.Bucket: got %v, want storage.BucketSession", stub.captured.Bucket)
	}
	if !stub.captured.Breakdown {
		t.Errorf("filter.Breakdown=true expected when Shared.Breakdown=true")
	}
}
