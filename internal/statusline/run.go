package statusline

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

// ActiveBlockReader is the subset of services the statusline command needs.
// It exists to keep this package decoupled from internal/blocks load helpers
// at test time.
type ActiveBlockReader interface {
	LoadActive(ctx context.Context) (*blocks.SessionBlock, error)
}

type CostReader interface {
	LoadSessionCost(ctx context.Context, sessionID string) (float64, bool, error)
	LoadTodayCost(ctx context.Context, now time.Time, loc *time.Location) (float64, error)
}

// Run reads input JSON from `in`, looks up the active block via `reader`,
// renders into `out`, all keyed off `now`. Any error is returned; on success
// the line has already been written.
func Run(ctx context.Context, in io.Reader, out io.Writer, reader ActiveBlockReader, cfg Config, now time.Time) error {
	return RunWithOptions(ctx, in, out, reader, cfg, now, RunOptions{})
}

// RunOptions controls statusline runtime behavior that is orthogonal to
// rendering. Zero values preserve the historical uncached Run behavior.
type RunOptions struct {
	CacheEnabled    bool
	CachePath       string
	RefreshInterval time.Duration
	Debug           bool
	DebugWriter     io.Writer
	CostSource      string
	Timezone        string
}

// RunWithOptions reads input JSON from `in`, optionally serves a fresh cached
// statusline, otherwise loads the active block, renders it, and writes it back
// to the cache.
func RunWithOptions(ctx context.Context, in io.Reader, out io.Writer, reader ActiveBlockReader, cfg Config, now time.Time, opts RunOptions) error {
	input, err := ParseInput(in)
	if err != nil {
		return err
	}
	cfg = normalizeConfig(cfg)

	var cachePath string
	var initialCache *statuslineCache
	if opts.CacheEnabled {
		cachePath = opts.CachePath
		if cachePath == "" {
			cachePath = DefaultCachePath(input.SessionID)
		}
		interval := opts.RefreshInterval
		if interval <= 0 {
			interval = time.Second
		}
		transcriptMTime := transcriptMTimeMillis(input.TranscriptPath)
		inputHash := statuslineInputHash(input, cfg, opts)
		cache, err := readStatuslineCache(cachePath)
		if err != nil {
			debugStatuslineCache(opts, "read failed path=%s err=%v", cachePath, err)
		}
		initialCache = cache
		if cache != nil {
			if reason := statuslineCacheMissReason(cache, input, transcriptMTime, inputHash, now, interval); reason == "" {
				debugStatuslineCache(opts, "hit path=%s session=%s", cachePath, input.SessionID)
				_, err := io.WriteString(out, cache.LastOutput)
				return err
			} else {
				debugStatuslineCache(opts, "stale path=%s session=%s reason=%s", cachePath, input.SessionID, reason)
			}
		} else {
			debugStatuslineCache(opts, "miss path=%s session=%s reason=empty", cachePath, input.SessionID)
		}
	}

	block, err := reader.LoadActive(ctx)
	if err != nil {
		if opts.CacheEnabled && initialCache != nil && initialCache.LastOutput != "" {
			debugStatuslineCache(opts, "using stale output after load error path=%s err=%v", cachePath, err)
			_, _ = io.WriteString(out, initialCache.LastOutput)
		}
		return err
	}
	var buf bytes.Buffer
	metrics, err := loadMetrics(ctx, reader, input, block, now, opts)
	if err != nil {
		if opts.CacheEnabled && initialCache != nil && initialCache.LastOutput != "" {
			debugStatuslineCache(opts, "using stale output after metric error path=%s err=%v", cachePath, err)
			_, _ = io.WriteString(out, initialCache.LastOutput)
		}
		return err
	}
	if err := RenderWithMetrics(&buf, input, block, cfg, now, metrics); err != nil {
		if opts.CacheEnabled && initialCache != nil && initialCache.LastOutput != "" {
			debugStatuslineCache(opts, "using stale output after render error path=%s err=%v", cachePath, err)
			_, _ = io.WriteString(out, initialCache.LastOutput)
		}
		return err
	}
	if _, err := out.Write(buf.Bytes()); err != nil {
		return err
	}
	if opts.CacheEnabled {
		cache := newStatuslineCache(input, cfg, opts, buf.String(), transcriptMTimeMillis(input.TranscriptPath), now)
		if err := writeStatuslineCache(cachePath, cache); err != nil {
			debugStatuslineCache(opts, "write failed path=%s err=%v", cachePath, err)
		} else {
			debugStatuslineCache(opts, "write path=%s session=%s", cachePath, input.SessionID)
		}
	}
	return nil
}

func loadMetrics(ctx context.Context, reader ActiveBlockReader, input Input, block *blocks.SessionBlock, now time.Time, opts RunOptions) (Metrics, error) {
	source := opts.CostSource
	if source == "" {
		source = "auto"
	}
	metrics := Metrics{CostSource: source}
	loc := time.UTC
	if opts.Timezone != "" {
		parsed, err := time.LoadLocation(opts.Timezone)
		if err != nil {
			return Metrics{}, err
		}
		loc = parsed
	}
	if cr, ok := reader.(CostReader); ok {
		if input.SessionID != "" {
			cost, found, err := cr.LoadSessionCost(ctx, input.SessionID)
			if err != nil {
				return Metrics{}, err
			}
			if found {
				metrics.CCUsageSessionCost = &cost
			}
		}
		today, err := cr.LoadTodayCost(ctx, now, loc)
		if err != nil {
			return Metrics{}, err
		}
		metrics.TodayCost = today
	}
	if metrics.CCUsageSessionCost == nil && block != nil && block.IsActive {
		metrics.CCUsageSessionCost = &block.Cost
	}
	return metrics, nil
}
