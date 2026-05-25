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
)

func TestParseSharedCompactAndStatuslineFlags(t *testing.T) {
	got, rest, err := cli.ParseShared([]string{
		"--compact",
		"--context-low-threshold", "40",
		"--context-medium-threshold", "70",
		"--burn-rate-display", "emoji-text",
		"statusline",
	})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if !got.Compact {
		t.Fatal("--compact must be true")
	}
	if got.ContextLowThreshold != 40 || got.ContextMediumThreshold != 70 || got.BurnRateDisplay != "emoji-text" {
		t.Fatalf("statusline flags not parsed: %+v", got)
	}
	if len(rest) != 1 || rest[0] != "statusline" {
		t.Fatalf("rest=%v want [statusline]", rest)
	}
}

func TestParseSharedRejectsInvalidBurnRateDisplay(t *testing.T) {
	_, _, err := cli.ParseShared([]string{"--burn-rate-display", "loud"})
	if err == nil {
		t.Fatal("expected invalid --burn-rate-display to fail")
	}
}

func TestRunStatuslineFlagOverridesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "statusline.json")
	if err := os.WriteFile(cfgPath, []byte(`{"color":true,"context_low_threshold":20,"context_medium_threshold":30,"burn_rate_display":"off"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var buf bytes.Buffer
	err := cli.RunStatusline(context.Background(),
		bytes.NewBufferString(`{"model_id":"claude-sonnet-4-6","session_id":"s","cwd":"/x","transcript_path":"","context_window":{"total_input_tokens":45000,"context_window_size":100000}}`),
		&buf, stubStatuslineLoader{active: nil}, cfgPath, time.Now(),
		cli.WithStatuslineOptions(cli.StatuslineOptions{
			ContextLowThreshold:    50,
			ContextMediumThreshold: 80,
			BurnRateDisplay:        "emoji",
		}))
	if err != nil {
		t.Fatalf("RunStatusline: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[32m45%\x1b[0m") {
		t.Fatalf("flag thresholds should override config thresholds, got:\n%q", buf.String())
	}
}
