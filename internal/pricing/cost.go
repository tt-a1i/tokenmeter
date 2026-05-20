package pricing

// Speed indicates which provider tier the request used. Codex requests can be
// "standard" or "fast"; Claude requests are always "standard" today, but the
// flag exists for future API tiers.
type Speed int

const (
	SpeedStandard Speed = iota
	SpeedFast
)

// Usage aggregates a single accounting row's token counts.
type Usage struct {
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
}

// CalculateCost computes the dollar cost of `u` under `p`, honoring the
// 200k-context tiered pricing when InputAbove200K is set and the request
// exceeds the threshold.
func CalculateCost(p Pricing, u Usage, speed Speed) float64 {
	mult := 1.0
	if speed == SpeedFast {
		mult = maxF(1.0, p.FastMultiplier)
	}
	inputCost := tieredCost(u.Input, p.Input, p.InputAbove200K)
	outputCost := tieredCost(u.Output, p.Output, p.OutputAbove200K)
	cacheCreateCost := tieredCost(u.CacheCreate, p.CacheCreate, p.CacheCreateAbove200K)
	cacheReadCost := tieredCost(u.CacheRead, p.CacheRead, p.CacheReadAbove200K)
	return (inputCost + outputCost + cacheCreateCost + cacheReadCost) * mult
}

func tieredCost(tokens int64, base float64, above200k *float64) float64 {
	if tokens <= 0 {
		return 0
	}
	const threshold = int64(200000)
	if above200k == nil || tokens <= threshold {
		return float64(tokens) * base
	}
	belowPart := float64(threshold) * base
	abovePart := float64(tokens-threshold) * (*above200k)
	return belowPart + abovePart
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
