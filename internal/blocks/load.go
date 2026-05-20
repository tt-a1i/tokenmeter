package blocks

import (
	"context"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

// Reader is the subset of storage.DB needed by blocks load helpers.
// Defined as an interface so tests can stub it easily.
type Reader interface {
	ListUsageForBlocks(ctx context.Context, since, until time.Time) ([]storage.TokenUsageEntry, error)
}

// LoadAll loads every token_usage entry and returns annotated blocks.
func LoadAll(ctx context.Context, r Reader, sessionDuration time.Duration, now time.Time) ([]SessionBlock, error) {
	entries, err := r.ListUsageForBlocks(ctx, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	return Annotate(Identify(entries, sessionDuration, now), now), nil
}

// LoadActive loads blocks and returns the currently active block, if any.
func LoadActive(ctx context.Context, r Reader, sessionDuration time.Duration, now time.Time) (*SessionBlock, error) {
	all, err := LoadAll(ctx, r, sessionDuration, now)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].IsActive {
			b := all[i]
			return &b, nil
		}
	}
	return nil, nil
}
