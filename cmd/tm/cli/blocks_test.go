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

func (s stubLoader) ListUsageForBlocksFilteredByPlatform(_ context.Context, _, _ time.Time, _, platform string) ([]storage.TokenUsageEntry, error) {
	if platform == "" {
		return s.items, nil
	}
	var out []storage.TokenUsageEntry
	for _, item := range s.items {
		if item.AgentID == platform {
			out = append(out, item)
		}
	}
	return out, nil
}

// blocksJSON mirrors the camelCase envelope produced by render.New().
// Defined once and reused across blocks tests.
type blocksJSON struct {
	Blocks []struct {
		ID          string   `json:"id"`
		StartTime   string   `json:"startTime"`
		EndTime     string   `json:"endTime"`
		IsActive    bool     `json:"isActive"`
		IsGap       bool     `json:"isGap"`
		Entries     int      `json:"entries"`
		Models      []string `json:"models"`
		TokenCounts struct {
			InputTokens  int64 `json:"inputTokens"`
			OutputTokens int64 `json:"outputTokens"`
		} `json:"tokenCounts"`
		TotalTokens      int64   `json:"totalTokens"`
		CostUSD          float64 `json:"costUSD"`
		TokenLimitStatus *struct {
			Limit          int64   `json:"limit"`
			ProjectedUsage int64   `json:"projectedUsage"`
			PercentUsed    float64 `json:"percentUsed"`
			Status         string  `json:"status"`
		} `json:"tokenLimitStatus"`
	} `json:"blocks"`
}

func TestRunBlocksJSONShape(t *testing.T) {
	// One non-active block: timestamps far enough in the past that "now" is
	// well outside the 5h window. Asserts the ccusage-style block envelope
	// and floored startTime.
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
	if got.Blocks[0].StartTime != "2026-05-20T14:00:00.000Z" {
		t.Errorf("startTime: got %s, want floored hour in ccusage millis UTC", got.Blocks[0].StartTime)
	}
	if got.Blocks[0].ID != "2026-05-20T14:00:00.000Z" {
		t.Errorf("id: got %q, want ccusage millis UTC", got.Blocks[0].ID)
	}
	if got.Blocks[0].TokenCounts.InputTokens != 100 || got.Blocks[0].TokenCounts.OutputTokens != 50 {
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
	if len(got.Blocks) != 1 || !strings.Contains(out.String(), `"isActive": true`) {
		t.Fatalf("expected one ACTIVE block, got %+v", got.Blocks)
	}
}

func TestRunBlocksRecentFiltersOlderThanThreeDays(t *testing.T) {
	now := mustTime("2026-05-23T12:00:00Z")
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "old", Timestamp: now.Add(-96 * time.Hour), Model: "claude-sonnet-4-6", InputTokens: 10},
		{SessionID: "recent", Timestamp: now.Add(-24 * time.Hour), Model: "claude-sonnet-4-6", InputTokens: 20},
	}}
	var out bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true},
		SessionLength: 5 * time.Hour,
		Recent:        true,
		Now:           now,
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(got.Blocks) != 1 || got.Blocks[0].TokenCounts.InputTokens != 20 {
		t.Fatalf("expected only recent block, got %+v", got.Blocks)
	}
}

func TestRunBlocksPlatformFilter(t *testing.T) {
	now := mustTime("2026-05-23T12:00:00Z")
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{AgentID: "claude", SessionID: "claude", Timestamp: now.Add(-24 * time.Hour), Model: "claude-sonnet-4-6", InputTokens: 10},
		{AgentID: "codex", SessionID: "codex", Timestamp: now.Add(-23 * time.Hour), Model: "gpt-5", InputTokens: 20},
	}}
	var out bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true},
		Platform:      "claude",
		SessionLength: 5 * time.Hour,
		Now:           now,
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out.String())
	}
	if len(got.Blocks) != 1 || got.Blocks[0].TokenCounts.InputTokens != 10 {
		t.Fatalf("expected only claude block, got %+v", got.Blocks)
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
	if !strings.Contains(out.String(), `"isGap": true`) {
		t.Errorf("middle block should be marked gap:\n%s", out.String())
	}
	if got.Blocks[1].ID != "gap-2026-05-20T15:00:00.000Z" {
		t.Errorf("gap id: got %q, want ccusage gap id", got.Blocks[1].ID)
	}
	if got.Blocks[1].StartTime != "2026-05-20T15:00:00.000Z" || got.Blocks[1].EndTime != "2026-05-20T18:00:00.000Z" {
		t.Errorf("gap time range: got %s-%s, want 15:00-18:00", got.Blocks[1].StartTime, got.Blocks[1].EndTime)
	}
}

