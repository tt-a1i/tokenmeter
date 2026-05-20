package cli_test

import (
	"testing"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
)

func TestRouterDispatchesDaily(t *testing.T) {
	got, err := cli.Route([]string{"daily", "--json"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Name != "daily" {
		t.Fatalf("expected daily, got %q", got.Name)
	}
}

func TestRouterUnknownReturnsHelp(t *testing.T) {
	got, err := cli.Route([]string{"asdf"})
	if err == nil && got.Name != "help" {
		t.Fatalf("unknown command must route to help or error, got %+v", got)
	}
}
