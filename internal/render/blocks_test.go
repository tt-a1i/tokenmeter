package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderBlocksTokenLimitZeroKeepsOldColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := New().RenderBlocks(&buf, []BlockRow{tokenLimitBlockRow(50, 0, "")}, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, notWant := range []string{"USAGE%", "PROGRESS"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("token-limit columns should be hidden when limit is zero:\n%s", out)
		}
	}
}

func TestRenderBlocksTokenLimitShowsUsageStatusAndProgress(t *testing.T) {
	var buf bytes.Buffer
	if err := New().RenderBlocks(&buf, []BlockRow{tokenLimitBlockRow(80, 100, "WARN")}, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"USAGE%", "PROGRESS", "80%", "WARN", "[██████████░░] 80%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("token-limit output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderBlocksTokenLimitColorsStatuses(t *testing.T) {
	var buf bytes.Buffer
	rows := []BlockRow{
		tokenLimitBlockRow(50, 100, "OK"),
		tokenLimitBlockRow(80, 100, "WARN"),
		tokenLimitBlockRow(100, 100, "ALERT"),
	}
	if err := New().RenderBlocks(&buf, rows, Options{Color: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"\x1b[32mOK\x1b[0m", "\x1b[33mWARN\x1b[0m", "\x1b[31mALERT\x1b[0m"} {
		if !strings.Contains(out, want) {
			t.Fatalf("colored status missing %q:\n%q", want, out)
		}
	}
}

func TestRenderBlocksTokenLimitCompactProgressUsesEightCells(t *testing.T) {
	var buf bytes.Buffer
	if err := New().RenderBlocks(&buf, []BlockRow{tokenLimitBlockRow(80, 100, "WARN")}, Options{Compact: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "[██████░░] 80%") {
		t.Fatalf("compact progress bar should use 8 cells:\n%s", out)
	}
	if strings.Contains(out, "[██████████░░] 80%") {
		t.Fatalf("compact progress bar should not use full width:\n%s", out)
	}
}

func tokenLimitBlockRow(total, limit int64, status string) BlockRow {
	return BlockRow{
		Period:           "2026-05-19 10:00",
		Models:           []string{"claude"},
		InputTokens:      total,
		TotalTokens:      total,
		Cost:             1.0,
		Status:           "closed",
		TokenLimit:       limit,
		UsagePct:         float64(total) * 100 / float64(limit),
		TokenLimitStatus: status,
	}
}
