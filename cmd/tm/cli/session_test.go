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
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SourceID: "1", SessionID: "abc", Model: "sonnet", InputTokens: 10, OutputTokens: 5, CostUSD: 0.001, Timestamp: time.Now()},
	}}
	if err := cli.RunSession(context.Background(), &out, cli.SessionArgs{Shared: cli.Shared{}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if !strings.Contains(out.String(), "SESSION") {
		t.Fatalf("expected SESSION header in output: %q", out.String())
	}
}

func TestRunSessionOrderDesc(t *testing.T) {
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "aaa", Timestamp: mustTime("2026-05-19T10:00:00Z"), Model: "claude"},
		{SessionID: "zzz", Timestamp: mustTime("2026-05-19T11:00:00Z"), Model: "claude"},
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
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "abc", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			Model: "claude-opus-4-7", InputTokens: 100, OutputTokens: 50, CostUSD: 0.1},
		{SessionID: "abc", Timestamp: mustTime("2026-05-19T11:30:00Z"),
			Model: "claude-opus-4-7", InputTokens: 200, OutputTokens: 75, CostUSD: 0.2},
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

func TestRunSessionModeCalculate(t *testing.T) {
	// CostUSD=0 + tokens>0 + Mode=calculate => pricing recomputes non-zero.
	loader := stubAggregateLoader{rows: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			InputTokens: 1_000_000, OutputTokens: 100_000, Model: "claude-opus-4-7", CostUSD: 0},
	}}
	var buf bytes.Buffer
	if err := cli.RunSession(context.Background(), &buf, cli.SessionArgs{Shared: cli.Shared{Mode: "calculate"}}, loader); err != nil {
		t.Fatalf("RunSession: %v", err)
	}
	if strings.Contains(buf.String(), "$0.00") {
		t.Fatalf("mode=calculate should recompute cost, got:\n%s", buf.String())
	}
}
