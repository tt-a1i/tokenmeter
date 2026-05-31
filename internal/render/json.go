package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// aggregateJSONRow / sessionJSONRow / blockJSONRow mirror render.AggregateRow,
// SessionRow, BlockRow with ccusage-aligned camelCase tags.

type aggregateJSONRow struct {
	Date                string          `json:"date,omitempty"`
	Week                string          `json:"week,omitempty"`
	Month               string          `json:"month,omitempty"`
	Project             string          `json:"project,omitempty"`
	ModelsUsed          []string        `json:"modelsUsed"`
	InputTokens         int64           `json:"inputTokens"`
	OutputTokens        int64           `json:"outputTokens"`
	CacheCreationTokens int64           `json:"cacheCreationTokens"`
	CacheReadTokens     int64           `json:"cacheReadTokens"`
	TotalTokens         int64           `json:"totalTokens"`
	TotalCost           float64         `json:"totalCost"`
	ModelBreakdowns     []breakdownJSON `json:"modelBreakdowns"`
}

type sessionJSONRow struct {
	SessionID           string          `json:"sessionId"`
	ProjectPath         *string         `json:"projectPath"`
	LastActivity        *string         `json:"lastActivity"`
	ModelsUsed          []string        `json:"modelsUsed"`
	InputTokens         int64           `json:"inputTokens"`
	OutputTokens        int64           `json:"outputTokens"`
	CacheCreationTokens int64           `json:"cacheCreationTokens"`
	CacheReadTokens     int64           `json:"cacheReadTokens"`
	TotalTokens         int64           `json:"totalTokens"`
	TotalCost           float64         `json:"totalCost"`
	ModelBreakdowns     []breakdownJSON `json:"modelBreakdowns"`
}

type blockJSONRow struct {
	ID                  string                `json:"id"`
	StartTime           string                `json:"startTime"`
	EndTime             string                `json:"endTime"`
	ActualEndTime       *string               `json:"actualEndTime"`
	IsActive            bool                  `json:"isActive"`
	IsGap               bool                  `json:"isGap"`
	Entries             int                   `json:"entries"`
	TokenCounts         tokenCountsJSON       `json:"tokenCounts"`
	TotalTokens         int64                 `json:"totalTokens"`
	CostUSD             float64               `json:"costUSD"`
	Models              []string              `json:"models"`
	BurnRate            *blockBurnRateJSON    `json:"burnRate"`
	Projection          *blockProjectionJSON  `json:"projection"`
	TokenLimitStatus    *tokenLimitStatusJSON `json:"tokenLimitStatus,omitempty"`
	UsageLimitResetTime *string               `json:"usageLimitResetTime,omitempty"`
}

type blockProjectionJSON struct {
	TotalTokens      int64   `json:"totalTokens"`
	TotalCost        float64 `json:"totalCost"`
	RemainingMinutes int64   `json:"remainingMinutes"`
}

type blockBurnRateJSON struct {
	TokensPerMinute float64 `json:"tokensPerMinute"`
	CostPerHour     float64 `json:"costPerHour"`
}

type tokenLimitStatusJSON struct {
	Limit          int64   `json:"limit"`
	ProjectedUsage int64   `json:"projectedUsage"`
	PercentUsed    float64 `json:"percentUsed"`
	Status         string  `json:"status"`
}

type tokenCountsJSON struct {
	InputTokens         int64 `json:"inputTokens"`
	OutputTokens        int64 `json:"outputTokens"`
	CacheCreationTokens int64 `json:"cacheCreationInputTokens"`
	CacheReadTokens     int64 `json:"cacheReadInputTokens"`
}