func TestRunBlocksJSONKeepsModelsList(t *testing.T) {
	// Two entries in the same 5h window, different models. With
	// --breakdown + --json, ccusage block JSON keeps models on the block row.
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
	if strings.Contains(out, `"modelBreakdowns"`) {
		t.Errorf("blocks JSON should not include legacy modelBreakdowns:\n%s", out)
	}
	if !strings.Contains(out, `"claude"`) || !strings.Contains(out, `"gpt"`) {
		t.Errorf("expected both models in models array:\n%s", out)
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
	// JSON output should include non-zero costUSD.
	if strings.Contains(buf.String(), `"costUSD": 0`) || strings.Contains(buf.String(), `"costUSD":0`) {
		t.Fatalf("expected recalculated cost, got:\n%s", buf.String())
	}
}

func TestParseSharedTokenLimitFlag(t *testing.T) {
	got, rest, err := cli.ParseShared([]string{"blocks", "--token-limit", "max"})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if got.TokenLimit != "max" {
		t.Fatalf("TokenLimit=%q want max", got.TokenLimit)
	}
	if len(rest) != 1 || rest[0] != "blocks" {
		t.Fatalf("rest=%v want [blocks]", rest)
	}
}

func TestRunBlocksTokenLimitMaxUsesHighestBlock(t *testing.T) {
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s1", Timestamp: mustTime("2026-05-19T10:00:00Z"), Model: "claude", InputTokens: 100, CostUSD: 1.0},
		{SessionID: "s2", Timestamp: mustTime("2026-05-19T18:00:00Z"), Model: "claude", InputTokens: 125, CostUSD: 1.0},
		{SessionID: "s2", Timestamp: mustTime("2026-05-19T18:30:00Z"), Model: "claude", InputTokens: 125, CostUSD: 1.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &buf, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true, TokenLimit: "max"},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-19T19:00:00Z"),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Blocks) == 0 {
		t.Fatalf("expected blocks in JSON:\n%s", buf.String())
	}
	for _, b := range got.Blocks {
		if !b.IsActive {
			continue
		}
		if b.TokenLimitStatus == nil || b.TokenLimitStatus.Limit != 250 {
			t.Fatalf("active block tokenLimitStatus.limit mismatch: %+v", b)
		}
		return
	}
	t.Fatalf("expected an active block with tokenLimitStatus: %+v", got.Blocks)
}

func TestRunBlocksJSONIncludesTokenLimitFields(t *testing.T) {
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s", Timestamp: mustTime("2026-05-19T10:00:00Z"), Model: "claude", InputTokens: 40, CostUSD: 0.5},
		{SessionID: "s", Timestamp: mustTime("2026-05-19T11:00:00Z"), Model: "claude", InputTokens: 40, CostUSD: 0.5},
	}}
	var buf bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &buf, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true, TokenLimit: "100"},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-19T12:00:00Z"),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	var got blocksJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if got.Blocks[0].TokenLimitStatus == nil ||
		got.Blocks[0].TokenLimitStatus.Limit != 100 ||
		got.Blocks[0].TokenLimitStatus.ProjectedUsage == 0 ||
		got.Blocks[0].TokenLimitStatus.PercentUsed == 0 ||
		got.Blocks[0].TokenLimitStatus.Status == "" {
		t.Fatalf("token-limit JSON mismatch: %+v", got.Blocks[0])
	}
}

func TestRunBlocksInstancesJSONStaysCcusageShape(t *testing.T) {
	loader := stubLoader{items: []storage.TokenUsageEntry{
		{SessionID: "s", Timestamp: mustTime("2026-05-19T10:00:00Z"), CWD: "/repo/agmon", Model: "claude", InputTokens: 80, CostUSD: 1.0},
	}}
	var buf bytes.Buffer
	if err := cli.RunBlocks(context.Background(), &buf, cli.BlocksArgs{
		Shared:        cli.Shared{JSON: true, Instances: true},
		SessionLength: 5 * time.Hour,
		Now:           mustTime("2026-05-20T12:00:00Z"),
	}, loader); err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	if strings.Contains(buf.String(), `"project"`) {
		t.Fatalf("blocks JSON should not add non-ccusage project field:\n%s", buf.String())
	}
}
