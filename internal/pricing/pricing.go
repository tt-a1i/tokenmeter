// Package pricing loads model price tables from a LiteLLM snapshot embedded
// at build time. See docs/superpowers/specs/2026-05-20-ccusage-aligned-refactor-design.md
// §4.4. Online refresh lives in refresh.go.
package pricing

//go:generate go run ../../scripts/refresh-pricing

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed litellm-snapshot.json
var embeddedSnapshot []byte

// Pricing holds tariff fields for one model.
type Pricing struct {
	Input                float64
	Output               float64
	CacheCreate          float64
	CacheRead            float64
	InputAbove200K       *float64
	OutputAbove200K      *float64
	CacheCreateAbove200K *float64
	CacheReadAbove200K   *float64
	FastMultiplier       float64 // 1.0 if not set
	MaxInputTokens       int64
}

// Map is a model-name → pricing lookup.
type Map struct {
	entries map[string]Pricing
}

// LoadEmbedded parses the build-time snapshot. Panics on malformed JSON to
// surface build issues during tests; in production the snapshot is committed
// and CI runs the test suite before release.
func LoadEmbedded() *Map {
	m := &Map{entries: map[string]Pricing{}}
	if err := m.LoadJSON(embeddedSnapshot); err != nil {
		panic("pricing: embedded snapshot invalid: " + err.Error())
	}
	seedBuiltinPrices(m)
	return m
}

// seedBuiltinPrices fills in prices the LiteLLM snapshot does not carry
// (the refresh-pricing script filters by provider prefix, and moonshot/
// is not in that allow-list). Mirrors ccusage's built-in Moonshot table
// in rust/crates/ccusage/src/pricing.rs:430-461. Existing entries from
// the embedded snapshot or a runtime LoadJSON merge are preserved so
// LiteLLM data (when available) wins over the hard-coded fallback.
func seedBuiltinPrices(m *Map) {
	if _, ok := m.entries["moonshot/kimi-k2.5"]; !ok {
		m.entries["moonshot/kimi-k2.5"] = Pricing{
			Input:          0.6e-6,
			Output:         3e-6,
			CacheCreate:    0.75e-6,
			CacheRead:      0.1e-6,
			FastMultiplier: 1.0,
		}
	}
	if _, ok := m.entries["moonshot/kimi-k2.6"]; !ok {
		m.entries["moonshot/kimi-k2.6"] = Pricing{
			Input:          0.95e-6,
			Output:         4e-6,
			CacheCreate:    1.1875e-6,
			CacheRead:      0.16e-6,
			FastMultiplier: 1.0,
		}
	}
}

