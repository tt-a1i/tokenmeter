package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func TestRenderToolErrorsTextIncludesThreeSections(t *testing.T) {
	report := ToolErrorReport{
		TopTools: []storage.ToolErrorStats{{
			Tool: "Bash", Total: 10, Failures: 3, FailureRate: 30, TopPattern: "exit code N",
		}},
		Patterns: []storage.ErrorPattern{{
			NormalizedKey: "no such file or directory",
			SampleResult:  "No such file or directory: /tmp/a.go",
			Count:         4,
			Tools:         []string{"Bash", "Read"},
			Sessions:      []string{"session-a", "session-b"},
		}},
		Daily: []storage.DailyRate{{Date: "2026-05-12", Total: 10, Failures: 1, FailureRate: 10}},
	}

	var buf bytes.Buffer
	if err := New().RenderToolErrors(&buf, report, Options{}); err != nil {
		t.Fatalf("RenderToolErrors: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Top Failing Tools",
		"Error Pattern Groups",
		"Daily failure rate",
		"Bash",
		"no such file or directory",
		"2026-05-12",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderToolErrorsJSONEnvelope(t *testing.T) {
	report := ToolErrorReport{
		TopTools: []storage.ToolErrorStats{{Tool: "Edit", Total: 5, Failures: 2, FailureRate: 40}},
		Patterns: []storage.ErrorPattern{{NormalizedKey: "string to replace not found", Count: 3}},
		Daily:    []storage.DailyRate{{Date: "2026-05-12", Total: 5, Failures: 2, FailureRate: 40}},
	}

	var buf bytes.Buffer
	if err := New().RenderToolErrors(&buf, report, Options{JSON: true}); err != nil {
		t.Fatalf("RenderToolErrors JSON: %v", err)
	}
	var payload struct {
		ToolErrors struct {
			TopTools []struct {
				Tool string `json:"tool"`
			} `json:"top_tools"`
			Patterns []struct {
				NormalizedKey string `json:"normalized_key"`
			} `json:"patterns"`
			Daily []struct {
				Date string `json:"date"`
			} `json:"daily"`
		} `json:"tool_errors"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(payload.ToolErrors.TopTools) != 1 || payload.ToolErrors.TopTools[0].Tool != "Edit" {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
}
