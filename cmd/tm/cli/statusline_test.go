package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

type stubStatuslineLoader struct{ active *blocks.SessionBlock }

func (s stubStatuslineLoader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	return s.active, nil
}

type metricsStatuslineLoader struct {
	active      *blocks.SessionBlock
	sessionCost float64
	found       bool
	todayCost   float64
}

func (s metricsStatuslineLoader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	return s.active, nil
}

func (s metricsStatuslineLoader) LoadSessionCost(_ context.Context, _ string) (float64, bool, error) {
	return s.sessionCost, s.found, nil
}

func (s metricsStatuslineLoader) LoadTodayCost(_ context.Context, _ time.Time, _ *time.Location) (float64, error) {
	return s.todayCost, nil
}

type countingStatuslineLoader struct {
	blocks []*blocks.SessionBlock
	calls  int
}

func (s *countingStatuslineLoader) LoadActive(_ context.Context) (*blocks.SessionBlock, error) {
	if len(s.blocks) == 0 {
		s.calls++
		return nil, nil
	}
	idx := s.calls
	if idx >= len(s.blocks) {
		idx = len(s.blocks) - 1
	}
	s.calls++
	return s.blocks[idx], nil
}

func TestRunStatuslineSmoke(t *testing.T) {
	var out bytes.Buffer
	loader := stubStatuslineLoader{active: nil}
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-sonnet-4-6","session_id":"s","cwd":"/x","transcript_path":""}`),
		&out, loader, "/tmp/no-such.json", time.Now(),
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true}))
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
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoColor: true, NoCache: true, Mode: "auto"}))
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
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, ConfigPath: unified}))
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
		&buf, stubStatuslineLoader{active: block}, legacy, now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true}))
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
			NoCache:                true,
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
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, ConfigPath: unified}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("$1.50/$40")) || bytes.Contains(buf.Bytes(), []byte("🔥")) {
		t.Fatalf("partial unified config should merge defaults, got:\n%q", buf.String())
	}
}

func TestRunStatuslineCostSourceControlsRecalculation(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)

	var ccusage bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":"","cost":{"total_cost_usd":9.99}}`),
		&ccusage, metricsStatuslineLoader{active: block, sessionCost: 2.25, found: true, todayCost: 5.50}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, CostSource: "ccusage"}))
	if err != nil {
		t.Fatalf("RunStatusline ccusage: %v", err)
	}
	if !strings.Contains(ccusage.String(), "session $2.25") || !strings.Contains(ccusage.String(), "today $5.50") {
		t.Fatalf("cost-source=ccusage should use calculated session/today costs, got:\n%q", ccusage.String())
	}

	block = activeStatuslineBlock(now)
	var cc bytes.Buffer
	err = cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":"","cost":{"total_cost_usd":9.99}}`),
		&cc, metricsStatuslineLoader{active: block, sessionCost: 2.25, found: true, todayCost: 5.50}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, CostSource: "cc"}))
	if err != nil {
		t.Fatalf("RunStatusline cc: %v", err)
	}
	if !strings.Contains(cc.String(), "session $9.99") {
		t.Fatalf("cost-source=cc should use hook cost, got:\n%q", cc.String())
	}

	var both bytes.Buffer
	err = cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":"","cost":{"total_cost_usd":9.99}}`),
		&both, metricsStatuslineLoader{active: block, sessionCost: 2.25, found: true, todayCost: 5.50}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, CostSource: "both"}))
	if err != nil {
		t.Fatalf("RunStatusline both: %v", err)
	}
	if !strings.Contains(both.String(), "session $9.99 cc / $2.25 ccusage") {
		t.Fatalf("cost-source=both should show hook and calculated costs, got:\n%q", both.String())
	}
}

func TestRunStatuslineExplicitModeOverridesCostSource(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	block := activeStatuslineBlock(now)
	block.Cost = 0

	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-opus-4-7","session_id":"s","cwd":"/x","transcript_path":""}`),
		&buf, stubStatuslineLoader{active: block}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
		cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, Mode: "display", ModeSet: true, CostSource: "ccusage"}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("$0.00")) {
		t.Fatalf("explicit mode=display should override cost-source=ccusage, got:\n%q", buf.String())
	}
}

