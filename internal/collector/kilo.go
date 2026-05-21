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
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	kiloEnvVar     = "KILO_DATA_DIR"
	kiloDBFileName = "kilo.db" // const KILO_DB_FILE_NAME from ccusage kilo.rs:22
	kiloSource     = "kilo"
)

// kiloMessageQuery is borrowed verbatim from ccusage v20
// (rust/crates/ccusage/src/adapter/kilo.rs:179). Kilo stores messages in
// a schemaless table — all model/token/cost data lives inside the JSON
// blob in the `data` column.
const kiloMessageQuery = `SELECT id, session_id, data FROM message`

// LoadKiloEntries scans Kilo CLI databases under the configured data
// directories and returns one UsageEntry per assistant message row.
//
// Path discovery (mirrors ccusage kilo.rs::paths):
//
//   - KILO_DATA_DIR set: comma-separated list of kilo roots, each
//     contributing <root>/kilo.db.
//   - Otherwise the single default $HOME/.local/share/kilo/kilo.db.
//
// Roots are dedup'd; missing/unopenable DBs are logged and skipped per
// the AllSource adapter contract. (nil, nil) when Kilo is not installed.
//
// Schema (verbatim from kilo.rs:410):
//
//	CREATE TABLE message (id TEXT, session_id TEXT, data TEXT)
//
// All meaningful fields live inside the JSON blob in `data`. Per-row
// parse (mirrors kilo.rs::message_value_to_entry):
//
//	role             must equal "assistant"; non-assistant rows dropped
//	modelID          -> Model (row dropped if missing)
//	time.created     -> Timestamp via normalizeKiloTimestamp
//	                    (i64; values < 1e12 are seconds, otherwise ms)
//	tokens.input     -> InputTokens
//	tokens.output    -> OutputTokens
//	tokens.cache.write -> CacheCreationInputTokens
//	tokens.cache.read  -> CacheReadInputTokens
//	tokens.reasoning -> folded INTO OutputTokens
//	                    (UsageEntry has no reasoning slot in v1.1;
//	                    same simplification used by goose.go/hermes.go)
//	tokens.total     -> fallback into OutputTokens when individual parts
//	                    are all zero but total > 0 (ccusage's
//	                    apply_total_token_fallback)
//	session_id (json
//	  fallback to row
//	  session_id column) -> SessionID
//	cost (when > 0)  -> CostUSD (passthrough; mirrors ccusage
//	                    calculate_kilo_cost's auto-mode preference for
//	                    data.cost_usd over recompute)
//
// CostUSD policy matches OpenCode: stored cost wins when positive, else
// 0 lets AllSource ModeAuto recompute via pricing.Resolve(model). The
// ccusage providerID/model candidate explosion is deferred to v1.1.x
// pricing enhancements.
func LoadKiloEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	dbPaths := kiloDBPaths()
	if len(dbPaths) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	var entries []UsageEntry
	for _, p := range dbPaths {
		rows, err := loadKiloFromDB(ctx, p)
		if err != nil {
			log.Printf("collector/kilo: skipping %s: %v", p, err)
			continue
		}
		for _, e := range rows {
			if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
				continue
			}
			if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
				continue
			}
			// Dedup across multiple roots pointing at the same install.
			// ccusage uses (db_path, row_id) when no JSON message id is
			// present; we synthesize the same shape via the row's id
			// column + SessionID + ts so identical content from a
			// remount still collapses.
			key := p + "|" + e.SessionID + "|" + fmt.Sprintf("%d|%d|%d|%d", e.Timestamp.UnixNano(),
				e.InputTokens, e.OutputTokens, e.CacheCreationInputTokens)
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

