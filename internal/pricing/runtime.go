package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/appdir"
)

const (
	RuntimeCacheTTL = 24 * time.Hour

	liteLLMPrimaryURL  = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window_from_litellm.json"
	liteLLMFallbackURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
)

// RuntimeOptions controls runtime LiteLLM pricing refresh. Zero values use
// production defaults; tests inject paths, time, fetch, and async behavior.
type RuntimeOptions struct {
	Offline   bool
	CachePath string
	Now       func() time.Time
	Fetch     func(context.Context, string) ([]byte, error)
	Async     func(func())
}

type runtimeCache struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Data      json.RawMessage `json:"data"`
}

// DefaultCachePath returns the user-writable runtime pricing cache path.
func DefaultCachePath() string {
	return appdir.PathFor("pricing-cache.json", "pricing-cache.json")
}

// LoadRuntime returns embedded pricing enriched by the runtime cache. Fresh
// cache is used directly. Stale cache is still preferred over the embedded
// fallback and refreshed asynchronously. Missing cache performs one best-effort
// synchronous refresh; network failures fall back without error.
func LoadRuntime(ctx context.Context, opts RuntimeOptions) (*Map, error) {
	opts = fillRuntimeOptions(opts)
	m := LoadEmbedded()
	offline := opts.Offline || envOffline()

	cache, cacheOK := readRuntimeCache(opts.CachePath)
	if cacheOK {
		_ = m.LoadJSON(cache.Data)
		if offline || opts.Now().Sub(cache.FetchedAt) < RuntimeCacheTTL {
			return m, nil
		}
		opts.Async(func() {
			_, _ = RefreshRuntime(context.Background(), opts)
		})
		return m, nil
	}

	if offline {
		return m, nil
	}
	refreshed, err := RefreshRuntime(ctx, opts)
	if err != nil {
		return m, nil
	}
	return refreshed, nil
}

// RefreshRuntime fetches LiteLLM pricing and updates the runtime cache.
func RefreshRuntime(ctx context.Context, opts RuntimeOptions) (*Map, error) {
	opts = fillRuntimeOptions(opts)
	if opts.Offline || envOffline() {
		return nil, fmt.Errorf("pricing refresh skipped: offline mode")
	}
	data, err := opts.Fetch(ctx, liteLLMPrimaryURL)
	if err != nil {
		data, err = opts.Fetch(ctx, liteLLMFallbackURL)
	}
	if err != nil {
		return nil, err
	}
	m := LoadEmbedded()
	if err := m.LoadJSON(data); err != nil {
		return nil, fmt.Errorf("parse LiteLLM pricing: %w", err)
	}
	if err := writeRuntimeCache(opts.CachePath, opts.Now(), data); err != nil {
		return nil, err
	}
	return m, nil
}

func fillRuntimeOptions(opts RuntimeOptions) RuntimeOptions {
	if opts.CachePath == "" {
		opts.CachePath = DefaultCachePath()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Fetch == nil {
		opts.Fetch = defaultRuntimeFetch
	}
	if opts.Async == nil {
		opts.Async = func(fn func()) { go fn() }
	}
	return opts
}

func readRuntimeCache(path string) (runtimeCache, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeCache{}, false
	}
	var cache runtimeCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return runtimeCache{}, false
	}
	if cache.FetchedAt.IsZero() || len(cache.Data) == 0 {
		return runtimeCache{}, false
	}
	return cache, true
}

func writeRuntimeCache(path string, fetchedAt time.Time, data []byte) error {
	cache := runtimeCache{FetchedAt: fetchedAt, Data: append(json.RawMessage(nil), data...)}
	out, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

func defaultRuntimeFetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing fetch %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func envOffline() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("TOKENMETER_OFFLINE")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
