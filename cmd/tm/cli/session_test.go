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
