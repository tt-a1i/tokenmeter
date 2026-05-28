// Package pricing loads model price tables from a LiteLLM snapshot embedded
// at build time. See docs/superpowers/specs/2026-05-20-ccusage-aligned-refactor-design.md
// §4.4. Online refresh lives in refresh.go.
package pricing

//go:generate go run ../../scripts/refresh-pricing

import (
	_ "embed"
	"encoding/json"
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

// Resolve returns pricing for a model name, attempting (in order):
//  1. exact match
//  2. provider-stripped match: "anthropic/claude-sonnet-4-6" → "claude-sonnet-4-6"
//  3. region-prefixed Bedrock IDs: "us.anthropic.claude-sonnet-4-6" → "claude-sonnet-4-6"
//
// Returns ok=false if no candidate matches.
func (m *Map) Resolve(model string) (Pricing, bool) {
	if p, ok := m.entries[model]; ok {
		return p, true
	}
	candidates := []string{}
	for _, sep := range []string{"/", "."} {
		if i := lastIndex(model, sep); i >= 0 {
			candidates = append(candidates, model[i+1:])
		}
	}
	// Bedrock-style: "us.anthropic.claude-…"
	if parts := splitAll(model, '.'); len(parts) >= 3 {
		candidates = append(candidates, joinFrom(parts, 2, '-'))
	}
	for _, c := range candidates {
		if p, ok := m.entries[c]; ok {
			return p, true
		}
	}
	return Pricing{}, false
}

func lastIndex(s, sep string) int {
	idx := -1
	for i := range s {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			idx = i
		}
	}
	return idx
}

func splitAll(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func joinFrom(parts []string, from int, sep byte) string {
	if from >= len(parts) {
		return ""
	}
	var b []byte
	for i := from; i < len(parts); i++ {
		if i > from {
			b = append(b, sep)
		}
		b = append(b, parts[i]...)
	}
	return string(b)
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
	if i := lastIndex(normalized, "/"); i >= 0 {
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
