package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tt-a1i/tokenmeter/internal/collector"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

func ConfigureRuntimePricing(ctx context.Context, offline bool, configPath string) error {
	m, err := pricing.LoadRuntime(ctx, pricing.RuntimeOptions{Offline: offline, ConfigPath: configPath})
	if err != nil {
		return err
	}
	pricingMap = m
	return nil
}

func RunPricingRefresh(ctx context.Context, out io.Writer, offline bool, configPath string) error {
	m, err := pricing.RefreshRuntime(ctx, pricing.RuntimeOptions{Offline: offline, ConfigPath: configPath})
	if err != nil {
		return err
	}
	pricingMap = m
	fmt.Fprintln(out, "pricing cache refreshed")
	return nil
}

func SetCodexSpeedMode(mode string) error {
	return collector.SetCodexSpeedMode(mode)
}

func speedForModel(model string) pricing.Speed {
	if !looksLikeCodexModel(model) {
		return pricing.SpeedStandard
	}
	if collector.ResolveCodexPricingSpeed() == collector.CodexSpeedFast {
		return pricing.SpeedFast
	}
	return pricing.SpeedStandard
}

func looksLikeCodexModel(model string) bool {
	model = strings.ToLower(model)
	return strings.Contains(model, "gpt-") ||
		strings.Contains(model, "openai/") ||
		strings.Contains(model, "azure/")
}
