package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// openClawDirEnv is the comma-separated env var ccusage honors as the
// first-class override before the home-directory fallback list.
const openClawDirEnv = "OPENCLAW_DIR"

// openClawHomeFallbacks mirrors ccusage's hard-coded list of directories
// OpenClaw + its forks write JSONL session files into.
var openClawHomeFallbacks = []string{".openclaw", ".clawdbot", ".moltbot", ".moldbot"}

// LoadOpenClawEntries walks every existing OpenClaw root, parses the JSONL
// session files, and translates assistant-usage records into UsageEntry.
// Missing roots return (nil, nil) — Phase B contract.
func LoadOpenClawEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := openClawRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := collectOpenClawFiles(root)
		if err != nil {
			log.Printf("warning: openclaw walk %s: %v", root, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := parseOpenClawFile(file)
			if err != nil {
				log.Printf("warning: openclaw parse %s: %v", file, err)
				continue
			}
			for _, e := range entries {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && !e.Timestamp.Before(opts.Until) {
					continue
				}
				key := openClawEntryKey(e)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, e)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

// openClawRoots resolves the directories to walk. Env takes precedence over
// the home-fallback list, and only entries that exist as directories make
// it through.
func openClawRoots() []string {
	if raw := strings.TrimSpace(os.Getenv(openClawDirEnv)); raw != "" {
		return openClawExistingDirs(strings.Split(raw, ","))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := make([]string, 0, len(openClawHomeFallbacks))
	for _, sub := range openClawHomeFallbacks {
		candidates = append(candidates, filepath.Join(home, sub))
	}
	return openClawExistingDirs(candidates)
}

func openClawExistingDirs(in []string) []string {
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

// collectOpenClawFiles recursively walks root and returns every file whose
// name matches OpenClaw's session-file naming convention.
func collectOpenClawFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Tolerate per-file walk errors the way ccusage does — log and skip.
			log.Printf("warning: openclaw walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isOpenClawSessionFile(d.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// isOpenClawSessionFile mirrors ccusage's filename matcher: live `.jsonl`
// files plus archived `.jsonl.deleted.*` / `.jsonl.reset.*` variants.
func isOpenClawSessionFile(name string) bool {
	idx := strings.Index(name, ".jsonl")
	if idx < 0 {
		return false
	}
	suffix := name[idx:]
	return suffix == ".jsonl" ||
		strings.HasPrefix(suffix, ".jsonl.deleted.") ||
		strings.HasPrefix(suffix, ".jsonl.reset.")
}

// parseOpenClawFile streams a session file. model_change records update a
// running cursor; assistant message records emit one UsageEntry each.
func parseOpenClawFile(path string) ([]UsageEntry, error) {
	sessionID := openClawSessionID(filepath.Base(path))
	fallbackTs := openClawFileModTime(path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []UsageEntry
	var curModel, curProvider string

	scanner := bufio.NewScanner(f)
	// OpenClaw assistant payloads can include base64 attachments; widen
	// the buffer so a typical record stays single-line.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Cheap pre-filter (same as ccusage) to avoid the JSON cost on
		// the bulk of session-state and tool-call rows we don't need.
		if !bytesContainsAny(line, []string{`"model_change"`, `"model-snapshot"`, `"usage"`}) {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if isOpenClawModelChange(record) {
			source := record
			if data, ok := record["data"].(map[string]any); ok {
				source = data
			}
			if m := openClawNonEmptyString(source["modelId"]); m != "" {
				curModel = m
			} else if m := openClawNonEmptyString(source["model"]); m != "" {
				curModel = m
			}
			if p := openClawNonEmptyString(source["provider"]); p != "" {
				curProvider = p
			}
			continue
		}
		entry, ok := parseOpenClawMessage(record, sessionID, curModel, curProvider, fallbackTs)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func bytesContainsAny(b []byte, needles []string) bool {
	for _, n := range needles {
		if bytes.Contains(b, []byte(n)) {
			return true
		}
	}
	return false
}

func isOpenClawModelChange(record map[string]any) bool {
	t, _ := record["type"].(string)
	if t == "model_change" {
		return true
	}
	if t == "custom" {
		c, _ := record["customType"].(string)
		return c == "model-snapshot"
	}
	return false
}

// parseOpenClawMessage decodes a single assistant-message record into a
// UsageEntry. Mirrors ccusage's parse_message_entry: same field names, same
// total-token fallback, same "[openclaw] " model prefix, same provider
// carry-over from the most recent model_change.
//
// curProvider is captured but not stored — UsageEntry has no Provider field
// in this codebase yet; ccusage threads it into the LoadedEntry.data.version
// slot, which we don't currently expose.
func parseOpenClawMessage(record map[string]any, sessionID, curModel, curProvider string, fallback time.Time) (UsageEntry, bool) {
	_ = curProvider
	if t, _ := record["type"].(string); t != "message" {
		return UsageEntry{}, false
	}
	msg, ok := record["message"].(map[string]any)
	if !ok {
		return UsageEntry{}, false
	}
	if r, _ := msg["role"].(string); r != "assistant" {
		return UsageEntry{}, false
	}
	usage, ok := msg["usage"].(map[string]any)
	if !ok {
		return UsageEntry{}, false
	}
	input := openClawJSONInt64(usage["input"])
	output := openClawJSONInt64(usage["output"])
	cacheRead := openClawJSONInt64(usage["cacheRead"])
	cacheCreate := openClawJSONInt64(usage["cacheWrite"])
	totalReported := openClawJSONInt64(usage["totalTokens"])

	// apply_total_token_fallback (ccusage): if no per-bucket tokens but the
	// record carries totalTokens, attribute the total to output so the row
	// still contributes to summaries.
	if input+output+cacheRead+cacheCreate == 0 && totalReported > 0 {
		output = totalReported
	}
	if input+output+cacheRead+cacheCreate == 0 {
		return UsageEntry{}, false
	}

	var ts time.Time
	if v, ok := msg["timestamp"]; ok {
		ts = openClawTimestamp(v, fallback)
	} else if v, ok := record["timestamp"]; ok {
		ts = openClawTimestamp(v, fallback)
	} else {
		ts = fallback
	}

	model := openClawNonEmptyString(msg["modelId"])
	if model == "" {
		model = openClawNonEmptyString(msg["model"])
	}
	if model == "" {
		model = curModel
	}
	if model == "" {
		model = "unknown"
	}
	model = "[openclaw] " + model

	var cost float64
	if costObj, ok := usage["cost"].(map[string]any); ok {
		cost = openClawJSONFloat64(costObj["total"])
	}

	return UsageEntry{
		Source:                   "openclaw",
		SessionID:                sessionID,
		ProjectPath:              "OpenClaw",
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheCreate,
		CacheReadInputTokens:     cacheRead,
		CostUSD:                  cost,
	}, true
}

// openClawSessionID strips the OpenClaw `.jsonl[.deleted.*|.reset.*]` suffix
// and returns the filename stem. Falls back to the raw filename when the
// stem would be empty.
func openClawSessionID(filename string) string {
	idx := strings.Index(filename, ".jsonl")
	if idx <= 0 {
		return filename
	}
	return filename[:idx]
}

func openClawFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func openClawTimestamp(v any, fallback time.Time) time.Time {
	switch x := v.(type) {
	case float64:
		return time.UnixMilli(int64(x)).UTC()
	case int64:
		return time.UnixMilli(x).UTC()
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return time.UnixMilli(n).UTC()
		}
	case string:
		if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
			return t.UTC()
		}
	}
	return fallback
}

func openClawJSONInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
	}
	return 0
}

func openClawJSONFloat64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
	}
	return 0
}

func openClawNonEmptyString(v any) string {
	s, _ := v.(string)
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}

// openClawEntryKey mirrors ccusage's entry_id so cross-root duplicates that
// shadow each other across `~/.openclaw` and `~/.clawdbot` collapse to one
// emitted UsageEntry.
func openClawEntryKey(e UsageEntry) string {
	return strings.Join([]string{
		"openclaw",
		e.SessionID,
		e.Timestamp.Format(time.RFC3339Nano),
		e.Model,
		strconv.FormatInt(e.InputTokens, 10),
		strconv.FormatInt(e.OutputTokens, 10),
		strconv.FormatInt(e.CacheCreationInputTokens, 10),
		strconv.FormatInt(e.CacheReadInputTokens, 10),
		strconv.FormatFloat(e.CostUSD, 'g', -1, 64),
	}, ":")
}
