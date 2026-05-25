package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSearchPathPriority(t *testing.T) {
	home := setConfigTestHome(t)
	wd := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	writeConfigFile(t, filepath.Join(home, ".tokenmeter", "config.json"), `{"defaults":{"timezone":"global"}}`)
	writeConfigFile(t, filepath.Join(wd, ".tokenmeter", "tokenmeter.json"), `{"defaults":{"timezone":"local"}}`)
	envPath := filepath.Join(t.TempDir(), "env.json")
	writeConfigFile(t, envPath, `{"defaults":{"timezone":"env"}}`)
	explicitPath := filepath.Join(t.TempDir(), "explicit.json")
	writeConfigFile(t, explicitPath, `{"defaults":{"timezone":"explicit"}}`)

	cfg, err := LoadWithOptions(Options{ExplicitPath: explicitPath})
	if err != nil {
		t.Fatalf("Load explicit: %v", err)
	}
	if cfg.Defaults.Timezone != "explicit" {
		t.Fatalf("explicit timezone=%q", cfg.Defaults.Timezone)
	}

	t.Setenv(EnvPath, envPath)
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load env: %v", err)
	}
	if cfg.Defaults.Timezone != "env" {
		t.Fatalf("env timezone=%q", cfg.Defaults.Timezone)
	}

	t.Setenv(EnvPath, "")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load local: %v", err)
	}
	if cfg.Defaults.Timezone != "local" {
		t.Fatalf("local timezone=%q", cfg.Defaults.Timezone)
	}
}

func TestLoadLegacyFallbackPricingAndWebhooks(t *testing.T) {
	home := setConfigTestHome(t)
	base := filepath.Join(home, ".tokenmeter")
	writeConfigFile(t, filepath.Join(base, "pricing.json"), `{"codex":[{"match":["legacy-model"],"inputPerMillion":1,"outputPerMillion":2}]}`)
	writeConfigFile(t, filepath.Join(base, "webhooks.json"), `{"endpoints":[{"url":"https://example.test/hook","events":["budget_warn"],"format":"json"}]}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Pricing.Codex) != 1 || cfg.Pricing.Codex[0].Match[0] != "legacy-model" {
		t.Fatalf("legacy pricing not merged: %+v", cfg.Pricing)
	}
	if len(cfg.Webhooks.Endpoints) != 1 || cfg.Webhooks.Endpoints[0].URL != "https://example.test/hook" {
		t.Fatalf("legacy webhooks not merged: %+v", cfg.Webhooks)
	}
	if !cfg.LegacyPricing || !cfg.LegacyWebhooks {
		t.Fatalf("legacy markers missing: pricing=%v webhooks=%v", cfg.LegacyPricing, cfg.LegacyWebhooks)
	}
}

func TestLoadNoFilesReturnsZeroValue(t *testing.T) {
	setConfigTestHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults != (Defaults{}) || cfg.HasPricing() || cfg.HasWebhooks() {
		t.Fatalf("cfg should be zero value, got %+v", cfg)
	}
}

func TestLoadMalformedJSONReturnsError(t *testing.T) {
	home := setConfigTestHome(t)
	writeConfigFile(t, filepath.Join(home, ".tokenmeter", "config.json"), `{"defaults":`)

	_, err := Load()
	if err == nil {
		t.Fatal("Load malformed JSON returned nil error")
	}
}

func TestLoadPartialUnifiedMergesLegacyMissingSections(t *testing.T) {
	home := setConfigTestHome(t)
	base := filepath.Join(home, ".tokenmeter")
	writeConfigFile(t, filepath.Join(base, "config.json"), `{"defaults":{"timezone":"UTC"}}`)
	writeConfigFile(t, filepath.Join(base, "pricing.json"), `{"claude":[{"match":["legacy-sonnet"],"inputPerMillion":3,"outputPerMillion":4}]}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Timezone != "UTC" {
		t.Fatalf("defaults not loaded: %+v", cfg.Defaults)
	}
	if len(cfg.Pricing.Claude) != 1 || cfg.Pricing.Claude[0].Match[0] != "legacy-sonnet" {
		t.Fatalf("legacy pricing should fill missing pricing section: %+v", cfg.Pricing)
	}
}

func TestSaveWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Save(&Config{Defaults: Defaults{Timezone: "UTC"}}, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg, err := LoadWithOptions(Options{ExplicitPath: path})
	if err != nil {
		t.Fatalf("Load saved: %v", err)
	}
	if cfg.Defaults.Timezone != "UTC" {
		t.Fatalf("saved config mismatch: %+v", cfg.Defaults)
	}
}

func setConfigTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv(EnvPath, "")
	return home
}

func writeConfigFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
