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

type stubLoader struct{ items []storage.TokenUsageEntry }

func (s stubLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return s.items, nil
}

func TestRunBlocksTableHasHeader(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SourceID: "u1", SessionID: "s", Timestamp: start, Model: "claude-sonnet-4-6",
			InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
	}}
	var out bytes.Buffer
	err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{},
		SessionLength: 5 * time.Hour,
		Active:        false,
		Now:           start.Add(time.Hour),
	}, loader)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	if !strings.Contains(out.String(), "PERIOD") || !strings.Contains(out.String(), "TOKENS") {
		t.Fatalf("table must include header, got:\n%s", out.String())
	}
}

func TestRunBlocksJSONOutputsValidArray(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SourceID: "u1", SessionID: "s", Timestamp: start, Model: "claude-sonnet-4-6",
			InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
	}}
	var out bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true},
		SessionLength: 5 * time.Hour,
		Now:           start.Add(time.Hour),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 block, got %d", len(got))
	}
}

func TestRunBlocksModeCalculate(t *testing.T) {
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			InputTokens: 1_000_000, Model: "claude-opus-4-7", CostUSD: 0},
	}}
	var buf bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &buf, cli.BlocksArgs{
		Shared:        cli.Shared{Mode: "calculate", JSON: true},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-19T12:00:00Z"),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	// JSON output should include non-zero cost.
	if strings.Contains(buf.String(), `"cost":0`) || strings.Contains(buf.String(), `"cost": 0`) {
		t.Fatalf("expected recalculated cost, got:\n%s", buf.String())
	}
}
