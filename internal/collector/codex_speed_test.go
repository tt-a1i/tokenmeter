package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexSpeedAutoReadsConfig(t *testing.T) {
	home := setCodexSpeedTestHome(t)
	writeCodexConfig(t, home, `service_tier = "fast"`)
	SetCodexSpeedMode("auto")
	t.Cleanup(func() { SetCodexSpeedMode("auto") })

	got := ResolveCodexPricingSpeed()
	if got != CodexSpeedFast {
		t.Fatalf("auto speed = %v, want fast", got)
	}
}

func TestCodexSpeedExplicitStandard(t *testing.T) {
	home := setCodexSpeedTestHome(t)
	writeCodexConfig(t, home, `service_tier = "fast"`)
	SetCodexSpeedMode("standard")
	t.Cleanup(func() { SetCodexSpeedMode("auto") })

	if got := ResolveCodexPricingSpeed(); got != CodexSpeedStandard {
		t.Fatalf("explicit standard = %v, want standard", got)
	}
}

func TestCodexSpeedExplicitFast(t *testing.T) {
	setCodexSpeedTestHome(t)
	SetCodexSpeedMode("fast")
	t.Cleanup(func() { SetCodexSpeedMode("auto") })

	if got := ResolveCodexPricingSpeed(); got != CodexSpeedFast {
		t.Fatalf("explicit fast = %v, want fast", got)
	}
}

func TestCodexSpeedAutoMissingConfigFallsBackStandard(t *testing.T) {
	setCodexSpeedTestHome(t)
	SetCodexSpeedMode("auto")
	t.Cleanup(func() { SetCodexSpeedMode("auto") })

	if got := ResolveCodexPricingSpeed(); got != CodexSpeedStandard {
		t.Fatalf("missing config auto = %v, want standard", got)
	}
}

func TestCodexPricingFastTierChangesCost(t *testing.T) {
	SetCodexSpeedMode("standard")
	standard := estimateCodexCost(1_000, 100, 100, "gpt-5.4")
	SetCodexSpeedMode("fast")
	fast := estimateCodexCost(1_000, 100, 100, "gpt-5.4")
	SetCodexSpeedMode("auto")

	if fast != standard*2 {
		t.Fatalf("fast cost = %v, want 2x standard %v", fast, standard)
	}
}

func setCodexSpeedTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("CODEX_HOME", "")
	return home
}

func writeCodexConfig(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