func TestRunStatuslineVisualBurnRateTextAcceptsNestedAndFlatModelInput(t *testing.T) {
	now := mustTime("2026-05-19T12:00:00Z")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "nested model",
			input: `{"model":{"id":"claude-sonnet-4-6","display_name":"Claude Sonnet 4.6"},"session_id":"s","cwd":"/x","transcript_path":""}`,
			want:  "sonnet",
		},
		{
			name:  "flat model_id",
			input: `{"model_id":"gpt-5-codex","session_id":"s","cwd":"/x","transcript_path":""}`,
			want:  "gpt-5-codex",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := activeStatuslineBlock(now)
			var buf bytes.Buffer
			err := cli.RunStatusline(context.Background(),
				bytes.NewBufferString(tt.input),
				&buf, stubStatuslineLoader{active: block}, filepath.Join(t.TempDir(), "missing-statusline.json"), now,
				cli.WithStatuslineOptions(cli.StatuslineOptions{NoCache: true, BurnRateDisplay: "text"}))
			if err != nil {
				t.Fatalf("RunStatusline: %v", err)
			}
			out := buf.String()
			if !strings.Contains(strings.ToLower(out), strings.ToLower(tt.want)) || !strings.Contains(out, "High") {
				t.Fatalf("statusline output missing model/burn text, got %q", out)
			}
			if strings.Contains(out, "🔥") {
				t.Fatalf("--visual-burn-rate text should not emit emoji, got %q", out)
			}
		})
	}
}

func TestRunStatuslineCacheFlagsAndDebug(t *testing.T) {
	t.Setenv("TOKENMETER_HOME", t.TempDir())
	now := mustTime("2026-05-19T12:00:00Z")
	input := `{"model_id":"claude-opus-4-7","session_id":"cache-cli","cwd":"/x","transcript_path":""}`
	loader := &countingStatuslineLoader{blocks: []*blocks.SessionBlock{
		activeStatuslineBlock(now),
		activeStatuslineBlock(now),
	}}
	loader.blocks[1].Cost = 9.99

	var first, second, debug bytes.Buffer
	opts := cli.StatuslineOptions{RefreshInterval: 60, Debug: true, DebugWriter: &debug}
	if err := cli.RunStatusline(context.Background(), bytes.NewBufferString(input), &first, loader, filepath.Join(t.TempDir(), "missing-statusline.json"), now, cli.WithStatuslineOptions(opts)); err != nil {
		t.Fatalf("first RunStatusline: %v", err)
	}
	if err := cli.RunStatusline(context.Background(), bytes.NewBufferString(input), &second, loader, filepath.Join(t.TempDir(), "missing-statusline.json"), now.Add(time.Second), cli.WithStatuslineOptions(opts)); err != nil {
		t.Fatalf("second RunStatusline: %v", err)
	}
	if loader.calls != 1 {
		t.Fatalf("default cache should avoid second load, calls=%d", loader.calls)
	}
	if second.String() != first.String() {
		t.Fatalf("second output should be cached\nfirst:  %q\nsecond: %q", first.String(), second.String())
	}
	if !strings.Contains(debug.String(), "hit") {
		t.Fatalf("--debug should report cache hit, got %q", debug.String())
	}

	noCacheLoader := &countingStatuslineLoader{blocks: []*blocks.SessionBlock{
		activeStatuslineBlock(now),
		activeStatuslineBlock(now),
	}}
	noCacheLoader.blocks[1].Cost = 7.77
	var uncached1, uncached2 bytes.Buffer
	noCacheOpts := cli.StatuslineOptions{NoCache: true, RefreshInterval: 60}
	if err := cli.RunStatusline(context.Background(), bytes.NewBufferString(input), &uncached1, noCacheLoader, filepath.Join(t.TempDir(), "missing-statusline.json"), now, cli.WithStatuslineOptions(noCacheOpts)); err != nil {
		t.Fatalf("first no-cache RunStatusline: %v", err)
	}
	if err := cli.RunStatusline(context.Background(), bytes.NewBufferString(input), &uncached2, noCacheLoader, filepath.Join(t.TempDir(), "missing-statusline.json"), now.Add(time.Second), cli.WithStatuslineOptions(noCacheOpts)); err != nil {
		t.Fatalf("second no-cache RunStatusline: %v", err)
	}
	if noCacheLoader.calls != 2 {
		t.Fatalf("--no-cache should load every time, calls=%d", noCacheLoader.calls)
	}
	if !strings.Contains(uncached2.String(), "$7.77") {
		t.Fatalf("--no-cache should freshly render second block, got %q", uncached2.String())
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
