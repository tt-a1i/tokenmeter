package blocks_test

import (
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

func TestBurnRateForActiveBlock(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	first := start
	last := start.Add(30 * time.Minute)
	b := blocks.SessionBlock{
		StartTime:  start,
		EndTime:    start.Add(5 * time.Hour),
		FirstEntry: &first,
		ActualEnd:  &last,
		IsActive:   true,
		Tokens:     blocks.TokenCounts{Input: 60000, Output: 30000},
		Cost:       1.50,
	}
	bb := blocks.WithBurnAndProjection(b, start.Add(35*time.Minute))
	if bb.BurnRate == nil {
		t.Fatalf("burn rate must be set on active blocks")
	}
	// 90000 tokens / 30 minutes elapsed = 3000 tok/min
	if got := bb.BurnRate.TokensPerMinute; got < 2999 || got > 3001 {
		t.Fatalf("TokensPerMinute=%f want ~3000", got)
	}
	if bb.Projection == nil || bb.Projection.RemainingTime <= 0 {
		t.Fatalf("projection must be set with positive remaining time, got %+v", bb.Projection)
	}
}

func TestBurnRateUsesFirstEntryAndRoundsProjectionLikeCcusage(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	first := start.Add(10 * time.Minute)
	last := start.Add(40 * time.Minute)
	b := blocks.SessionBlock{
		StartTime:  start,
		EndTime:    start.Add(5 * time.Hour),
		FirstEntry: &first,
		ActualEnd:  &last,
		IsActive:   true,
		Tokens:     blocks.TokenCounts{Input: 100},
		Cost:       1.00,
	}
	bb := blocks.WithBurnAndProjection(b, start.Add(59*time.Minute+40*time.Second))
	if bb.BurnRate == nil {
		t.Fatal("burn rate must be set")
	}
	// ccusage uses first entry to last entry, not floored block start.
	if got := bb.BurnRate.TokensPerMinute; got < 3.32 || got > 3.34 {
		t.Fatalf("TokensPerMinute=%f want ~3.33", got)
	}
	if bb.Projection == nil {
		t.Fatal("projection must be set")
	}
	if got := bb.Projection.RemainingTime; got != 240*time.Minute {
		t.Fatalf("RemainingTime=%s want rounded 240m", got)
	}
	if got := bb.Projection.TotalTokens; got != 900 {
		t.Fatalf("TotalTokens=%d want rounded 900", got)
	}
	if got := bb.Projection.TotalCost; got != 9.00 {
		t.Fatalf("TotalCost=%f want 9.00", got)
	}
}

func TestBurnRateSkipsInactiveBlock(t *testing.T) {
	b := blocks.SessionBlock{IsActive: false}
	bb := blocks.WithBurnAndProjection(b, time.Now())
	if bb.BurnRate != nil || bb.Projection != nil {
		t.Fatalf("burn/projection must be nil for inactive blocks")
	}
}
