// Package pricing loads model price tables from a LiteLLM snapshot embedded
// at build time. See docs/superpowers/specs/2026-05-20-ccusage-aligned-refactor-design.md
// §4.4. Online refresh lives in refresh.go.
package pricing

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
	return m
}

// LoadJSON merges entries from a raw LiteLLM-style JSON blob. Unknown fields
// are ignored. Returns the first parse error.
func (m *Map) LoadJSON(data []byte) error {
	if m.entries == nil {
		m.entries = map[string]Pricing{}
	}
	var raw map[string]liteLLMEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for model, r := range raw {
		p := Pricing{
			Input:                ptrOrZero(r.InputCostPerToken),
			Output:               ptrOrZero(r.OutputCostPerToken),
			CacheCreate:          ptrOrZero(r.CacheCreationInputTokenCost),
			CacheRead:            ptrOrZero(r.CacheReadInputTokenCost),
			InputAbove200K:       r.InputCostPerTokenAbove200K,
			OutputAbove200K:      r.OutputCostPerTokenAbove200K,
			CacheCreateAbove200K: r.CacheCreationInputTokenCostAbove200K,
			CacheReadAbove200K:   r.CacheReadInputTokenCostAbove200K,
			MaxInputTokens:       ptrOrZeroInt(r.MaxInputTokens),
			FastMultiplier:       1.0,
		}
		if r.ProviderSpecificEntry != nil && r.ProviderSpecificEntry.Fast != nil {
			p.FastMultiplier = *r.ProviderSpecificEntry.Fast
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

type liteLLMEntry struct {
	InputCostPerToken                    *float64               `json:"input_cost_per_token"`
	OutputCostPerToken                   *float64               `json:"output_cost_per_token"`
	CacheCreationInputTokenCost          *float64               `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost              *float64               `json:"cache_read_input_token_cost"`
	InputCostPerTokenAbove200K           *float64               `json:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200K          *float64               `json:"output_cost_per_token_above_200k_tokens"`
	CacheCreationInputTokenCostAbove200K *float64               `json:"cache_creation_input_token_cost_above_200k_tokens"`
	CacheReadInputTokenCostAbove200K     *float64               `json:"cache_read_input_token_cost_above_200k_tokens"`
	MaxInputTokens                       *int64                 `json:"max_input_tokens"`
	ProviderSpecificEntry                *providerSpecificEntry `json:"provider_specific_entry"`
}

type providerSpecificEntry struct {
	Fast *float64 `json:"fast"`
}

func ptrOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func ptrOrZeroInt(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
