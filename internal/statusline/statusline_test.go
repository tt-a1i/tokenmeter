package statusline_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	for _, want := range []string{"session", "today", "block", "ctx"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output must include %q core statusline segment, got %q", want, got)
		}
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
	if cfg.QuotaUSD != 0 || cfg.Color || cfg.ContextLowThreshold != 50 || cfg.ContextMediumThreshold != 80 || cfg.BurnRateDisplay != "off" {
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
		{mode: "off", notWant: []string{"$4.00/hr", "🔥", "High"}},
		{mode: "emoji", want: []string{"$4.00/hr", "🔥"}, notWant: []string{"High"}},
		{mode: "text", want: []string{"$4.00/hr", "High"}, notWant: []string{"🔥"}},
		{mode: "emoji-text", want: []string{"$4.00/hr", "🔥", "High"}},
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

func TestParseInputAcceptsNestedModel(t *testing.T) {
	in := strings.NewReader(`{
		"model": {"id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6"},
		"session_id": "sess",
		"cost": {"total_cost_usd": 12.34},
		"extra_ccusage_field": true
	}`)
	got, err := statusline.ParseInput(in)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}
	if got.Model == nil || got.Model.ID != "claude-sonnet-4-6" || got.Model.DisplayName != "Claude Sonnet 4.6" {
		t.Fatalf("nested model not parsed: %+v", got)
	}
	if got.Cost == nil || got.Cost.TotalCostUSD != 12.34 {
		t.Fatalf("ccusage hook cost not parsed: %+v", got.Cost)
	}
}

func TestRenderCostSourceBothShowsHookAndCCUsageCosts(t *testing.T) {
	now := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	block := statuslineTestBlock(now, 2.50)
	ccusageCost := 3.75
	in := statusline.Input{
		ModelID: "claude-sonnet-4-6",
		Cost:    &statusline.InputCost{TotalCostUSD: 1.25},
	}
	var buf bytes.Buffer
	err := statusline.RenderWithMetrics(&buf, in, block, statusline.Config{},
		now, statusline.Metrics{CostSource: "both", CCUsageSessionCost: &ccusageCost, TodayCost: 4.50})
	if err != nil {
		t.Fatalf("RenderWithMetrics: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"session $1.25 cc / $3.75 ccusage", "today $4.50", "block $2.50"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%q", want, out)
		}
	}
}

