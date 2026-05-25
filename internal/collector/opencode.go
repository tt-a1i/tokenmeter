package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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
//	<root>/opencode.db                (SQLite message table)
//	<root>/opencode-<channel>.db      (channel SQLite message tables)
//	<root>/storage/message/**/*.json  (recursive; one message per file)
//
// SQLite rows are loaded before JSON files so a database message wins when
// OpenCode writes both stores for the same id.
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
		for _, dbPath := range discoverOpencodeDBs(root) {
			for _, dbEntry := range loadOpencodeDBEntries(dbPath) {
				e, msgID := dbEntry.entry, dbEntry.messageID
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
					continue
				}
				key := opencodeDedupKey(e, msgID, dbPath)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				entries = append(entries, e)
			}
		}
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

func discoverOpencodeDBs(root string) []string {
	var out []string
	defaultPath := filepath.Join(root, "opencode.db")
	if isFile(defaultPath) {
		out = append(out, defaultPath)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	var channelDBs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isOpencodeChannelDBName(name) {
			continue
		}
		channelDBs = append(channelDBs, filepath.Join(root, name))
	}
	sort.Strings(channelDBs)
	return append(out, channelDBs...)
}

func isOpencodeChannelDBName(name string) bool {
	if !strings.HasPrefix(name, "opencode-") || !strings.HasSuffix(name, ".db") {
		return false
	}
	channel := strings.TrimSuffix(strings.TrimPrefix(name, "opencode-"), ".db")
	if channel == "" {
		return false
	}
	for _, ch := range channel {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

type opencodeDBEntry struct {
	entry     UsageEntry
	messageID string
}

func loadOpencodeDBEntries(path string) []opencodeDBEntry {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, session_id, data FROM message`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []opencodeDBEntry
	for rows.Next() {
		var id, sessionID, data string
		if err := rows.Scan(&id, &sessionID, &data); err != nil {
			continue
		}
		e, msgID, ok := parseOpencodeMessageData([]byte(data), id, sessionID)
		if !ok {
			continue
		}
		out = append(out, opencodeDBEntry{entry: e, messageID: msgID})
	}
	return out
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
	e, msgID, ok := parseOpencodeMessageData(data, "", "")
	return e, msgID, ok, nil
}

func parseOpencodeMessageData(data []byte, fallbackID, fallbackSessionID string) (UsageEntry, string, bool) {
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
		return UsageEntry{}, "", false
	}
	if strings.TrimSpace(raw.ModelID) == "" {
		// ccusage parser drops rows without modelID; OpenCode emits
		// user-message files with no tokens/model at all.
		return UsageEntry{}, "", false
	}
	in, out := raw.Tokens.Input, raw.Tokens.Output
	cc, cr := raw.Tokens.Cache.Write, raw.Tokens.Cache.Read
	if in+out+cc+cr == 0 {
		if raw.Tokens.Total > 0 {
			// Mirrors apply_total_token_fallback — drop the total into
			// the output bucket so the row still surfaces non-zero usage.
			out = raw.Tokens.Total
		} else {
			return UsageEntry{}, "", false
		}
	}
	msgID := raw.ID
	if msgID == "" {
		msgID = fallbackID
	}
	sessionID := raw.SessionID
	if sessionID == "" {
		sessionID = fallbackSessionID
	}
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
	}, msgID, true
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