// LoadJSON merges entries from a raw LiteLLM-style JSON blob. Unknown fields
// are ignored. Returns the first parse error.
func (m *Map) LoadJSON(data []byte) error {
	if m.entries == nil {
		m.entries = map[string]Pricing{}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for model, entry := range raw {
		if model == "sample_spec" {
			continue
		}
		var r liteLLMEntry
		if err := json.Unmarshal(entry, &r); err != nil {
			return err
		}
		p := Pricing{
			Input:                ptrOrZero(r.InputCostPerToken),
			Output:               ptrOrZero(r.OutputCostPerToken),
			CacheCreate:          ptrOrZero(r.CacheCreationInputTokenCost),
			CacheRead:            ptrOrZero(r.CacheReadInputTokenCost),
			InputAbove200K:       ptrOrFloat(r.InputCostPerTokenAbove200K),
			OutputAbove200K:      ptrOrFloat(r.OutputCostPerTokenAbove200K),
			CacheCreateAbove200K: ptrOrFloat(r.CacheCreationInputTokenCostAbove200K),
			CacheReadAbove200K:   ptrOrFloat(r.CacheReadInputTokenCostAbove200K),
			MaxInputTokens:       ptrOrZeroInt(r.MaxInputTokens),
			FastMultiplier:       1.0,
		}
		if r.ProviderSpecificEntry != nil && r.ProviderSpecificEntry.Fast != nil {
			p.FastMultiplier = float64(*r.ProviderSpecificEntry.Fast)
		} else if override := fastMultiplierOverride(model); override != 0 {
			p.FastMultiplier = override
		}
		m.entries[model] = p
	}
	return nil
}

// Lookup returns pricing for an exact model name. Use Resolve for prefix/
// alias matching (Task 8).
func (m *Map) Lookup(model string) (Pricing, bool) {
	p, ok := m.entries[model]
	return p, ok
}

// Resolve returns pricing for a model name. After the exact-match miss
// it generates a candidate set, tries each by length descending so the
// longest plausible match wins, and returns the first hit. Candidates
// cover, in roughly this order:
//
//  1. Exact match.
//  2. Provider-strip — substring after the last "/" or "." in the input
//     ("anthropic/claude-sonnet-4-6" → "claude-sonnet-4-6").
//  3. Dot/at-separator normalization — replaces "." and "@" with "-"
//     ("claude.sonnet.4" → "claude-sonnet-4"), mirroring ccusage v20's
//     normalized_pricing_key (rust/crates/ccusage/src/pricing.rs:604-610).
//  4. Leading-segment drops on the normalized form so vendor-prefixed
//     dotted aliases land on the canonical key without an O(N) scan
//     ("anthropic.claude.sonnet.4" → "claude-sonnet-4").
//  5. Region-prefixed Bedrock joinFrom — keeps the existing
//     "us.anthropic.claude-…" → "claude-…" fallback for unknown regions
//     when the verbatim "<region>.anthropic.<model>" key is absent.
//
// Returns ok=false if no candidate matches any entry.
func (m *Map) Resolve(model string) (Pricing, bool) {
	if p, ok := m.entries[model]; ok {
		return p, true
	}

	seen := map[string]bool{model: true}
	var candidates []string
	normalizer := strings.NewReplacer(".", "-", "@", "-")
	add := func(c string) {
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		candidates = append(candidates, c)
		// Mirror ccusage's normalize-then-match by also seeding the
		// dot/at-stripped form of every candidate. This catches inputs
		// like "anthropic/claude.sonnet.4" where the after-"/" suffix
		// still carries dotted separators.
		if norm := normalizer.Replace(c); norm != c {
			if !seen[norm] {
				seen[norm] = true
				candidates = append(candidates, norm)
			}
		}
	}

	for _, sep := range []string{"/", "."} {
		if i := strings.LastIndex(model, sep); i >= 0 {
			add(model[i+1:])
		}
	}

	normalized := normalizer.Replace(model)
	if normalized != model {
		add(normalized)
	}

	// Drop leading hyphen-separated segments on the normalized form so a
	// vendor prefix collapses cleanly: "anthropic-claude-sonnet-4" →
	// "claude-sonnet-4" → "sonnet-4" → "4". Each shorter form joins the
	// candidate pool; the length-desc sort below picks the longest hit.
	if dashParts := strings.Split(normalized, "-"); len(dashParts) > 1 {
		for i := 1; i < len(dashParts); i++ {
			add(strings.Join(dashParts[i:], "-"))
		}
	}

	// Bedrock-style fallback: "us.anthropic.claude-…" → "claude-…"
	if parts := strings.Split(model, "."); len(parts) >= 3 {
		add(strings.Join(parts[2:], "-"))
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})

	for _, c := range candidates {
		if p, ok := m.entries[c]; ok {
			return p, true
		}
	}

	// Fallback: bidirectional boundary-aware contains scan, mirroring
	// ccusage's pricing_key_matches (rust/crates/ccusage/src/pricing.rs:577-602).
	// Lets a user-facing short alias like "claude-sonnet-4" land on the
	// longer canonical "claude-sonnet-4-6" entry without enumerating every
	// possible version suffix in the candidate set. Length-then-lex
	// tie-breaking picks the most specific entry on the rare case where
	// multiple keys contain the input.
	return m.bidirectionalContainsMatch(model, normalizer)
}

