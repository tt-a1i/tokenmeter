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

// TestResolveNormalizesDotAndAtSeparators mirrors ccusage v20's
// pricing-key normalization (rust/crates/ccusage/src/pricing.rs:604-610):
// "." and "@" between segments are equivalent to "-" so model names from
// the Anthropic console / API console paste ("claude.sonnet.4") and the
// alternative "@"-separated alias ("claude@sonnet@4") still find the
// canonical pricing entry. Boundary-aware longest-match also strips
// vendor prefixes ("anthropic.claude.sonnet.4" → "claude-sonnet-4")
// without an O(N) entry scan.
func TestResolveNormalizesDotAndAtSeparators(t *testing.T) {
	m := pricing.LoadEmbedded()
	// Seed a known canonical short name so the test does not depend on a
	// specific snapshot version (the embedded snapshot ships
	// claude-sonnet-4-6 etc., but not a plain claude-sonnet-4).
	if err := m.LoadJSON([]byte(`{
        "claude-sonnet-4": {"input_cost_per_token": "0.000003"}
    }`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cases := []struct {
		in   string
		desc string
	}{
		{"claude-sonnet-4", "canonical name is not broken"},
		{"claude.sonnet.4", "dot-separator normalizes"},
		{"claude@sonnet@4", "at-separator normalizes"},
		{"anthropic.claude.sonnet.4", "vendor-prefixed dot form strips leading segment"},
		{"anthropic/claude.sonnet.4", "slash vendor + dotted model"},
	}
	for _, tc := range cases {
		p, ok := m.Resolve(tc.in)
		if !ok {
			t.Errorf("Resolve(%q) = false; want hit (%s)", tc.in, tc.desc)
			continue
		}
		if p.Input != 3e-6 {
			t.Errorf("Resolve(%q) input=%v; want 3e-6 (%s)", tc.in, p.Input, tc.desc)
		}
	}
}

// TestResolveBidirectionalContainsMatch pins ccusage's
// pricing_key_matches semantics (rust/crates/ccusage/src/pricing.rs:577-602):
// when no candidate lookup hits, the resolver scans every entry and
// matches on bidirectional boundary-aware substring containment
// (haystack contains needle, or vice versa), so a partial user-facing
// name like "claude-sonnet-4" still lands on the longer canonical
// "claude-sonnet-4-6" entry. The boundary check rejects spurious
// overlaps where the surrounding character is alphanumeric (digit-vs-
// digit version bumps like "sonnet-4-7" must NOT shadow "claude-sonnet-4-6").
//
// The test uses a freshly-seeded *pricing.Map (not LoadEmbedded) so the
// scan space stays small and predictable; the embedded snapshot already
// has many "claude-sonnet-4-*" variants that would obscure the
// longest-match selection.
func TestResolveBidirectionalContainsMatch(t *testing.T) {
	m := &pricing.Map{}
	if err := m.LoadJSON([]byte(`{
        "claude-sonnet-4-6": {"input_cost_per_token": "0.000003"},
        "claude-haiku-4-5": {"input_cost_per_token": "0.000001"},
        "unrelated-model":  {"input_cost_per_token": "0.000009"}
    }`)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	hit := func(t *testing.T, in, wantKey string) {
		t.Helper()
		got, ok := m.Resolve(in)
		if !ok {
			t.Errorf("Resolve(%q) = (_, false); want hit on %q", in, wantKey)
			return
		}
		want, _ := m.Lookup(wantKey)
		if got != want {
			t.Errorf("Resolve(%q) = %+v; want pricing of %q = %+v", in, got, wantKey, want)
		}
	}
	miss := func(t *testing.T, in string) {
		t.Helper()
		if p, ok := m.Resolve(in); ok {
			t.Errorf("Resolve(%q) = (%+v, true); want miss", in, p)
		}
	}

	t.Run("input shorter than entry resolves via entry-contains-input", func(t *testing.T) {
		hit(t, "claude-sonnet-4", "claude-sonnet-4-6")
		hit(t, "claude-sonnet", "claude-sonnet-4-6")
		hit(t, "claude-haiku", "claude-haiku-4-5")
	})
	t.Run("exact match still wins", func(t *testing.T) {
		hit(t, "claude-sonnet-4-6", "claude-sonnet-4-6")
		hit(t, "claude-haiku-4-5", "claude-haiku-4-5")
	})
	t.Run("boundary-violating overlap does not match", func(t *testing.T) {
		// "sonnet-4-7" is not a substring of any seeded entry and no
		// entry is a substring of "sonnet-4-7", so neither direction
		// of the bidirectional contains rule produces a hit. Critically,
		// the trailing -7 must not be quietly lumped onto "claude-sonnet-4-6"
		// — version-suffix mismatches must surface as a real miss.
		miss(t, "sonnet-4-7")
	})
	t.Run("unrelated model misses", func(t *testing.T) {
		miss(t, "totally-different-vendor")
	})
}

// TestResolveBedrockFormStillWorks pins that the normalization changes
// do not break existing Bedrock-style resolution. "us.anthropic.claude-…"
// keys exist verbatim in the snapshot with region-specific rates, so the
// exact-match path must still win over any normalization fallback.
func TestResolveBedrockFormStillWorks(t *testing.T) {
	m := pricing.LoadEmbedded()
	got, ok := m.Resolve("us.anthropic.claude-sonnet-4-6")
	if !ok {
		t.Fatal("Bedrock form us.anthropic.claude-sonnet-4-6 must still resolve")
	}
	exact, _ := m.Lookup("us.anthropic.claude-sonnet-4-6")
	if got != exact {
		t.Errorf("Bedrock Resolve must equal exact Lookup (no normalization shadowing)\n got=%+v\nwant=%+v", got, exact)
	}
}

// TestResolveBedrockJoinFromForUnknownPrefix keeps the existing
// "us.anthropic.<canonical>" → "<canonical>" fallback live for unknown
// regional prefixes the snapshot does not carry verbatim. The synthetic
// region "xx" exercises the Bedrock joinFrom path.
func TestResolveBedrockJoinFromForUnknownPrefix(t *testing.T) {
	m := pricing.LoadEmbedded()
	got, ok := m.Resolve("xx.anthropic.claude-sonnet-4-6")
	if !ok {
		t.Fatal("Unknown-region Bedrock alias must fall back to canonical key")
	}
	want, _ := m.Lookup("claude-sonnet-4-6")
	if got != want {
		t.Errorf("Bedrock joinFrom fallback differs from canonical lookup\n got=%+v\nwant=%+v", got, want)
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
