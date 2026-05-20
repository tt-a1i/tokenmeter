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

func TestResolveStripsAnthropicVendorPrefix(t *testing.T) {
	m := pricing.LoadEmbedded()
	p, ok := m.Resolve("anthropic/claude-sonnet-4-6")
	if !ok {
		t.Fatalf("expected anthropic/ prefix to resolve")
	}
	exact, _ := m.Lookup("claude-sonnet-4-6")
	if p != exact {
		t.Fatalf("resolved pricing must equal exact lookup")
	}
}

func TestResolveExactWins(t *testing.T) {
	m := pricing.LoadEmbedded()
	_, ok := m.Resolve("claude-sonnet-4-6")
	if !ok {
		t.Fatal("exact match must resolve")
	}
}

func TestResolveUnknownReturnsFalse(t *testing.T) {
	m := pricing.LoadEmbedded()
	if _, ok := m.Resolve("totally-fake-model-xyz"); ok {
		t.Fatal("unknown model must return false")
	}
}
