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
