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

func writeRuntimeCacheFile(t *testing.T, fetchedAt time.Time, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pricing-cache.json")
	body := `{"fetchedAt":"` + fetchedAt.Format(time.RFC3339Nano) + `","data":` + raw + `}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
