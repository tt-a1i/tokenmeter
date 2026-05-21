package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	opencodeEnvVar         = "OPENCODE_DATA_DIR"
	opencodeStorageDir     = "storage"
	opencodeMessagesDir    = "message"
	opencodeJSONExt        = ".json"
	opencodeSource         = "opencode"
	opencodeUnknownSession = "unknown"
)

// LoadOpenCodeEntries scans OpenCode CLI message.json files under the
// directories listed in $OPENCODE_DATA_DIR (comma-separated) or, if the
// env var is unset, $HOME/.local/share/opencode.
//
// Per-root layout (mirrors ccusage v20's opencode/loader.rs):
//
//	<root>/storage/message/**/*.json  (recursive; one message per file)
//	<root>/opencode.db                (SQLite path — NOT supported yet, see below)
//	<root>/opencode-<channel>.db      (channel DB — NOT supported yet)
//
// SQLite path is a deliberate v1.1.1 follow-up: it would pull the
// modernc.org/sqlite driver into the collector package, increasing the
// dependency surface and a per-process driver init cost. OpenCode CLI
// writes both stores in parallel (verified in ccusage's
// prefers_database_messages_over_duplicate_json_files test), so the JSON
// path covers the typical user; pure-SQLite installs surface as an empty
// adapter in this release and can fall back to `tm --no-scan` until
// v1.1.1.
//
// JSON field mapping per ccusage opencode/parser.rs:
//
//	tokens.input         -> InputTokens
//	tokens.output        -> OutputTokens
//	tokens.cache.write   -> CacheCreationInputTokens
//	tokens.cache.read    -> CacheReadInputTokens
//	tokens.total (fb)    -> OutputTokens (when individual parts are all zero)
//	modelID              -> Model      (required; row dropped if missing)
//	providerID           -> (recorded as part of model resolution metadata
//	                        for future pricing logic; not surfaced on
//	                        UsageEntry — kept off the unified type)
//	time.created (ms)    -> Timestamp  (epoch milliseconds)
//	id                   -> dedup key (entry skipped if reused across roots)
//	sessionID            -> SessionID  (falls back to "unknown" if absent)
//	cost (positive)      -> CostUSD    (passthrough; mirrors ccusage's
//	                        cost-or-recompute logic — 0/missing leaves
//	                        CostUSD=0 so RunAggregateAllSource's ModeAuto
//	                        recomputes from pricing.Resolve)
//
// Cost decision: ccusage's calculate_open_code_cost picks `cost_usd > 0`
// first and otherwise falls back to per-model pricing. We mirror the
// "positive cost wins" half here (so OpenCode's stored cost shows up
// untouched), and defer the per-model fallback to the AllSource ModeAuto
// path, which already re-prices CostUSD == 0 rows against the embedded
// pricing snapshot. Result: under default ModeAuto the user sees the
// same number ccusage would, even though the adapter itself does no
// pricing math.
//
// A missing/unreadable data root yields (nil, nil) so the merge loop
// treats the source as "user does not have OpenCode installed".
func LoadOpenCodeEntries(_ context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := opencodeRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var entries []UsageEntry
	for _, root := range roots {
		messagesDir := filepath.Join(root, opencodeStorageDir, opencodeMessagesDir)
		files := discoverOpencodeMessageFiles(messagesDir)
		for _, f := range files {
			e, msgID, ok, err := readOpencodeMessage(f)
			if err != nil || !ok {
				continue
			}
			if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
				continue
			}
			if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
				continue
			}
			// Prefer ccusage-style dedup by message id (stable across
			// rebuilds + multiple roots pointing at the same install).
			// Fall back to a synthesized tuple key when id is absent so
			// identical content still collapses inside one run.
			key := opencodeDedupKey(e, msgID, f)
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

// opencodeRoots resolves the candidate OpenCode data roots. When
// OPENCODE_DATA_DIR is set it wins outright; otherwise we fall back to
// $HOME/.local/share/opencode.
func opencodeRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	if env := strings.TrimSpace(os.Getenv(opencodeEnvVar)); env != "" {
		for _, raw := range strings.Split(env, ",") {
			p := strings.TrimSpace(raw)
			if p == "" {
				continue
			}
			if !isDir(p) {
				continue
			}
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			roots = append(roots, p)
		}
		return roots
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	p := filepath.Join(home, ".local", "share", "opencode")
	if isDir(p) {
		roots = append(roots, p)
	}
	return roots
}

// discoverOpencodeMessageFiles recursively collects every *.json file
// under messagesDir. Order is sorted for stable dedup across runs.
func discoverOpencodeMessageFiles(messagesDir string) []string {
	if !isDir(messagesDir) {
		return nil
	}
	var files []string
	_ = filepath.WalkDir(messagesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != opencodeJSONExt {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

// readOpencodeMessage parses one storage/message/.../*.json file into a
// UsageEntry plus the raw message id (used for dedup; not surfaced on
// UsageEntry to keep the unified type stable). Returns (zero, "", false,
// nil) for files with no useful usage payload (e.g. user-sent messages
// without model/tokens) or malformed JSON; a non-nil error means the
// file could not be read at all.
func readOpencodeMessage(path string) (UsageEntry, string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UsageEntry{}, "", false, err
	}
	var raw struct {
		ID         string `json:"id"`
		SessionID  string `json:"sessionID"`
		ProviderID string `json:"providerID"`
		ModelID    string `json:"modelID"`
		Time       struct {
			Created int64 `json:"created"`
		} `json:"time"`
		Tokens struct {
			Input  int64 `json:"input"`
			Output int64 `json:"output"`
			Cache  struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
			Total int64 `json:"total"`
		} `json:"tokens"`
		Cost float64 `json:"cost"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return UsageEntry{}, "", false, nil
	}
	if strings.TrimSpace(raw.ModelID) == "" {
		// ccusage parser drops rows without modelID; OpenCode emits
		// user-message files with no tokens/model at all.
		return UsageEntry{}, "", false, nil
	}
	in, out := raw.Tokens.Input, raw.Tokens.Output
	cc, cr := raw.Tokens.Cache.Write, raw.Tokens.Cache.Read
	if in+out+cc+cr == 0 {
		if raw.Tokens.Total > 0 {
			// Mirrors apply_total_token_fallback — drop the total into
			// the output bucket so the row still surfaces non-zero usage.
			out = raw.Tokens.Total
		} else {
			return UsageEntry{}, "", false, nil
		}
	}
	sessionID := raw.SessionID
	if sessionID == "" {
		sessionID = opencodeUnknownSession
	}
	// time.created is epoch milliseconds (ccusage parses it as i64 ms).
	ts := time.Unix(0, raw.Time.Created*int64(time.Millisecond)).UTC()
	return UsageEntry{
		Source:                   opencodeSource,
		SessionID:                sessionID,
		Timestamp:                ts,
		Model:                    raw.ModelID,
		InputTokens:              in,
		OutputTokens:             out,
		CacheCreationInputTokens: cc,
		CacheReadInputTokens:     cr,
		CostUSD:                  opencodePositiveCost(raw.Cost),
	}, raw.ID, true, nil
}

// opencodePositiveCost mirrors ccusage's "use stored cost when > 0,
// otherwise let pricing.Resolve recompute" logic — zero or negative
// becomes 0 so the AllSource ModeAuto fallback gets a clean chance.
func opencodePositiveCost(c float64) float64 {
	if c > 0 {
		return c
	}
	return 0
}

// opencodeDedupKey mirrors ccusage's entry_id-based dedup: when the
// message id is present it is the sole key (stable across rebuilds and
// multiple roots pointing at the same install). When id is empty we
// fall back to a (SessionID, Model, ts, tokens) tuple — file path is
// excluded so the same content seen from a second mount/root still
// dedups inside one run.
func opencodeDedupKey(e UsageEntry, msgID, _ string) string {
	if msgID != "" {
		return "id|" + msgID
	}
	return fmt.Sprintf("tuple|%s|%s|%d|%d|%d|%d|%d",
		e.SessionID, e.Model, e.Timestamp.UnixNano(),
		e.InputTokens, e.OutputTokens,
		e.CacheCreationInputTokens, e.CacheReadInputTokens,
	)
}
