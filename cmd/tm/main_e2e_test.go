package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAdapterSubcommandsReachable builds the `tm` binary and invokes each
// of the 13 v1.1 adapter subcommands end-to-end via os/exec. It catches
// the class of P0 regression where the cli/Route + cli/RunAdapter unit
// tests pass but main.go's outer or inner dispatch switch is missing the
// adapter wiring — a gap that lets `tm amp daily` exit with
// "unknown command: amp" even though every internal layer is correct.
//
// Every adapter env-var is pinned to a single non-existent path so the
// test is hermetic (dev machines that happen to have real OpenCode /
// Hermes / etc. data on disk don't bleed in). With no real data, each
// adapter should return an empty entry set and the rendered JSON
// envelope should be a `(no data in range)` line or an empty JSON
// object/array — what we assert is "exit 0 + parsable JSON or empty
// envelope", not the row count.
func TestAdapterSubcommandsReachable(t *testing.T) {
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "tm-e2e")
	out, err := exec.Command("go", "build", "-o", bin, "./").CombinedOutput()
	if err != nil {
		t.Fatalf("build tm: %v\n%s", err, out)
	}

	hermetic := filepath.Join(t.TempDir(), "no-such-source")
	hermeticEnv := []string{
		"OPENCODE_DATA_DIR=" + hermetic,
		"AMP_DATA_DIR=" + hermetic,
		"GEMINI_DATA_DIR=" + hermetic,
		"COPILOT_OTEL_FILE_EXPORTER_PATH=" + hermetic,
		"GOOSE_PATH_ROOT=" + hermetic,
		"CODEBUFF_DATA_DIR=" + hermetic,
		"HERMES_HOME=" + hermetic,
		"KILO_DATA_DIR=" + hermetic,
		"KIMI_DATA_DIR=" + hermetic,
		"OPENCLAW_DIR=" + hermetic,
		"PI_AGENT_DIR=" + hermetic,
		"DROID_SESSIONS_DIR=" + hermetic,
		"QWEN_DATA_DIR=" + hermetic,
	}

	sources := []string{
		"amp", "codebuff", "copilot", "droid", "gemini", "goose",
		"hermes", "kilo", "kimi", "openclaw", "opencode", "pi", "qwen",
	}

	for _, src := range sources {
		src := src // capture for parallel-safe subtest
		t.Run(src, func(t *testing.T) {
			cmd := exec.Command(bin, src, "daily", "--json", "--no-color")
			cmd.Env = append(os.Environ(), hermeticEnv...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("tm %s daily failed: %v\nstdout:%s\nstderr:%s",
					src, err, stdout.String(), stderr.String())
			}
			// Validate JSON envelope. Empty data + --json should still
			// produce a parseable JSON document; tolerate both object
			// and array top-level shapes (renderer choice may evolve).
			trimmed := strings.TrimSpace(stdout.String())
			if trimmed == "" {
				// `--json` with no data: tolerable as long as exit 0.
				return
			}
			if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
				t.Fatalf("tm %s daily: stdout not JSON: %q", src, trimmed)
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("tm %s daily: stdout is not valid JSON:\n%s", src, stdout.String())
			}
		})
	}
}
