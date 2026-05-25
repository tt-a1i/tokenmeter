package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
)

func TestParseSharedFlags(t *testing.T) {
	args := []string{"--since", "20260520", "--until", "20260521", "--json", "--mode", "calculate"}
	got, rest, err := cli.ParseShared(args)
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if got.Since != "20260520" || got.Until != "20260521" {
		t.Fatalf("date flags not parsed: %+v", got)
	}
	if !got.JSON {
		t.Fatalf("--json must be true")
	}
	if got.Mode != "calculate" {
		t.Fatalf("mode=%q want calculate", got.Mode)
	}
	if len(rest) != 0 {
		t.Fatalf("rest must be empty, got %v", rest)
	}
}

func TestParseSharedPositionalUntouched(t *testing.T) {
	args := []string{"daily", "--json"}
	_, rest, err := cli.ParseShared(args)
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if len(rest) != 1 || rest[0] != "daily" {
		t.Fatalf("positional 'daily' must remain in rest, got %v", rest)
	}
}

func TestParseSharedSessionLength(t *testing.T) {
	got, _, err := cli.ParseShared([]string{"--session-length", "1h30m"})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if got.SessionLength != 90*time.Minute {
		t.Fatalf("SessionLength=%v want 1h30m", got.SessionLength)
	}
}

func TestParseSharedSpeedFlag(t *testing.T) {
	got, rest, err := cli.ParseShared([]string{"--speed", "fast", "daily"})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if got.Speed != "fast" {
		t.Fatalf("Speed=%q want fast", got.Speed)
	}
	if len(rest) != 1 || rest[0] != "daily" {
		t.Fatalf("rest=%v want [daily]", rest)
	}
}

func TestParseSharedRejectsInvalidSpeedFlag(t *testing.T) {
	_, _, err := cli.ParseShared([]string{"--speed", "turbo"})
	if err == nil {
		t.Fatal("expected invalid --speed to fail")
	}
}

func TestParseSharedUsesConfigDefaults(t *testing.T) {
	path := writeCLIConfig(t, `{"defaults":{"offline":true,"order":"desc","speed":"fast","compact":true,"token_limit":"max"}}`)
	got, _, err := cli.ParseShared([]string{"daily", "--config", path})
	if err != nil {
		t.Fatalf("ParseShared: %v", err)
	}
	if !got.Offline || got.Order != "desc" || got.Speed != "fast" || !got.Compact || got.TokenLimit != "max" {
		t.Fatalf("defaults not applied: %+v", got)
	}
}

func TestParseSharedUsesCommandOverrides(t *testing.T) {
	path := writeCLIConfig(t, `{"defaults":{"offline":true,"order":"asc"},"commands":{"daily":{"breakdown":true,"order":"desc"},"session":{"order":"desc"}}}`)
	daily, _, err := cli.ParseSharedForCommand("daily", []string{"--config", path})
	if err != nil {
		t.Fatalf("daily ParseSharedForCommand: %v", err)
	}
	if !daily.Offline || !daily.Breakdown || daily.Order != "desc" {
		t.Fatalf("daily command defaults mismatch: %+v", daily)
	}
	session, _, err := cli.ParseSharedForCommand("session", []string{"--config", path})
	if err != nil {
		t.Fatalf("session ParseSharedForCommand: %v", err)
	}
	if !session.Offline || session.Breakdown || session.Order != "desc" {
		t.Fatalf("session command defaults mismatch: %+v", session)
	}
}

func TestParseSharedCommandFalseOverridesConfigDefaultTrue(t *testing.T) {
	path := writeCLIConfig(t, `{"defaults":{"breakdown":true,"offline":true},"commands":{"daily":{"breakdown":false}}}`)
	got, _, err := cli.ParseSharedForCommand("daily", []string{"--config", path})
	if err != nil {
		t.Fatalf("ParseSharedForCommand: %v", err)
	}
	if got.Breakdown {
		t.Fatalf("commands.daily.breakdown=false should override default true: %+v", got)
	}
	if !got.Offline {
		t.Fatalf("unset command offline should inherit default true: %+v", got)
	}
}

func TestParseSharedCLIOverridesConfig(t *testing.T) {
	path := writeCLIConfig(t, `{"defaults":{"offline":true,"order":"asc","speed":"standard"},"commands":{"daily":{"order":"desc"}}}`)
	got, _, err := cli.ParseSharedForCommand("daily", []string{"--config", path, "--order", "asc", "--speed", "fast", "--offline=false"})
	if err != nil {
		t.Fatalf("ParseSharedForCommand: %v", err)
	}
	if got.Offline || got.Order != "asc" || got.Speed != "fast" {
		t.Fatalf("CLI flags should override config: %+v", got)
	}
}

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokenmeter.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
