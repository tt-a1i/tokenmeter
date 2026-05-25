package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tt-a1i/tokenmeter/internal/appdir"
)

const EnvPath = "TOKENMETER_CONFIG"

type Config struct {
	Schema         string                     `json:"$schema,omitempty"`
	Defaults       Defaults                   `json:"defaults,omitempty"`
	Commands       map[string]CommandOverride `json:"commands,omitempty"`
	Pricing        PricingConfig              `json:"pricing,omitempty"`
	Webhooks       WebhookConfig              `json:"webhooks,omitempty"`
	Statusline     StatuslineConfig           `json:"statusline,omitempty"`
	Sources        map[string]SourceConfig    `json:"sources,omitempty"`
	LegacyPricing  bool                       `json:"-"`
	LegacyWebhooks bool                       `json:"-"`
}

type Defaults struct {
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	JSON       bool   `json:"json,omitempty"`
	Offline    bool   `json:"offline,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
	Mode       string `json:"mode,omitempty"`
	Order      string `json:"order,omitempty"`
	Breakdown  bool   `json:"breakdown,omitempty"`
	Project    string `json:"project,omitempty"`
	NoColor    bool   `json:"noColor,omitempty"`
	Speed      string `json:"speed,omitempty"`
	Compact    bool   `json:"compact,omitempty"`
	TokenLimit string `json:"token_limit,omitempty"`
}

type CommandOverride = Defaults

type SourceConfig struct {
	Defaults Defaults                   `json:"defaults,omitempty"`
	Commands map[string]CommandOverride `json:"commands,omitempty"`
}

type StatuslineConfig struct {
	QuotaUSD               float64 `json:"quota_usd,omitempty"`
	Format                 string  `json:"format,omitempty"`
	Color                  string  `json:"color,omitempty"`
	ContextLowThreshold    int     `json:"context_low_threshold,omitempty"`
	ContextMediumThreshold int     `json:"context_medium_threshold,omitempty"`
	BurnRateDisplay        string  `json:"burn_rate_display,omitempty"`
}

type PricingConfig struct {
	Claude         []PricingRule `json:"claude,omitempty"`
	Codex          []PricingRule `json:"codex,omitempty"`
	RuntimeSyncURL string        `json:"runtimeSyncURL,omitempty"`
	CacheTTLHours  int           `json:"cacheTTLHours,omitempty"`
}

type PricingRule struct {
	Match              []string `json:"match"`
	InputPerMillion    float64  `json:"inputPerMillion"`
	OutputPerMillion   float64  `json:"outputPerMillion"`
	CacheCreatePerMill float64  `json:"cacheCreatePerMill"`
	CacheReadPerMill   float64  `json:"cacheReadPerMill"`
	FastMultiplier     float64  `json:"fastMultiplier"`
}

type WebhookConfig struct {
	Endpoints []EndpointConfig `json:"endpoints,omitempty"`
}

type EndpointConfig struct {
	URL        string            `json:"url"`
	Events     []string          `json:"events,omitempty"`
	Format     string            `json:"format,omitempty"`
	Retry      RetryPolicy       `json:"retry,omitempty"`
	Thresholds WebhookThresholds `json:"thresholds,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts           int `json:"max_attempts,omitempty"`
	InitialBackoffSeconds int `json:"initial_backoff_seconds,omitempty"`
}

type WebhookThresholds struct {
	SessionHighCostUSD        float64 `json:"session_high_cost_usd,omitempty"`
	ToolFailureRatePct        float64 `json:"tool_failure_rate_pct,omitempty"`
	CostSpikeRatio            float64 `json:"cost_spike_ratio,omitempty"`
	RegressionFailureCountMin int     `json:"regression_failure_count_min,omitempty"`
	RegressionRatioMin        float64 `json:"regression_ratio_min,omitempty"`
}

type Options struct {
	ExplicitPath string
}

func Load() (*Config, error) {
	return LoadWithOptions(Options{})
}

