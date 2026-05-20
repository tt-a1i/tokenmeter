// Package render emits aggregate / session / block reports in either
// boxed-table or JSON form, with column layout and field naming aligned
// to ccusage v20.
package render

import (
	"fmt"
	"io"
	"time"
)

// AggregateRow is one row of daily/weekly/monthly output.
type AggregateRow struct {
	Bucket            string // "2026-05-19" / "2026-W21" / "2026-05"
	Models            []string
	InputTokens       int64
	OutputTokens      int64
	CacheCreateTokens int64
	CacheReadTokens   int64
	TotalTokens       int64
	Cost              float64
	Breakdown         []ModelBreakdown
}

// ModelBreakdown is the per-model substructure under one AggregateRow / SessionRow.
type ModelBreakdown struct {
	Model             string
	InputTokens       int64
	OutputTokens      int64
	CacheCreateTokens int64
	CacheReadTokens   int64
	TotalTokens       int64
	Cost              float64
}

// SessionRow mirrors AggregateRow but is keyed by session id and carries
// last-activity and project-path metadata.
type SessionRow struct {
	SessionID         string
	ProjectPath       string
	LastActivity      time.Time
	Models            []string
	InputTokens       int64
	OutputTokens      int64
	CacheCreateTokens int64
	CacheReadTokens   int64
	TotalTokens       int64
	Cost              float64
	Breakdown         []ModelBreakdown
}

// BlockRow mirrors blocks.SessionBlock in render-layer-friendly form.
type BlockRow struct {
	Period            string // "2026-05-19 10:00"
	Models            []string
	InputTokens       int64
	OutputTokens      int64
	CacheCreateTokens int64
	CacheReadTokens   int64
	TotalTokens       int64
	Cost              float64
	Status            string // "ACTIVE" | "closed" | "gap"
	Projection        *BlockProjection
}

// BlockProjection holds extrapolated totals for an active block.
type BlockProjection struct {
	TotalTokens   int64
	TotalCost     float64
	RemainingTime time.Duration
}

// Options controls rendering knobs.
type Options struct {
	Color     bool
	Breakdown bool
	JSON      bool
}

// Renderer is the entrypoint used by cli/* handlers.
type Renderer interface {
	RenderAggregate(w io.Writer, kind string, rows []AggregateRow, opts Options) error
	RenderSessions(w io.Writer, rows []SessionRow, opts Options) error
	RenderBlocks(w io.Writer, rows []BlockRow, opts Options) error
}

// New returns a Renderer that dispatches to either boxed-table or JSON
// output based on opts.JSON.
func New() Renderer { return defaultRenderer{} }

type defaultRenderer struct{}

// RenderSessions / RenderBlocks dispatch the JSON path immediately (Task 9
// landed the camelCase encoders); the boxed-table arms still stub pending
// cli/{session,blocks}.go integration in Task 11 / 12.

func (d defaultRenderer) RenderSessions(w io.Writer, rows []SessionRow, opts Options) error {
	if opts.JSON {
		return d.renderSessionsJSON(w, rows)
	}
	return fmt.Errorf("render: RenderSessions boxed table not implemented yet (Task 11)")
}

func (d defaultRenderer) RenderBlocks(w io.Writer, rows []BlockRow, opts Options) error {
	if opts.JSON {
		return d.renderBlocksJSON(w, rows)
	}
	return fmt.Errorf("render: RenderBlocks boxed table not implemented yet (Task 12)")
}
