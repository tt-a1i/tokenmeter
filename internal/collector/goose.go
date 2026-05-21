package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	gooseEnvVar     = "GOOSE_PATH_ROOT"
	gooseDBFileName = "sessions.db"
	gooseSource     = "goose"
)

// gooseSessionQuery is borrowed verbatim from ccusage v20
// (rust/crates/ccusage/src/adapter/goose.rs::GOOSE_SESSION_QUERY).
// Goose CLI stores one row per session with cumulative token counters
// updated on every model turn; the "accumulated_*" columns are the
// live-running totals and "input_tokens / output_tokens / total_tokens"
// are the per-turn snapshots that may or may not be populated depending
// on Goose's release. Both halves are SELECTed so the row mapper can
// prefer accumulated and fall back to per-turn when accumulated is zero.
const gooseSessionQuery = `
SELECT
    id,
    model_config_json,
    provider_name,
    created_at,
    total_tokens,
    input_tokens,
    output_tokens,
    accumulated_total_tokens,
    accumulated_input_tokens,
    accumulated_output_tokens
FROM sessions
WHERE model_config_json IS NOT NULL
    AND TRIM(model_config_json) != ''
`

// LoadGooseEntries scans Goose CLI SQLite session databases under the
// configured Goose data root and returns one UsageEntry per session row.
//
// Path discovery (mirrors ccusage goose.rs::goose_db_paths):
//
//   - $GOOSE_PATH_ROOT set: the single candidate is
//     $GOOSE_PATH_ROOT/data/sessions/sessions.db.
//   - Otherwise three platform-default candidates are tried, in order:
//     $HOME/.local/share/goose/sessions/sessions.db        (Linux)
//     $HOME/Library/Application Support/goose/sessions/sessions.db (macOS)
//     $HOME/.local/share/Block/goose/sessions/sessions.db  (Block fork)
//
// Candidates are EvalSymlinks-resolved and dedup'd so two paths pointing
// at the same file (via symlink) aren't queried twice. A missing or
// unopenable DB is logged and skipped — never fatal for this source.
// An empty path set yields (nil, nil) so the AllSource merge loop
// treats the source as "user does not have Goose installed".
//
// Row mapping (ccusage goose.rs::row_to_entry):
//
//   - id                            -> SessionID + entry dedup key
//   - model_config_json["model_name"] -> Model (row dropped if missing)
//   - created_at                    -> Timestamp (multi-format; see
//                                       parseGooseTimestamp)
//   - input_tokens (preferred from
//     accumulated_input_tokens then
//     input_tokens)                 -> InputTokens
//   - output_tokens (same prefer)   -> OutputTokens
//   - total - (input + output)
//     when total > input+output     -> folded INTO OutputTokens so the
//                                       AllSource daily/weekly/monthly
//                                       view matches ccusage's report
//                                       total. Goose's "reasoning"
//                                       remainder is recorded by ccusage
//                                       as extra_total_tokens; v1.1's
//                                       UsageEntry doesn't carry a
//                                       reasoning slot yet, so the
//                                       cheapest equivalent is to add
//                                       it onto Output. Documented as a
//                                       v1.1 behavior; v1.1.x may grow
//                                       UsageEntry.ReasoningTokens.
//   - provider_name                 -> intentionally unused here. The
//                                       v1.1 UsageEntry has no provider
//                                       slot; pricing.Resolve picks up
//                                       the bare model name under
//                                       ModeAuto. Goose's per-provider
//                                       prefix lookup (e.g.
//                                       "anthropic/claude-sonnet-4")
//                                       is deferred to v1.1.x pricing
//                                       enhancements.
//
// CostUSD is intentionally left at 0 — Goose's SQLite schema does not
// store cost, and ccusage recomputes it from pricing. The AllSource
// ModeAuto fallback handles that recompute via pricing.Resolve(model).
func LoadGooseEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	dbPaths := gooseDBPaths()
	if len(dbPaths) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	var entries []UsageEntry
	for _, p := range dbPaths {
		rows, err := loadGooseFromDB(ctx, p)
		if err != nil {
			log.Printf("collector/goose: skipping %s: %v", p, err)
			continue
		}
		for _, e := range rows {
			if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
				continue
			}
			if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
				continue
			}
			key := p + "|" + e.SessionID
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

