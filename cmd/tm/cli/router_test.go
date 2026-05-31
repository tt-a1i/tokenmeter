package cli_test

import (
	"testing"
	"time"

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

func TestRouteBlocksCCUsageShortFlags(t *testing.T) {
	got, err := cli.Route([]string{"blocks", "-a", "-n", "1.5"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got.Name != "blocks" || !got.BlocksArgs.Active {
		t.Fatalf("expected active blocks route, got %+v", got)
	}
	if got.BlocksArgs.SessionLength != 90*time.Minute {
		t.Fatalf("SessionLength=%v want 90m", got.BlocksArgs.SessionLength)
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

func TestRouteDailyShortIIsInstances(t *testing.T) {
	got, err := cli.Route([]string{"daily", "-i", "--json"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if !got.Shared.Instances || got.Shared.ID != "" || !got.Shared.JSON {
		t.Fatalf("daily -i should mean instances without consuming --json: %+v", got.Shared)
	}

	got, err = cli.Route([]string{"claude", "daily", "-i", "--json"})
	if err != nil {
		t.Fatalf("Route claude daily: %v", err)
	}
	if !got.Shared.Instances || got.Shared.ID != "" || !got.Shared.JSON {
		t.Fatalf("claude daily -i should mean instances without consuming --json: %+v", got.Shared)
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

func TestRouteTopLevelSessionWithoutIDIsAllSource(t *testing.T) {
	cmd, err := cli.Route([]string{"session", "--json"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if cmd.Name != "session-all" || cmd.AggregateArgs.Bucket != cli.BucketSession || !cmd.Shared.JSON {
		t.Fatalf("expected all-source session route, got %+v", cmd)
	}
}

func TestRouteTopLevelSessionIDStillDetails(t *testing.T) {
	for _, argv := range [][]string{
		{"session", "--id", "abc"},
		{"session", "-i", "abc"},
		{"session", "abc"},
	} {
		cmd, err := cli.Route(argv)
		if err != nil {
			t.Fatalf("Route(%v): %v", argv, err)
		}
		if cmd.Name != "session" || cmd.SessionArgs.SessionID != "abc" || !cmd.SessionArgs.Detail {
			t.Fatalf("Route(%v) expected detail session, got %+v", argv, cmd)
		}
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

func TestRouteLegacyColonAgentCommands(t *testing.T) {
	tests := []struct {
		argv   []string
		source string
		bucket cli.Bucket
	}{
		{[]string{"codex:daily", "--json"}, "codex", cli.BucketDaily},
		{[]string{"opencode:weekly"}, "opencode", cli.BucketWeekly},
		{[]string{"amp:session"}, "amp", cli.BucketSession},
	}
	for _, tt := range tests {
		cmd, err := cli.Route(tt.argv)
		if err != nil {
			t.Fatalf("Route(%v): %v", tt.argv, err)
		}
		if cmd.Source != tt.source || cmd.AggregateArgs.Bucket != tt.bucket {
			t.Fatalf("Route(%v) source=%q bucket=%v", tt.argv, cmd.Source, cmd.AggregateArgs.Bucket)
		}
	}
}

func TestRouteSourceCommands(t *testing.T) {
	tests := []struct {
		argv     []string
		name     string
		platform string
		bucket   cli.Bucket
	}{
		{[]string{"claude"}, "daily", "claude", cli.BucketDaily},
		{[]string{"claude", "weekly"}, "weekly", "claude", cli.BucketWeekly},
		{[]string{"claude", "monthly"}, "monthly", "claude", cli.BucketMonthly},
		{[]string{"codex"}, "daily", "codex", cli.BucketDaily},
		{[]string{"codex", "monthly"}, "monthly", "codex", cli.BucketMonthly},
	}
	for _, tt := range tests {
		cmd, err := cli.Route(tt.argv)
		if err != nil {
			t.Fatalf("Route(%v): %v", tt.argv, err)
		}
		if cmd.Name != tt.name || cmd.AggregateArgs.Platform != tt.platform || cmd.AggregateArgs.Bucket != tt.bucket {
			t.Fatalf("Route(%v) = name %q platform %q bucket %v", tt.argv, cmd.Name, cmd.AggregateArgs.Platform, cmd.AggregateArgs.Bucket)
		}
	}
}

func TestRouteWeeklyStartOfWeek(t *testing.T) {
	for _, argv := range [][]string{
		{"weekly", "-w", "monday"},
		{"claude", "weekly", "--start-of-week", "monday"},
	} {
		cmd, err := cli.Route(argv)
		if err != nil {
			t.Fatalf("Route(%v): %v", argv, err)
		}
		if cmd.AggregateArgs.Shared.StartOfWeek != "monday" {
			t.Fatalf("Route(%v) StartOfWeek=%q", argv, cmd.AggregateArgs.Shared.StartOfWeek)
		}
	}
}

func TestRouteSourceSessionAndBlocks(t *testing.T) {
	session, err := cli.Route([]string{"claude", "session", "--id", "abc"})
	if err != nil {
		t.Fatalf("Route claude session: %v", err)
	}
	if session.Name != "session" || session.SessionArgs.Platform != "claude" || session.SessionArgs.SessionID != "abc" || !session.SessionArgs.Detail {
		t.Fatalf("unexpected claude session route: %+v", session)
	}
	blocks, err := cli.Route([]string{"claude", "blocks", "--recent"})
	if err != nil {
		t.Fatalf("Route claude blocks: %v", err)
	}
	if blocks.Name != "blocks" || blocks.BlocksArgs.Platform != "claude" || !blocks.BlocksArgs.Recent {
		t.Fatalf("unexpected claude blocks route: %+v", blocks)
	}
}

func TestRouteSourceSessionDefaultsOrderDesc(t *testing.T) {
	for _, source := range []string{"claude", "codex"} {
		t.Run(source, func(t *testing.T) {
			cmd, err := cli.Route([]string{source, "session"})
			if err != nil {
				t.Fatalf("Route: %v", err)
			}
			if cmd.SessionArgs.Shared.Order != "desc" {
				t.Fatalf("source session default order=%q want desc", cmd.SessionArgs.Shared.Order)
			}
		})
	}
}

func TestRouteSourceSessionHonorsExplicitOrderAsc(t *testing.T) {
	cmd, err := cli.Route([]string{"claude", "session", "--order", "asc"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if cmd.SessionArgs.Shared.Order != "asc" {
		t.Fatalf("explicit order=%q want asc", cmd.SessionArgs.Shared.Order)
	}
}

func TestRouteRejectsUnsupportedSourceCommands(t *testing.T) {
	for _, argv := range [][]string{
		{"codex", "weekly"},
		{"codex", "blocks"},
		{"codex", "statusline"},
	} {
		if _, err := cli.Route(argv); err == nil {
			t.Fatalf("Route(%v) expected error", argv)
		}
	}
}

func TestRouteAdapterSubcommandsMatchCCUsageSupport(t *testing.T) {
	if _, err := cli.Route([]string{"amp", "weekly"}); err == nil {
		t.Fatal("standard adapter weekly should fail")
	}
	if _, err := cli.Route([]string{"opencode", "weekly"}); err != nil {
		t.Fatalf("opencode weekly should route: %v", err)
	}
	if _, err := cli.Route([]string{"amp", "blocks"}); err == nil {
		t.Fatal("unknown adapter subcommand should fail")
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

func TestRouteConfigSubcommands(t *testing.T) {
	for _, sub := range []string{"show", "path", "init"} {
		cmd, err := cli.Route([]string{"config", sub})
		if err != nil {
			t.Fatalf("Route config %s: %v", sub, err)
		}
		if cmd.Name != "config:"+sub {
			t.Fatalf("Name=%q want config:%s", cmd.Name, sub)
		}
	}
}
