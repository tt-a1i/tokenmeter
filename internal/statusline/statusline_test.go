package statusline_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
)

func TestRenderCompactNoConfigNoColor(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	now := start.Add(30 * time.Minute)
	end := start.Add(5 * time.Hour)
	last := now
	block := &blocks.SessionBlock{
		StartTime: start, EndTime: end, ActualEnd: &last, IsActive: true,
		Tokens:     blocks.TokenCounts{Input: 60000, Output: 30000},
		Cost:       1.50,
		BurnRate:   &blocks.BurnRate{TokensPerMinute: 3000, CostPerHour: 3.0},
		Projection: &blocks.Projection{TotalTokens: 900000, TotalCost: 15.0, RemainingTime: 4*time.Hour + 30*time.Minute},
		Models:     []string{"claude-sonnet-4-6"},
	}
	in := statusline.Input{ModelID: "claude-sonnet-4-6"}
	var buf bytes.Buffer
	if err := statusline.Render(&buf, in, block, statusline.Config{}, now); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "sonnet") {
		t.Fatalf("output must include model short name, got %q", got)
	}
	if !strings.Contains(got, "$1.50") && !strings.Contains(got, "$1.5") {
		t.Fatalf("output must include current cost, got %q", got)
	}
}

func TestRenderHandlesNilBlock(t *testing.T) {
	var buf bytes.Buffer
	if err := statusline.Render(&buf, statusline.Input{ModelID: "claude-sonnet-4-6"}, nil, statusline.Config{}, time.Now()); err != nil {
		t.Fatalf("Render with nil block: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("must still produce output when block is nil")
	}
}

func TestLoadConfigMissingReturnsDefaults(t *testing.T) {
	// Use a temp dir that we know is empty.
	dir := t.TempDir()
	cfg, err := statusline.LoadConfig(dir + "/missing.json")
	if err != nil {
		t.Fatalf("LoadConfig missing: %v", err)
	}
	if cfg.QuotaUSD != 0 || cfg.Color || cfg.ContextLowThreshold != 50 || cfg.ContextMediumThreshold != 80 || cfg.BurnRateDisplay != "emoji" {
		t.Fatalf("missing config must default statusline knobs, got %+v", cfg)
	}
}

func TestLoadConfigParsesQuota(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/statusline.json"
	if err := os.WriteFile(path, []byte(`{"quota_usd":30.0,"color":true,"format":"compact"}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := statusline.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.QuotaUSD != 30.0 || !cfg.Color {
		t.Fatalf("LoadConfig parse mismatch: %+v", cfg)
	}
}

func TestRenderColorsContextByThresholdBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		input      int64
		wantEscape string
	}{
		{name: "below low is green", input: 49_000, wantEscape: "\x1b[32m49%\x1b[0m"},
		{name: "at low is yellow", input: 50_000, wantEscape: "\x1b[33m50%\x1b[0m"},
		{name: "at medium is red", input: 80_000, wantEscape: "\x1b[31m80%\x1b[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			in := statusline.Input{
				ModelID: "claude-sonnet-4-6",
				ContextWindow: &statusline.ContextWindow{
					TotalInputTokens:  tt.input,
					ContextWindowSize: 100_000,
				},
			}
			cfg := statusline.Config{Color: true, ContextLowThreshold: 50, ContextMediumThreshold: 80, BurnRateDisplay: "off"}
			if err := statusline.Render(&buf, in, nil, cfg, time.Now()); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(buf.String(), tt.wantEscape) {
				t.Fatalf("context color mismatch, want %q got:\n%q", tt.wantEscape, buf.String())
			}
		})
	}
}

func TestRenderBurnRateDisplayModes(t *testing.T) {
	tests := []struct {
		mode    string
		want    []string
		notWant []string
	}{
		{mode: "off", notWant: []string{"🔥", "High"}},
		{mode: "emoji", want: []string{"🔥"}, notWant: []string{"High"}},
		{mode: "text", want: []string{"High"}, notWant: []string{"🔥"}},
		{mode: "emoji-text", want: []string{"🔥", "High"}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			start := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
			last := start.Add(30 * time.Minute)
			block := &blocks.SessionBlock{
				StartTime: start, EndTime: start.Add(5 * time.Hour), ActualEnd: &last, IsActive: true,
				Tokens:     blocks.TokenCounts{Input: 100, Output: 50},
				Cost:       1,
				BurnRate:   &blocks.BurnRate{TokensPerMinute: 6_000, CostPerHour: 4},
				Projection: &blocks.Projection{RemainingTime: 4 * time.Hour},
			}
			var buf bytes.Buffer
			cfg := statusline.Config{BurnRateDisplay: tt.mode}
			if err := statusline.Render(&buf, statusline.Input{ModelID: "claude-sonnet-4-6"}, block, cfg, start); err != nil {
				t.Fatalf("Render: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(buf.String(), want) {
					t.Fatalf("mode %s missing %q:\n%s", tt.mode, want, buf.String())
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(buf.String(), notWant) {
					t.Fatalf("mode %s should omit %q:\n%s", tt.mode, notWant, buf.String())
				}
			}
		})
	}
}
