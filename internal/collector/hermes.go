package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	hermesEnvVar     = "HERMES_HOME"
	hermesDBFileName = "state.db"
	hermesSource     = "hermes"
)

// hermesSessionQuery is borrowed verbatim from ccusage v20
// (rust/crates/ccusage/src/adapter/hermes.rs read_session_row's prepared
// statement). Hermes stores one row per session in a single sessions
// table, with both estimated and actual costs that fall back to each
// other (see CostUSD resolution below).
const hermesSessionQuery = `
SELECT
    id,
    model,
    billing_provider,
    started_at,
    message_count,
    input_tokens,
    output_tokens,
    cache_read_tokens,
    cache_write_tokens,
    reasoning_tokens,
    estimated_cost_usd,
    actual_cost_usd
FROM sessions
WHERE model IS NOT NULL
    AND TRIM(model) != ''
`

// LoadHermesEntries scans Hermes CLI state databases for billable session
// rows. One row per session contributes one UsageEntry.
//
// Path discovery (mirrors ccusage hermes.rs::hermes_state_db_paths):
//
//   - $HERMES_HOME set: comma-separated list of home directories; each
//     candidate is <home>/state.db.
//   - Otherwise the single default $HOME/.hermes/state.db.
//
// Each candidate is EvalSymlinks-resolved and dedup'd by canonical path.
// Missing/unopenable DBs are logged-and-skipped per the AllSource
// adapter contract; (nil, nil) returns when Hermes is not installed.
//
// Row mapping (ccusage hermes.rs::read_session_row):
//
//	id                                -> SessionID (row dropped if empty)
//	model                             -> Model (trimmed; row dropped if
//	                                     empty)
//	billing_provider                  -> READ but not surfaced; v1.1's
//	                                     UsageEntry has no provider slot
//	                                     and Hermes' own cost columns
//	                                     supersede pricing recompute in
//	                                     the typical case.
//	started_at (REAL unix seconds or
//	            milliseconds)         -> Timestamp (values > 1e12 are
//	                                     treated as ms; otherwise seconds
//	                                     and converted)
//	input_tokens                      -> InputTokens
//	output_tokens                     -> OutputTokens
//	cache_read_tokens                 -> CacheReadInputTokens
//	cache_write_tokens                -> CacheCreationInputTokens
//	                                     (Hermes' "write" maps to
//	                                     ccusage's "creation")
//	reasoning_tokens                  -> folded INTO OutputTokens
//	                                     (same v1.1 simplification used
//	                                     by goose.go — UsageEntry has no
//	                                     reasoning slot)
//	actual_cost_usd OR
//	  estimated_cost_usd              -> CostUSD (actual wins when
//	                                     populated; estimated is fallback;
//	                                     both NULL leaves CostUSD=0 so the
//	                                     AllSource ModeAuto can recompute)
//
// Skip condition: all tokens + reasoning + both cost columns are zero/NULL.
//
// CostUSD policy: Hermes is a passthrough adapter (like OpenCode) — its
// stored cost number reflects the real billing endpoint and is preferred
// over our embedded LiteLLM snapshot. When both cost columns are NULL,
// CostUSD=0 lets the AllSource ModeAuto fallback recompute via
// pricing.Resolve(model). Bare model names (claude-sonnet-..., gemini-...)
// typically hit the LiteLLM map; the ccusage anthropic/claude-... provider
// prefix candidate is deferred to v1.1.x pricing enhancements.
func LoadHermesEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	dbPaths := hermesDBPaths()
	if len(dbPaths) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	var entries []UsageEntry
	for _, p := range dbPaths {
		rows, err := loadHermesFromDB(ctx, p)
		if err != nil {
			log.Printf("collector/hermes: skipping %s: %v", p, err)
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

func hermesDBPaths() []string {
	var homes []string
	if env := strings.TrimSpace(os.Getenv(hermesEnvVar)); env != "" {
		for _, raw := range strings.Split(env, ",") {
			p := strings.TrimSpace(raw)
			if p != "" {
				homes = append(homes, p)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		homes = []string{filepath.Join(home, ".hermes")}
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, h := range homes {
		p := filepath.Join(h, hermesDBFileName)
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

func loadHermesFromDB(ctx context.Context, path string) ([]UsageEntry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, hermesSessionQuery)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []UsageEntry
	for rows.Next() {
		var (
			id         string
			model      string
			provider   sql.NullString
			startedAt  sql.NullFloat64
			msgCount   sql.NullInt64
			inputTok   sql.NullInt64
			outputTok  sql.NullInt64
			cacheRead  sql.NullInt64
			cacheWrite sql.NullInt64
			reasoning  sql.NullInt64
			estimated  sql.NullFloat64
			actual     sql.NullFloat64
		)
		if err := rows.Scan(&id, &model, &provider, &startedAt, &msgCount,
			&inputTok, &outputTok, &cacheRead, &cacheWrite, &reasoning,
			&estimated, &actual); err != nil {
			continue
		}
		// provider intentionally unused; see header comment.
		_ = provider
		_ = msgCount

		id = strings.TrimSpace(id)
		model = strings.TrimSpace(model)
		if id == "" || model == "" {
			continue
		}
		ts, ok := hermesTimestamp(startedAt)
		if !ok {
			continue
		}
		input := hermesPositiveInt(inputTok)
		output := hermesPositiveInt(outputTok)
		cRead := hermesPositiveInt(cacheRead)
		cWrite := hermesPositiveInt(cacheWrite)
		reason := hermesPositiveInt(reasoning)
		cost := hermesPreferredCost(actual, estimated)
		if input == 0 && output == 0 && cRead == 0 && cWrite == 0 && reason == 0 && cost == 0 {
			continue
		}
		// Fold reasoning into Output (UsageEntry has no reasoning slot in
		// v1.1 — same simplification goose.go uses).
		output += reason
		out = append(out, UsageEntry{
			Source:                   hermesSource,
			SessionID:                id,
			Timestamp:                ts,
			Model:                    model,
			InputTokens:              input,
			OutputTokens:             output,
			CacheCreationInputTokens: cWrite,
			CacheReadInputTokens:     cRead,
			CostUSD:                  cost,
		})
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iter: %w", err)
	}
	return out, nil
}

func hermesPositiveInt(v sql.NullInt64) int64 {
	if v.Valid && v.Int64 > 0 {
		return v.Int64
	}
	return 0
}

// hermesPreferredCost mirrors ccusage's `actual_cost.or(estimated_cost)`:
// actual wins when populated and non-negative; estimated is the fallback.
// Negative values (corrupt rows) collapse to 0 so the ModeAuto recompute
// can still surface a meaningful number from pricing.
func hermesPreferredCost(actual, estimated sql.NullFloat64) float64 {
	if c, ok := hermesFinitePositive(actual); ok {
		return c
	}
	if c, ok := hermesFinitePositive(estimated); ok {
		return c
	}
	return 0
}

func hermesFinitePositive(v sql.NullFloat64) (float64, bool) {
	if !v.Valid {
		return 0, false
	}
	if math.IsNaN(v.Float64) || math.IsInf(v.Float64, 0) || v.Float64 <= 0 {
		return 0, false
	}
	return v.Float64, true
}

// hermesTimestamp converts started_at (stored as REAL) to a UTC Time.
// ccusage's heuristic: values > 1e12 are already milliseconds; smaller
// floats are unix seconds with sub-second precision.
func hermesTimestamp(v sql.NullFloat64) (time.Time, bool) {
	if !v.Valid {
		return time.Time{}, false
	}
	f := v.Float64
	if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return time.Time{}, false
	}
	var millis float64
	if f > 1e12 {
		millis = f
	} else {
		millis = f * 1000
	}
	secs := int64(millis / 1000)
	nanos := int64(math.Mod(millis, 1000) * 1e6)
	return time.Unix(secs, nanos).UTC(), true
}
