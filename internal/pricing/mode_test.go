package pricing_test

import (
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

func TestModeAutoPrefersDisplayWhenCostKnown(t *testing.T) {
	got := pricing.Apply(pricing.ModeAuto, 0.05, pricing.Pricing{Input: 1, Output: 1}, pricing.Usage{Input: 100, Output: 100}, pricing.SpeedStandard)
	if got != 0.05 {
		t.Fatalf("auto with known cost must trust display, got %f", got)
	}
}

func TestModeAutoFallsBackToCalculate(t *testing.T) {
	got := pricing.Apply(pricing.ModeAuto, 0, pricing.Pricing{Input: 0.1, Output: 0.1}, pricing.Usage{Input: 100, Output: 100}, pricing.SpeedStandard)
	if got != 20 {
		t.Fatalf("auto with zero display must recompute = 200 * 0.1 = 20, got %f", got)
	}
}

func TestModeDisplayHonorsZeroForCodex(t *testing.T) {
	// Codex rows lack costUSD; display mode returns 0 by design.
	got := pricing.Apply(pricing.ModeDisplay, 0, pricing.Pricing{Input: 5e-6, Output: 2e-5}, pricing.Usage{Input: 1000, Output: 500}, pricing.SpeedStandard)
	if got != 0 {
		t.Fatalf("display mode must return 0 when display value is 0, got %f", got)
	}
}

func TestModeCalculateIgnoresDisplay(t *testing.T) {
	got := pricing.Apply(pricing.ModeCalculate, 999, pricing.Pricing{Input: 1, Output: 1}, pricing.Usage{Input: 10, Output: 10}, pricing.SpeedStandard)
	if got != 20 {
		t.Fatalf("calculate mode must recompute regardless of display, got %f", got)
	}
}
