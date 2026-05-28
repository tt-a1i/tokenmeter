package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/pricing"
)

const (
	kimiEnvVar         = "KIMI_DATA_DIR"
	kimiDefaultModel   = "kimi-for-coding"
	kimiSessionsSubdir = "sessions"
	kimiWireFileName   = "wire.jsonl"
	kimiConfigFile     = "config.json"
	kimiSource         = "kimi"

	// kimiForCodingK26CutoffMs is the inclusive boundary at which the
	// default "kimi-for-coding" model switches from kimi-k2.5 to
	// kimi-k2.6 pricing. Mirrors ccusage's KIMI_FOR_CODING_K2_6_CUTOFF_MS
	// in rust/crates/ccusage/src/adapter/kimi/parser.rs:18.
	kimiForCodingK26CutoffMs int64 = 1_776_698_890_072
)

var (
	kimiPricingOnce sync.Once
	kimiPricingMap  *pricing.Map
)

// getKimiPricing returns a lazily-initialized pricing map. Defers the
// embedded-snapshot parse off package init so importing collector stays
// cheap for callers that never touch Kimi entries.
func getKimiPricing() *pricing.Map {
	kimiPricingOnce.Do(func() {
		kimiPricingMap = pricing.LoadEmbedded()
	})
	return kimiPricingMap
}

// kimiForCodingPricingModel maps a timestamp to the per-period Moonshot
// model name used for cost calculation when the configured display model
// is the default "kimi-for-coding". Display model is intentionally left
// unchanged on the UsageEntry so the user-facing breakdown still shows
// "kimi-for-coding"; only cost calculation re-routes to k2.5 or k2.6.
func kimiForCodingPricingModel(ts time.Time) string {
	if ts.UnixMilli() < kimiForCodingK26CutoffMs {
		return "moonshot/kimi-k2.5"
	}
	return "moonshot/kimi-k2.6"
}

// computeKimiCostUSD returns the per-entry cost. Non-default models stay
// at 0 so the AllSource ModeAuto path re-prices via pricing.Resolve on
// the configured model name.
func computeKimiCostUSD(model string, ts time.Time, in, out, cc, cr int64) float64 {
	if model != kimiDefaultModel {
		return 0
	}
	candidate := kimiForCodingPricingModel(ts)
	p, ok := getKimiPricing().Resolve(candidate)
	if !ok {
		return 0
	}
	return pricing.CalculateCost(p, pricing.Usage{
		Input:       in,
		Output:      out,
		CacheCreate: cc,
		CacheRead:   cr,
	}, pricing.SpeedStandard)
}

