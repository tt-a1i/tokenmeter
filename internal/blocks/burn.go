package blocks

import (
	"math"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// WithBurnAndProjection returns a copy of b with BurnRate and Projection
// populated when b is active. For non-active or gap blocks, returns b
// unchanged. Use `now` as the moment of computation (injection point for
// tests; pass time.Now() in production).
func WithBurnAndProjection(b SessionBlock, now time.Time) SessionBlock {
	if !b.IsActive || b.IsGap || b.ActualEnd == nil {
		return b
	}
	first := b.StartTime
	if b.FirstEntry != nil {
		first = *b.FirstEntry
	}
	elapsed := b.ActualEnd.Sub(first)
	if elapsed <= 0 {
		return b
	}
	totalTokens := float64(b.Tokens.Total())
	tokensPerMinute := totalTokens / elapsed.Minutes()
	costPerHour := b.Cost / elapsed.Hours()
	b.BurnRate = &BurnRate{
		TokensPerMinute: tokensPerMinute,
		CostPerHour:     costPerHour,
	}
	remainingMinutes := math.Round(b.EndTime.Sub(now).Minutes())
	if remainingMinutes < 0 {
		remainingMinutes = 0
	}
	projectedTokens := int64(math.Round(totalTokens + tokensPerMinute*remainingMinutes))
	projectedCost := b.Cost + (costPerHour/60.0)*remainingMinutes
	b.Projection = &Projection{
		TotalTokens:   projectedTokens,
		TotalCost:     math.Round(projectedCost*100) / 100,
		RemainingTime: time.Duration(remainingMinutes) * time.Minute,
	}
	return b
}

// Annotate enriches every active block in `in` with BurnRate and Projection.
// Returns a new slice.
func Annotate(in []SessionBlock, now time.Time) []SessionBlock {
	out := make([]SessionBlock, len(in))
	for i, b := range in {
		out[i] = WithBurnAndProjection(b, now)
	}
	return out
}

// PopulatePerModel walks `entries` and accumulates per-model TokenCounts into
// each block's PerModel map. Identify partitions entries into blocks by
// chronological gap, so an entry's timestamp falls within exactly one
// non-gap block's [StartTime, EndTime). Gap blocks stay empty.
//
// Lives here (not identify.go) so the existing Identify/sumTokens hot path
// pays no allocation when callers don't request a breakdown; cli/blocks.go
// calls this only under args.Shared.Breakdown.
func PopulatePerModel(blocks []SessionBlock, entries []storage.TokenUsageEntry) []SessionBlock {
	for _, e := range entries {
		for i := range blocks {
			if blocks[i].IsGap {
				continue
			}
			if e.Timestamp.Before(blocks[i].StartTime) {
				continue
			}
			if !e.Timestamp.Before(blocks[i].EndTime) {
				continue
			}
			if blocks[i].PerModel == nil {
				blocks[i].PerModel = map[string]TokenCounts{}
			}
			pm := blocks[i].PerModel[e.Model]
			pm.Input += e.InputTokens
			pm.Output += e.OutputTokens
			pm.CacheCreate += e.CacheCreationInputTokens
			pm.CacheRead += e.CacheReadInputTokens
			blocks[i].PerModel[e.Model] = pm
			break
		}
	}
	return blocks
}
