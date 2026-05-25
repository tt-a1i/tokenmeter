package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
)

func TestConfigPath_UsesAppdirRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TOKENMETER_HOME", root)
	t.Setenv("TOKENMETER_CONFIG", "")

	var out bytes.Buffer
	if err := cli.RunConfig(t.Context(), &out, "path", ""); err != nil {
		t.Fatalf("RunConfig path: %v", err)
	}
	want := filepath.Join(root, "config.json")
	if got := strings.TrimSpace(out.String()); got != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}
}

func TestConfigInit_UsesAppdirRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TOKENMETER_HOME", root)
	t.Setenv("TOKENMETER_CONFIG", "")

	var out bytes.Buffer
	if err := cli.RunConfig(t.Context(), &out, "init", ""); err != nil {
		t.Fatalf("RunConfig init: %v", err)
	}
	want := filepath.Join(root, "config.json")
	if got := strings.TrimSpace(out.String()); got != "created "+want {
		t.Fatalf("config init output = %q, want %q", got, "created "+want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("config file was not created at %q: %v", want, err)
	}
}

func TestConfigShow_UsesAppdirRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TOKENMETER_HOME", root)
	t.Setenv("TOKENMETER_CONFIG", "")
	if err := cli.RunConfig(t.Context(), bytes.NewBuffer(nil), "init", ""); err != nil {
		t.Fatalf("RunConfig init: %v", err)
	}

	var out bytes.Buffer
	if err := cli.RunConfig(t.Context(), &out, "show", ""); err != nil {
		t.Fatalf("RunConfig show: %v", err)
	}
	if !strings.Contains(out.String(), `"defaults"`) {
		t.Fatalf("config show output missing defaults: %s", out.String())
	}
}
