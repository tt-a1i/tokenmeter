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
	ModelID        string         `json:"model_id"`
	Model          *InputModel    `json:"model,omitempty"`
	SessionID      string         `json:"session_id"`
	CWD            string         `json:"cwd"`
	TranscriptPath string         `json:"transcript_path"`
	Cost           *InputCost     `json:"cost,omitempty"`
	ContextWindow  *ContextWindow `json:"context_window,omitempty"`
}

type InputModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type InputCost struct {
	TotalCostUSD float64 `json:"total_cost_usd"`
}

type ContextWindow struct {
	TotalInputTokens  int64 `json:"total_input_tokens"`
	ContextWindowSize int64 `json:"context_window_size"`
}

type Metrics struct {
	CostSource         string
	CCUsageSessionCost *float64
	TodayCost          float64
}

// Config controls quota + format. When QuotaUSD is 0, coloring is disabled
// (D11: default behavior — no color, just numbers).
type Config struct {
	QuotaUSD               float64 `json:"quota_usd"`
	Format                 string  `json:"format"` // "compact" (default) | "detailed"
	Color                  bool    `json:"color"`
	ContextLowThreshold    int     `json:"context_low_threshold"`
	ContextMediumThreshold int     `json:"context_medium_threshold"`
	BurnRateDisplay        string  `json:"burn_rate_display"`
}

