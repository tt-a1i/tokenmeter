package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// geminiDirEnv is the comma-separated env var ccusage honors as the only
// override; when unset the adapter falls back to ~/.gemini/tmp.
const geminiDirEnv = "GEMINI_DATA_DIR"

// geminiDefaultModel is the placeholder ccusage uses for stats-style events
// that don't carry a per-model breakdown. Kept identical so token totals
// still surface under a stable label.
const geminiDefaultModel = "unknown"

// LoadGeminiEntries discovers every existing Gemini CLI root, walks the
// `.json` AND `.jsonl` files in each (skipping other extensions like `.txt`),
// parses both formats, and emits one UsageEntry per token-bearing event.
// Missing roots return (nil, nil) — Phase B contract.
func LoadGeminiEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := geminiRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := collectGeminiLogFiles(root)
		if err != nil {
			log.Printf("warning: gemini walk %s: %v", root, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var entries []UsageEntry
			if strings.HasSuffix(file, ".jsonl") {
				entries, err = parseGeminiJSONLFile(file)
			} else {
				entries, err = parseGeminiJSONFile(file)
			}
			if err != nil {
				log.Printf("warning: gemini parse %s: %v", file, err)
				continue
			}
			for _, e := range entries {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && !e.Timestamp.Before(opts.Until) {
					continue
				}
				out = append(out, e)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func geminiRoots() []string {
	if raw := strings.TrimSpace(os.Getenv(geminiDirEnv)); raw != "" {
		return ampExistingDirs(strings.Split(raw, ","))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return ampExistingDirs([]string{filepath.Join(home, ".gemini", "tmp")})
}

// collectGeminiLogFiles walks the root recursively (mirrors ccusage's
// `collect_files_with_extension` once per .json + .jsonl call) and returns
// every matching file, sorted + deduped.
func collectGeminiLogFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: gemini walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	// Dedup adjacent duplicates (sort.Strings groups them).
	out := files[:0]
	var prev string
	for _, p := range files {
		if p == prev {
			continue
		}
		out = append(out, p)
		prev = p
	}
	return out, nil
}

// geminiTokens captures the multi-key token bag ccusage tolerates per
// fixture quirks (input/prompt/input_tokens/prompt_tokens etc.).
type geminiTokens struct {
	input    int64
	output   int64
	cached   int64
	thoughts int64
	tool     int64
	total    int64
	hasTotal bool
}

// parseGeminiJSONFile reads a single-document Gemini log. Three flavors:
//
//  1. Top-level `messages[]` array: each `type=="gemini"` message becomes
//     an entry.
//  2. Top-level `type=="gemini"`: treat the whole document as one direct
//     event.
//  3. Top-level `stats` (or `result.stats`): one entry per model under
//     `stats.models` or a single entry from aggregate stats.
func parseGeminiJSONFile(path string) ([]UsageEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(raw, &record); err != nil {
		// Malformed json → silently skip the file (matches ccusage).
		return nil, nil
	}

	fallback := geminiFileModTime(path)
	sessionID := geminiStringAt(record, "sessionId")
	if sessionID == "" {
		sessionID = geminiStringAt(record, "session_id")
	}
	if sessionID == "" {
		sessionID = geminiSessionIDFromFile(path)
	}
	sessionTs := geminiTimestampAt(record, "startTime", fallback)
	if recTs, ok := record["lastUpdated"]; ok {
		sessionTs = geminiParseTimestamp(recTs, sessionTs)
	}

	// Flavor 1: messages[] array.
	if raw, ok := record["messages"]; ok {
		var msgs []json.RawMessage
		if err := json.Unmarshal(raw, &msgs); err == nil {
			var out []UsageEntry
			for _, m := range msgs {
				var msgRec map[string]json.RawMessage
				if err := json.Unmarshal(m, &msgRec); err != nil {
					continue
				}
				if t := geminiStringAt(msgRec, "type"); t != "gemini" {
					continue
				}
				if e, ok := parseGeminiDirectEvent(msgRec, "", sessionID, sessionTs, geminiNormalizeSessionInput); ok {
					out = append(out, e)
				}
			}
			return out, nil
		}
	}

	// Flavor 2: top-level direct gemini event.
	if t := geminiStringAt(record, "type"); t == "gemini" {
		if e, ok := parseGeminiDirectEvent(record, "", sessionID, fallback, geminiNormalizeSessionInput); ok {
			return []UsageEntry{e}, nil
		}
		return nil, nil
	}

	// Flavor 3: stats[].
	statsRaw, ok := record["stats"]
	if !ok {
		if resRaw, found := record["result"]; found {
			var resRec map[string]json.RawMessage
			if err := json.Unmarshal(resRaw, &resRec); err == nil {
				statsRaw = resRec["stats"]
				ok = statsRaw != nil
			}
		}
	}
	if !ok || len(statsRaw) == 0 {
		return nil, nil
	}
	ts := geminiTimestampAt(record, "timestamp", fallback)
	modelHint := geminiStringAt(record, "model")
	return parseGeminiStatsEvents(statsRaw, modelHint, sessionID, ts), nil
}

// parseGeminiJSONLFile streams a `.jsonl` Gemini log, maintaining session-id
// and current-model cursors that subsequent direct events fall back onto.
// Direct events carrying a `id` field dedup by id (later same-id replaces
// earlier).
func parseGeminiJSONLFile(path string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fallback := geminiFileModTime(path)
	sessionID := geminiSessionIDFromFile(path)
	var currentModel string

	var events []UsageEntry
	idxByID := map[string]int{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if v := geminiStringAt(record, "sessionId"); v != "" {
			sessionID = v
		} else if v := geminiStringAt(record, "session_id"); v != "" {
			sessionID = v
		}
		if v := geminiStringAt(record, "model"); v != "" {
			currentModel = v
		}
		if t := geminiStringAt(record, "type"); t == "gemini" {
			e, ok := parseGeminiDirectEvent(record, currentModel, sessionID, fallback, geminiNormalizeSessionInput)
			if !ok {
				continue
			}
			if id := geminiStringAt(record, "id"); id != "" {
				if idx, exists := idxByID[id]; exists {
					events[idx] = e
				} else {
					idxByID[id] = len(events)
					events = append(events, e)
				}
			} else {
				events = append(events, e)
			}
			continue
		}
		// stats[] flavor inside a JSONL line.
		statsRaw, ok := record["stats"]
		if !ok {
			if resRaw, found := record["result"]; found {
				var resRec map[string]json.RawMessage
				if err := json.Unmarshal(resRaw, &resRec); err == nil {
					statsRaw = resRec["stats"]
					ok = statsRaw != nil
				}
			}
		}
		if ok && len(statsRaw) > 0 {
			ts := geminiTimestampAt(record, "timestamp", fallback)
			events = append(events, parseGeminiStatsEvents(statsRaw, currentModel, sessionID, ts)...)
		}
	}
	return events, scanner.Err()
}

// parseGeminiDirectEvent decodes a "type=gemini" record with a `tokens` bag.
// modelHint is the cursor value to fall back on; normalize controls the
// cached-overlap subtraction (session vs stats parser pass different fns).
func parseGeminiDirectEvent(
	record map[string]json.RawMessage,
	modelHint, sessionID string,
	fallback time.Time,
	normalize func(geminiTokens) (int64, int64),
) (UsageEntry, bool) {
	tokensRaw, ok := record["tokens"]
	if !ok {
		return UsageEntry{}, false
	}
	tokens, ok := parseGeminiTokens(tokensRaw)
	if !ok {
		return UsageEntry{}, false
	}
	model := geminiStringAt(record, "model")
	if model == "" {
		model = modelHint
	}
	ts := geminiTimestampAt(record, "timestamp", time.Time{})
	if ts.IsZero() {
		ts = geminiTimestampAt(record, "created_at", fallback)
	}
	return geminiBuildEvent(model, sessionID, ts, tokens, normalize)
}

// parseGeminiStatsEvents handles the `stats.models` map or fallback flat
// stats record. Uses subtract_cached_overlap_tokens for cached normalization.
func parseGeminiStatsEvents(statsRaw json.RawMessage, modelHint, sessionID string, ts time.Time) []UsageEntry {
	var stats map[string]json.RawMessage
	if err := json.Unmarshal(statsRaw, &stats); err != nil {
		return nil
	}
	if modelsRaw, ok := stats["models"]; ok {
		var modelMap map[string]json.RawMessage
		if err := json.Unmarshal(modelsRaw, &modelMap); err == nil {
			var out []UsageEntry
			for model, dataRaw := range modelMap {
				var dataRec map[string]json.RawMessage
				if err := json.Unmarshal(dataRaw, &dataRec); err != nil {
					continue
				}
				tokensRaw, ok := dataRec["tokens"]
				if !ok {
					continue
				}
				tokens, ok := parseGeminiTokens(tokensRaw)
				if !ok {
					continue
				}
				if e, ok := geminiBuildEvent(model, sessionID, ts, tokens, geminiSubtractCachedOverlap); ok {
					out = append(out, e)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	// Flat stats: treat the whole stats object as a token bag.
	tokens, ok := parseGeminiTokens(statsRaw)
	if !ok {
		return nil
	}
	model := modelHint
	if model == "" {
		model = geminiDefaultModel
	}
	if e, ok := geminiBuildEvent(model, sessionID, ts, tokens, geminiSubtractCachedOverlap); ok {
		return []UsageEntry{e}
	}
	return nil
}

// parseGeminiTokens reads the multi-key token bag.
func parseGeminiTokens(raw json.RawMessage) (geminiTokens, bool) {
	var record map[string]json.RawMessage
	if err := json.Unmarshal(raw, &record); err != nil {
		return geminiTokens{}, false
	}
	t := geminiTokens{
		input:    geminiTokenNumber(record, []string{"input", "prompt", "input_tokens", "prompt_tokens"}),
		output:   geminiTokenNumber(record, []string{"output", "candidates", "output_tokens", "candidates_tokens"}),
		cached:   geminiTokenNumber(record, []string{"cached", "cached_tokens"}),
		thoughts: geminiTokenNumber(record, []string{"thoughts", "reasoning", "thoughts_tokens", "reasoning_tokens"}),
		tool:     geminiTokenNumber(record, []string{"tool", "tool_tokens"}),
	}
	if v, ok := record["total"]; ok {
		t.total = geminiValueInt64(v)
		t.hasTotal = true
	} else if v, ok := record["total_tokens"]; ok {
		t.total = geminiValueInt64(v)
		t.hasTotal = true
	}
	return t, true
}

func geminiTokenNumber(record map[string]json.RawMessage, keys []string) int64 {
	for _, k := range keys {
		if v, ok := record[k]; ok {
			return geminiValueInt64(v)
		}
	}
	return 0
}

func geminiValueInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f < 0 {
			return 0
		}
		return int64(f)
	}
	return 0
}

// geminiNormalizeSessionInput is the session/messages parser flavor: if
// `cached > 0` and `total == input+output+thoughts+tool` (inclusive), the
// cached count overlaps with input and must be subtracted out. Stats events
// use geminiSubtractCachedOverlap unconditionally.
func geminiNormalizeSessionInput(t geminiTokens) (int64, int64) {
	inclusive := t.input + t.output + t.thoughts + t.tool
	exclusive := inclusive + t.cached
	if t.cached > 0 && t.hasTotal && t.total == inclusive && t.total != exclusive {
		return geminiSubtractCachedOverlap(t)
	}
	return t.input, t.cached
}

func geminiSubtractCachedOverlap(t geminiTokens) (int64, int64) {
	cachedPortion := t.input
	if t.cached < cachedPortion {
		cachedPortion = t.cached
	}
	out := t.input - cachedPortion
	if out < 0 {
		out = 0
	}
	return out, t.cached
}

// geminiBuildEvent finishes a UsageEntry from a normalized token bag.
// Model is required (skipped if empty). Falls back to total → output when
// every per-bucket counter is zero. thoughts (reasoning) is folded into
// OutputTokens because UsageEntry has no dedicated extra-total slot;
// ccusage tracks it separately on LoadedEntry but our SQLite schema can't
// surface it without losing parity in the aggregation pipeline.
func geminiBuildEvent(
	model, sessionID string,
	ts time.Time,
	tokens geminiTokens,
	normalize func(geminiTokens) (int64, int64),
) (UsageEntry, bool) {
	if strings.TrimSpace(model) == "" {
		return UsageEntry{}, false
	}
	inputWithoutCache, cacheRead := normalize(tokens)
	inputTokens := inputWithoutCache + tokens.tool
	totalTokens := tokens.total
	if !tokens.hasTotal {
		totalTokens = inputTokens + tokens.output + cacheRead + tokens.thoughts
	}
	output := tokens.output

	// apply_total_token_fallback parity. If every display counter is zero
	// but the source row carried a total, surface (total - thoughts) on
	// output so summaries don't go silent.
	if inputTokens == 0 && output == 0 && cacheRead == 0 {
		if totalTokens > 0 {
			extra := totalTokens - tokens.thoughts
			if extra < 0 {
				extra = 0
			}
			output = extra
		}
		if output == 0 && tokens.thoughts == 0 {
			return UsageEntry{}, false
		}
	}
	// Fold reasoning tokens into output_tokens so they survive the
	// aggregation pipeline. Documented deviation — ccusage stores
	// reasoning separately on LoadedEntry.extra_total_tokens.
	output += tokens.thoughts

	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return UsageEntry{
		Source:                   "gemini",
		SessionID:                sessionID,
		ProjectPath:              "Gemini",
		Timestamp:                ts.UTC(),
		Model:                    model,
		InputTokens:              inputTokens,
		OutputTokens:             output,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     cacheRead,
		// CostUSD = 0 by design — see amp.go / openclaw.go for the same
		// rationale. cli pricing layer recomputes when needed.
	}, true
}

func geminiStringAt(record map[string]json.RawMessage, key string) string {
	raw, ok := record[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func geminiTimestampAt(record map[string]json.RawMessage, key string, fallback time.Time) time.Time {
	raw, ok := record[key]
	if !ok {
		return fallback
	}
	return geminiParseTimestamp(raw, fallback)
}

func geminiParseTimestamp(raw json.RawMessage, fallback time.Time) time.Time {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC()
		}
	}
	return fallback
}

func geminiFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func geminiSessionIDFromFile(path string) string {
	base := filepath.Base(path)
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[:i]
	}
	return base
}
