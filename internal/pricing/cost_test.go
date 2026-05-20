package pricing_test

import (
	"math"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

func TestCalculateCostBasic(t *testing.T) {
	m := pricing.LoadEmbedded()
	p, _ := m.Resolve("claude-sonnet-4-6")
	c := pricing.CalculateCost(p, pricing.Usage{
		Input: 1000, Output: 500,
	}, pricing.SpeedStandard)
	// 1000 * 3e-6 + 500 * 1.5e-5 = 0.003 + 0.0075 = 0.0105
	if !approx(c, 0.0105) {
		t.Fatalf("cost=%f want ~0.0105", c)
	}
}

func TestCalculateCostFastMultiplier(t *testing.T) {
	// gpt-5 has no fast multiplier upstream; claude-opus-4-6 ships fast=6.
	m := pricing.LoadEmbedded()
	p, _ := m.Resolve("claude-opus-4-6")
	std := pricing.CalculateCost(p, pricing.Usage{Input: 100, Output: 50}, pricing.SpeedStandard)
	fast := pricing.CalculateCost(p, pricing.Usage{Input: 100, Output: 50}, pricing.SpeedFast)
	if !approx(fast, std*6) {
		t.Fatalf("fast cost %f must be 6x standard %f", fast, std)
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
