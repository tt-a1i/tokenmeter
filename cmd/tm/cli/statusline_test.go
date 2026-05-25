package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

type stubStatuslineLoader struct{ active *blocks.SessionBlock }

func (s stubStatuslineLoader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	return s.active, nil
}

func TestRunStatuslineSmoke(t *testing.T) {
	var out bytes.Buffer
	loader := stubStatuslineLoader{active: nil}
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-sonnet-4-6","session_id":"s","cwd":"/x","transcript_path":""}`),
		&out, loader, "/tmp/no-such.json", time.Now())
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("must produce output")
	}
}

func TestRunStatuslineNoColor(t *testing.T) {
	// Build a block whose projection exceeds the quota so the statusline
	// renderer would normally call colorize() and emit an ANSI ESC. With
	// --no-color wired through StatuslineOptions, the ESC must be suppressed.
	now := mustTime("2026-05-19T12:00:00Z")
	start := now.Add(-30 * time.Minute)
	last := now
	proj := blocks.Projection{TotalCost: 100}
	block := &blocks.SessionBlock{
		StartTime:  start,
		EndTime:    start.Add(5 * time.Hour),
		ActualEnd:  &last,
		IsActive:   true,
		Tokens:     blocks.TokenCounts{Input: 60000, Output: 30000},
		Cost:       1.50,
		Models:     []string{"claude-opus-4-7"},
		Projection: &proj,
	}
	cfgPath := filepath.Join(t.TempDir(), "statusline.json")
	if err := os.WriteFile(cfgPath, []byte(`{"quota_usd":50.0,"color":true,"format":"compact"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":""}`),
		&buf, stubStatuslineLoader{active: block}, cfgPath, now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoColor: true, Mode: "auto"}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte{0x1b}) {
		t.Fatalf("--no-color should suppress ANSI escape, got:\n%q", buf.String())
	}
}

func TestRunStatuslineUsesUnifiedConfigBeforeLegacy(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)
	dir := t.TempDir()
	legacy := filepath.Join(dir, "statusline.json")
	if err := os.WriteFile(legacy, []byte(`{"quota_usd":10,"color":true,"burn_rate_display":"text"}`), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	unified := filepath.Join(dir, "tokenmeter.json")
	if err := os.WriteFile(unified, []byte(`{"statusline":{"quota_usd":50,"color":"true","burn_rate_display":"off"}}`), 0o644); err != nil {
		t.Fatalf("write unified: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":""}`),
		&buf, stubStatuslineLoader{active: block}, legacy, now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{ConfigPath: unified}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("$1.50/$50")) || bytes.Contains(buf.Bytes(), []byte("High")) {
		t.Fatalf("unified config should override legacy quota and burn display, got:\n%q", buf.String())
	}
}

func TestRunStatuslineFallsBackToLegacyConfig(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)
	legacy := filepath.Join(t.TempDir(), "statusline.json")
	if err := os.WriteFile(legacy, []byte(`{"quota_usd":30,"burn_rate_display":"text"}`), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":""}`),
		&buf, stubStatuslineLoader{active: block}, legacy, now)
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("$1.50/$30")) || !bytes.Contains(buf.Bytes(), []byte("High")) {
		t.Fatalf("legacy config should be used as fallback, got:\n%q", buf.String())
	}
}

func TestRunStatuslineCLIFlagsOverrideUnifiedAndLegacy(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)
	dir := t.TempDir()
	legacy := filepath.Join(dir, "statusline.json")
	if err := os.WriteFile(legacy, []byte(`{"burn_rate_display":"text"}`), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	unified := filepath.Join(dir, "tokenmeter.json")
	if err := os.WriteFile(unified, []byte(`{"statusline":{"burn_rate_display":"off","context_low_threshold":10,"context_medium_threshold":20}}`), 0o644); err != nil {
		t.Fatalf("write unified: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":"","context_window":{"total_input_tokens":15000,"context_window_size":100000}}`),
		&buf, stubStatuslineLoader{active: block}, legacy, now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{
			ConfigPath:             unified,
			BurnRateDisplay:        "emoji",
			ContextLowThreshold:    50,
			ContextMediumThreshold: 80,
		}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("🔥")) || !bytes.Contains(buf.Bytes(), []byte("15%")) {
		t.Fatalf("CLI flags should override config, got:\n%q", buf.String())
	}
}

func TestRunStatuslinePartialUnifiedMergesDefaults(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)
	unified := filepath.Join(t.TempDir(), "tokenmeter.json")
	if err := os.WriteFile(unified, []byte(`{"statusline":{"quota_usd":40}}`), 0o644); err != nil {
		t.Fatalf("write unified: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":""}`),
		&buf, stubStatuslineLoader{active: block}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{ConfigPath: unified}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("$1.50/$40")) || !bytes.Contains(buf.Bytes(), []byte("🔥")) {
		t.Fatalf("partial unified config should merge defaults, got:\n%q", buf.String())
	}
}

func activeStatuslineBlock(now time.Time) *blocks.SessionBlock {
	start := now.Add(-30 * time.Minute)
	last := now
	return &blocks.SessionBlock{
		StartTime: start,
		EndTime:   start.Add(5 * time.Hour),
		ActualEnd: &last,
		IsActive:  true,
		Tokens:    blocks.TokenCounts{Input: 60000, Output: 30000},
		Cost:      1.50,
		Models:    []string{"claude-opus-4-7"},
		BurnRate:  &blocks.BurnRate{TokensPerMinute: 6000, CostPerHour: 4},
	}
}
