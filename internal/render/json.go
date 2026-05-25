package render

import (
	"encoding/json"
	"io"
	"time"
)

// aggregateJSONRow / sessionJSONRow / blockJSONRow mirror render.AggregateRow,
// SessionRow, BlockRow with ccusage-aligned camelCase tags.

type aggregateJSONRow struct {
	Date                string          `json:"date,omitempty"`
	Week                string          `json:"week,omitempty"`
	Month               string          `json:"month,omitempty"`
	ModelsUsed          []string        `json:"modelsUsed"`
	InputTokens         int64           `json:"inputTokens"`
	OutputTokens        int64           `json:"outputTokens"`
	CacheCreationTokens int64           `json:"cacheCreationTokens"`
	CacheReadTokens     int64           `json:"cacheReadTokens"`
	TotalTokens         int64           `json:"totalTokens"`
	TotalCost           float64         `json:"totalCost"`
	ModelBreakdowns     []breakdownJSON `json:"modelBreakdowns,omitempty"`
}

type sessionJSONRow struct {
	SessionID           string          `json:"sessionId"`
	ProjectPath         string          `json:"projectPath,omitempty"`
	LastActivity        time.Time       `json:"lastActivity,omitempty"`
	ModelsUsed          []string        `json:"modelsUsed"`
	InputTokens         int64           `json:"inputTokens"`
	OutputTokens        int64           `json:"outputTokens"`
	CacheCreationTokens int64           `json:"cacheCreationTokens"`
	CacheReadTokens     int64           `json:"cacheReadTokens"`
	TotalTokens         int64           `json:"totalTokens"`
	TotalCost           float64         `json:"totalCost"`
	ModelBreakdowns     []breakdownJSON `json:"modelBreakdowns,omitempty"`
}

type blockJSONRow struct {
	Period              string               `json:"period"`
	ModelsUsed          []string             `json:"modelsUsed"`
	InputTokens         int64                `json:"inputTokens"`
	OutputTokens        int64                `json:"outputTokens"`
	CacheCreationTokens int64                `json:"cacheCreationTokens"`
	CacheReadTokens     int64                `json:"cacheReadTokens"`
	TotalTokens         int64                `json:"totalTokens"`
	TotalCost           float64              `json:"totalCost"`
	Status              string               `json:"status"`
	TokenLimit          int64                `json:"token_limit,omitempty"`
	UsagePct            float64              `json:"usage_pct,omitempty"`
	Projection          *blockProjectionJSON `json:"projection,omitempty"`
	ModelBreakdowns     []breakdownJSON      `json:"modelBreakdowns,omitempty"`
}

type blockProjectionJSON struct {
	TotalTokens          int64   `json:"totalTokens"`
	TotalCost            float64 `json:"totalCost"`
	RemainingTimeSeconds float64 `json:"remainingTimeSeconds"`
}

type breakdownJSON struct {
	Model               string  `json:"model"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

type totalsJSON struct {
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

func (defaultRenderer) renderAggregateJSON(w io.Writer, kind string, rows []AggregateRow) error {
	outRows := make([]aggregateJSONRow, 0, len(rows))
	var totals totalsJSON
	for _, r := range rows {
		row := aggregateJSONRow{
			ModelsUsed:          r.Models,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreateTokens,
			CacheReadTokens:     r.CacheReadTokens,
			TotalTokens:         r.TotalTokens,
			TotalCost:           r.Cost,
		}
		switch kind {
		case "weekly":
			row.Week = r.Bucket
		case "monthly":
			row.Month = r.Bucket
		default:
			row.Date = r.Bucket
		}
		for _, b := range r.Breakdown {
			row.ModelBreakdowns = append(row.ModelBreakdowns, breakdownJSON{
				Model: b.Model, InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
				CacheCreationTokens: b.CacheCreateTokens, CacheReadTokens: b.CacheReadTokens,
				TotalTokens: b.TotalTokens, TotalCost: b.Cost,
			})
		}
		outRows = append(outRows, row)
		totals.InputTokens += r.InputTokens
		totals.OutputTokens += r.OutputTokens
		totals.CacheCreationTokens += r.CacheCreateTokens
		totals.CacheReadTokens += r.CacheReadTokens
		totals.TotalTokens += r.TotalTokens
		totals.TotalCost += r.Cost
	}
	envelope := map[string]any{
		kind:     outRows,
		"totals": totals,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func (defaultRenderer) renderSessionsJSON(w io.Writer, rows []SessionRow) error {
	outRows := make([]sessionJSONRow, 0, len(rows))
	var totals totalsJSON
	for _, r := range rows {
		row := sessionJSONRow{
			SessionID:           r.SessionID,
			ProjectPath:         r.ProjectPath,
			LastActivity:        r.LastActivity,
			ModelsUsed:          r.Models,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreateTokens,
			CacheReadTokens:     r.CacheReadTokens,
			TotalTokens:         r.TotalTokens,
			TotalCost:           r.Cost,
		}
		for _, b := range r.Breakdown {
			row.ModelBreakdowns = append(row.ModelBreakdowns, breakdownJSON{
				Model: b.Model, InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
				CacheCreationTokens: b.CacheCreateTokens, CacheReadTokens: b.CacheReadTokens,
				TotalTokens: b.TotalTokens, TotalCost: b.Cost,
			})
		}
		outRows = append(outRows, row)
		totals.InputTokens += r.InputTokens
		totals.OutputTokens += r.OutputTokens
		totals.CacheCreationTokens += r.CacheCreateTokens
		totals.CacheReadTokens += r.CacheReadTokens
		totals.TotalTokens += r.TotalTokens
		totals.TotalCost += r.Cost
	}
	envelope := map[string]any{
		"sessions": outRows,
		"totals":   totals,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func (defaultRenderer) renderBlocksJSON(w io.Writer, rows []BlockRow) error {
	outRows := make([]blockJSONRow, 0, len(rows))
	var totals totalsJSON
	for _, r := range rows {
		row := blockJSONRow{
			Period:              r.Period,
			ModelsUsed:          r.Models,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreateTokens,
			CacheReadTokens:     r.CacheReadTokens,
			TotalTokens:         r.TotalTokens,
			TotalCost:           r.Cost,
			Status:              r.Status,
		}
		if r.TokenLimit > 0 {
			row.TokenLimit = r.TokenLimit
			row.UsagePct = r.UsagePct
			row.Status = r.TokenLimitStatus
		}
		if r.Projection != nil {
			row.Projection = &blockProjectionJSON{
				TotalTokens:          r.Projection.TotalTokens,
				TotalCost:            r.Projection.TotalCost,
				RemainingTimeSeconds: r.Projection.RemainingTime.Seconds(),
			}
		}
		for _, b := range r.Breakdown {
			row.ModelBreakdowns = append(row.ModelBreakdowns, breakdownJSON{
				Model: b.Model, InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
				CacheCreationTokens: b.CacheCreateTokens, CacheReadTokens: b.CacheReadTokens,
				TotalTokens: b.TotalTokens, TotalCost: b.Cost,
			})
		}
		outRows = append(outRows, row)
		totals.InputTokens += r.InputTokens
		totals.OutputTokens += r.OutputTokens
		totals.CacheCreationTokens += r.CacheCreateTokens
		totals.CacheReadTokens += r.CacheReadTokens
		totals.TotalTokens += r.TotalTokens
		totals.TotalCost += r.Cost
	}
	envelope := map[string]any{
		"blocks": outRows,
		"totals": totals,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
