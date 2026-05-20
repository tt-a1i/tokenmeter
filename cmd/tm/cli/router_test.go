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

func TestRouterBlocksActive(t *testing.T) {
	got, err := cli.Route([]string{"blocks", "--active"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Name != "blocks" {
		t.Fatalf("name=%q want blocks", got.Name)
	}
	if !got.BlocksArgs.Active {
		t.Fatal("BlocksArgs.Active should be true")
	}
}

func TestRouterDailyJSON(t *testing.T) {
	got, err := cli.Route([]string{"daily", "--json"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if !got.Shared.JSON {
		t.Fatal("Shared.JSON should be true")
	}
}

func TestRouterDailySince(t *testing.T) {
	got, err := cli.Route([]string{"daily", "--since", "20260101"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Shared.Since != "20260101" {
		t.Fatalf("Since=%q want 20260101", got.Shared.Since)
	}
}
