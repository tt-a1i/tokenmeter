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

// blocksJSON mirrors the camelCase envelope produced by render.New().
// Defined once and reused across blocks tests.
type blocksJSON struct {
	Blocks []struct {
		Period       string   `json:"period"`
		ModelsUsed   []string `json:"modelsUsed"`
		InputTokens  int64    `json:"inputTokens"`
		OutputTokens int64    `json:"outputTokens"`
		TotalTokens  int64    `json:"totalTokens"`
		TotalCost    float64  `json:"totalCost"`
		Status       string   `json:"status"`
	} `json:"blocks"`
	Totals struct {
		TotalCost float64 `json:"totalCost"`
	} `json:"totals"`
}

func TestRunBlocksJSONShape(t *testing.T) {
	// One non-active block: timestamps far enough in the past that "now" is
	// well outside the 5h window. Asserts the camelCase envelope, the
	// period-string column, and the closed status.
	start := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SourceID: "u1", SessionID: "s", Timestamp: start, Model: "claude-sonnet-4-6",
			InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
	}}
	var out bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true},
		SessionLength: 5 * time.Hour,
		Now:           start.Add(48 * time.Hour),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(got.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(got.Blocks))
	}
	if got.Blocks[0].Period != "2026-05-20 14:00" {
		t.Errorf("period: got %q, want \"2026-05-20 14:00\" (StartTime floored to hour)", got.Blocks[0].Period)
	}
	if got.Blocks[0].Status != "closed" {
		t.Errorf("status: got %q, want \"closed\"", got.Blocks[0].Status)
	}
	if got.Blocks[0].InputTokens != 100 || got.Blocks[0].OutputTokens != 50 {
		t.Errorf("token totals: %+v", got.Blocks[0])
	}
}

func TestRunBlocksActiveStatus(t *testing.T) {
	// "Now" lies inside the 5h window starting from the entry hour, so the
	// block is active. JSON status must be "ACTIVE".
	start := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s", Timestamp: start, Model: "claude-sonnet-4-6",
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
	var got blocksJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(got.Blocks) != 1 || got.Blocks[0].Status != "ACTIVE" {
		t.Fatalf("expected one ACTIVE block, got %+v", got.Blocks)
	}
}

func TestRunBlocksGapStatusInsertedOnLongQuiet(t *testing.T) {
	// Two entries spaced > 5h apart force an inserted gap block between
	// the two activity blocks.
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s", Timestamp: mustTime("2026-05-20T10:00:00Z"),
			Model: "claude-sonnet-4-6", InputTokens: 100, CostUSD: 0.01},
		{SessionID: "s", Timestamp: mustTime("2026-05-20T18:00:00Z"),
			Model: "claude-sonnet-4-6", InputTokens: 200, CostUSD: 0.02},
	}}
	var out bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-22T00:00:00Z"),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(got.Blocks) != 3 {
		t.Fatalf("want 3 blocks (active, gap, active), got %d:\n%+v", len(got.Blocks), got.Blocks)
	}
	if got.Blocks[1].Status != "gap" {
		t.Errorf("middle block status: got %q, want \"gap\"", got.Blocks[1].Status)
	}
}

func TestRunBlocksBreakdownNests(t *testing.T) {
	// Two entries in the same 5h window, different models. With
	// --breakdown + --json, the JSON envelope must surface a
	// modelBreakdowns array carrying both models.
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"),
			Model: "claude", InputTokens: 100, CostUSD: 1.0},
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T11:00:00Z"),
			Model: "gpt", InputTokens: 200, CostUSD: 2.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &buf, cli.BlocksArgs{
		Shared:        cli.Shared{Breakdown: true, JSON: true},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-19T12:00:00Z"),
	}, loader); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"modelBreakdowns"`) {
		t.Errorf("expected modelBreakdowns in JSON output:\n%s", out)
	}
	if !strings.Contains(out, `"model": "claude"`) {
		t.Errorf("expected claude in breakdown:\n%s", out)
	}
	if !strings.Contains(out, `"model": "gpt"`) {
		t.Errorf("expected gpt in breakdown:\n%s", out)
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
	// JSON output should include non-zero totalCost.
	if strings.Contains(buf.String(), `"totalCost": 0`) || strings.Contains(buf.String(), `"totalCost":0`) {
		t.Fatalf("expected recalculated cost, got:\n%s", buf.String())
	}
}