func kiloDBPaths() []string {
	var roots []string
	if env := strings.TrimSpace(os.Getenv(kiloEnvVar)); env != "" {
		for _, raw := range strings.Split(env, ",") {
			p := strings.TrimSpace(raw)
			if p != "" {
				roots = append(roots, p)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		roots = []string{filepath.Join(home, ".local", "share", "kilo")}
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, root := range roots {
		// Validate root is a directory before joining the DB file name —
		// matches ccusage's `path.is_dir()` precheck.
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			continue
		}
		dbPath := filepath.Join(root, kiloDBFileName)
		if resolved, err := filepath.EvalSymlinks(dbPath); err == nil {
			dbPath = resolved
		}
		info, err := os.Stat(dbPath)
		if err != nil || info.IsDir() {
			continue
		}
		if _, dup := seen[dbPath]; dup {
			continue
		}
		seen[dbPath] = struct{}{}
		paths = append(paths, dbPath)
	}
	return paths
}

func loadKiloFromDB(ctx context.Context, path string) ([]UsageEntry, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, kiloMessageQuery)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []UsageEntry
	for rows.Next() {
		var (
			rowID        string
			rowSessionID string
			data         string
		)
		if err := rows.Scan(&rowID, &rowSessionID, &data); err != nil {
			continue
		}
		e, ok := parseKiloMessage(data, rowSessionID)
		if !ok {
			continue
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iter: %w", err)
	}
	return out, nil
}

// parseKiloMessage extracts a UsageEntry from one row's JSON `data`
// blob. rowSessionID is the SQLite session_id column; it's used as a
// fallback when the JSON itself doesn't carry a session_id.
func parseKiloMessage(data, rowSessionID string) (UsageEntry, bool) {
	if data == "" {
		return UsageEntry{}, false
	}
	var raw struct {
		Role       string  `json:"role"`
		ID         string  `json:"id"`
		SessionID  string  `json:"session_id"`
		ProviderID string  `json:"providerID"`
		ModelID    string  `json:"modelID"`
		Cost       float64 `json:"cost"`
		Time       struct {
			Created int64 `json:"created"`
		} `json:"time"`
		Tokens struct {
			Input     int64 `json:"input"`
			Output    int64 `json:"output"`
			Reasoning int64 `json:"reasoning"`
			Total     int64 `json:"total"`
			Cache     struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return UsageEntry{}, false
	}
	if raw.Role != "assistant" {
		return UsageEntry{}, false
	}
	model := strings.TrimSpace(raw.ModelID)
	if model == "" {
		return UsageEntry{}, false
	}

	in := raw.Tokens.Input
	out := raw.Tokens.Output
	cc := raw.Tokens.Cache.Write
	cr := raw.Tokens.Cache.Read
	reasoning := raw.Tokens.Reasoning
	total := raw.Tokens.Total

	// ccusage's apply_total_token_fallback: when individual parts plus
	// reasoning are all zero, fall back to total -> Output so the row
	// still surfaces non-zero usage.
	if in+out+cc+cr+reasoning == 0 {
		if total > 0 {
			out = total
		} else {
			return UsageEntry{}, false
		}
	}
	// Fold reasoning into OutputTokens (UsageEntry has no reasoning slot
	// in v1.1; same simplification used by goose.go/hermes.go).
	out += reasoning

	ts, ok := normalizeKiloTimestamp(raw.Time.Created)
	if !ok {
		return UsageEntry{}, false
	}

	sessionID := strings.TrimSpace(raw.SessionID)
	if sessionID == "" {
		sessionID = rowSessionID
	}

	return UsageEntry{
		Source:                   kiloSource,
		SessionID:                sessionID,
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              in,
		OutputTokens:             out,
		CacheCreationInputTokens: cc,
		CacheReadInputTokens:     cr,
		CostUSD:                  kiloPositiveCost(raw.Cost),
	}, true
}

// normalizeKiloTimestamp mirrors ccusage kilo.rs::normalize_timestamp.
// Values < 1e12 are treated as unix seconds and scaled to milliseconds;
// >= 1e12 are already milliseconds. Zero or negative drops the row.
func normalizeKiloTimestamp(v int64) (time.Time, bool) {
	if v <= 0 {
		return time.Time{}, false
	}
	var millis int64
	if v < 1_000_000_000_000 {
		millis = v * 1000
	} else {
		millis = v
	}
	return time.UnixMilli(millis).UTC(), true
}

// kiloPositiveCost mirrors ccusage's auto-mode "stored cost wins when
// positive, else recompute" logic — zero/negative drops to 0 so the
// AllSource ModeAuto fallback gets a clean shot at re-pricing.
func kiloPositiveCost(c float64) float64 {
	if c > 0 {
		return c
	}
	return 0
}
