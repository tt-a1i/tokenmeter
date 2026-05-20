package blocks

import "time"

// WithBurnAndProjection returns a copy of b with BurnRate and Projection
// populated when b is active. For non-active or gap blocks, returns b
// unchanged. Use `now` as the moment of computation (injection point for
// tests; pass time.Now() in production).
func WithBurnAndProjection(b SessionBlock, now time.Time) SessionBlock {
	if !b.IsActive || b.IsGap || b.ActualEnd == nil {
		return b
	}
	elapsed := b.ActualEnd.Sub(b.StartTime)
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
	remaining := b.EndTime.Sub(now)
	if remaining < 0 {
		remaining = 0
	}
	projectedTokens := b.Tokens.Total() + int64(tokensPerMinute*remaining.Minutes())
	projectedCost := b.Cost + costPerHour*remaining.Hours()
	b.Projection = &Projection{
		TotalTokens:   projectedTokens,
		TotalCost:     projectedCost,
		RemainingTime: remaining,
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
