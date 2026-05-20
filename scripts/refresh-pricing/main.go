// Command refresh-pricing downloads the LiteLLM model price catalog,
// strips it down to models TokenMeter cares about, and writes the result to
// internal/pricing/litellm-snapshot.json. Run via `go generate ./internal/pricing/...`.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	url     = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	target  = "internal/pricing/litellm-snapshot.json"
	timeout = 30 * time.Second
)

var allowedPrefixes = []string{
	"claude-", "anthropic.", "anthropic/", "us.anthropic.", "eu.anthropic.",
	"global.anthropic.", "jp.anthropic.", "au.anthropic.",
	"gpt-", "openai/", "azure/", "openrouter/openai/",
}

var keepFields = []string{
	"input_cost_per_token",
	"output_cost_per_token",
	"cache_creation_input_token_cost",
	"cache_read_input_token_cost",
	"input_cost_per_token_above_200k_tokens",
	"output_cost_per_token_above_200k_tokens",
	"cache_creation_input_token_cost_above_200k_tokens",
	"cache_read_input_token_cost_above_200k_tokens",
	"max_input_tokens",
	"provider_specific_entry",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "refresh-pricing:", err)
		os.Exit(1)
	}
}

func run() error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var raw map[string]map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("parse upstream JSON: %w", err)
	}
	out := map[string]map[string]any{}
	for model, fields := range raw {
		if !isAllowed(model) {
			continue
		}
		stripped := map[string]any{}
		for _, k := range keepFields {
			if v, ok := fields[k]; ok && v != nil {
				stripped[k] = v
			}
		}
		if _, hasIn := stripped["input_cost_per_token"]; !hasIn {
			continue
		}
		if _, hasOut := stripped["output_cost_per_token"]; !hasOut {
			continue
		}
		out[model] = stripped
	}
	// Determine output path relative to repo root.
	root, err := repoRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, target)
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]any, len(keys))
	for _, k := range keys {
		ordered[k] = out[k]
	}
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "refresh-pricing: wrote %d models to %s\n", len(out), path)
	return nil
}

func isAllowed(model string) bool {
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}

// repoRoot walks up from CWD until it finds a go.mod file.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate go.mod")
		}
		dir = parent
	}
}