type breakdownJSON struct {
	ModelName           string  `json:"modelName"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	Cost                float64 `json:"cost"`
}

type totalsJSON struct {
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
	TotalCost           float64 `json:"totalCost"`
}

func (defaultRenderer) renderAggregateJSON(w io.Writer, kind string, rows []AggregateRow, opts Options) error {
	outRows := make([]aggregateJSONRow, 0, len(rows))
	var totals totalsJSON
	for _, r := range rows {
		row := aggregateJSONRow{
			ModelsUsed:          nonNilStrings(r.Models),
			Project:             r.Project,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreateTokens,
			CacheReadTokens:     r.CacheReadTokens,
			TotalTokens:         r.TotalTokens,
			TotalCost:           r.Cost,
			ModelBreakdowns:     []breakdownJSON{},
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
				ModelName: b.Model, InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
				CacheCreationTokens: b.CacheCreateTokens, CacheReadTokens: b.CacheReadTokens,
				Cost: b.Cost,
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
	return writeJSON(w, envelope, opts)
}

func (defaultRenderer) renderSessionsJSON(w io.Writer, rows []SessionRow, opts Options) error {
	outRows := make([]sessionJSONRow, 0, len(rows))
	var totals totalsJSON
	for _, r := range rows {
		var projectPath *string
		if r.ProjectPath != "" {
			projectPath = &r.ProjectPath
		}
		var lastActivity *string
		if !r.LastActivity.IsZero() {
			formatted := formatJSONDate(r.LastActivity, opts.Location)
			lastActivity = &formatted
		}
		row := sessionJSONRow{
			SessionID:           r.SessionID,
			ProjectPath:         projectPath,
			LastActivity:        lastActivity,
			ModelsUsed:          nonNilStrings(r.Models),
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreateTokens,
			CacheReadTokens:     r.CacheReadTokens,
			TotalTokens:         r.TotalTokens,
			TotalCost:           r.Cost,
			ModelBreakdowns:     []breakdownJSON{},
		}
		for _, b := range r.Breakdown {
			row.ModelBreakdowns = append(row.ModelBreakdowns, breakdownJSON{
				ModelName: b.Model, InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
				CacheCreationTokens: b.CacheCreateTokens, CacheReadTokens: b.CacheReadTokens,
				Cost: b.Cost,
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
	return writeJSON(w, envelope, opts)
}

func (defaultRenderer) renderBlocksJSON(w io.Writer, rows []BlockRow, opts Options) error {
	outRows := make([]blockJSONRow, 0, len(rows))
	for _, r := range rows {
		id := r.ID
		if id == "" {
			id = formatBlockID(r.StartTime, r.IsGap)
		}
		var actualEnd *string
		if r.ActualEndTime != nil {
			formatted := formatRFC3339MillisUTC(*r.ActualEndTime)
			actualEnd = &formatted
		}
		var usageLimitResetTime *string
		if r.UsageLimitResetTime != nil {
			formatted := formatRFC3339MillisUTC(*r.UsageLimitResetTime)
			usageLimitResetTime = &formatted
		}
		row := blockJSONRow{
			ID:            id,
			StartTime:     formatRFC3339MillisUTC(r.StartTime),
			EndTime:       formatRFC3339MillisUTC(r.EndTime),
			ActualEndTime: actualEnd,
			IsActive:      r.IsActive,
			IsGap:         r.IsGap,
			Entries:       r.EntryCount,
			TokenCounts: tokenCountsJSON{
				InputTokens:         r.InputTokens,
				OutputTokens:        r.OutputTokens,
				CacheCreationTokens: r.CacheCreateTokens,
				CacheReadTokens:     r.CacheReadTokens,
			},
			TotalTokens:         r.TotalTokens,
			CostUSD:             r.Cost,
			Models:              nonNilStrings(r.Models),
			UsageLimitResetTime: usageLimitResetTime,
		}
		if r.Projection != nil {
			row.Projection = &blockProjectionJSON{
				TotalTokens:      r.Projection.TotalTokens,
				TotalCost:        r.Projection.TotalCost,
				RemainingMinutes: int64(r.Projection.RemainingTime.Round(time.Minute) / time.Minute),
			}
		}
		if r.BurnRate != nil {
			row.BurnRate = &blockBurnRateJSON{
				TokensPerMinute: r.BurnRate.TokensPerMinute,
				CostPerHour:     r.BurnRate.CostPerHour,
			}
		}
		if r.TokenLimit > 0 && r.Projection != nil {
			projectedPercent := float64(r.Projection.TotalTokens) * 100 / float64(r.TokenLimit)
			status := "ok"
			if r.Projection.TotalTokens > r.TokenLimit {
				status = "exceeds"
			} else if projectedPercent > 80 {
				status = "warning"
			}
			row.TokenLimitStatus = &tokenLimitStatusJSON{
				Limit:          r.TokenLimit,
				ProjectedUsage: r.Projection.TotalTokens,
				PercentUsed:    projectedPercent,
				Status:         status,
			}
		}
		outRows = append(outRows, row)
	}
	return writeJSON(w, map[string]any{"blocks": outRows}, opts)
}

func writeJSON(w io.Writer, v any, opts Options) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	if opts.JQ == "" {
		_, err := w.Write(buf.Bytes())
		return err
	}
	path, err := exec.LookPath("jq")
	if err != nil {
		return fmt.Errorf("--jq requires jq executable in PATH: %w", err)
	}
	cmd := exec.Command(path, opts.JQ)
	cmd.Stdin = bytes.NewReader(buf.Bytes())
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("jq failed: %s", bytes.TrimSpace(stderr.Bytes()))
		}
		return fmt.Errorf("jq failed: %w", err)
	}
	return nil
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func formatJSONDate(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02")
}

func formatBlockID(t time.Time, isGap bool) string {
	id := formatRFC3339MillisUTC(t)
	if isGap {
		return "gap-" + id
	}
	return id
}

func formatRFC3339MillisUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
