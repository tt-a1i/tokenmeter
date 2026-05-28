package storage

import (
	"context"
	"fmt"
)

// DeleteTokenUsageBySourceID removes a token_usage row by source_id.
// Missing source IDs are a no-op so replay reconciliation can be idempotent.
func (s *DB) DeleteTokenUsageBySourceID(ctx context.Context, sourceID string) error {
	if sourceID == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM token_usage WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("delete token usage by source id: %w", err)
	}
	return nil
}