func TestRenderUsesNestedModelDisplayName(t *testing.T) {
	var buf bytes.Buffer
	in := statusline.Input{Model: &statusline.InputModel{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6"}}
	if err := statusline.Render(&buf, in, nil, statusline.Config{}, time.Now()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "sonnet") {
		t.Fatalf("nested model display name should be rendered, got %q", buf.String())
	}
}

func TestRenderDefaultBurnRateOff(t *testing.T) {
	start := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	last := start.Add(30 * time.Minute)
	block := &blocks.SessionBlock{
		StartTime: start, EndTime: start.Add(5 * time.Hour), ActualEnd: &last, IsActive: true,
		Tokens:   blocks.TokenCounts{Input: 100, Output: 50},
		Cost:     1,
		BurnRate: &blocks.BurnRate{TokensPerMinute: 6_000, CostPerHour: 4},
	}
	var buf bytes.Buffer
	if err := statusline.Render(&buf, statusline.Input{ModelID: "claude-sonnet-4-6"}, block, statusline.Config{}, start); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(buf.String(), "🔥") || strings.Contains(buf.String(), "High") {
		t.Fatalf("default burn rate should be off, got %q", buf.String())
	}
}

func TestRunWithOptionsCachesFreshOutput(t *testing.T) {
	now := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	if err := os.Chtimes(transcript, now, now); err != nil {
		t.Fatalf("chtimes transcript: %v", err)
	}
	cachePath := filepath.Join(t.TempDir(), "statusline-cache.json")
	reader := &countingStatuslineReader{block: statuslineTestBlock(now, 1.50)}
	input := statuslineTestInput(transcript)

	var first, second, debug bytes.Buffer
	opts := statusline.RunOptions{
		CacheEnabled:    true,
		CachePath:       cachePath,
		RefreshInterval: time.Minute,
		Debug:           true,
		DebugWriter:     &debug,
	}
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &first, reader, statusline.Config{}, now, opts); err != nil {
		t.Fatalf("first RunWithOptions: %v", err)
	}
	reader.block = statuslineTestBlock(now, 9.99)
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &second, reader, statusline.Config{}, now.Add(500*time.Millisecond), opts); err != nil {
		t.Fatalf("second RunWithOptions: %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("fresh cache should avoid second reader call, calls=%d", reader.calls)
	}
	if second.String() != first.String() {
		t.Fatalf("second output should be cached\nfirst:  %q\nsecond: %q", first.String(), second.String())
	}
	if !strings.Contains(debug.String(), "write") || !strings.Contains(debug.String(), "hit") {
		t.Fatalf("debug output should mention write and hit, got %q", debug.String())
	}
}

func TestRunWithOptionsInvalidatesWhenTranscriptMTimeChanges(t *testing.T) {
	now := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	if err := os.Chtimes(transcript, now, now); err != nil {
		t.Fatalf("chtimes transcript: %v", err)
	}
	cachePath := filepath.Join(t.TempDir(), "statusline-cache.json")
	reader := &countingStatuslineReader{block: statuslineTestBlock(now, 1.50)}
	input := statuslineTestInput(transcript)
	opts := statusline.RunOptions{CacheEnabled: true, CachePath: cachePath, RefreshInterval: time.Minute}
	var first bytes.Buffer
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &first, reader, statusline.Config{}, now, opts); err != nil {
		t.Fatalf("first RunWithOptions: %v", err)
	}
	if err := os.Chtimes(transcript, now.Add(time.Second), now.Add(time.Second)); err != nil {
		t.Fatalf("update transcript mtime: %v", err)
	}
	reader.block = statuslineTestBlock(now, 2.50)
	var second, debug bytes.Buffer
	opts.Debug = true
	opts.DebugWriter = &debug
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &second, reader, statusline.Config{}, now.Add(500*time.Millisecond), opts); err != nil {
		t.Fatalf("second RunWithOptions: %v", err)
	}
	if reader.calls != 2 {
		t.Fatalf("mtime change should reload active block, calls=%d", reader.calls)
	}
	if !strings.Contains(second.String(), "$2.50") {
		t.Fatalf("second output should be freshly rendered, got %q", second.String())
	}
	if !strings.Contains(debug.String(), "transcript-mtime") {
		t.Fatalf("debug output should include stale reason, got %q", debug.String())
	}
}

func TestRunWithOptionsRefreshIntervalExpiresCache(t *testing.T) {
	now := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "statusline-cache.json")
	reader := &countingStatuslineReader{block: statuslineTestBlock(now, 1.50)}
	input := statuslineTestInput("")
	opts := statusline.RunOptions{CacheEnabled: true, CachePath: cachePath, RefreshInterval: time.Second}

	var first bytes.Buffer
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &first, reader, statusline.Config{}, now, opts); err != nil {
		t.Fatalf("first RunWithOptions: %v", err)
	}
	reader.block = statuslineTestBlock(now, 2.50)
	var second, debug bytes.Buffer
	opts.Debug = true
	opts.DebugWriter = &debug
	if err := statusline.RunWithOptions(context.Background(), strings.NewReader(input), &second, reader, statusline.Config{}, now.Add(2*time.Second), opts); err != nil {
		t.Fatalf("second RunWithOptions: %v", err)
	}
	if reader.calls != 2 {
		t.Fatalf("expired refresh interval should reload active block, calls=%d", reader.calls)
	}
	if !strings.Contains(second.String(), "$2.50") {
		t.Fatalf("second output should be freshly rendered, got %q", second.String())
	}
	if !strings.Contains(debug.String(), "refresh-interval") {
		t.Fatalf("debug output should include refresh-interval stale reason, got %q", debug.String())
	}
}

type countingStatuslineReader struct {
	block *blocks.SessionBlock
	calls int
}

func (r *countingStatuslineReader) LoadActive(context.Context) (*blocks.SessionBlock, error) {
	r.calls++
	return r.block, nil
}

func statuslineTestInput(transcript string) string {
	return fmt.Sprintf(`{"model_id":"claude-sonnet-4-6","session_id":"cache-session","cwd":"/x","transcript_path":%q}`, transcript)
}

func statuslineTestBlock(now time.Time, cost float64) *blocks.SessionBlock {
	start := now.Add(-30 * time.Minute)
	last := now
	return &blocks.SessionBlock{
		StartTime: start,
		EndTime:   start.Add(5 * time.Hour),
		ActualEnd: &last,
		IsActive:  true,
		Tokens:    blocks.TokenCounts{Input: 60000, Output: 30000},
		Cost:      cost,
		Models:    []string{"claude-sonnet-4-6"},
	}
}
