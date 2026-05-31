package cli

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	tmconfig "github.com/tt-a1i/tokenmeter/internal/config"
	"github.com/tt-a1i/tokenmeter/internal/pricing"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
	"github.com/tt-a1i/tokenmeter/internal/storage"
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
	ModeSet                bool
	CostSource             string
	Cache                  bool
	NoCache                bool
	RefreshInterval        int
	Debug                  bool
	Timezone               string
	ContextLowThreshold    int
	ContextMediumThreshold int
	BurnRateDisplay        string
	ConfigPath             string
	DebugWriter            io.Writer
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
	mode := statuslineCostMode(o.Mode, o.ModeSet)
	if mode != "" {
		reader = &modeAwareReader{inner: reader, mode: pricing.ParseMode(mode)}
	}
	refreshInterval := time.Duration(o.RefreshInterval) * time.Second
	debugWriter := o.DebugWriter
	if debugWriter == nil && o.Debug {
		debugWriter = os.Stderr
	}
	cacheEnabled := o.Cache || !o.NoCache
	if o.NoCache {
		cacheEnabled = false
	}
	return statusline.RunWithOptions(ctx, in, out, reader, cfg, now, statusline.RunOptions{
		CacheEnabled:    cacheEnabled,
		RefreshInterval: refreshInterval,
		Debug:           o.Debug,
		DebugWriter:     debugWriter,
		CostSource:      defaultString(o.CostSource, "auto"),
		Timezone:        o.Timezone,
	})
}

func statuslineCostMode(mode string, modeSet bool) string {
	if modeSet && mode != "" {
		return mode
	}
	return "auto"
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

func (m *modeAwareReader) LoadSessionCost(ctx context.Context, sessionID string) (float64, bool, error) {
	if er, ok := m.inner.(statuslineEntryReader); ok {
		entries, err := er.ListStatuslineEntries(ctx, time.Time{}, time.Time{})
		if err != nil {
			return 0, false, err
		}
		entries = applyPricingMode(entries, m.mode)
		var sum float64
		found := false
		for _, e := range entries {
			if e.SessionID == sessionID {
				sum += e.CostUSD
				found = true
			}
		}
		return sum, found, nil
	}
	if cr, ok := m.inner.(statusline.CostReader); ok {
		return cr.LoadSessionCost(ctx, sessionID)
	}
	return 0, false, nil
}

func (m *modeAwareReader) LoadTodayCost(ctx context.Context, now time.Time, loc *time.Location) (float64, error) {
	if er, ok := m.inner.(statuslineEntryReader); ok {
		start, end := dayBounds(now, loc)
		entries, err := er.ListStatuslineEntries(ctx, start, end)
		if err != nil {
			return 0, err
		}
		entries = applyPricingMode(entries, m.mode)
		var sum float64
		for _, e := range entries {
			sum += e.CostUSD
		}
		return sum, nil
	}
	if cr, ok := m.inner.(statusline.CostReader); ok {
		return cr.LoadTodayCost(ctx, now, loc)
	}
	return 0, nil
}

// activeBlockAdapter wraps a blocks.Reader so it satisfies StatuslineReader.
type activeBlockAdapter struct {
	r               blocks.Reader
	sessionDuration time.Duration
	now             time.Time
}

type statuslineEntryReader interface {
	ListStatuslineEntries(ctx context.Context, since, until time.Time) ([]storage.TokenUsageEntry, error)
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

func (a *activeBlockAdapter) ListStatuslineEntries(ctx context.Context, since, until time.Time) ([]storage.TokenUsageEntry, error) {
	return a.r.ListUsageForBlocks(ctx, since, until)
}

func (a *activeBlockAdapter) LoadSessionCost(ctx context.Context, sessionID string) (float64, bool, error) {
	entries, err := a.ListStatuslineEntries(ctx, time.Time{}, time.Time{})
	if err != nil {
		return 0, false, err
	}
	var sum float64
	found := false
	for _, e := range entries {
		if e.SessionID == sessionID {
			sum += e.CostUSD
			found = true
		}
	}
	return sum, found, nil
}

func (a *activeBlockAdapter) LoadTodayCost(ctx context.Context, now time.Time, loc *time.Location) (float64, error) {
	start, end := dayBounds(now, loc)
	entries, err := a.ListStatuslineEntries(ctx, start, end)
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, e := range entries {
		sum += e.CostUSD
	}
	return sum, nil
}

func dayBounds(now time.Time, loc *time.Location) (time.Time, time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	startLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return startLocal.UTC(), startLocal.AddDate(0, 0, 1).Add(-time.Nanosecond).UTC()
}