// LoadConfig reads the statusline config from path. If the file does not
// exist, returns a zero Config (no quota, no color). Other I/O errors are
// returned.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return Config{}, err
	}
	c := defaultConfig()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	c = normalizeConfig(c)
	if err := ValidateConfig(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ParseInput reads and decodes Claude Code's stdin JSON.
func ParseInput(r io.Reader) (Input, error) {
	var in Input
	dec := json.NewDecoder(r)
	if err := dec.Decode(&in); err != nil {
		return Input{}, err
	}
	return in, nil
}

// Render writes a single-line statusline to w. `block` may be nil (e.g. no
// activity yet). `now` controls remaining-time math.
func Render(w io.Writer, in Input, block *blocks.SessionBlock, cfg Config, now time.Time) error {
	return RenderWithMetrics(w, in, block, cfg, now, defaultMetrics(in, block))
}

func RenderWithMetrics(w io.Writer, in Input, block *blocks.SessionBlock, cfg Config, now time.Time, metrics Metrics) error {
	cfg = normalizeConfig(cfg)
	modelID := inputModelID(in)
	modelName := inputModelName(in)
	var parts []string
	parts = append(parts, modelEmoji(modelID)+" "+shortModel(modelName))
	parts = append(parts, "session "+sessionCostDisplay(in, metrics))
	parts = append(parts, "today "+formatCurrency(metrics.TodayCost))
	if block != nil && block.IsActive {
		costStr := formatCurrency(block.Cost)
		if cfg.QuotaUSD > 0 {
			costStr += "/" + formatWholeCurrency(cfg.QuotaUSD)
		}
		costStr += " (5h"
		if rem := remainingTime(block, now); rem > 0 {
			costStr += ", " + formatRemaining(rem)
		}
		if cfg.QuotaUSD > 0 && block.Projection != nil {
			costStr += ", " + onTrackLabel(block.Projection.TotalCost, cfg.QuotaUSD)
		}
		costStr += ")"
		parts = append(parts, "block "+costStr)
		parts = append(parts, formatTokens(block.Tokens.Total())+" tok")
	} else {
		parts = append(parts, "no active block")
	}
	if in.ContextWindow != nil && in.ContextWindow.ContextWindowSize > 0 {
		percent := int(float64(in.ContextWindow.TotalInputTokens) * 100 / float64(in.ContextWindow.ContextWindowSize))
		parts = append(parts, "ctx "+colorContextPercent(percent, cfg))
	} else {
		parts = append(parts, "ctx N/A")
	}
	if block != nil && block.BurnRate != nil {
		if label := burnRateLabel(block.BurnRate, cfg.BurnRateDisplay); label != "" {
			parts = append(parts, "burn "+label)
		}
	}
	line := strings.Join(parts, " ▎ ")
	if cfg.Color && cfg.QuotaUSD > 0 && block != nil && block.Projection != nil {
		line = colorize(line, block.Projection.TotalCost, cfg.QuotaUSD)
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func defaultMetrics(in Input, block *blocks.SessionBlock) Metrics {
	var ccusage *float64
	if block != nil && block.IsActive {
		ccusage = floatPtr(block.Cost)
	}
	return Metrics{CostSource: "auto", CCUsageSessionCost: ccusage}
}

func defaultConfig() Config {
	return Config{
		ContextLowThreshold:    50,
		ContextMediumThreshold: 80,
		BurnRateDisplay:        "off",
	}
}

func normalizeConfig(c Config) Config {
	if c.ContextLowThreshold == 0 {
		c.ContextLowThreshold = 50
	}
	if c.ContextMediumThreshold == 0 {
		c.ContextMediumThreshold = 80
	}
	if c.BurnRateDisplay == "" {
		c.BurnRateDisplay = "off"
	}
	return c
}

func inputModelID(in Input) string {
	if in.ModelID != "" {
		return in.ModelID
	}
	if in.Model != nil && in.Model.ID != "" {
		return in.Model.ID
	}
	if in.Model != nil {
		return in.Model.DisplayName
	}
	return ""
}

func inputModelName(in Input) string {
	if in.Model != nil && in.Model.DisplayName != "" {
		return in.Model.DisplayName
	}
	return inputModelID(in)
}

func sessionCostDisplay(in Input, metrics Metrics) string {
	source := metrics.CostSource
	if source == "" {
		source = "auto"
	}
	cc := hookCost(in)
	ccusage := metrics.CCUsageSessionCost
	switch source {
	case "cc":
		return formatOptionalCurrency(cc)
	case "ccusage":
		return formatOptionalCurrency(ccusage)
	case "both":
		return fmt.Sprintf("%s cc / %s ccusage", formatOptionalCurrency(cc), formatOptionalCurrency(ccusage))
	default:
		if cc != nil {
			return formatCurrency(*cc)
		}
		return formatOptionalCurrency(ccusage)
	}
}

func hookCost(in Input) *float64 {
	if in.Cost == nil {
		return nil
	}
	return &in.Cost.TotalCostUSD
}

func formatOptionalCurrency(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return formatCurrency(*v)
}

func formatCurrency(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}

func formatWholeCurrency(v float64) string {
	return fmt.Sprintf("$%.0f", v)
}

func floatPtr(v float64) *float64 {
	return &v
}

func ValidateConfig(c Config) error {
	if c.ContextLowThreshold >= c.ContextMediumThreshold {
		return fmt.Errorf("context_low_threshold must be less than context_medium_threshold")
	}
	if !validBurnRateDisplay(c.BurnRateDisplay) {
		return fmt.Errorf("invalid burn_rate_display %q (want off, emoji, text, or emoji-text)", c.BurnRateDisplay)
	}
	return nil
}

func colorContextPercent(percent int, cfg Config) string {
	text := fmt.Sprintf("%d%%", percent)
	if !cfg.Color {
		return text
	}
	switch {
	case percent >= cfg.ContextMediumThreshold:
		return "\x1b[31m" + text + "\x1b[0m"
	case percent >= cfg.ContextLowThreshold:
		return "\x1b[33m" + text + "\x1b[0m"
	default:
		return "\x1b[32m" + text + "\x1b[0m"
	}
}

func burnRateLabel(rate *blocks.BurnRate, mode string) string {
	if mode == "" {
		mode = "emoji"
	}
	if mode == "off" {
		return ""
	}
	emoji, level := burnRateStatus(rate.TokensPerMinute)
	base := fmt.Sprintf("%s/hr", formatCurrency(rate.CostPerHour))
	switch mode {
	case "text":
		return base + " " + level
	case "emoji-text":
		return base + " " + emoji + " " + level
	default:
		return base + " " + emoji
	}
}

func burnRateStatus(tokensPerMinute float64) (string, string) {
	switch {
	case tokensPerMinute < 2_000:
		return "📉", "Low"
	case tokensPerMinute < 5_000:
		return "📈", "Steady"
	default:
		return "🔥", "High"
	}
}

func validBurnRateDisplay(mode string) bool {
	switch mode {
	case "off", "emoji", "text", "emoji-text":
		return true
	default:
		return false
	}
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
