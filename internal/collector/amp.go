package collector

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ampDirEnv is the comma-separated env var ccusage honors as the only
// override; when unset the adapter falls back to ~/.local/share/amp.
const ampDirEnv = "AMP_DATA_DIR"

// LoadAmpEntries walks every existing Amp root, reads each
// `<root>/threads/<name>.json` thread file, joins the assistant-message
// cache token totals into each `usageLedger.events[]` row, and emits one
// UsageEntry per event. Missing roots return (nil, nil) — Phase B contract.
func LoadAmpEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := ampRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := collectAmpThreadFiles(root)
		if err != nil {
			log.Printf("warning: amp walk %s: %v", root, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := parseAmpThreadFile(file)
			if err != nil {
				log.Printf("warning: amp parse %s: %v", file, err)
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

// ampRoots resolves the directories to walk. AMP_DATA_DIR takes precedence
// (comma-separated, dedup) over the single ~/.local/share/amp fallback.
func ampRoots() []string {
	if raw := strings.TrimSpace(os.Getenv(ampDirEnv)); raw != "" {
		return ampExistingDirs(strings.Split(raw, ","))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return ampExistingDirs([]string{filepath.Join(home, ".local/share/amp")})
}

func ampExistingDirs(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range in {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// collectAmpThreadFiles enumerates `<root>/threads/*.json` (flat readdir,
// matching ccusage's collect_files_with_extension behavior). Threads dir
// absent returns (nil, nil) — the root may exist but be empty pre-onboarding.
func collectAmpThreadFiles(root string) ([]string, error) {
	threadsDir := filepath.Join(root, "threads")
	dirents, err := os.ReadDir(threadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, d := range dirents {
		if d.IsDir() {
			continue
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			continue
		}
		files = append(files, filepath.Join(threadsDir, d.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// parseAmpThreadFile reads one thread JSON and joins messages[].cache fields
// into events[]. Mirrors ccusage's read_thread_file:
//
//  1. messages[] (role=="assistant") → cacheTokens[messageId] = (create, read)
//  2. for each event in usageLedger.events[]:
//     timestamp, model, tokens.input/output required
//     cache pulled from cacheTokens[event.toMessageId], else (0, 0)
//     total fallback: if all 4 are zero but tokens.total > 0 → output = total
//     skip if everything is still zero
func parseAmpThreadFile(path string) ([]UsageEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		ID          string            `json:"id"`
		Messages    []json.RawMessage `json:"messages"`
		UsageLedger struct {
			Events []json.RawMessage `json:"events"`
		} `json:"usageLedger"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		// ccusage swallows malformed threads silently. Match that — a
		// broken file shouldn't taint the whole run.
		return nil, nil
	}
	if strings.TrimSpace(doc.ID) == "" {
		return nil, nil
	}
	cache := ampCacheTokensByMessageID(doc.Messages)

	out := make([]UsageEntry, 0, len(doc.UsageLedger.Events))
	for _, ev := range doc.UsageLedger.Events {
		entry, ok := parseAmpEvent(ev, doc.ID, cache)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

// ampCacheTokens captures the two cache counters as they appear on an
// assistant message; the JOIN key is messageId.
type ampCacheTokens struct {
	creation int64
	read     int64
}

// ampCacheTokensByMessageID builds the messageId → cache token map.
// Non-assistant messages and rows missing usage are skipped.
func ampCacheTokensByMessageID(messages []json.RawMessage) map[int64]ampCacheTokens {
	out := map[int64]ampCacheTokens{}
	for _, raw := range messages {
		var msg struct {
			Role      string `json:"role"`
			MessageID *int64 `json:"messageId"`
			Usage     *struct {
				CacheCreationInputTokens json.RawMessage `json:"cacheCreationInputTokens"`
				CacheReadInputTokens     json.RawMessage `json:"cacheReadInputTokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Role != "assistant" {
			continue
		}
		if msg.MessageID == nil {
			continue
		}
		var ct ampCacheTokens
		if msg.Usage != nil {
			ct.creation = ampParseInt64(msg.Usage.CacheCreationInputTokens)
			ct.read = ampParseInt64(msg.Usage.CacheReadInputTokens)
		}
		out[*msg.MessageID] = ct
	}
	return out
}

// parseAmpEvent decodes a single usageLedger event row into a UsageEntry.
// Returns (zero, false) when the row is missing a required field or the
// total/per-bucket tokens add up to zero.
func parseAmpEvent(raw json.RawMessage, threadID string, cache map[int64]ampCacheTokens) (UsageEntry, bool) {
	var ev struct {
		ID          string          `json:"id"`
		Timestamp   string          `json:"timestamp"`
		Model       string          `json:"model"`
		Credits     json.RawMessage `json:"credits"`
		ToMessageID *int64          `json:"toMessageId"`
		Tokens      *struct {
			Input  json.RawMessage `json:"input"`
			Output json.RawMessage `json:"output"`
			Total  json.RawMessage `json:"total"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return UsageEntry{}, false
	}
	if strings.TrimSpace(ev.Timestamp) == "" || strings.TrimSpace(ev.Model) == "" || ev.Tokens == nil {
		return UsageEntry{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
	if err != nil {
		return UsageEntry{}, false
	}
	input := ampParseInt64(ev.Tokens.Input)
	output := ampParseInt64(ev.Tokens.Output)
	total := ampParseInt64(ev.Tokens.Total)

	var ct ampCacheTokens
	if ev.ToMessageID != nil {
		ct = cache[*ev.ToMessageID]
	}

	// apply_total_token_fallback (ccusage): if all four counters are zero
	// and tokens.total > 0, attribute the total to output_tokens so the
	// row still contributes to summaries.
	if input == 0 && output == 0 && ct.creation == 0 && ct.read == 0 && total > 0 {
		output = total
	}
	if input == 0 && output == 0 && ct.creation == 0 && ct.read == 0 {
		return UsageEntry{}, false
	}

	return UsageEntry{
		Source:                   "amp",
		SessionID:                threadID,
		ProjectPath:              "Amp",
		Timestamp:                ts.UTC(),
		Model:                    ev.Model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: ct.creation,
		CacheReadInputTokens:     ct.read,
		// CostUSD intentionally 0. ccusage carries `credits` separately
		// and computes the dollar cost from pricing; our pricing layer
		// lives in cli/aggregate.go and recomputes when --mode auto sees
		// a zero-cost row or --mode calculate is set.
	}, true
}

// ampParseInt64 reads a JSON number that may have arrived as a float, int,
// or json.Number. Strings are not accepted (ccusage's amp counters are
// always numeric in practice).
func ampParseInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	// Try int first to avoid float drift on large counter values.
	var asInt int64
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt
	}
	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil {
		return int64(asFloat)
	}
	return 0
}
