package statusline

import (
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

// Run reads input JSON from `in`, looks up the active block via `reader`,
// renders into `out`, all keyed off `now`. Any error is returned; on success
// the line has already been written.
func Run(ctx context.Context, in io.Reader, out io.Writer, reader ActiveBlockReader, cfg Config, now time.Time) error {
	input, err := ParseInput(in)
	if err != nil {
		return err
	}
	block, err := reader.LoadActive(ctx)
	if err != nil {
		return err
	}
	return Render(out, input, block, cfg, now)
}
