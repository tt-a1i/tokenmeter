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

// piAgentDirEnv is the comma-separated env var ccusage honors as the
// first-class override before the home-directory fallback.
const piAgentDirEnv = "PI_AGENT_DIR"

// piDefaultSubdir is the home-relative sessions directory ccusage walks
// when no env override is set.
const piDefaultSubdir = ".pi/agent/sessions"

// LoadPiEntries walks every existing pi-agent root, reads each
// `<project>/<prefix>_<session>.jsonl` (or `agent_<session>.jsonl`) session
// file, and emits one UsageEntry per assistant-message-with-usage row.
// Missing roots return (nil, nil) — Phase B contract.
func LoadPiEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := piRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := piCollectFiles(root)
		if err != nil {
			log.Printf("warning: pi walk %s: %v", root, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := parsePiSessionFile(file)
			if err != nil {
				log.Printf("warning: pi parse %s: %v", file, err)
				continue
			}
			for _, e := range entries {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && !e.Timestamp.Before(opts.Until) {
					continue
				}
				key := piEntryKey(e)
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

// piRoots resolves the directories to walk. PI_AGENT_DIR (comma-separated,
// dedup) takes precedence over the single ~/.pi/agent/sessions fallback.
func piRoots() []string {
	if raw := strings.TrimSpace(os.Getenv(piAgentDirEnv)); raw != "" {
		return ampExistingDirs(strings.Split(raw, ","))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return ampExistingDirs([]string{filepath.Join(home, piDefaultSubdir)})
}

// piCollectFiles walks the root recursively and returns every `*.jsonl`
// file, sorted. ccusage's `collect_files_with_extension` is recursive too.
func piCollectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: pi walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") {
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

// parsePiSessionFile streams one session file. Each assistant-message row
// with a usage object becomes one UsageEntry; everything else is skipped
// via the cheap `"usage"` / `"message"` substring pre-filter.
func parsePiSessionFile(path string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sessionID := piSessionIDFromPath(path)
	project := piProjectFromPath(path)

	var out []UsageEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte(`"usage"`)) || !bytes.Contains(line, []byte(`"message"`)) {
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		entry, ok := parsePiMessage(record, sessionID, project)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out, scanner.Err()
}

// parsePiMessage decodes one message envelope into a UsageEntry. Returns
// false for non-message records, non-assistant messages, rows missing
// usage, missing/unparsable timestamp, or rows that net out to zero tokens.
func parsePiMessage(record map[string]json.RawMessage, sessionID, project string) (UsageEntry, bool) {
	// Optional outer type — when present it must be "message".
	if raw, ok := record["type"]; ok {
		var t string
		if err := json.Unmarshal(raw, &t); err == nil && t != "message" {
			return UsageEntry{}, false
		}
	}

	tsRaw, ok := record["timestamp"]
	if !ok {
		return UsageEntry{}, false
	}
	var tsText string
	if err := json.Unmarshal(tsRaw, &tsText); err != nil || strings.TrimSpace(tsText) == "" {
		return UsageEntry{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, tsText)
	if err != nil {
		return UsageEntry{}, false
	}

	msgRaw, ok := record["message"]
	if !ok {
		return UsageEntry{}, false
	}
	var msg struct {
		Role  string                     `json:"role"`
		Model string                     `json:"model"`
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return UsageEntry{}, false
	}
	if msg.Role != "assistant" || msg.Usage == nil {
		return UsageEntry{}, false
	}

	input := piParseNumber(msg.Usage["input"])
	output := piParseNumber(msg.Usage["output"])
	cacheRead := piParseNumber(msg.Usage["cacheRead"])
	cacheWrite := piParseNumber(msg.Usage["cacheWrite"])
	total := piParseNumber(msg.Usage["totalTokens"])

	// apply_total_token_fallback parity: when every per-bucket counter is
	// zero but totalTokens > 0, fold the total into output_tokens so the
	// row still contributes to summaries.
	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 && total > 0 {
		output = total
	}
	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 {
		return UsageEntry{}, false
	}

	// model: prefix "[pi] " per ccusage's display contract.
	model := strings.TrimSpace(msg.Model)
	if model == "" {
		model = "[pi] unknown"
	} else {
		model = "[pi] " + model
	}

	var cost float64
	if costRaw, ok := msg.Usage["cost"]; ok {
		var costObj struct {
			Total float64 `json:"total"`
		}
		if err := json.Unmarshal(costRaw, &costObj); err == nil {
			cost = costObj.Total
		}
	}

	projectPath := project
	if projectPath == "" {
		projectPath = "pi-agent"
	}

	return UsageEntry{
		Source:                   "pi",
		SessionID:                sessionID,
		ProjectPath:              projectPath,
		Timestamp:                ts.UTC(),
		Model:                    model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheWrite,
		CacheReadInputTokens:     cacheRead,
		CostUSD:                  cost,
	}, true
}

// piParseNumber tolerates JSON numbers encoded as either int or float.
func piParseNumber(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
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

// piSessionIDFromPath strips the `.jsonl` suffix and the leading
// `agent_` / `prefix_` (or any other `_`-separated word) from the filename
// stem. Mirrors ccusage's `extract_session_id`.
func piSessionIDFromPath(path string) string {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, ".jsonl")
	if stem == "" {
		return base
	}
	if i := strings.Index(stem, "_"); i >= 0 {
		return stem[i+1:]
	}
	return stem
}

// piProjectFromPath walks the path components and returns the segment
// immediately after the first "sessions" component (so
// `<root>/sessions/<project>/<file>` yields `<project>`). Returns empty
// string when no "sessions" marker exists.
func piProjectFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "sessions" {
			return parts[i+1]
		}
	}
	return ""
}

// piEntryKey mirrors ccusage's entry_id so duplicates across overlapping
// roots collapse to one emitted UsageEntry.
func piEntryKey(e UsageEntry) string {
	return strings.Join([]string{
		"pi",
		e.ProjectPath,
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
