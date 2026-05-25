package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

func TestRenderFileChurnTextIncludesThreeSections(t *testing.T) {
	report := FileChurnReport{
		TopFiles: []storage.FileChurnStats{{
			Path:       "internal/storage/db.go",
			Changes:    10,
			Sessions:   3,
			FirstSeen:  time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			LastSeen:   time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC),
			ModeCounts: map[string]int64{"edit": 8, "create": 2},
		}},
		Hotspots: []storage.HotspotStats{{
			Path: "internal/storage", Changes: 12, Files: 2, TopFile: "db.go",
		}},
		Daily: []storage.DailyChurn{{Date: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), Changes: 7}},
	}

	var buf bytes.Buffer
	if err := New().RenderFileChurn(&buf, report, Options{}); err != nil {
		t.Fatalf("RenderFileChurn: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Top Changed Files",
		"File Churn Hotspots",
		"Daily file changes",
		"internal/storage/db.go",
		"internal/storage",
		"edit:8 create:2",
		"2026-05-12",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderFileChurnCompactOmitsSessionsAndMode(t *testing.T) {
	report := FileChurnReport{
		TopFiles: []storage.FileChurnStats{{
			Path:       "internal/storage/db.go",
			Changes:    10,
			Sessions:   3,
			FirstSeen:  time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
			LastSeen:   time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC),
			ModeCounts: map[string]int64{"edit": 8},
		}},
	}
	var buf bytes.Buffer
	if err := New().RenderFileChurn(&buf, report, Options{Compact: true}); err != nil {
		t.Fatalf("RenderFileChurn compact: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "SESSIONS") || strings.Contains(out, "MODE") || strings.Contains(out, "edit:8") {
		t.Fatalf("compact output should omit Sessions/Mode columns:\n%s", out)
	}
}

func TestRenderFileChurnJSONEnvelope(t *testing.T) {
	report := FileChurnReport{
		TopFiles: []storage.FileChurnStats{{Path: "cmd/tm/main.go", Changes: 5}},
		Hotspots: []storage.HotspotStats{{Path: "cmd/tm", Changes: 5}},
		Daily:    []storage.DailyChurn{{Date: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), Changes: 5}},
	}

	var buf bytes.Buffer
	if err := New().RenderFileChurn(&buf, report, Options{JSON: true}); err != nil {
		t.Fatalf("RenderFileChurn JSON: %v", err)
	}
	var payload struct {
		FileChurn struct {
			TopFiles []struct {
				Path string `json:"path"`
			} `json:"top_files"`
			Hotspots []struct {
				Path string `json:"path"`
			} `json:"hotspots"`
			Daily []struct {
				Changes int64 `json:"changes"`
			} `json:"daily"`
		} `json:"file_churn"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(payload.FileChurn.TopFiles) != 1 || payload.FileChurn.TopFiles[0].Path != "cmd/tm/main.go" {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
}
