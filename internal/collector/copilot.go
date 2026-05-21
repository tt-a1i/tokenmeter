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
	"strings"
	"time"
)

// copilotExporterPathEnv is the env override ccusage honors. Unlike every
// other batch adapter, this variable points directly at a JSONL FILE — not
// a directory — because the Copilot CLI writes its OpenTelemetry stream to
// one rotating file at a time.
const copilotExporterPathEnv = "COPILOT_OTEL_FILE_EXPORTER_PATH"

// copilotDefaultSubdir is the home-relative directory the Copilot CLI uses
// when the env override is unset.
const copilotDefaultSubdir = ".copilot/otel"

// LoadCopilotEntries scans Copilot CLI's OTEL JSONL output and emits one
// UsageEntry per `gen_ai.usage.*`-bearing row. Discovery rules:
//
//   - If COPILOT_OTEL_FILE_EXPORTER_PATH is set and resolves to an existing
//     regular file, only that file is parsed (no directory fallback).
//   - Otherwise every `*.jsonl` under `~/.copilot/otel/` is parsed.
//   - Missing path / dir / file → return (nil, nil) per Phase B contract.
//
// Note: ccusage combines both default dir AND env file in one run. We treat
// the env override as exclusive for deterministic tests and to keep the
// "user pointed me at a single export" semantic unambiguous. Cross-source
// dedup (Chat span > Inference log > Agent turn > Agent summary) is not yet
// implemented — every row that carries gen_ai.usage.input_tokens surfaces
// as its own entry. Track as a follow-up if dedup becomes critical.
func LoadCopilotEntries(ctx context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	files, err := copilotResolveFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	var out []UsageEntry
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := parseCopilotOtelFile(file)
		if err != nil {
			log.Printf("warning: copilot parse %s: %v", file, err)
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
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func copilotResolveFiles() ([]string, error) {
	if raw := strings.TrimSpace(os.Getenv(copilotExporterPathEnv)); raw != "" {
		info, err := os.Stat(raw)
		if err != nil || info.IsDir() {
			return nil, nil
		}
		return []string{raw}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	root := filepath.Join(home, copilotDefaultSubdir)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var files []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("warning: copilot walk-entry %s: %v", path, err)
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

// copilotSessionAttrs orders session-id attribute keys by ccusage's priority
// scheme; higher numbers win. Stable across both file format (span attrs +
// log attrs use the same keys).
var copilotSessionAttrs = []struct {
	key      string
	priority int
}{
	{"gen_ai.conversation.id", 3},
	{"copilot_chat.session_id", 3},
	{"copilot_chat.chat_session_id", 3},
	{"session.id", 3},
	{"github.copilot.interaction_id", 2},
	{"gen_ai.response.id", 1},
}

var copilotModelAttrs = []string{"gen_ai.response.model", "gen_ai.request.model"}

// parseCopilotOtelFile streams a single Copilot OTEL JSONL file. Rows are
// pre-filtered on the cheap `\"attributes\"` substring (the bulk of OTEL
// output is metric / context envelopes that don't carry token usage).
func parseCopilotOtelFile(path string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fallback := copilotFileModTime(path)

	var out []UsageEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte(`"attributes"`)) {
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		attrs, ok := record["attributes"]
		if !ok {
			continue
		}
		var attrMap map[string]json.RawMessage
		if err := json.Unmarshal(attrs, &attrMap); err != nil {
			continue
		}
		entry, ok := parseCopilotRecord(record, attrMap, fallback)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out, scanner.Err()
}

// parseCopilotRecord pulls a UsageEntry out of a single OTEL row. Returns
// false when the row carries no gen_ai.usage.* counters (i.e. it's some
// other span/metric/log type that happens to have an `attributes` block).
func parseCopilotRecord(record, attrMap map[string]json.RawMessage, fallback time.Time) (UsageEntry, bool) {
	input := copilotAttrNumber(attrMap, "gen_ai.usage.input_tokens")
	output := copilotAttrNumber(attrMap, "gen_ai.usage.output_tokens")
	cacheRead := copilotAttrNumber(attrMap, "gen_ai.usage.cache_read.input_tokens")
	cacheCreate := copilotAttrNumberFirst(attrMap, []string{
		"gen_ai.usage.cache_write.input_tokens",
		"gen_ai.usage.cache_creation.input_tokens",
	})
	reasoning := copilotAttrNumberFirst(attrMap, []string{
		"gen_ai.usage.reasoning.output_tokens",
		"gen_ai.usage.reasoning_tokens",
	})
	total := copilotAttrNumberFirst(attrMap, []string{
		"gen_ai.usage.total_tokens",
		"gen_ai.usage.total.token_count",
	})

	// Mirror ccusage: subtract the cached overlap from input before display.
	if cacheRead > 0 && cacheRead < input {
		input -= cacheRead
	} else if cacheRead >= input {
		input = 0
	}

	// Skip rows with no token signal at all (these are metric envelopes,
	// non-usage spans, etc.).
	if input == 0 && output == 0 && cacheRead == 0 && cacheCreate == 0 && total == 0 && reasoning == 0 {
		return UsageEntry{}, false
	}

	// apply_total_token_fallback parity: when every per-bucket counter is
	// zero and `total` carries a usable count, surface (total − reasoning)
	// on output so the row still contributes to summaries.
	if input == 0 && output == 0 && cacheRead == 0 && cacheCreate == 0 && total > 0 {
		extra := total - reasoning
		if extra < 0 {
			extra = 0
		}
		output = extra
	}

	model := copilotAttrFirstNonEmpty(attrMap, copilotModelAttrs)
	if model == "" {
		model = "unknown"
	}

	sessionID, _ := copilotBestSessionAttr(attrMap)
	if sessionID == "" {
		sessionID = copilotStringAt(record, "traceId")
	}
	if sessionID == "" {
		sessionID = "unknown-session"
	}

	ts := copilotTimestamp(record, fallback)

	// Fold reasoning into output_tokens — UsageEntry has no extra-total slot
	// and the aggregation pipeline needs the count to survive. Same approach
	// as the Gemini adapter.
	output += reasoning

	return UsageEntry{
		Source:                   "copilot",
		SessionID:                sessionID,
		ProjectPath:              "GitHub Copilot CLI",
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: cacheCreate,
		CacheReadInputTokens:     cacheRead,
		// CostUSD = 0 by design — OTEL doesn't carry billing data; the cli
		// pricing layer recomputes when --mode auto/calculate is requested.
	}, true
}

// copilotAttrNumber reads a numeric attribute. OTEL exporters frequently
// encode counters as floats; we accept both forms.
func copilotAttrNumber(attrs map[string]json.RawMessage, key string) int64 {
	raw, ok := attrs[key]
	if !ok {
		return 0
	}
	return copilotParseNumber(raw)
}

func copilotAttrNumberFirst(attrs map[string]json.RawMessage, keys []string) int64 {
	for _, k := range keys {
		if v := copilotAttrNumber(attrs, k); v > 0 {
			return v
		}
	}
	return 0
}

func copilotParseNumber(raw json.RawMessage) int64 {
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
	// String numeric fallback (rare but ccusage tolerates).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var parsed int64
		if _, scanErr := stringToInt64(strings.TrimSpace(s), &parsed); scanErr == nil {
			return parsed
		}
	}
	return 0
}

// stringToInt64 is a tiny stdlib-only number parser to avoid pulling
// strconv into the call site twice.
func stringToInt64(s string, out *int64) (int, error) {
	if s == "" {
		return 0, errEmpty
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errNotNumber
		}
	}
	var n int64
	for i := 0; i < len(s); i++ {
		n = n*10 + int64(s[i]-'0')
	}
	*out = n
	return len(s), nil
}

var (
	errEmpty     = &stringErr{"empty"}
	errNotNumber = &stringErr{"not a number"}
)

type stringErr struct{ msg string }

func (e *stringErr) Error() string { return e.msg }

func copilotAttrFirstNonEmpty(attrs map[string]json.RawMessage, keys []string) string {
	for _, k := range keys {
		if v := copilotAttrString(attrs, k); v != "" {
			return v
		}
	}
	return ""
}

func copilotAttrString(attrs map[string]json.RawMessage, key string) string {
	raw, ok := attrs[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func copilotStringAt(record map[string]json.RawMessage, key string) string {
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

// copilotBestSessionAttr picks the highest-priority session attribute that
// has a non-empty value. Priority ties are resolved by document order.
func copilotBestSessionAttr(attrs map[string]json.RawMessage) (string, int) {
	bestPriority := -1
	var bestValue string
	for _, spec := range copilotSessionAttrs {
		if v := copilotAttrString(attrs, spec.key); v != "" {
			if spec.priority > bestPriority {
				bestPriority = spec.priority
				bestValue = v
			}
		}
	}
	return bestValue, bestPriority
}

// copilotTimestamp tries the OTEL timestamp shapes in ccusage order:
// endTime [seconds, nanos] → startTime → hrTime → time → timestamp scalar →
// timeUnixNano. Falls back to file mod time.
func copilotTimestamp(record map[string]json.RawMessage, fallback time.Time) time.Time {
	for _, key := range []string{"endTime", "startTime", "hrTime", "_hrTime", "time"} {
		if t, ok := copilotTimestampFromParts(record[key]); ok {
			return t
		}
	}
	if t, ok := copilotTimestampFromScalar(record["timestamp"]); ok {
		return t
	}
	if t, ok := copilotTimestampFromScalar(record["observedTimestamp"]); ok {
		return t
	}
	if t, ok := copilotTimestampFromUnixNanos(record["timeUnixNano"]); ok {
		return t
	}
	return fallback
}

func copilotTimestampFromParts(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 {
		return time.Time{}, false
	}
	var parts [2]int64
	if err := json.Unmarshal(raw, &parts); err != nil {
		return time.Time{}, false
	}
	if parts[0] < 0 {
		return time.Time{}, false
	}
	return time.Unix(parts[0], parts[1]).UTC(), true
}

func copilotTimestampFromScalar(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 {
		return time.Time{}, false
	}
	// Try string RFC3339 first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UTC(), true
		}
	}
	// Then numeric: ccusage tolerates millis/micros/nanos based on magnitude.
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
		return copilotMagnitudeToTime(n), true
	}
	return time.Time{}, false
}

func copilotTimestampFromUnixNanos(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 {
		return time.Time{}, false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil || n <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, n).UTC(), true
}

// copilotMagnitudeToTime mirrors ccusage's timestamp_from_scalar — it
// interprets a bare integer as the most plausible unit based on size.
func copilotMagnitudeToTime(n int64) time.Time {
	const (
		nanosThreshold  = int64(100_000_000_000_000_000)
		microsThreshold = int64(100_000_000_000_000)
		millisThreshold = int64(100_000_000_000)
	)
	switch {
	case n >= nanosThreshold:
		return time.Unix(0, n).UTC()
	case n >= microsThreshold:
		return time.UnixMicro(n).UTC()
	case n >= millisThreshold:
		return time.UnixMilli(n).UTC()
	default:
		return time.Unix(n, 0).UTC()
	}
}

func copilotFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
