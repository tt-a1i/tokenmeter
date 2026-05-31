package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/projectalias"
	"github.com/tt-a1i/tokenmeter/internal/render"
	"github.com/tt-a1i/tokenmeter/internal/storage"
)

type SessionArgs struct {
	Shared    Shared
	Platform  string
	SessionID string // optional; "" lists all
	Detail    bool   // true when requested via --id/-i
}

// renderOpts builds render.Options from Shared, including the color
// detection that depends on the actual output writer. Shared by RunSession,
// RunBlocks, and RunAggregate.
func renderOpts(s Shared, w io.Writer) render.Options {
	return render.Options{
		JSON:      s.JSON || s.JQ != "",
		Breakdown: s.Breakdown,
		Color:     render.Resolve(s.JSON, s.NoColor, w),
		Compact:   s.Compact,
		Instances: s.Instances,
		JQ:        s.JQ,
	}
}

func RunSession(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
	if a.Detail {
		return runSessionDetail(ctx, w, a, loader)
	}

	ul, ok := loader.(AggregateUsageLoader)
	if !ok {
		return fmt.Errorf("session loader %T does not implement AggregateUsage; rebuild against storage v1.0.2", loader)
	}

	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}

	// Note: timezone offset uses the offset at query time, which may
	// mis-bucket historical data that crossed a DST boundary. Known
	// limitation; acceptable for the typical ccusage workflow.
	loc := time.UTC
	if a.Shared.Timezone != "" {
		parsed, err := time.LoadLocation(a.Shared.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", a.Shared.Timezone, err)
		}
		loc = parsed
	}

	mode := pricing.ParseMode(a.Shared.Mode)
	// Same fix as RunAggregate: ModeAuto without --breakdown silently drops
	// per-model zero-cost shares when the bucket SUM > 0. Force storage to
	// return per-(sessionId, model) rows so each can independently trigger
	// the fallback, then fold them back into one row per session for
	// rendering. See convertSessionRows foldForAuto branch.
	needAutoFallback := mode == pricing.ModeAuto && !a.Shared.Breakdown

	filter := storage.AggregateFilter{
		Since:     since,
		Until:     until,
		Project:   a.Shared.Project,
		Platform:  a.Platform,
		Bucket:    storage.BucketSession,
		Breakdown: a.Shared.Breakdown || needAutoFallback,
		Location:  loc,
	}
	aggRows, err := ul.AggregateUsage(ctx, filter)
	if err != nil {
		return err
	}

	rows := convertSessionRows(aggRows, a.Shared.Breakdown, mode, a.SessionID, needAutoFallback)
	if a.Shared.ProjectAliases != "" {
		aliases, err := projectalias.Load(a.Shared.ProjectAliases)
		if err != nil {
			return err
		}
		for i := range rows {
			rows[i].ProjectPath = aliases.Resolve(rows[i].ProjectPath)
		}
	}

	if a.Shared.Order == "asc" {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Cost < rows[j].Cost })
	} else {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Cost > rows[j].Cost })
	}

	opts := renderOpts(a.Shared, w)
	opts.Location = loc
	return render.New().RenderSessions(w, rows, opts)
}

type sessionDetailEntryJSON struct {
	Timestamp           time.Time `json:"timestamp"`
	InputTokens         int64     `json:"inputTokens"`
	OutputTokens        int64     `json:"outputTokens"`
	CacheCreationTokens int64     `json:"cacheCreationTokens"`
	CacheReadTokens     int64     `json:"cacheReadTokens"`
	Model               string    `json:"model"`
	CostUSD             float64   `json:"costUSD"`
}

type sessionDetailJSON struct {
	SessionID   string                   `json:"sessionId"`
	TotalCost   float64                  `json:"totalCost"`
	TotalTokens int64                    `json:"totalTokens"`
	Entries     []sessionDetailEntryJSON `json:"entries"`
}

func runSessionDetail(ctx context.Context, w io.Writer, a SessionArgs, loader AggregateLoader) error {
	since, err := parseDateFlag(a.Shared.Since)
	if err != nil {
		return err
	}
	until, err := ParseDateFlagUntil(a.Shared.Until)
	if err != nil {
		return err
	}
	entries, err := listUsageForBlocks(ctx, loader, since, until, a.Shared.Project, a.Platform)
	if err != nil {
		return err
	}
	entries = applyPricingMode(entries, pricing.ParseMode(a.Shared.Mode))
	filtered := entries[:0]
	for _, e := range entries {
		if e.SessionID == a.SessionID {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Timestamp.Before(filtered[j].Timestamp) })
	wantsJSON := a.Shared.JSON || a.Shared.JQ != ""
	if len(filtered) == 0 {
		if wantsJSON {
			return writeSessionJSON(w, nil, a.Shared.JQ)
		}
		fmt.Fprintf(w, "No session found with ID: %s\n", a.SessionID)
		return nil
	}

	detail := sessionDetailJSON{
		SessionID: a.SessionID,
		Entries:   make([]sessionDetailEntryJSON, 0, len(filtered)),
	}
	for _, e := range filtered {
		total := e.InputTokens + e.OutputTokens + e.CacheCreationInputTokens + e.CacheReadInputTokens
		detail.TotalTokens += total
		detail.TotalCost += e.CostUSD
		model := e.Model
		if model == "" {
			model = "unknown"
		}
		detail.Entries = append(detail.Entries, sessionDetailEntryJSON{
			Timestamp:           e.Timestamp,
			InputTokens:         e.InputTokens,
			OutputTokens:        e.OutputTokens,
			CacheCreationTokens: e.CacheCreationInputTokens,
			CacheReadTokens:     e.CacheReadInputTokens,
			Model:               model,
			CostUSD:             e.CostUSD,
		})
	}
	if wantsJSON {
		return writeSessionJSON(w, detail, a.Shared.JQ)
	}
	fmt.Fprintf(w, "Claude Code Session Usage - %s\n", a.SessionID)
	fmt.Fprintf(w, "Total Cost: $%.2f\n", detail.TotalCost)
	fmt.Fprintf(w, "Total Tokens: %d\n", detail.TotalTokens)
	fmt.Fprintf(w, "Total Entries: %d\n", len(detail.Entries))
	return nil
}

