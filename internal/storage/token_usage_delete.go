package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// DeleteTokenUsageBySourceID removes a token_usage row by source_id.
// Missing source IDs are a no-op so replay reconciliation can be idempotent.
func (s *DB) DeleteTokenUsageBySourceID(ctx context.Context, sourceID string) error {
	if sourceID == "" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete token usage by source id: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var row struct {
		sessionID           string
		inputTokens         int
		outputTokens        int
		cacheCreationTokens int
		cacheReadTokens     int
		costUSD             float64
		timestamp           string
	}
	err = tx.QueryRowContext(ctx, `
		SELECT session_id, input_tokens, output_tokens, cache_creation_tokens,
		       cache_read_tokens, cost_usd, timestamp
		FROM token_usage
		WHERE source_id = ?
	`, sourceID).Scan(&row.sessionID, &row.inputTokens, &row.outputTokens, &row.cacheCreationTokens, &row.cacheReadTokens, &row.costUSD, &row.timestamp)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("select token usage by source id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM token_usage WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("delete token usage by source id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET
			total_input_tokens = total_input_tokens - ?,
			total_output_tokens = total_output_tokens - ?,
			total_cost_usd = total_cost_usd - ?,
			total_cache_read_tokens = total_cache_read_tokens - ?,
			total_cache_creation_tokens = total_cache_creation_tokens - ?
		WHERE session_id = ?
	`, row.inputTokens, row.outputTokens, row.costUSD, row.cacheReadTokens, row.cacheCreationTokens, row.sessionID); err != nil {
		return fmt.Errorf("rollback session token totals: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE daily_cost_cache
		SET cost_usd = cost_usd - ?
		WHERE day = DATE(?, 'localtime')
	`, row.costUSD, row.timestamp); err != nil {
		return fmt.Errorf("rollback daily cost cache: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete token usage by source id: %w", err)
	}
	return nil
}
