package collector

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	droidEnvVar      = "DROID_SESSIONS_DIR"
	droidSettingsExt = ".settings.json"
	droidSource      = "droid"
)

// LoadDroidEntries scans Droid CLI session settings files and returns one
// UsageEntry per session.
//
// Path discovery (mirrors ccusage droid.rs::droid_session_paths):
//
//   - DROID_SESSIONS_DIR set: comma-separated list of session roots
//     (dedup'd by canonical path; non-directories dropped).
//   - Otherwise the single default $HOME/.factory/sessions.
//
// File matcher (ccusage droid.rs::discover_settings_files): every .json
// under a root is candidate, then filtered to file names ending in
// ".settings.json". Note the suffix is two extensions deep — a bare
// .json or .jsonl file is intentionally NOT a settings file.
//
// JSON parse (ccusage droid.rs::load_settings_file):
//
//	model                         -> Model (normalizeDroidModelName lowercases
//	                                  and strips [Brand] brackets; e.g.
//	                                  "Claude-Sonnet-4-[Anthropic]" ->
//	                                  "claude-sonnet-4")
//	providerLock                  -> READ but not surfaced; UsageEntry has
//	                                  no provider slot in v1.1. ccusage uses
//	                                  it to drive per-provider cost recompute
//	                                  candidates; deferred to v1.1.x pricing.
//	providerLockTimestamp         -> Timestamp (RFC3339); falls back to file
//	                                  mtime when missing or malformed.
//	tokenUsage.inputTokens        -> InputTokens
//	tokenUsage.outputTokens       -> OutputTokens
//	tokenUsage.cacheCreationTokens -> CacheCreationInputTokens
//	tokenUsage.cacheReadTokens    -> CacheReadInputTokens
//	tokenUsage.thinkingTokens     -> folded INTO OutputTokens (UsageEntry
//	                                  has no reasoning slot in v1.1; same
//	                                  trade-off used by goose/hermes/kilo)
//	tokenUsage.totalTokens        -> fallback into Output when all
//	                                  individual parts + thinking are zero
//	                                  (ccusage's apply_total_token_fallback)
//
// SessionID is the file name with ".settings.json" stripped — Droid
// stores one settings file per session and ccusage uses the basename as
// the canonical session id.
//
// CostUSD intentionally left at 0: Droid's settings file doesn't carry
// cost. AllSource ModeAuto recomputes via pricing.Resolve(model). The
// per-provider candidate explosion ccusage's calculate_droid_cost
// performs (anthropic/, openrouter/anthropic/, ...) is deferred to
// v1.1.x pricing enhancements — bare normalized model usually hits
// LiteLLM's snapshot for Claude/GPT/Gemini in practice.
//
// Dedup: by SessionID (one entry per session, latest timestamp wins).
// Multiple roots pointing at the same install therefore collapse.
func LoadDroidEntries(_ context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := droidSessionRoots()
	if len(roots) == 0 {
		return nil, nil
	}
	// Latest-per-session: walk every file, then keep only the highest
	// timestamp per SessionID. Equivalent to ccusage's reverse-iterate +
	// HashSet dedup.
	bySession := map[string]UsageEntry{}
	for _, root := range roots {
		files := discoverDroidSettings(root)
		for _, f := range files {
			e, ok := readDroidSettings(f)
			if !ok {
				continue
			}
			if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
				continue
			}
			if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
				continue
			}
			if prev, dup := bySession[e.SessionID]; !dup || e.Timestamp.After(prev.Timestamp) {
				bySession[e.SessionID] = e
			}
		}
	}
	out := make([]UsageEntry, 0, len(bySession))
	for _, e := range bySession {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func droidSessionRoots() []string {
	var rawRoots []string
	if env := strings.TrimSpace(os.Getenv(droidEnvVar)); env != "" {
		for _, raw := range strings.Split(env, ",") {
			p := strings.TrimSpace(raw)
			if p != "" {
				rawRoots = append(rawRoots, p)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		rawRoots = []string{filepath.Join(home, ".factory", "sessions")}
	}
	seen := map[string]struct{}{}
	var roots []string
	for _, r := range rawRoots {
		if info, err := os.Stat(r); err != nil || !info.IsDir() {
			continue
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		roots = append(roots, r)
	}
	return roots
}

// discoverDroidSettings recursively collects every file whose basename
// ends with ".settings.json". A bare .json file (e.g. "config.json") is
// dropped — the two-extension suffix is the contract from ccusage's
// discover_settings_files.
func discoverDroidSettings(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, droidSettingsExt) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

// readDroidSettings parses one .settings.json file into a UsageEntry.
// Returns (zero, false) for missing tokens / model / timestamp; a file
// I/O error is also treated as "skip" since Droid leaves stray files
// behind during session swap.
func readDroidSettings(path string) (UsageEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UsageEntry{}, false
	}
	var raw struct {
		Model                 string `json:"model"`
		ProviderLock          string `json:"providerLock"`
		ProviderLockTimestamp string `json:"providerLockTimestamp"`
		TokenUsage            *struct {
			Input         int64 `json:"inputTokens"`
			Output        int64 `json:"outputTokens"`
			CacheCreation int64 `json:"cacheCreationTokens"`
			CacheRead     int64 `json:"cacheReadTokens"`
			Thinking      int64 `json:"thinkingTokens"`
			Total         int64 `json:"totalTokens"`
		} `json:"tokenUsage"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return UsageEntry{}, false
	}
	if raw.TokenUsage == nil {
		return UsageEntry{}, false
	}
	in := raw.TokenUsage.Input
	out := raw.TokenUsage.Output
	cc := raw.TokenUsage.CacheCreation
	cr := raw.TokenUsage.CacheRead
	think := raw.TokenUsage.Thinking
	total := raw.TokenUsage.Total
	if in+out+cc+cr+think == 0 {
		if total > 0 {
			out = total
		} else {
			return UsageEntry{}, false
		}
	}
	// Fold thinking into Output (UsageEntry has no reasoning slot in
	// v1.1; same simplification used by goose/hermes/kilo).
	out += think

	model := normalizeDroidModelName(raw.Model)
	if model == "" {
		// Sidecar JSONL lookup (ccusage extract_model_from_sidecar_jsonl)
		// is a v1.1.1 follow-up. Drop the row rather than guessing a
		// default — pricing would mis-attribute "claude-unknown" rows.
		return UsageEntry{}, false
	}

	ts, ok := droidTimestamp(raw.ProviderLockTimestamp, path)
	if !ok {
		return UsageEntry{}, false
	}

	sessionID := strings.TrimSuffix(filepath.Base(path), droidSettingsExt)
	if sessionID == "" {
		sessionID = "unknown"
	}

	return UsageEntry{
		Source:                   droidSource,
		SessionID:                sessionID,
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              in,
		OutputTokens:             out,
		CacheCreationInputTokens: cc,
		CacheReadInputTokens:     cr,
	}, true
}

// normalizeDroidModelName mirrors ccusage's normalize_droid_model_name:
// strip "custom:" prefix, drop everything inside [Brand] brackets,
// lowercase, replace whitespace/'.'/'-' with '-', collapse consecutive
// '-', and trim '-' from both ends.
//
//	"Claude-Sonnet-4-[Anthropic]"          -> "claude-sonnet-4"
//	"custom:Claude-Opus-4.5-Thinking-..."  -> "claude-opus-4-5-thinking"
//	"gemini-2.5-pro"                       -> "gemini-2-5-pro"
func normalizeDroidModelName(model string) string {
	raw := strings.TrimPrefix(strings.TrimSpace(model), "custom:")
	// Drop bracketed segments.
	var nb strings.Builder
	depth := 0
	for _, ch := range raw {
		switch ch {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				nb.WriteRune(ch)
			}
		}
	}
	lower := strings.ToLower(strings.TrimRight(strings.TrimSpace(nb.String()), "-"))
	var out strings.Builder
	prevDash := false
	for _, ch := range lower {
		next := ch
		if ch == '.' || ch == '-' || ch == ' ' || ch == '\t' {
			next = '-'
		}
		if next == '-' {
			if !prevDash {
				out.WriteByte('-')
				prevDash = true
			}
		} else {
			out.WriteRune(next)
			prevDash = false
		}
	}
	return strings.Trim(out.String(), "-")
}

// droidTimestamp prefers providerLockTimestamp (RFC3339) when populated
// and parseable; otherwise falls back to the file's mtime. ccusage does
// the same fallback in settings_timestamp().
func droidTimestamp(providerLockTS, path string) (time.Time, bool) {
	if s := strings.TrimSpace(providerLockTS); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.UTC(), true
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime().UTC(), true
}
