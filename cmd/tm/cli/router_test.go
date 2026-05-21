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

func TestRouteNoArgsDefaultsToDaily(t *testing.T) {
	cmd, err := cli.Route(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "daily" {
		t.Fatalf("expected default 'daily', got %q", cmd.Name)
	}
	if cmd.AggregateArgs.Bucket != cli.BucketDaily {
		t.Fatalf("expected BucketDaily, got %v", cmd.AggregateArgs.Bucket)
	}
}

func TestRouteAmpDaily(t *testing.T) {
	cmd, err := cli.Route([]string{"amp", "daily", "--json"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if cmd.Name != "adapter" {
		t.Fatalf("Name=%q want adapter", cmd.Name)
	}
	if cmd.Source != "amp" {
		t.Fatalf("Source=%q want amp", cmd.Source)
	}
	if !cmd.Shared.JSON {
		t.Fatalf("shared flag not propagated")
	}
}

func TestRouteAllSourceUnchanged(t *testing.T) {
	cmd, err := cli.Route([]string{"daily", "--no-scan"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if cmd.Name != "daily" {
		t.Fatalf("Name=%q want daily", cmd.Name)
	}
	if !cmd.Shared.NoScan {
		t.Fatalf("--no-scan flag did not propagate")
	}
}
