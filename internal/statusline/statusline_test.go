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

func TestLoadConfigMissingReturnsZero(t *testing.T) {
	// Use a temp dir that we know is empty.
	dir := t.TempDir()
	cfg, err := statusline.LoadConfig(dir + "/missing.json")
	if err != nil {
		t.Fatalf("LoadConfig missing: %v", err)
	}
	if cfg.QuotaUSD != 0 || cfg.Color {
		t.Fatalf("missing config must default to zero, got %+v", cfg)
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
