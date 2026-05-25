package render

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestRenderAggregateUsesFullLayoutAtWideTerminal(t *testing.T) {
	withTerminalWidth(t, 120, true)
	out := renderAggregateForWidth(t, Options{})

	for _, want := range []string{"MODELS", "CACHE CRT.", "CACHE READ"} {
		if !strings.Contains(out, want) {
			t.Fatalf("wide layout missing %q:\n%s", want, out)
		}
	}
}

func TestRenderAggregateUsesCompactLayoutBelowThreshold(t *testing.T) {
	withTerminalWidth(t, 119, true)
	out := renderAggregateForWidth(t, Options{})

	for _, notWant := range []string{"MODELS", "CACHE CRT.", "CACHE READ"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("compact layout should omit %q:\n%s", notWant, out)
		}
	}
	for _, want := range []string{"CACHE", "30", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Fatalf("compact layout missing %q:\n%s", want, out)
		}
	}
}

func TestRenderAggregateCompactOptionOverridesWideTerminal(t *testing.T) {
	withTerminalWidth(t, 160, true)
	out := renderAggregateForWidth(t, Options{Compact: true})

	if strings.Contains(out, "CACHE CRT.") || strings.Contains(out, "CACHE READ") {
		t.Fatalf("--compact should force compact layout:\n%s", out)
	}
	if !strings.Contains(out, "CACHE") {
		t.Fatalf("compact layout missing CACHE column:\n%s", out)
	}
}

func TestRenderAggregateDefaultsFullWhenTerminalWidthUnavailable(t *testing.T) {
	withTerminalWidth(t, 0, false)
	out := renderAggregateForWidth(t, Options{})

	for _, want := range []string{"MODELS", "CACHE CRT.", "CACHE READ"} {
		if !strings.Contains(out, want) {
			t.Fatalf("non-TTY fallback missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSessionsAndBlocksUseCompactColumns(t *testing.T) {
	withTerminalWidth(t, 100, true)

	var sessions bytes.Buffer
	if err := New().RenderSessions(&sessions, []SessionRow{{
		SessionID: "s1", ProjectPath: "/repo", Models: []string{"claude"},
		InputTokens: 1, OutputTokens: 2, CacheCreateTokens: 3, CacheReadTokens: 4, TotalTokens: 10,
	}}, Options{}); err != nil {
		t.Fatal(err)
	}
	assertCompactColumns(t, sessions.String())

	var blocks bytes.Buffer
	if err := New().RenderBlocks(&blocks, []BlockRow{{
		Period: "2026-05-19 10:00", Models: []string{"claude"}, Status: "ACTIVE",
		InputTokens: 1, OutputTokens: 2, CacheCreateTokens: 3, CacheReadTokens: 4, TotalTokens: 10,
	}}, Options{}); err != nil {
		t.Fatal(err)
	}
	assertCompactColumns(t, blocks.String())
}

func withTerminalWidth(t *testing.T, width int, ok bool) {
	t.Helper()
	old := terminalWidth
	terminalWidth = func(io.Writer) (int, bool) { return width, ok }
	t.Cleanup(func() { terminalWidth = old })
}

func renderAggregateForWidth(t *testing.T, opts Options) string {
	t.Helper()
	rows := []AggregateRow{
		{
			Bucket:            "2026-05-19",
			Models:            []string{"claude-sonnet-4-6"},
			InputTokens:       100,
			OutputTokens:      200,
			CacheCreateTokens: 10,
			CacheReadTokens:   20,
			TotalTokens:       330,
			Cost:              1.25,
		},
	}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, opts); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func assertCompactColumns(t *testing.T, out string) {
	t.Helper()
	for _, notWant := range []string{"MODELS", "CACHE CRT.", "CACHE READ"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("compact layout should omit %q:\n%s", notWant, out)
		}
	}
	if !strings.Contains(out, "CACHE") {
		t.Fatalf("compact layout missing CACHE column:\n%s", out)
	}
}
