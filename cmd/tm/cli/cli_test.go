package cli_test

import (
	"testing"

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
