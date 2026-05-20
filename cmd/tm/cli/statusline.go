package cli

import (
	"context"
	"io"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
)

// StatuslineReader is the loader contract the statusline subcommand needs.
// It exists as a thin package-local alias for statusline.ActiveBlockReader so
// callers in cmd/tm don't need to import internal/statusline directly.
type StatuslineReader interface {
	statusline.ActiveBlockReader
}

// RunStatusline loads the optional config file at configPath and hands the
// stdin → block lookup → stdout pipeline off to internal/statusline.Run.
func RunStatusline(ctx context.Context, in io.Reader, out io.Writer, reader StatuslineReader, configPath string, now time.Time) error {
	cfg, err := statusline.LoadConfig(configPath)
	if err != nil {
		return err
	}
	return statusline.Run(ctx, in, out, reader, cfg, now)
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
