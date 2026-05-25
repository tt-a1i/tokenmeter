package pricing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRuntimeUsesFreshCache(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	cachePath := writeRuntimeCacheFile(t, now.Add(-time.Hour), `{
		"runtime-model": {
			"input_cost_per_token": 0.000001,
			"output_cost_per_token": 0.000002,
			"cache_read_input_token_cost": 0.0000001
		}
	}`)
	fetches := 0

	m, err := LoadRuntime(context.Background(), RuntimeOptions{
		CachePath: cachePath,
		Now:       func() time.Time { return now },
		Fetch: func(context.Context, string) ([]byte, error) {
			fetches++
			return nil, errors.New("should not fetch fresh cache")
		},
		Async: func(fn func()) { fn() },
	})
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if fetches != 0 {
		t.Fatalf("fresh cache should not fetch, got %d fetches", fetches)
	}
	p, ok := m.Resolve("runtime-model")
	if !ok || p.Input != 0.000001 || p.Output != 0.000002 || p.CacheRead != 0.0000001 {
		t.Fatalf("cache pricing not loaded: ok=%v pricing=%+v", ok, p)
	}
}

func TestLoadRuntimeExpiredCacheTriggersRefresh(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	cachePath := writeRuntimeCacheFile(t, now.Add(-25*time.Hour), `{
		"runtime-model": {
			"input_cost_per_token": 0.000001,
			"output_cost_per_token": 0.000002
		}
	}`)
	refreshed := make(chan struct{}, 1)

	m, err := LoadRuntime(context.Background(), RuntimeOptions{
		CachePath: cachePath,
		Now:       func() time.Time { return now },
		Fetch: func(context.Context, string) ([]byte, error) {
			refreshed <- struct{}{}
			return []byte(`{"runtime-model":{"input_cost_per_token":0.000003,"output_cost_per_token":0.000004}}`), nil
		},
		Async: func(fn func()) { go fn() },
	})
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	p, ok := m.Resolve("runtime-model")
	if !ok || p.Input != 0.000001 {
		t.Fatalf("expired cache should still be used immediately: ok=%v pricing=%+v", ok, p)
	}
	select {
	case <-refreshed:
	case <-time.After(time.Second):
		t.Fatal("expired cache did not trigger background refresh")
	}
}

func TestLoadRuntimeOfflineSkipsNetwork(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	cachePath := writeRuntimeCacheFile(t, now.Add(-48*time.Hour), `{
		"offline-model": {
			"input_cost_per_token": 0.000005,
			"output_cost_per_token": 0.000006
		}
	}`)
	fetches := 0

	m, err := LoadRuntime(context.Background(), RuntimeOptions{
		CachePath: cachePath,
		Offline:   true,
		Now:       func() time.Time { return now },
		Fetch: func(context.Context, string) ([]byte, error) {
			fetches++
			return nil, nil
		},
		Async: func(fn func()) { fn() },
	})
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if fetches != 0 {
		t.Fatalf("offline mode should skip network, got %d fetches", fetches)
	}
	if _, ok := m.Resolve("offline-model"); !ok {
		t.Fatal("offline mode should still use stale cache")
	}
}

func TestLoadRuntimeNetworkFailureFallsBack(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	cachePath := filepath.Join(t.TempDir(), "missing-cache.json")

	m, err := LoadRuntime(context.Background(), RuntimeOptions{
		CachePath: cachePath,
		Now:       func() time.Time { return now },
		Fetch: func(context.Context, string) ([]byte, error) {
			return nil, errors.New("network down")
		},
		Async: func(fn func()) { fn() },
	})
	if err != nil {
		t.Fatalf("LoadRuntime should degrade gracefully, got %v", err)
	}
	if _, ok := m.Resolve("claude-sonnet-4-6"); !ok {
		t.Fatal("fallback embedded pricing should remain available")
	}
}

func TestLoadRuntimeUsesConfigPricingOptions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("TOKENMETER_CONFIG", "")
	configPath := filepath.Join(home, ".tokenmeter", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"pricing":{"runtimeSyncURL":"https://example.test/pricing.json","cacheTTLHours":72}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotURL string
	m, err := LoadRuntime(context.Background(), RuntimeOptions{
		CachePath: filepath.Join(t.TempDir(), "pricing-cache.json"),
		Now:       func() time.Time { return time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC) },
		Fetch: func(_ context.Context, url string) ([]byte, error) {
			gotURL = url
			return []byte(`{"config-model":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000002}}`), nil
		},
		Async: func(fn func()) { fn() },
	})
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if gotURL != "https://example.test/pricing.json" {
		t.Fatalf("runtime sync URL = %q", gotURL)
	}
	if _, ok := m.Resolve("config-model"); !ok {
		t.Fatal("config runtime sync pricing was not loaded")
	}
}

func TestLiteLLMParseRealSnapshot(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "litellm", "sample.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Map{}
	if err := m.LoadJSON(data); err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	if _, ok := m.Lookup("sample_spec"); ok {
		t.Fatal("sample_spec documentation entry should not be loaded as pricing")
	}

	stringModel, ok := m.Resolve("openai/gpt-5-string-schema")
	if !ok {
		t.Fatal("string schema model was not loaded")
	}
	if stringModel.Input != 0.00000125 || stringModel.Output != 0.00001 {
		t.Fatalf("string model costs = input %g output %g", stringModel.Input, stringModel.Output)
	}
	if stringModel.MaxInputTokens != 1048576 {
		t.Fatalf("string model max input tokens = %d", stringModel.MaxInputTokens)
	}
	if stringModel.FastMultiplier != 2.5 {
		t.Fatalf("string model fast multiplier = %g", stringModel.FastMultiplier)
	}
	if stringModel.InputAbove200K == nil || *stringModel.InputAbove200K != 0.0000025 {
		t.Fatalf("string model above-200k input cost = %v", stringModel.InputAbove200K)
	}

	numberModel, ok := m.Resolve("openai/gpt-5-number-schema")
	if !ok {
		t.Fatal("number schema model was not loaded")
	}
	if numberModel.Input != 0.000003 || numberModel.MaxInputTokens != 128000 || numberModel.FastMultiplier != 2 {
		t.Fatalf("number model pricing = %+v", numberModel)
	}
}

func writeRuntimeCacheFile(t *testing.T, fetchedAt time.Time, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pricing-cache.json")
	body := `{"fetchedAt":"` + fetchedAt.Format(time.RFC3339Nano) + `","data":` + raw + `}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
