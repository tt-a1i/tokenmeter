package pricing_test

import (
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

func TestLoadEmbeddedHasClaudeSonnet(t *testing.T) {
	m := pricing.LoadEmbedded()
	p, ok := m.Lookup("claude-sonnet-4-6")
	if !ok {
		t.Fatalf("claude-sonnet-4-6 must exist in embedded snapshot")
	}
	if p.Input <= 0 || p.Output <= 0 {
		t.Fatalf("input/output cost must be positive: %+v", p)
	}
}

func TestLookupGPT5HasFastMultiplier(t *testing.T) {
	m := pricing.LoadEmbedded()
	p, ok := m.Lookup("gpt-5")
	if !ok {
		t.Fatal("gpt-5 must exist")
	}
	if p.FastMultiplier != 2.0 {
		t.Fatalf("FastMultiplier=%f want 2.0", p.FastMultiplier)
	}
}