func writeSessionJSON(w io.Writer, v any, jq string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	if jq == "" {
		_, err := w.Write(buf.Bytes())
		return err
	}
	path, err := exec.LookPath("jq")
	if err != nil {
		return fmt.Errorf("--jq requires jq executable in PATH: %w", err)
	}
	cmd := exec.Command(path, jq)
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

// convertSessionRows folds storage rows (one row per (sessionId, model) when
// Breakdown=true, one row per sessionId otherwise) into render rows with
// Breakdown[] populated as needed. Cost mode follows the same rules as
// RunAggregate: Calculate always recomputes; Auto recomputes only when the
// source bucket cost is 0 (Codex zero-cost fallback parity); Display leaves
// cost as SUM-ed by storage. filterID restricts the output to one session
// when SessionArgs.SessionID is set.
//
// foldForAuto mirrors RunAggregate's strategy for ModeAuto without
// --breakdown: per-(sessionId, model) rows come back from storage so each
// independently triggers the zero-cost fallback, then collapse to one
// SessionRow per session (no user-visible Breakdown[]).
func convertSessionRows(in []storage.AggregateUsageRow, breakdown bool, mode pricing.Mode, filterID string, foldForAuto bool) []render.SessionRow {
	if foldForAuto {
		bySession := map[string]*render.SessionRow{}
		order := []string{}
		for _, r := range in {
			if filterID != "" && r.Bucket != filterID {
				continue
			}
			row, ok := bySession[r.Bucket]
			if !ok {
				row = &render.SessionRow{
					SessionID:    r.Bucket,
					ProjectPath:  r.Project,
					LastActivity: r.LastActivity,
				}
				bySession[r.Bucket] = row
				order = append(order, r.Bucket)
			}
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			row.Models = appendUnique(row.Models, r.Model)
			row.InputTokens += r.InputTokens
			row.OutputTokens += r.OutputTokens
			row.CacheCreateTokens += r.CacheCreateTokens
			row.CacheReadTokens += r.CacheReadTokens
			row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
			row.Cost += cost
			if r.LastActivity.After(row.LastActivity) {
				row.LastActivity = r.LastActivity
			}
			// Deliberately NOT populating row.Breakdown.
		}
		out := make([]render.SessionRow, 0, len(order))
		for _, k := range order {
			out = append(out, *bySession[k])
		}
		return out
	}
	if !breakdown {
		out := make([]render.SessionRow, 0, len(in))
		for _, r := range in {
			if filterID != "" && r.Bucket != filterID {
				continue
			}
			cost := r.Cost
			if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
				cost = recalculateCost(r)
			}
			out = append(out, render.SessionRow{
				SessionID:         r.Bucket,
				ProjectPath:       r.Project,
				LastActivity:      r.LastActivity,
				Models:            r.Models,
				InputTokens:       r.InputTokens,
				OutputTokens:      r.OutputTokens,
				CacheCreateTokens: r.CacheCreateTokens,
				CacheReadTokens:   r.CacheReadTokens,
				TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
				Cost:              cost,
			})
		}
		return out
	}
	// Breakdown=true: storage emits one row per (sessionId, model). Fold by
	// sessionId so the render layer sees one SessionRow per session with the
	// per-model rows in Breakdown[].
	bySession := map[string]*render.SessionRow{}
	order := []string{}
	for _, r := range in {
		if filterID != "" && r.Bucket != filterID {
			continue
		}
		row, ok := bySession[r.Bucket]
		if !ok {
			row = &render.SessionRow{
				SessionID:    r.Bucket,
				ProjectPath:  r.Project,
				LastActivity: r.LastActivity,
			}
			bySession[r.Bucket] = row
			order = append(order, r.Bucket)
		}
		cost := r.Cost
		if mode == pricing.ModeCalculate || (mode == pricing.ModeAuto && cost == 0) {
			cost = recalculateCost(r)
		}
		row.Models = appendUnique(row.Models, r.Model)
		row.InputTokens += r.InputTokens
		row.OutputTokens += r.OutputTokens
		row.CacheCreateTokens += r.CacheCreateTokens
		row.CacheReadTokens += r.CacheReadTokens
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreateTokens + row.CacheReadTokens
		row.Cost += cost
		if r.LastActivity.After(row.LastActivity) {
			row.LastActivity = r.LastActivity
		}
		row.Breakdown = append(row.Breakdown, render.ModelBreakdown{
			Model:             r.Model,
			InputTokens:       r.InputTokens,
			OutputTokens:      r.OutputTokens,
			CacheCreateTokens: r.CacheCreateTokens,
			CacheReadTokens:   r.CacheReadTokens,
			TotalTokens:       r.InputTokens + r.OutputTokens + r.CacheCreateTokens + r.CacheReadTokens,
			Cost:              cost,
		})
	}
	out := make([]render.SessionRow, 0, len(order))
	for _, k := range order {
		out = append(out, *bySession[k])
	}
	return out
}