// LoadKimiEntries scans Kimi CLI wire.jsonl files under the directories
// listed in $KIMI_DATA_DIR (comma-separated) or, if the env var is unset,
// $HOME/.kimi.
//
// Per-root layout (mirrors ccusage v20's kimi.rs adapter):
//
//	<root>/config.json                       (optional; "model" key)
//	<root>/sessions/<group>/<session>/wire.jsonl
//
// Each wire.jsonl line is JSON; only lines whose message.type is
// "StatusUpdate" and whose payload carries a token_usage object
// contribute a UsageEntry. Field mapping:
//
//	input_other          -> InputTokens
//	output               -> OutputTokens
//	input_cache_creation -> CacheCreationInputTokens
//	input_cache_read     -> CacheReadInputTokens
//	total (fallback)     -> OutputTokens (when individual parts are all zero)
//
// CostUSD policy mirrors ccusage's calculate_kimi_cost
// (rust/crates/ccusage/src/adapter/kimi/parser.rs:208-254): when the
// display model is the default "kimi-for-coding", cost is computed per
// entry against the period-appropriate Moonshot price
// (moonshot/kimi-k2.5 before the k2.6 cutoff, moonshot/kimi-k2.6 at or
// after) without changing the visible Model field. Any other configured
// model leaves CostUSD at zero so RunAggregateAllSource's ModeAuto path
// re-prices via pricing.Resolve.
//
// A missing/unreadable data root is treated as "user does not have Kimi
// installed" and yields (nil, nil) so the merge loop continues.
func LoadKimiEntries(_ context.Context, opts AdapterOpts) ([]UsageEntry, error) {
	roots := kimiRoots()
	if len(roots) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var entries []UsageEntry
	for _, root := range roots {
		model := readKimiModel(filepath.Join(root, kimiConfigFile))
		files := discoverKimiWireFiles(filepath.Join(root, kimiSessionsSubdir))
		for _, file := range files {
			sessionID := filepath.Base(filepath.Dir(file))
			rows, err := readKimiWireFile(file, sessionID, model)
			if err != nil {
				continue
			}
			for _, e := range rows {
				if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
					continue
				}
				if !opts.Until.IsZero() && e.Timestamp.After(opts.Until) {
					continue
				}
				key := kimiDedupKey(e)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				entries = append(entries, e)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

// kimiRoots resolves the candidate Kimi data roots. When KIMI_DATA_DIR is
// set it wins outright (no home fallback); empty/non-directory paths are
// silently dropped. Duplicates are removed in input order.
func kimiRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	if env := strings.TrimSpace(os.Getenv(kimiEnvVar)); env != "" {
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
	p := filepath.Join(home, ".kimi")
	if isDir(p) {
		roots = append(roots, p)
	}
	return roots
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// readKimiModel parses the optional <root>/config.json "model" field.
// Any failure (file missing, malformed JSON, empty value) returns the
// ccusage-compatible default model name.
func readKimiModel(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return kimiDefaultModel
	}
	var cfg struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return kimiDefaultModel
	}
	if cfg.Model == "" {
		return kimiDefaultModel
	}
	return cfg.Model
}

// discoverKimiWireFiles returns wire.jsonl files whose relative path under
// sessionsDir is exactly <group>/<session>/wire.jsonl (3 components). This
// matches ccusage's is_kimi_wire_file check and prevents stray .jsonl
// files at other depths from being parsed.
func discoverKimiWireFiles(sessionsDir string) []string {
	if !isDir(sessionsDir) {
		return nil
	}
	var files []string
	_ = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || d.Name() != kimiWireFileName {
			return nil
		}
		rel, err := filepath.Rel(sessionsDir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 3 {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

// readKimiWireFile reads wire.jsonl line-by-line and extracts every
// StatusUpdate token_usage record. Malformed lines and lines lacking the
// expected "StatusUpdate" / "token_usage" string markers are skipped
// without erroring (the file may contain TurnBegin, metadata, and other
// envelope types).
func readKimiWireFile(path, sessionID, model string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	statusUpdateMarker := []byte(`"StatusUpdate"`)
	tokenUsageMarker := []byte(`"token_usage"`)
	scanner := bufio.NewScanner(f)
	// Kimi wire.jsonl lines can exceed the default 64KiB.
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)

	var out []UsageEntry
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, statusUpdateMarker) || !bytes.Contains(line, tokenUsageMarker) {
			continue
		}
		if e, ok := parseKimiWireLine(line, sessionID, model); ok {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}

// parseKimiWireLine extracts a UsageEntry from one StatusUpdate JSONL row.
// Returns (zero, false) for malformed JSON, missing payload, or rows whose
// token_usage payload sums to zero with no usable total fallback.
func parseKimiWireLine(line []byte, sessionID, model string) (UsageEntry, bool) {
	var raw struct {
		Type      string  `json:"type"`
		Timestamp float64 `json:"timestamp"`
		Message   struct {
			Type    string `json:"type"`
			Payload struct {
				MessageID  string `json:"message_id"`
				TokenUsage struct {
					InputOther         int64 `json:"input_other"`
					Output             int64 `json:"output"`
					InputCacheCreation int64 `json:"input_cache_creation"`
					InputCacheRead     int64 `json:"input_cache_read"`
					Total              int64 `json:"total"`
				} `json:"token_usage"`
			} `json:"payload"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return UsageEntry{}, false
	}
	if raw.Type == "metadata" || raw.Message.Type != "StatusUpdate" {
		return UsageEntry{}, false
	}
	u := raw.Message.Payload.TokenUsage
	in, out, cc, cr := u.InputOther, u.Output, u.InputCacheCreation, u.InputCacheRead
	if in+out+cc+cr == 0 {
		if u.Total > 0 {
			// ccusage's apply_total_token_fallback drops the total into
			// the output bucket when no individual parts are present.
			out = u.Total
		} else {
			return UsageEntry{}, false
		}
	}
	if raw.Timestamp == 0 || math.IsNaN(raw.Timestamp) || math.IsInf(raw.Timestamp, 0) {
		return UsageEntry{}, false
	}
	secs := int64(raw.Timestamp)
	nanos := int64((raw.Timestamp - float64(secs)) * 1e9)
	ts := time.Unix(secs, nanos).UTC()
	return UsageEntry{
		Source:                   kimiSource,
		SessionID:                sessionID,
		Timestamp:                ts,
		Model:                    model,
		InputTokens:              in,
		OutputTokens:             out,
		CacheCreationInputTokens: cc,
		CacheReadInputTokens:     cr,
		CostUSD:                  computeKimiCostUSD(model, ts, in, out, cc, cr),
	}, true
}

// kimiDedupKey mirrors ccusage's kimi_entry_key — multiple roots
// pointing at the same Kimi installation must not double-count rows.
func kimiDedupKey(e UsageEntry) string {
	return fmt.Sprintf("%s|%s|%d|%d|%d|%d|%d",
		e.SessionID, e.Model, e.Timestamp.UnixNano(),
		e.InputTokens, e.OutputTokens,
		e.CacheCreationInputTokens, e.CacheReadInputTokens,
	)
}
