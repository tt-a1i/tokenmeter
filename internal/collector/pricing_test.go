package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPricingOverridesFromUnifiedConfig(t *testing.T) {
	restorePricingTablesAfterTest(t)
	home := setPricingTestHome(t)
	path := filepath.Join(home, ".tokenmeter", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"pricing":{"codex":[{"match":["unified-model"],"inputPerMillion":7,"outputPerMillion":11,"cacheReadPerMill":0.7}]}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	LoadPricingOverrides()

	input, output, cache := CodexPricing("unified-model")
	if input != 7 || output != 11 || cache != 0.7 {
		t.Fatalf("unified config pricing not applied: input=%v output=%v cache=%v", input, output, cache)
	}
}
