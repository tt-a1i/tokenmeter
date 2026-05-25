package blocks_test

import (
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

func TestAnnotateWithTokenLimitClassifiesUsage(t *testing.T) {
	in := []blocks.Block{
		{Tokens: blocks.TokenCounts{Input: 79}},
		{Tokens: blocks.TokenCounts{Input: 80}},
		{Tokens: blocks.TokenCounts{Input: 100}},
	}

	got := blocks.AnnotateWithTokenLimit(in, 100)

	if got[0].TokenLimitStatus != "OK" || got[0].UsagePct != 79 {
		t.Fatalf("first block = %+v, want OK at 79%%", got[0])
	}
	if got[1].TokenLimitStatus != "WARN" || got[1].UsagePct != 80 {
		t.Fatalf("second block = %+v, want WARN at 80%%", got[1])
	}
	if got[2].TokenLimitStatus != "ALERT" || got[2].UsagePct != 100 {
		t.Fatalf("third block = %+v, want ALERT at 100%%", got[2])
	}
}

func TestAnnotateWithTokenLimitZeroDisablesAnnotation(t *testing.T) {
	got := blocks.AnnotateWithTokenLimit([]blocks.Block{{Tokens: blocks.TokenCounts{Input: 100}}}, 0)
	if got[0].TokenLimit != 0 || got[0].UsagePct != 0 || got[0].TokenLimitStatus != "" {
		t.Fatalf("zero limit should leave token-limit fields empty, got %+v", got[0])
	}
}

func TestMaxTokenLimitUsesHighestNonGapBlock(t *testing.T) {
	in := []blocks.Block{
		{Tokens: blocks.TokenCounts{Input: 100}},
		{IsGap: true, Tokens: blocks.TokenCounts{Input: 999}},
		{Tokens: blocks.TokenCounts{Input: 250}},
	}
	if got := blocks.MaxTokenLimit(in); got != 250 {
		t.Fatalf("MaxTokenLimit=%d want 250", got)
	}
}
