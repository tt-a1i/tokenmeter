package statusline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/appdir"
)

type statuslineCache struct {
	Date            string `json:"date"`
	LastOutput      string `json:"lastOutput"`
	LastUpdateTime  int64  `json:"lastUpdateTime"`
	TranscriptPath  string `json:"transcriptPath"`
	TranscriptMTime int64  `json:"transcriptMtime"`
	SessionID       string `json:"sessionId"`
	InputHash       string `json:"inputHash"`
}

// DefaultCachePath returns TokenMeter's per-session statusline cache path.
func DefaultCachePath(sessionID string) string {
	if sessionID == "" {
		sessionID = "unknown"
	}
	sum := sha256.Sum256([]byte(sessionID))
	return appdir.Path("statusline", hex.EncodeToString(sum[:])+".json")
}

func readStatuslineCache(path string) (*statuslineCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache statuslineCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func writeStatuslineCache(path string, cache statuslineCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func newStatuslineCache(input Input, cfg Config, opts RunOptions, output string, transcriptMTime int64, now time.Time) statuslineCache {
	return statuslineCache{
		Date:            now.UTC().Format(time.RFC3339Nano),
		LastOutput:      output,
		LastUpdateTime:  now.UnixMilli(),
		TranscriptPath:  input.TranscriptPath,
		TranscriptMTime: transcriptMTime,
		SessionID:       input.SessionID,
		InputHash:       statuslineInputHash(input, cfg, opts),
	}
}

func statuslineCacheMissReason(cache *statuslineCache, input Input, transcriptMTime int64, inputHash string, now time.Time, refreshInterval time.Duration) string {
	switch {
	case cache.LastOutput == "":
		return "empty-output"
	case cache.SessionID != "" && cache.SessionID != input.SessionID:
		return "session"
	case cache.TranscriptPath != input.TranscriptPath:
		return "transcript-path"
	case cache.TranscriptMTime != transcriptMTime:
		return "transcript-mtime"
	case cache.InputHash != "" && cache.InputHash != inputHash:
		return "input"
	case now.UnixMilli()-cache.LastUpdateTime >= refreshInterval.Milliseconds():
		return "refresh-interval"
	default:
		return ""
	}
}

func statuslineInputHash(input Input, cfg Config, opts RunOptions) string {
	payload := struct {
		Input      Input
		Config     Config
		CostSource string
		Timezone   string
	}{
		Input:      input,
		Config:     normalizeConfig(cfg),
		CostSource: opts.CostSource,
		Timezone:   opts.Timezone,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func transcriptMTimeMillis(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixMilli()
}

func debugStatuslineCache(opts RunOptions, format string, args ...any) {
	if !opts.Debug {
		return
	}
	w := opts.DebugWriter
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "statusline cache: "+format+"\n", args...)
}
