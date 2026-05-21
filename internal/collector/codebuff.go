package collector

import (
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

// codebuffDataDirEnv is the comma-separated env var ccusage honors as the
// first-class override before the default channel list.
const codebuffDataDirEnv = "CODEBUFF_DATA_DIR"

// codebuffChannels mirrors ccusage's hard-coded list — each is a sibling
// directory under `~/.config/` that codebuff and its forks write into.
var codebuffChannels = []string{"manicode", "manicode-dev", "manicode-staging"}

// codebuffChatFileName is the canonical name of the JSON array the adapter
// reads. Any file with a different name under the chats subtree is skipped.
const codebuffChatFileName = "chat-messages.json"

// LoadCodebuffEntries walks every existing codebuff project root, parses
// each `chats/<chat>/chat-messages.json` file, and emits one UsageEntry
// per assistant message that carries token signal. Missing roots return
// (nil, nil) per Phase B contract.
func LoadCodebuffEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := codebuffProjectRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var out []UsageEntry
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := codebuffCollectChatFiles(root)
		if err != nil {
			log.Printf("warning: codebuff walk %s: %v", root, err)
			continue
		}
		for _, file := range files {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := parseCodebuffChatFile(file)
			if err != nil {
				log.Printf("warning: codebuff parse %s: %v", file, err)
				continue
			}
			for _, e := range entries {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && !e.Timestamp.Before(opts.Until) {
					continue
				}
				key := codebuffEntryKey(e)
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

// codebuffProjectRoots resolves the project-root directories to walk.
// CODEBUFF_DATA_DIR may either point at the channel root (`~/.config/<channel>`)
// or directly at its `projects/` subdir; ccusage tolerates both forms.
func codebuffProjectRoots() []string {
	var raws []string
	if env := strings.TrimSpace(os.Getenv(codebuffDataDirEnv)); env != "" {
		for _, p := range strings.Split(env, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				raws = append(raws, p)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		for _, ch := range codebuffChannels {
			raws = append(raws, filepath.Join(home, ".config", ch))
		}
	}

	seen := map[string]struct{}{}
	var roots []string
	for _, raw := range raws {
		root := raw
		if filepath.Base(root) != "projects" {
			root = filepath.Join(root, "projects")
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, dup := seen[root]; dup {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	return roots
}

// codebuffCollectChatFiles recursively walks `<project_root>` for files
// named exactly `chat-messages.json` — any other `.json` file under the
// tree is ignored.
func codebuffCollectChatFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: codebuff walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == codebuffChatFileName {
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

// codebuffContext captures the path-derived identifiers ccusage threads
// into each entry's session id and dedup key.
type codebuffContext struct {
	chatID    string
	project   string
	channel   string
	sessionID string
}

// codebuffDeriveContext mirrors ccusage's derive_context. The path layout
// expected is `<channel>/projects/<project>/chats/<chat>/chat-messages.json`;
// fewer segments fall back to "unknown" without failing the whole walk.
func codebuffDeriveContext(path string) codebuffContext {
	chatDir := filepath.Dir(path)           // .../chats/<chat>
	chatID := filepath.Base(chatDir)        // <chat>
	chatsDir := filepath.Dir(chatDir)       // .../chats
	projectDir := filepath.Dir(chatsDir)    // .../<project>
	project := filepath.Base(projectDir)    // <project>
	projectsDir := filepath.Dir(projectDir) // .../projects
	channelDir := filepath.Dir(projectsDir) // .../<channel>
	channel := filepath.Base(channelDir)    // <channel>
	if chatID == "" || chatID == "." || chatID == "/" {
		chatID = "unknown"
	}
	if project == "" || project == "." || project == "/" {
		project = "unknown"
	}
	if channel == "" || channel == "." || channel == "/" {
		channel = "manicode"
	}
	return codebuffContext{
		chatID:    chatID,
		project:   project,
		channel:   channel,
		sessionID: channel + "/" + project + "/" + chatID,
	}
}

// codebuffMessage captures the subset of fields the adapter inspects per
// message; ccusage's struct is wider but the Go side only needs assistant
// detection, usage, model, credits, and timestamp.
type codebuffMessage struct {
	Role      string          `json:"role"`
	Variant   string          `json:"variant"`
	Timestamp json.RawMessage `json:"timestamp"`
	CreatedAt json.RawMessage `json:"createdAt"`
	Credits   json.RawMessage `json:"credits"`
	Metadata  *struct {
		Model     string          `json:"model"`
		Timestamp json.RawMessage `json:"timestamp"`
		Usage     json.RawMessage `json:"usage"`
		Codebuff  *struct {
			Model string          `json:"model"`
			Usage json.RawMessage `json:"usage"`
		} `json:"codebuff"`
	} `json:"metadata"`
}

// parseCodebuffChatFile reads a single chat-messages.json file. The file is
// a JSON array of messages; one UsageEntry comes out per assistant message
// with non-zero token signal or non-zero credits.
func parseCodebuffChatFile(path string) ([]UsageEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages []codebuffMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		// Malformed chat-messages.json → silently skip; matches ccusage's
		// best-effort `serde_json::from_str(...).ok()` flow.
		return nil, nil
	}

	ctx := codebuffDeriveContext(path)
	fallbackTs := codebuffFileModTime(path)
	chatTs, _ := codebuffParseChatIDTimestamp(ctx.chatID)

	var out []UsageEntry
	for ordinal, msg := range messages {
		if !codebuffIsAssistant(msg) {
			continue
		}
		usage := codebuffExtractUsage(msg)
		if !codebuffHasSignal(usage) {
			continue
		}
		ts := codebuffMessageTimestamp(msg)
		if ts.IsZero() {
			ts = chatTs
		}
		if ts.IsZero() {
			ts = fallbackTs
		}
		model := strings.TrimSpace(usage.model)
		if model == "" {
			model = "codebuff-unknown"
		}
		out = append(out, UsageEntry{
			Source:                   "codebuff",
			SessionID:                ctx.sessionID,
			ProjectPath:              "Codebuff/" + ctx.project,
			Timestamp:                ts.UTC(),
			Model:                    model,
			InputTokens:              usage.input,
			OutputTokens:             usage.output,
			CacheCreationInputTokens: usage.cacheCreation,
			CacheReadInputTokens:     usage.cacheRead,
			// CostUSD: prefer the credits scalar from the source row;
			// downstream pricing layer can override under --mode calculate.
			CostUSD: usage.credits,
		})
		_ = ordinal // reserved for dedup_key extension when fixtures need it
	}
	return out, nil
}

func codebuffIsAssistant(m codebuffMessage) bool {
	role := strings.TrimSpace(m.Variant)
	if role == "" {
		role = strings.TrimSpace(m.Role)
	}
	switch role {
	case "ai", "agent", "assistant":
		return true
	default:
		return false
	}
}

// codebuffUsageBag is the merged token/credits view the adapter assembles
// for one message. It mirrors AssistantUsage in ccusage.
type codebuffUsageBag struct {
	model         string
	credits       float64
	input         int64
	output        int64
	cacheCreation int64
	cacheRead     int64
	total         int64
}

func codebuffExtractUsage(m codebuffMessage) codebuffUsageBag {
	var bag codebuffUsageBag
	if m.Metadata == nil {
		// No metadata block → can't extract usage, but message-level
		// credits may still surface.
		bag.credits = codebuffNumberFloat(m.Credits)
		return bag
	}
	bag.model = strings.TrimSpace(m.Metadata.Model)
	bag.merge(codebuffParseUsageObject(m.Metadata.Usage))
	if m.Metadata.Codebuff != nil {
		bag.merge(codebuffParseUsageObject(m.Metadata.Codebuff.Usage))
		if bag.model == "" {
			bag.model = strings.TrimSpace(m.Metadata.Codebuff.Model)
		}
	}
	// apply_total_token_fallback parity: when the per-bucket counters are
	// zero but `total` is populated, fold the total into output_tokens so
	// the message still contributes to summaries.
	if bag.input == 0 && bag.output == 0 && bag.cacheCreation == 0 && bag.cacheRead == 0 && bag.total > 0 {
		bag.output = bag.total
	}
	// Top-level credits win over metadata.usage.credits when the latter is
	// missing — codebuff sometimes reports credits at the message envelope.
	if c := codebuffNumberFloat(m.Credits); c > 0 && bag.credits <= 0 {
		bag.credits = c
	}
	return bag
}

// codebuffNumberFloat reads a numeric JSON token as float64 (for credits,
// which is fractional). Falls back to 0 for non-numeric inputs.
func codebuffNumberFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return f
		}
	}
	return 0
}

// merge folds `other` into `b` only where `b` has no value yet (mirrors
// ccusage's merge_fallback).
func (b *codebuffUsageBag) merge(other codebuffUsageBag) {
	if b.input == 0 {
		b.input = other.input
	}
	if b.output == 0 {
		b.output = other.output
	}
	if b.cacheCreation == 0 {
		b.cacheCreation = other.cacheCreation
	}
	if b.cacheRead == 0 {
		b.cacheRead = other.cacheRead
	}
	if b.total == 0 {
		b.total = other.total
	}
	if b.credits <= 0 {
		b.credits = other.credits
	}
	if b.model == "" {
		b.model = other.model
	}
}

func codebuffParseUsageObject(raw json.RawMessage) codebuffUsageBag {
	var bag codebuffUsageBag
	if len(raw) == 0 {
		return bag
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(raw, &record); err != nil {
		return bag
	}
	bag.input = codebuffPickInt64(record, []string{"inputTokens", "input_tokens", "promptTokens", "prompt_tokens"})
	bag.output = codebuffPickInt64(record, []string{"outputTokens", "output_tokens", "completionTokens", "completion_tokens"})
	bag.cacheRead = codebuffPickInt64(record, []string{"cacheReadInputTokens", "cache_read_input_tokens"})
	if v := codebuffPickNestedInt64(record, "promptTokensDetails", []string{"cachedTokens"}); v > bag.cacheRead {
		bag.cacheRead = v
	}
	if v := codebuffPickNestedInt64(record, "prompt_tokens_details", []string{"cached_tokens"}); v > bag.cacheRead {
		bag.cacheRead = v
	}
	bag.cacheCreation = codebuffPickInt64(record, []string{
		"cacheCreationInputTokens", "cache_creation_input_tokens",
		"cacheCreationTokens", "cache_creation_tokens",
		"cachedTokensCreated", "cached_tokens_created",
	})
	bag.total = codebuffPickInt64(record, []string{"totalTokens", "total_tokens", "total"})
	bag.credits = codebuffNumberFromMap(record, "credits")
	if v, ok := record["model"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			bag.model = strings.TrimSpace(s)
		}
	}
	return bag
}

func codebuffHasSignal(b codebuffUsageBag) bool {
	return b.input > 0 || b.output > 0 || b.cacheCreation > 0 || b.cacheRead > 0 || b.total > 0 || b.credits > 0
}

func codebuffPickInt64(record map[string]json.RawMessage, keys []string) int64 {
	for _, k := range keys {
		if v, ok := record[k]; ok {
			if n := codebuffNumber(v); n > 0 {
				return n
			}
		}
	}
	return 0
}

func codebuffPickNestedInt64(record map[string]json.RawMessage, outer string, inner []string) int64 {
	raw, ok := record[outer]
	if !ok {
		return 0
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return 0
	}
	return codebuffPickInt64(nested, inner)
}

func codebuffNumberFromMap(record map[string]json.RawMessage, key string) float64 {
	raw, ok := record[key]
	if !ok {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	return 0
}

// codebuffNumber tolerates ints, floats, and numeric strings. Returns the
// integer floor (cap at zero); used for token counters.
func codebuffNumber(raw json.RawMessage) int64 {
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
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func codebuffMessageTimestamp(m codebuffMessage) time.Time {
	if t := codebuffParseTimestamp(m.Timestamp); !t.IsZero() {
		return t
	}
	if t := codebuffParseTimestamp(m.CreatedAt); !t.IsZero() {
		return t
	}
	if m.Metadata != nil {
		if t := codebuffParseTimestamp(m.Metadata.Timestamp); !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

func codebuffParseTimestamp(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC()
		}
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
		if n < 10_000_000_000 {
			return time.Unix(n, 0).UTC()
		}
		return time.UnixMilli(n).UTC()
	}
	return time.Time{}
}

// codebuffParseChatIDTimestamp tries to recover a timestamp from a
// codebuff chat-id of the form "2026-04-15T10-30-00". Two `-` after the
// `T` are rewritten to `:` to give a parseable RFC3339 string.
func codebuffParseChatIDTimestamp(chatID string) (time.Time, bool) {
	if chatID == "" {
		return time.Time{}, false
	}
	idx := strings.Index(chatID, "T")
	if idx < 0 {
		return time.Time{}, false
	}
	date := chatID[:idx]
	timePart := chatID[idx+1:]
	// Replace at most two `-` with `:` to convert HH-MM-SS to HH:MM:SS.
	for i := 0; i < 2; i++ {
		j := strings.Index(timePart, "-")
		if j < 0 {
			break
		}
		timePart = timePart[:j] + ":" + timePart[j+1:]
	}
	candidate := date + "T" + timePart
	if t, err := time.Parse(time.RFC3339Nano, candidate); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02T15:04:05", candidate); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

func codebuffFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func codebuffEntryKey(e UsageEntry) string {
	return strings.Join([]string{
		"codebuff",
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
