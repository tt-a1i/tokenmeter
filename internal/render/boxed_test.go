package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderAggregateBoxedHasAllColumns(t *testing.T) {
	rows := []AggregateRow{
		{
			Bucket: "2026-05-19", Models: []string{"claude-opus-4-7"},
			InputTokens: 1200, OutputTokens: 45000, CacheCreateTokens: 8000,
			CacheReadTokens: 420000, TotalTokens: 474200, Cost: 18.83,
		},
	}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"DATE", "MODELS", "INPUT", "OUTPUT", "CACHE", "TOTAL", "COST", "2026-05-19", "claude-opus-4-7", "$18.83"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderAggregateBoxedHasTotalsRow(t *testing.T) {
	rows := []AggregateRow{
		{Bucket: "2026-05-19", InputTokens: 100, OutputTokens: 200, TotalTokens: 300, Cost: 1.0},
		{Bucket: "2026-05-20", InputTokens: 50, OutputTokens: 150, TotalTokens: 200, Cost: 0.5},
	}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "TOTAL") {
		t.Fatalf("expected TOTAL summary row, got:\n%s", out)
	}
	// totals should reflect sums (300+200=500 total tokens, 1.5 cost)
	if !strings.Contains(out, "$1.50") {
		t.Errorf("expected $1.50 in totals row, got:\n%s", out)
	}
}

func TestRenderAggregateBoxedEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no data in range") {
		t.Fatalf("expected empty-range message, got:\n%s", buf.String())
	}
}

func TestRenderAggregateBoxedBreakdownNests(t *testing.T) {
	rows := []AggregateRow{
		{
			Bucket: "2026-05-19", Models: []string{"claude", "gpt"},
			TotalTokens: 300, Cost: 1.0,
			Breakdown: []ModelBreakdown{
				{Model: "claude", TotalTokens: 200, Cost: 0.7},
				{Model: "gpt", TotalTokens: 100, Cost: 0.3},
			},
		},
	}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, Options{Breakdown: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "└─ claude") {
		t.Fatalf("expected └─ claude breakdown row, got:\n%s", buf.String())
	}
}