// bidirectionalContainsMatch is the O(N) fallback for Resolve. Returns
// the pricing of the longest entry key whose normalized form contains
// the normalized input as a boundary-aware substring, or vice versa.
// Both sides are lowercased so the matcher is robust to occasional
// upper-case model strings without forcing the snapshot to be lowercase.
func (m *Map) bidirectionalContainsMatch(model string, normalizer *strings.Replacer) (Pricing, bool) {
	inputNorm := strings.ToLower(normalizer.Replace(model))
	if inputNorm == "" {
		return Pricing{}, false
	}
	var bestKey string
	for key := range m.entries {
		keyNorm := strings.ToLower(normalizer.Replace(key))
		if keyNorm == "" {
			continue
		}
		if !containsWithBoundary(keyNorm, inputNorm) && !containsWithBoundary(inputNorm, keyNorm) {
			continue
		}
		if len(key) > len(bestKey) || (len(key) == len(bestKey) && key > bestKey) {
			bestKey = key
		}
	}
	if bestKey == "" {
		return Pricing{}, false
	}
	return m.entries[bestKey], true
}

// containsWithBoundary reports whether needle occurs in haystack with
// both sides bounded by either the string edge or a non-alphanumeric
// byte. This is the boundary semantics from ccusage's
// is_pricing_key_boundary / contains_pricing_key in
// rust/crates/ccusage/src/pricing.rs:587-601: alphanumerics adjacent to
// the match are not boundaries, so "sonnet-4" does not match into
// "sonnet-4-5" without an explicit separator before the 4 (it does, the
// "-" before 4 is the boundary), but "claude" alone won't bleed into
// "claudex-…" if such a key existed.
func containsWithBoundary(haystack, needle string) bool {
	if needle == "" || len(needle) > len(haystack) {
		return false
	}
	for start := 0; ; {
		idx := strings.Index(haystack[start:], needle)
		if idx < 0 {
			return false
		}
		idx += start
		beforeOK := idx == 0 || !isPricingKeyAlnum(haystack[idx-1])
		end := idx + len(needle)
		afterOK := end == len(haystack) || !isPricingKeyAlnum(haystack[end])
		if beforeOK && afterOK {
			return true
		}
		start = idx + 1
	}
}

func isPricingKeyAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

type liteLLMEntry struct {
	InputCostPerToken                    *stringOrFloat         `json:"input_cost_per_token"`
	OutputCostPerToken                   *stringOrFloat         `json:"output_cost_per_token"`
	CacheCreationInputTokenCost          *stringOrFloat         `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost              *stringOrFloat         `json:"cache_read_input_token_cost"`
	InputCostPerTokenAbove200K           *stringOrFloat         `json:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200K          *stringOrFloat         `json:"output_cost_per_token_above_200k_tokens"`
	CacheCreationInputTokenCostAbove200K *stringOrFloat         `json:"cache_creation_input_token_cost_above_200k_tokens"`
	CacheReadInputTokenCostAbove200K     *stringOrFloat         `json:"cache_read_input_token_cost_above_200k_tokens"`
	MaxInputTokens                       *stringOrInt           `json:"max_input_tokens"`
	MaxOutputTokens                      *stringOrInt           `json:"max_output_tokens"`
	MaxTokens                            *stringOrInt           `json:"max_tokens"`
	ProviderSpecificEntry                *providerSpecificEntry `json:"provider_specific_entry"`
}

type providerSpecificEntry struct {
	Fast *stringOrFloat `json:"fast"`
}

func fastMultiplierOverride(model string) float64 {
	switch model {
	case "gpt-5.5":
		return 2.5
	case "gpt-5.4", "gpt-5.3-codex":
		return 2.0
	}
	normalized := model
	if i := strings.LastIndex(normalized, "/"); i >= 0 {
		normalized = normalized[i+1:]
	}
	for _, prefix := range []string{"us.", "eu.", "global.", "jp.", "au."} {
		if len(normalized) > len(prefix) && normalized[:len(prefix)] == prefix {
			normalized = normalized[len(prefix):]
			break
		}
	}
	if normalized == "anthropic.claude-opus-4-6" || normalized == "anthropic.claude-opus-4-7" ||
		normalized == "claude-opus-4-6" || normalized == "claude-opus-4-7" {
		return 6.0
	}
	return 0
}

func ptrOrZero(p *stringOrFloat) float64 {
	if p == nil {
		return 0
	}
	return float64(*p)
}

func ptrOrFloat(p *stringOrFloat) *float64 {
	if p == nil {
		return nil
	}
	v := float64(*p)
	return &v
}

func ptrOrZeroInt(p *stringOrInt) int64 {
	if p == nil {
		return 0
	}
	return int64(*p)
}
