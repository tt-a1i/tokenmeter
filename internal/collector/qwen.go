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

// qwenDataDirEnv is the comma-separated env var ccusage honors as the
// first-class override before the home-directory fallback.
const qwenDataDirEnv = "QWEN_DATA_DIR"

// qwenDefaultSubdir is the home-relative root ccusage walks when the env
// override is unset.
const qwenDefaultSubdir = ".qwen"

// qwenDefaultModel is the placeholder ccusage uses when a row carries
// usageMetadata but no `model` field.
const qwenDefaultModel = "unknown"

// LoadQwenEntries walks every existing Qwen root, parses the JSONL chat
// files that sit at exactly `<root>/projects/<project>/chats/<file>.jsonl`,
// and emits one UsageEntry per assistant message that carries token
// signal. Files at any other depth are skipped — the strict 3-component
// shape under `projects` mirrors ccusage's is_chat_file contract.
//
// Missing roots return (nil, nil) per Phase B contract.
func LoadQwenEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := qwenRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		projectsRoot := filepath.Join(root, "projects")
		info, err := os.Stat(projectsRoot)
		if err != nil || !info.IsDir() {
			continue
		}
		files, err := qwenCollectChatFiles(projectsRoot)
		if err != nil {
			log.Printf("warning: qwen walk %s: %v", projectsRoot, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := parseQwenChatFile(file, projectsRoot)
			if err != nil {
				log.Printf("warning: qwen parse %s: %v", file, err)
				continue
			}
			for _, e := range entries {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && !e.Timestamp.Before(opts.Until) {
					continue
				}
				key := qwenEntryKey(e)
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

// qwenRoots resolves the directories to walk. QWEN_DATA_DIR
// (comma-separated, dedup) overrides the single ~/.qwen fallback. Only
// existing directories make it through.
func qwenRoots() []string {
	if raw := strings.TrimSpace(os.Getenv(qwenDataDirEnv)); raw != "" {
		return ampExistingDirs(strings.Split(raw, ","))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return ampExistingDirs([]string{filepath.Join(home, qwenDefaultSubdir)})
}

// qwenCollectChatFiles recursively walks `<projectsRoot>` for `.jsonl`
// files whose path strictly matches `<project>/chats/<file>.jsonl` after
// stripping the projectsRoot prefix. ccusage's is_chat_file enforces the
// same 3-component shape — sub-sub-dirs or stray `.jsonl` files at other
// depths are filtered out.
func qwenCollectChatFiles(projectsRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: qwen walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if qwenIsChatFile(projectsRoot, path) {
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

// qwenIsChatFile checks that the file path under projectsRoot is exactly
// `<project>/chats/<file>.jsonl`. Anything shallower or deeper is skipped.
func qwenIsChatFile(projectsRoot, file string) bool {
	rel, err := filepath.Rel(projectsRoot, file)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 3 {
		return false
	}
	project, chats, leaf := parts[0], parts[1], parts[2]
	return project != "" && chats == "chats" && strings.HasSuffix(leaf, ".jsonl")
}

// qwenProjectFromFile walks the absolute path components from the end and
// returns the `<project>` segment of `projects/<project>/chats/<file>.jsonl`.
// Mirrors ccusage's project_from_file.
func qwenProjectFromFile(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	// Walk back-to-front looking for the 4-tuple [projects, project, chats, leaf].
	for i := len(parts) - 4; i >= 0; i-- {
		if parts[i] == "projects" && parts[i+2] == "chats" {
			return parts[i+1]
		}
	}
	return ""
}

// parseQwenChatFile streams one chat file. Each assistant row with a
// `usageMetadata` object becomes one UsageEntry.
func parseQwenChatFile(path, projectsRoot string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fallbackTs := qwenFileModTime(path)
	project := qwenProjectFromFile(path)
	if project == "" {
		project = "unknown"
	}
	fileStem := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	var out []UsageEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte(`"usageMetadata"`)) {
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		entry, ok := parseQwenLine(record, project, fileStem, fallbackTs)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	_ = projectsRoot
	return out, scanner.Err()
}

// parseQwenLine decodes one assistant record. Mirrors ccusage's parse_line
// — same usageMetadata key map, same apply_total_token_fallback, same
// session-id fallback to `<project>-<file-stem>`.
func parseQwenLine(record map[string]json.RawMessage, project, fileStem string, fallback time.Time) (UsageEntry, bool) {
	if t := qwenStringField(record, "type"); t != "assistant" {
		return UsageEntry{}, false
	}
	usageRaw, ok := record["usageMetadata"]
	if !ok {
		return UsageEntry{}, false
	}
	var usage map[string]json.RawMessage
	if err := json.Unmarshal(usageRaw, &usage); err != nil {
		return UsageEntry{}, false
	}
	input := qwenNumberField(usage, "promptTokenCount")
	output := qwenNumberField(usage, "candidatesTokenCount")
	reasoning := qwenNumberField(usage, "thoughtsTokenCount")
	cacheRead := qwenNumberField(usage, "cachedContentTokenCount")
	total := qwenNumberField(usage, "totalTokenCount")

	// apply_total_token_fallback parity: when input/output/cache_read are
	// all zero but `total` is populated, fold (total − reasoning) into
	// output so the row still contributes to summaries.
	if input == 0 && output == 0 && cacheRead == 0 {
		if total > 0 {
			extra := total - reasoning
			if extra < 0 {
				extra = 0
			}
			output = extra
		}
		if output == 0 && reasoning == 0 {
			return UsageEntry{}, false
		}
	}
	// Reasoning (thoughtsTokenCount) folds into OutputTokens — UsageEntry
	// has no dedicated extra-total slot. Same precedent the Gemini /
	// Copilot adapters set on the way in.
	output += reasoning

	ts := fallback
	if tsText := qwenStringField(record, "timestamp"); tsText != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, tsText); err == nil {
			ts = parsed
		}
	}

	model := qwenStringField(record, "model")
	if model == "" {
		model = qwenDefaultModel
	}

	sessionID := qwenStringField(record, "sessionId")
	if sessionID == "" {
		sessionID = project + "-" + fileStem
	}

	return UsageEntry{
		Source:                   "qwen",
		SessionID:                sessionID,
		ProjectPath:              project,
		Timestamp:                ts.UTC(),
		Model:                    model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     cacheRead,
		// CostUSD = 0 — Qwen pricing is computed by the cli layer when
		// --mode calculate (or auto with a zero source cost) is set.
	}, true
}

func qwenStringField(record map[string]json.RawMessage, key string) string {
	raw, ok := record[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func qwenNumberField(record map[string]json.RawMessage, key string) int64 {
	raw, ok := record[key]
	if !ok {
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

func qwenFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// qwenEntryKey mirrors ccusage's entry_id so duplicates across overlapping
// roots collapse to one emitted UsageEntry.
func qwenEntryKey(e UsageEntry) string {
	return strings.Join([]string{
		"qwen",
		e.SessionID,
		e.Timestamp.Format(time.RFC3339Nano),
		e.Model,
		strconv.FormatInt(e.InputTokens, 10),
		strconv.FormatInt(e.OutputTokens, 10),
		strconv.FormatInt(e.CacheReadInputTokens, 10),
	}, ":")
}
