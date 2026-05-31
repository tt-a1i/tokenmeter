package render

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRenderAggregateJSONSchema(t *testing.T) {
	rows := []AggregateRow{
		{
			Bucket: "2026-05-19", Models: []string{"claude"},
			InputTokens: 100, OutputTokens: 200, CacheCreateTokens: 10,
			CacheReadTokens: 50, TotalTokens: 360, Cost: 1.23,
		},
	}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Daily []struct {
			Date                string   `json:"date"`
			ModelsUsed          []string `json:"modelsUsed"`
			InputTokens         int64    `json:"inputTokens"`
			OutputTokens        int64    `json:"outputTokens"`
			CacheCreationTokens int64    `json:"cacheCreationTokens"`
			CacheReadTokens     int64    `json:"cacheReadTokens"`
			TotalTokens         int64    `json:"totalTokens"`
			TotalCost           float64  `json:"totalCost"`
			ModelBreakdowns     []any    `json:"modelBreakdowns"`
		} `json:"daily"`
		Totals struct {
			InputTokens  int64   `json:"inputTokens"`
			OutputTokens int64   `json:"outputTokens"`
			TotalCost    float64 `json:"totalCost"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got.Daily) != 1 || got.Daily[0].Date != "2026-05-19" {
		t.Fatalf("daily row wrong: %+v", got.Daily)
	}
	if got.Daily[0].TotalCost != 1.23 {
		t.Errorf("totalCost: got %v, want 1.23", got.Daily[0].TotalCost)
	}
	if got.Daily[0].ModelBreakdowns == nil {
		t.Errorf("modelBreakdowns must be present as an array")
	}
	if got.Totals.InputTokens != 100 {
		t.Errorf("totals.inputTokens: got %d, want 100", got.Totals.InputTokens)
	}
}

func TestRenderAggregateJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", nil, Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	// Empty JSON should still be valid + parseable
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal empty: %v\n%s", err, buf.String())
	}
}

func TestRenderSessionsJSONSchema(t *testing.T) {
	rows := []SessionRow{
		{SessionID: "abc", ProjectPath: "/path", Models: []string{"claude"},
			InputTokens: 100, TotalTokens: 100, Cost: 1.0},
	}
	var buf bytes.Buffer
	if err := New().RenderSessions(&buf, rows, Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"sessionId":"abc"`)) &&
		!bytes.Contains(buf.Bytes(), []byte(`"sessionId": "abc"`)) {
		t.Fatalf("expected sessionId field, got:\n%s", buf.String())
	}
	for _, want := range []string{`"projectPath"`, `"lastActivity"`, `"modelBreakdowns"`} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("expected stable %s field, got:\n%s", want, buf.String())
		}
	}
}

func TestRenderSessionsJSONLastActivityDate(t *testing.T) {
	loc := time.FixedZone("UTC+9", 9*60*60)
	rows := []SessionRow{{
		SessionID:    "abc",
		LastActivity: time.Date(2026, 5, 19, 23, 30, 0, 0, time.UTC),
		InputTokens:  1,
		TotalTokens:  1,
	}}
	var buf bytes.Buffer
	if err := New().RenderSessions(&buf, rows, Options{JSON: true, Location: loc}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"lastActivity": "2026-05-20"`)) {
		t.Fatalf("lastActivity should be YYYY-MM-DD in report timezone:\n%s", buf.String())
	}
}

func TestRenderBreakdownJSONUsesCcusageNames(t *testing.T) {
	rows := []AggregateRow{{
		Bucket: "2026-05-19",
		Breakdown: []ModelBreakdown{{
			Model: "claude", InputTokens: 1, TotalTokens: 1, Cost: 0.25,
		}},
	}}
	var buf bytes.Buffer
	if err := New().RenderAggregate(&buf, "daily", rows, Options{JSON: true, Breakdown: true}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"modelName": "claude"`)) {
		t.Fatalf("expected modelName in breakdown:\n%s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"cost": 0.25`)) {
		t.Fatalf("expected cost in breakdown:\n%s", buf.String())
	}
	if bytes.Contains(buf.Bytes(), []byte(`"model":`)) || bytes.Contains(buf.Bytes(), []byte(`"totalCost": 0.25`)) {
		t.Fatalf("legacy breakdown fields should not be present:\n%s", buf.String())
	}
}

func TestRenderBlocksJSONCcusageEnvelope(t *testing.T) {
	start := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	actualEnd := start.Add(30 * time.Minute)
	rows := []BlockRow{{
		ID:                "block-1",
		Period:            "2026-05-19 10:00",
		StartTime:         start,
		EndTime:           start.Add(5 * time.Hour),
		ActualEndTime:     &actualEnd,
		IsActive:          true,
		EntryCount:        2,
		Models:            []string{"claude"},
		InputTokens:       100,
		OutputTokens:      50,
		CacheCreateTokens: 10,
		CacheReadTokens:   5,
		TotalTokens:       165,
		Cost:              1.25,
		BurnRate:          &BlockBurnRate{TokensPerMinute: 10, CostPerHour: 2},
		Projection:        &BlockProjection{TotalTokens: 300, TotalCost: 2.5, RemainingTime: 90 * time.Minute},
		TokenLimit:        1000,
		UsagePct:          30,
		TokenLimitStatus:  "OK",
	}}
	var buf bytes.Buffer
	if err := New().RenderBlocks(&buf, rows, Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if _, ok := got["blocks"]; !ok {
		t.Fatalf("expected blocks envelope:\n%s", buf.String())
	}
	if _, ok := got["totals"]; ok {
		t.Fatalf("blocks JSON should not include totals:\n%s", buf.String())
	}
	for _, want := range []string{
		`"id": "block-1"`,
		`"startTime": "2026-05-19T10:00:00.000Z"`,
		`"actualEndTime": "2026-05-19T10:30:00.000Z"`,
		`"isActive": true`,
		`"isGap": false`,
		`"entries": 2`,
		`"tokenCounts"`,
		`"cacheCreationInputTokens": 10`,
		`"cacheReadInputTokens": 5`,
		`"costUSD": 1.25`,
		`"models"`,
		`"burnRate"`,
		`"remainingMinutes": 90`,
		`"tokenLimitStatus"`,
		`"projectedUsage": 300`,
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("blocks JSON missing %s:\n%s", want, buf.String())
		}
	}
}

func TestRenderJSONAppliesJQ(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}
	var buf bytes.Buffer
	rows := []AggregateRow{{Bucket: "2026-05-19", InputTokens: 7, TotalTokens: 7}}
	if err := New().RenderAggregate(&buf, "daily", rows, Options{JSON: true, JQ: ".totals.inputTokens"}); err != nil {
		t.Fatal(err)
	}
	if got := bytes.TrimSpace(buf.Bytes()); !bytes.Equal(got, []byte("7")) {
		t.Fatalf("jq output = %q, want 7", got)
	}
}

func TestRenderJSONJQMissingExecutableReturnsClearError(t *testing.T) {
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPath)
	var buf bytes.Buffer
	err := New().RenderAggregate(&buf, "daily", nil, Options{JSON: true, JQ: "."})
	if err == nil {
		t.Fatal("expected missing jq error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("requires jq executable")) {
		t.Fatalf("missing jq error should be clear, got %v", err)
	}
}
