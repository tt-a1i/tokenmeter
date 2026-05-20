// Package statusline implements TokenMeter's Claude Code statusline provider:
// stdin → JSON status → stdout text line. See §4.3 of the design doc.
package statusline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/blocks"
)

// Input is the JSON payload Claude Code writes to stdin.
type Input struct {
	ModelID        string `json:"model_id"`
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
}

// Config controls quota + format. When QuotaUSD is 0, coloring is disabled
// (D11: default behavior — no color, just numbers).
type Config struct {
	QuotaUSD float64 `json:"quota_usd"`
	Format   string  `json:"format"` // "compact" (default) | "detailed"
	Color    bool    `json:"color"`
}

// LoadConfig reads the statusline config from path. If the file does not
// exist, returns a zero Config (no quota, no color). Other I/O errors are
// returned.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ParseInput reads and decodes Claude Code's stdin JSON.
func ParseInput(r io.Reader) (Input, error) {
	var in Input
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields() // be strict so upstream protocol changes surface early
	if err := dec.Decode(&in); err != nil {
		return Input{}, err
	}
	return in, nil
}

// Render writes a single-line statusline to w. `block` may be nil (e.g. no
// activity yet). `now` controls remaining-time math.
func Render(w io.Writer, in Input, block *blocks.SessionBlock, cfg Config, now time.Time) error {
	var parts []string
	parts = append(parts, modelEmoji(in.ModelID)+" "+shortModel(in.ModelID))
	if block != nil && block.IsActive {
		costStr := fmt.Sprintf("$%.2f", block.Cost)
		if cfg.QuotaUSD > 0 {
			costStr += "/" + fmt.Sprintf("$%.0f", cfg.QuotaUSD)
		}
		costStr += " (5h"
		if rem := remainingTime(block, now); rem > 0 {
			costStr += ", " + formatRemaining(rem)
		}
		if cfg.QuotaUSD > 0 && block.Projection != nil {
			costStr += ", " + onTrackLabel(block.Projection.TotalCost, cfg.QuotaUSD)
		}
		costStr += ")"
		parts = append(parts, costStr)
		parts = append(parts, formatTokens(block.Tokens.Total())+" tok")
	} else {
		parts = append(parts, "no active block")
	}
	line := strings.Join(parts, " ▎ ")
	if cfg.Color && cfg.QuotaUSD > 0 && block != nil && block.Projection != nil {
		line = colorize(line, block.Projection.TotalCost, cfg.QuotaUSD)
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

// remainingTime prefers the burn-rate annotated projection so the renderer
// honors the caller's `now` parameter (block.EndTime is wall-clock relative;
// using time.Until would ignore `now`). Falls back to EndTime - now when
// the projection is absent (e.g. brand-new block before burn rate is known).
func remainingTime(block *blocks.SessionBlock, now time.Time) time.Duration {
	if block.Projection != nil {
		return block.Projection.RemainingTime
	}
	return block.EndTime.Sub(now)
}

func modelEmoji(id string) string {
	switch {
	case strings.Contains(id, "opus"):
		return "🛸"
	case strings.Contains(id, "haiku"):
		return "🪶"
	case strings.Contains(id, "gpt"):
		return "🧠"
	default:
		return "🤖"
	}
}

func shortModel(id string) string {
	switch {
	case strings.Contains(id, "opus"):
		return "opus"
	case strings.Contains(id, "haiku"):
		return "haiku"
	case strings.Contains(id, "sonnet"):
		return "sonnet"
	case strings.HasPrefix(id, "gpt-"):
		return id
	}
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatRemaining(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf("%dm left", mins)
	}
	hrs := mins / 60
	rest := mins % 60
	if rest == 0 {
		return fmt.Sprintf("%dh left", hrs)
	}
	return fmt.Sprintf("%dh%dm left", hrs, rest)
}

func onTrackLabel(projected, quota float64) string {
	if projected <= quota*0.8 {
		return "on track"
	}
	if projected <= quota {
		return "approaching"
	}
	return "over budget"
}

func colorize(line string, projected, quota float64) string {
	const (
		green  = "\x1b[32m"
		yellow = "\x1b[33m"
		red    = "\x1b[31m"
		reset  = "\x1b[0m"
	)
	var pre string
	switch {
	case projected <= quota*0.8:
		pre = green
	case projected <= quota:
		pre = yellow
	default:
		pre = red
	}
	return pre + line + reset
}