func LoadWithOptions(opts Options) (*Config, error) {
	cfg := &Config{}
	if path := firstExistingPath(opts.ExplicitPath); path != "" {
		loaded, err := readConfig(path)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}
	if !cfg.HasPricing() {
		if pricing, ok, err := readLegacyPricing(); err != nil {
			return nil, err
		} else if ok {
			cfg.Pricing = pricing
			cfg.LegacyPricing = true
		}
	}
	if !cfg.HasWebhooks() {
		if webhooks, ok, err := readLegacyWebhooks(); err != nil {
			return nil, err
		} else if ok {
			cfg.Webhooks = webhooks
			cfg.LegacyWebhooks = true
		}
	}
	return cfg, nil
}

func Save(c *Config, path string) error {
	if c == nil {
		c = &Config{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func GlobalPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, appdir.CurrentDir, "config.json")
}

func EffectivePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv(EnvPath); env != "" {
		return env
	}
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, ".tokenmeter", "tokenmeter.json")
		if exists(local) {
			return local
		}
	}
	global := GlobalPath()
	if exists(global) {
		return global
	}
	return global
}

func Example() *Config {
	return &Config{
		Schema: "https://tokenmeter.dev/config-schema.json",
		Defaults: Defaults{
			Mode:     "auto",
			Order:    "asc",
			Timezone: "UTC",
		},
		Pricing: PricingConfig{
			CacheTTLHours: 24,
		},
		Webhooks: WebhookConfig{
			Endpoints: []EndpointConfig{},
		},
	}
}

func (c *Config) HasPricing() bool {
	if c == nil {
		return false
	}
	return len(c.Pricing.Claude) > 0 ||
		len(c.Pricing.Codex) > 0 ||
		c.Pricing.RuntimeSyncURL != "" ||
		c.Pricing.CacheTTLHours != 0
}

func (c *Config) HasWebhooks() bool {
	return c != nil && len(c.Webhooks.Endpoints) > 0
}

func (c *Config) EffectiveDefaults(command string) Defaults {
	if c == nil {
		return Defaults{}
	}
	out := Defaults{}
	mergeDefaults(&out, c.Defaults)
	if c.Commands != nil {
		mergeDefaults(&out, c.Commands[command])
	}
	return out
}

func mergeDefaults(dst *Defaults, src Defaults) {
	if src.Since != "" {
		dst.Since = src.Since
	}
	if src.Until != "" {
		dst.Until = src.Until
	}
	if src.JSON {
		dst.JSON = true
	}
	if src.Offline {
		dst.Offline = true
	}
	if src.Timezone != "" {
		dst.Timezone = src.Timezone
	}
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.Order != "" {
		dst.Order = src.Order
	}
	if src.Breakdown {
		dst.Breakdown = true
	}
	if src.Project != "" {
		dst.Project = src.Project
	}
	if src.NoColor {
		dst.NoColor = true
	}
	if src.Speed != "" {
		dst.Speed = src.Speed
	}
	if src.Compact {
		dst.Compact = true
	}
	if src.TokenLimit != "" {
		dst.TokenLimit = src.TokenLimit
	}
}

func firstExistingPath(explicit string) string {
	for _, path := range searchPaths(explicit) {
		if exists(path) {
			return path
		}
	}
	return ""
}

func searchPaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	if env := os.Getenv(EnvPath); env != "" {
		return []string{env}
	}
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".tokenmeter", "tokenmeter.json"))
	}
	paths = append(paths, GlobalPath())
	return paths
}

func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

func readLegacyPricing() (PricingConfig, bool, error) {
	path := appdir.PathFor("pricing.json", "pricing.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PricingConfig{}, false, nil
		}
		return PricingConfig{}, false, err
	}
	var cfg PricingConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return PricingConfig{}, false, fmt.Errorf("parse legacy pricing %s: %w", path, err)
	}
	return cfg, true, nil
}

func readLegacyWebhooks() (WebhookConfig, bool, error) {
	path := appdir.PathFor("webhooks.json", "webhooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WebhookConfig{}, false, nil
		}
		return WebhookConfig{}, false, err
	}
	var cfg WebhookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return WebhookConfig{}, false, fmt.Errorf("parse legacy webhooks %s: %w", path, err)
	}
	return cfg, true, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
