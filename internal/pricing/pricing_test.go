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

func TestLookupClaudeOpus46HasFastMultiplier(t *testing.T) {
	// Upstream LiteLLM ships `provider_specific_entry.fast=6` for claude-opus-4-6;
	// gpt-5 has no fast multiplier in the live snapshot, so we exercise the field
	// against a model that actually carries it.
	m := pricing.LoadEmbedded()
	p, ok := m.Lookup("claude-opus-4-6")
	if !ok {
		t.Fatal("claude-opus-4-6 must exist")
	}
	if p.FastMultiplier != 6.0 {
		t.Fatalf("FastMultiplier=%f want 6.0", p.FastMultiplier)
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

// TestEmbeddedMoonshotKimiBuiltinPrices pins the hard-coded Moonshot
// price fallback the refresh-pricing prefix filter would otherwise drop.
// The Kimi adapter routes "kimi-for-coding" usage to one of these names
// by timestamp; if a future snapshot starts shipping them, the LiteLLM
// data wins — but the lookup must keep succeeding either way.
func TestEmbeddedMoonshotKimiBuiltinPrices(t *testing.T) {
	m := pricing.LoadEmbedded()
	k25, ok := m.Lookup("moonshot/kimi-k2.5")
	if !ok {
		t.Fatal("moonshot/kimi-k2.5 must be available (LiteLLM or built-in fallback)")
	}
	if k25.Input <= 0 || k25.Output <= 0 || k25.CacheCreate <= 0 || k25.CacheRead <= 0 {
		t.Fatalf("moonshot/kimi-k2.5 must have positive rates: %+v", k25)
	}
	k26, ok := m.Lookup("moonshot/kimi-k2.6")
	if !ok {
		t.Fatal("moonshot/kimi-k2.6 must be available (LiteLLM or built-in fallback)")
	}
	if k26.Input <= k25.Input || k26.Output <= k25.Output {
		t.Fatalf("k2.6 should be priced higher than k2.5: k25=%+v k26=%+v", k25, k26)
	}
}