func gooseDBPaths() []string {
	var candidates []string
	if env := strings.TrimSpace(os.Getenv(gooseEnvVar)); env != "" {
		candidates = []string{filepath.Join(env, "data", "sessions", gooseDBFileName)}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		candidates = []string{
			filepath.Join(home, ".local", "share", "goose", "sessions", gooseDBFileName),
			filepath.Join(home, "Library", "Application Support", "goose", "sessions", gooseDBFileName),
			filepath.Join(home, ".local", "share", "Block", "goose", "sessions", gooseDBFileName),
		}
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, p := range candidates {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

func loadGooseFromDB(ctx context.Context, path string) ([]UsageEntry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, gooseSessionQuery)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []UsageEntry
	for rows.Next() {
		var (
			id          string
			modelConfig string
			provider    sql.NullString
			createdAt   sql.NullString
			totalRaw    sql.NullInt64
			inputRaw    sql.NullInt64
			outputRaw   sql.NullInt64
			totalAcc    sql.NullInt64
			inputAcc    sql.NullInt64
			outputAcc   sql.NullInt64
		)
		if err := rows.Scan(&id, &modelConfig, &provider, &createdAt,
			&totalRaw, &inputRaw, &outputRaw,
			&totalAcc, &inputAcc, &outputAcc); err != nil {
			continue
		}
		model := parseGooseModel(modelConfig)
		if model == "" {
			continue
		}
		ts, ok := parseGooseTimestamp(createdAt.String)
		if !ok {
			continue
		}
		input := preferGoosePositive(inputAcc, inputRaw)
		output := preferGoosePositive(outputAcc, outputRaw)
		total := preferGoosePositive(totalAcc, totalRaw)
		if total == 0 {
			total = input + output
		}
		if input == 0 && output == 0 && total == 0 {
			continue
		}
		// Fold "reasoning remainder" (total - input - output) into
		// OutputTokens so the AllSource report totalTokens matches
		// ccusage's report total.
		if total > input+output {
			output += total - (input + output)
		}
		out = append(out, UsageEntry{
			Source:       gooseSource,
			SessionID:    id,
			Timestamp:    ts,
			Model:        model,
			InputTokens:  input,
			OutputTokens: output,
		})
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iter: %w", err)
	}
	return out, nil
}

// preferGoosePositive returns the first column whose value is > 0.
// ccusage's read_token_value treats 0 / NULL as "missing", so the
// accumulated columns are preferred when populated; the per-turn
// columns are the fallback when accumulated is unset.
func preferGoosePositive(primary, fallback sql.NullInt64) int64 {
	if primary.Valid && primary.Int64 > 0 {
		return primary.Int64
	}
	if fallback.Valid && fallback.Int64 > 0 {
		return fallback.Int64
	}
	return 0
}

func parseGooseModel(jsonText string) string {
	if jsonText == "" {
		return ""
	}
	var raw struct {
		ModelName string `json:"model_name"`
	}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.ModelName)
}

// parseGooseTimestamp handles every Goose schema variant ccusage
// supports: integer epoch (seconds or milliseconds), full RFC3339
// (with or without sub-second precision), the "YYYY-MM-DD HH:MM:SS"
// space-separated variant Goose uses by default, and the bare
// "YYYY-MM-DD" date when a row only has day granularity.
func parseGooseTimestamp(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		// ccusage: values > 1e12 are already milliseconds; smaller
		// values are seconds.
		if n > 1_000_000_000_000 {
			return time.UnixMilli(n).UTC(), n > 0
		}
		if n > 0 {
			return time.Unix(n, 0).UTC(), true
		}
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC(), true
	}
	if len(trimmed) == 19 {
		if t, err := time.Parse("2006-01-02 15:04:05", trimmed); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse("2006-01-02T15:04:05", trimmed); err == nil {
			return t.UTC(), true
		}
	}
	if len(trimmed) == 10 {
		if t, err := time.Parse("2006-01-02", trimmed); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
