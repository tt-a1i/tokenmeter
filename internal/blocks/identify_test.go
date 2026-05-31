package blocks_test

import (
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func entry(ts time.Time, model string, in, out int64, cost float64) storage.TokenUsageEntry {
	return storage.TokenUsageEntry{
		Timestamp: ts, Model: model, InputTokens: in, OutputTokens: out, CostUSD: cost,
	}
}

func TestIdentifyEmpty(t *testing.T) {
	got := blocks.Identify(nil, 5*time.Hour, time.Now())
	if len(got) != 0 {
		t.Fatalf("want 0 blocks for empty input, got %d", len(got))
	}
}

func TestIdentifySingleBlock(t *testing.T) {
	base := time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC)
	in := []storage.TokenUsageEntry{
		entry(base, "sonnet", 10, 5, 0.001),
		entry(base.Add(2*time.Hour), "sonnet", 20, 10, 0.002),
	}
	got := blocks.Identify(in, 5*time.Hour, base.Add(3*time.Hour))
	if len(got) != 1 {
		t.Fatalf("want 1 block, got %d", len(got))
	}
	if !got[0].StartTime.Equal(time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("start must be floored to hour: %s", got[0].StartTime)
	}
	if got[0].Tokens.Total() != 45 {
		t.Fatalf("tokens=%d want 45", got[0].Tokens.Total())
	}
	if !got[0].IsActive {
		t.Fatalf("block should be active (now < end and last-entry < 5h ago)")
	}
}

func TestIdentifySplitOn5hGap(t *testing.T) {
	base := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	in := []storage.TokenUsageEntry{
		entry(base, "sonnet", 10, 5, 0.001),
		entry(base.Add(6*time.Hour), "sonnet", 20, 10, 0.002), // > 5h gap
	}
	got := blocks.Identify(in, 5*time.Hour, base.Add(7*time.Hour))
	// Expect: block1, gap, block2
	if len(got) != 3 {
		t.Fatalf("want 3 entries (block, gap, block), got %d", len(got))
	}
	if got[0].IsGap || !got[1].IsGap || got[2].IsGap {
		t.Fatalf("expected gap at index 1, got isGap=%v,%v,%v", got[0].IsGap, got[1].IsGap, got[2].IsGap)
	}
	if !got[1].StartTime.Equal(base.Add(5*time.Hour)) || !got[1].EndTime.Equal(base.Add(6*time.Hour)) {
		t.Fatalf("gap range = %s-%s, want last activity + duration through next entry", got[1].StartTime, got[1].EndTime)
	}
}
