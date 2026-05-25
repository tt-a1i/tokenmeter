package cli

import (
	"context"
	"io"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	tmconfig "github.com/tt-a1i/tokenmeter/internal/config"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
)

// StatuslineReader is the loader contract the statusline subcommand needs.
// It exists as a thin package-local alias for statusline.ActiveBlockReader so
// callers in cmd/tm don't need to import internal/statusline directly.
type StatuslineReader interface {
	statusline.ActiveBlockReader
}

// StatuslineOptions carries shared-flag overrides that affect the statusline
// render path:
//   - NoColor forces Config.Color = false even if the on-disk config asks
//     for color, so `tm statusline --no-color` always emits a plain line.
//   - Mode mirrors the cost-mode shared flag. The block's Cost is normally
//     pre-aggregated from token_usage.cost_usd, so Mode here is a one-shot
//     post-load recompute hook used when the source rows lacked a cost
//     (e.g. Codex) or the user wants the calculated value to override.
type StatuslineOptions struct {
	NoColor                bool
	Mode                   string
	ContextLowThreshold    int
	ContextMediumThreshold int
	BurnRateDisplay        string
	ConfigPath             string
}

type statuslineOpt func(*StatuslineOptions)

// WithStatuslineOptions overwrites the options block. Designed to be the only
// option helper for now; future per-field With… helpers can compose against
// the same statuslineOpt type.
func WithStatuslineOptions(o StatuslineOptions) statuslineOpt {
	return func(dst *StatuslineOptions) { *dst = o }
}

// RunStatusline loads the optional config file at configPath and hands the
// stdin → block lookup → stdout pipeline off to internal/statusline.Run.
// Options (NoColor / Mode) are resolved here so the internal/statusline
// package stays agnostic to CLI flag plumbing.
func RunStatusline(ctx context.Context, in io.Reader, out io.Writer, reader StatuslineReader, configPath string, now time.Time, opts ...statuslineOpt) error {
	var o StatuslineOptions
	for _, fn := range opts {
		fn(&o)
	}
	cfg, err := statusline.LoadConfig(configPath)
	if err != nil {
		return err
	}
	unified, err := tmconfig.LoadWithOptions(tmconfig.Options{ExplicitPath: o.ConfigPath})
	if err != nil {
		return err
	}
	applyUnifiedStatuslineConfig(&cfg, unified.Statusline)
	if o.NoColor {
		cfg.Color = false
	}
	if o.ContextLowThreshold > 0 {
		cfg.ContextLowThreshold = o.ContextLowThreshold
	}
	if o.ContextMediumThreshold > 0 {
		cfg.ContextMediumThreshold = o.ContextMediumThreshold
	}
	if o.BurnRateDisplay != "" {
		cfg.BurnRateDisplay = o.BurnRateDisplay
	}
	if err := statusline.ValidateConfig(cfg); err != nil {
		return err
	}
	if o.Mode != "" {
		reader = &modeAwareReader{inner: reader, mode: pricing.ParseMode(o.Mode)}
	}
	return statusline.Run(ctx, in, out, reader, cfg, now)
}

func applyUnifiedStatuslineConfig(dst *statusline.Config, src tmconfig.StatuslineConfig) {
	if src.QuotaUSD != 0 {
		dst.QuotaUSD = src.QuotaUSD
	}
	if src.Format != "" {
		dst.Format = src.Format
	}
	switch src.Color {
	case "true", "1", "yes", "on":
		dst.Color = true
	case "false", "0", "no", "off":
		dst.Color = false
	}
	if src.ContextLowThreshold != 0 {
		dst.ContextLowThreshold = src.ContextLowThreshold
	}
	if src.ContextMediumThreshold != 0 {
		dst.ContextMediumThreshold = src.ContextMediumThreshold
	}
	if src.BurnRateDisplay != "" {
		dst.BurnRateDisplay = src.BurnRateDisplay
	}
}

// modeAwareReader wraps a StatuslineReader and rewrites the returned block's
// Cost field according to pricing.Mode. For ModeDisplay it passes through;
// for ModeCalculate it always recomputes from block.Tokens × pricing.Resolve(
// Models[0]); for ModeAuto it only recomputes when block.Cost == 0 (so
// missing Codex costs still surface a value without overriding rows that
// already have one).
//
// Multi-model blocks use Models[0] as a proxy; statusline blocks usually
// span one or two models so the approximation is small. Full per-entry
// rewriting belongs deeper in blocks.LoadActive and is out of scope here.
type modeAwareReader struct {
	inner StatuslineReader
	mode  pricing.Mode
}

func (m *modeAwareReader) LoadActive(ctx context.Context) (*blocks.SessionBlock, error) {
	b, err := m.inner.LoadActive(ctx)
	if err != nil || b == nil {
		return b, err
	}
	if m.mode == pricing.ModeDisplay {
		return b, nil
	}
	if m.mode == pricing.ModeAuto && b.Cost > 0 {
		return b, nil
	}
	if len(b.Models) == 0 {
		return b, nil
	}
	p, ok := pricingMap.Resolve(b.Models[0])
	if !ok {
		return b, nil
	}
	b.Cost = pricing.CalculateCost(p, pricing.Usage{
		Input:       b.Tokens.Input,
		Output:      b.Tokens.Output,
		CacheCreate: b.Tokens.CacheCreate,
		CacheRead:   b.Tokens.CacheRead,
	}, speedForModel(b.Models[0]))
	return b, nil
}

// activeBlockAdapter wraps a blocks.Reader so it satisfies StatuslineReader.
type activeBlockAdapter struct {
	r               blocks.Reader
	sessionDuration time.Duration
	now             time.Time
}

// NewActiveBlockAdapter returns a StatuslineReader that loads the active
// block from a blocks.Reader, fixing sessionDuration and now at construction
// time so the caller can drive the wall clock for tests.
func NewActiveBlockAdapter(r blocks.Reader, sessionDuration time.Duration, now time.Time) StatuslineReader {
	return &activeBlockAdapter{r: r, sessionDuration: sessionDuration, now: now}
}

func (a *activeBlockAdapter) LoadActive(ctx context.Context) (*blocks.SessionBlock, error) {
	return blocks.LoadActive(ctx, a.r, a.sessionDuration, a.now)
}
