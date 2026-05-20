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
