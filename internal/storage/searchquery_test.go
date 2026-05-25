package storage

import (
	"strings"
	"testing"
)

func TestParseQueryKeywords(t *testing.T) {
	q, err := ParseQuery("exit code")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if strings.Join(q.Keywords, " ") != "exit code" {
		t.Fatalf("keywords=%+v", q.Keywords)
	}
}

func TestParseQueryFilters(t *testing.T) {
	q, err := ParseQuery("error tool:Bash session:abc status:failed platform:codex cost:>5 cost:<10 tokens:>100 tokens:<200 since:2026-05-01 until:2026-05-25")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if q.Tool != "Bash" || q.Session != "abc" || q.Status != "failed" || q.Platform != "codex" {
		t.Fatalf("unexpected string filters: %+v", q)
	}
	if q.CostMin == nil || *q.CostMin != 5 || q.CostMax == nil || *q.CostMax != 10 {
		t.Fatalf("unexpected cost filters: %+v", q)
	}
	if q.TokensMin == nil || *q.TokensMin != 100 || q.TokensMax == nil || *q.TokensMax != 200 {
		t.Fatalf("unexpected token filters: %+v", q)
	}
	if q.Since == nil || q.Since.Format("2006-01-02") != "2026-05-01" {
		t.Fatalf("unexpected since: %+v", q.Since)
	}
	if q.Until == nil || q.Until.Format("2006-01-02") != "2026-05-25" {
		t.Fatalf("unexpected until: %+v", q.Until)
	}
	if q.String() != "error tool:Bash session:abc status:failed platform:codex cost:>5 cost:<10 tokens:>100 tokens:<200 since:2026-05-01 until:2026-05-25" {
		t.Fatalf("String()=%q", q.String())
	}
}

func TestParseQueryExactRangesBecomeMin(t *testing.T) {
	q, err := ParseQuery("cost:5 tokens:100")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if q.CostMin == nil || *q.CostMin != 5 || q.TokensMin == nil || *q.TokensMin != 100 {
		t.Fatalf("exact range filters should become lower bounds: %+v", q)
	}
}

func TestParseQueryErrors(t *testing.T) {
	for _, input := range []string{
		"foo:bar",
		"cost:>nope",
		"tokens:<nope",
		"since:20260501",
		"status:maybe",
		"platform:other",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseQuery(input); err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}
