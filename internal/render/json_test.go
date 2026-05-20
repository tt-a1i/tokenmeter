package render

import (
	"bytes"
	"encoding/json"
	"testing"
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
}
