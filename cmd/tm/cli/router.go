package cli

import (
	"fmt"
	"strings"
	"time"
)

// Command is a routed command — a Name plus the resolved args struct for the
// caller to dispatch on.
type Command struct {
	Name string
	// Source is the batch-adapter source name when Name=="adapter" (one of
	// "amp", "opencode", etc.); empty for SQLite-backed sources and the
	// all-source merge path.
	Source        string
	Alias         string // original argv[0] when arrived via a deprecated alias; "" otherwise
	Shared        Shared
	BlocksArgs    BlocksArgs
	AggregateArgs AggregateArgs
	SessionArgs   SessionArgs
	Rest          []string
}

// adapterNameSet lists every batch-adapter source the router accepts as a
// top-level command. Each adapter supports the daily/weekly/monthly/session
// sub-commands via the same Shared flag set; main dispatches them through
// the adapter registry in Task 3+.
var adapterNameSet = map[string]struct{}{
	"amp": {}, "opencode": {}, "gemini": {}, "copilot": {}, "goose": {}, "codebuff": {},
	"hermes": {}, "kilo": {}, "kimi": {}, "openclaw": {}, "pi": {}, "droid": {}, "qwen": {},
}

var sourceCommandReports = map[string]map[string]Bucket{
	"claude": {
		"daily":      BucketDaily,
		"weekly":     BucketWeekly,
		"monthly":    BucketMonthly,
		"session":    BucketSession,
		"blocks":     BucketDaily,
		"statusline": BucketDaily,
	},
	"codex": {
		"daily":   BucketDaily,
		"monthly": BucketMonthly,
		"session": BucketSession,
	},
}

// bucketFromName maps the sub-command string (rest[0] when an adapter source
// drives the route) onto the cli Bucket enum.
func bucketFromName(s string) (Bucket, bool) {
	switch s {
	case "", "daily":
		return BucketDaily, true
	case "weekly":
		return BucketWeekly, true
	case "monthly":
		return BucketMonthly, true
	case "session":
		return BucketSession, true
	default:
		return BucketDaily, false
	}
}

// Route parses argv into a Command. argv[0] is the subcommand name; any
// remaining tokens are passed through ParseShared so the shared flag set
// is honored uniformly across subcommands.
func Route(argv []string) (Command, error) {
	argv = normalizeLegacyAgentCommandArgs(argv)
	if len(argv) == 0 {
		return Command{
			Name:          "daily",
			AggregateArgs: AggregateArgs{Bucket: BucketDaily},
		}, nil
	}
	name := argv[0]
	tail := argv[1:]
	shared, rest, err := ParseSharedForCommand(name, tail)
	if err != nil {
		return Command{}, err
	}
	cmd := Command{Name: name, Shared: shared, Rest: rest}
	if target, ok := IsDeprecatedAlias(name); ok {
		cmd.Alias = name
		cmd.Name = "deprecated:" + target
		return cmd, nil
	}
	if _, ok := adapterNameSet[name]; ok {
		// Sub-command is rest[0] if present, else "daily".
		bucketName := "daily"
		if len(rest) > 0 {
			bucketName = rest[0]
		}
		bucket, ok := bucketFromName(bucketName)
		if !ok {
			return Command{Name: "help"}, fmt.Errorf("unknown %s command: %s", name, bucketName)
		}
		if bucket == BucketWeekly && name != "opencode" {
			return Command{Name: "help"}, fmt.Errorf("%s does not support weekly reports", name)
		}
		cmd.Name = "adapter"
		cmd.Source = name
		cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: bucket}
		return cmd, nil
	}
	if reports, ok := sourceCommandReports[name]; ok {
		reportName := "daily"
		if len(rest) > 0 {
			reportName = rest[0]
		}
		bucket, ok := reports[reportName]
		if !ok {
			return Command{Name: "help"}, fmt.Errorf("%s does not support %s reports", name, reportName)
		}
		cmd.Source = name
		switch reportName {
		case "blocks":
			cmd.Name = "blocks"
			cmd.BlocksArgs = BlocksArgs{
				Shared:        shared,
				Platform:      name,
				SessionLength: shared.SessionLength,
				Active:        shared.Active,
				Recent:        shared.Recent,
				Now:           time.Now(),
			}
		case "statusline":
			cmd.Name = "statusline"
		case "session":
			cmd.Name = "session"
			if !shared.OrderSet && shared.Order == "asc" {
				shared.Order = "desc"
			}
			args := SessionArgs{Shared: shared, Platform: name, SessionID: shared.ID, Detail: shared.ID != ""}
			if args.SessionID == "" && len(rest) > 1 {
				args.SessionID = rest[1]
			}
			cmd.SessionArgs = args
		default:
			cmd.Name = reportName
			cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: bucket, Platform: name}
		}
		return cmd, nil
	}
	switch name {
	case "daily":
		cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: BucketDaily}
	case "weekly":
		cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: BucketWeekly}
	case "monthly":
		cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: BucketMonthly}
	case "session":
		args := SessionArgs{Shared: shared, SessionID: shared.ID, Detail: shared.ID != ""}
		if len(rest) > 0 {
			args.SessionID = rest[0]
			args.Detail = true
		}
		if args.SessionID == "" {
			cmd.Name = "session-all"
			cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: BucketSession}
		} else {
			cmd.SessionArgs = args
		}
	case "blocks":
		cmd.BlocksArgs = BlocksArgs{
			Shared:        shared,
			SessionLength: shared.SessionLength,
			Active:        shared.Active,
			Recent:        shared.Recent,
			Now:           time.Now(),
		}
	case "statusline":
		// Args parsed inline; main wires stdin/stdout.
	case "pricing":
		if len(rest) == 0 || rest[0] != "refresh" {
			return Command{Name: "help"}, fmt.Errorf("unknown pricing command")
		}
		cmd.Name = "pricing:refresh"
	case "config":
		if len(rest) == 0 {
			return Command{Name: "help"}, fmt.Errorf("unknown config command")
		}
		switch rest[0] {
		case "show", "path", "init":
			cmd.Name = "config:" + rest[0]
		default:
			return Command{Name: "help"}, fmt.Errorf("unknown config command: %s", rest[0])
		}
	case "help", "":
		cmd.Name = "help"
	default:
		return Command{Name: "help"}, fmt.Errorf("unknown command: %s", name)
	}
	return cmd, nil
}

func normalizeLegacyAgentCommandArgs(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	agent, report, ok := splitLegacyAgentCommand(argv[0])
	if !ok {
		return argv
	}
	out := make([]string, 0, len(argv)+1)
	out = append(out, agent, report)
	out = append(out, argv[1:]...)
	return out
}

func splitLegacyAgentCommand(arg string) (string, string, bool) {
	agent, report, ok := strings.Cut(arg, ":")
	if !ok || !agentReportSupported(agent, report) {
		return "", "", false
	}
	return agent, report, true
}

func agentReportSupported(agent, report string) bool {
	if _, ok := adapterNameSet[agent]; ok {
		bucket, ok := bucketFromName(report)
		if !ok {
			return false
		}
		return bucket != BucketWeekly || agent == "opencode"
	}
	reports, ok := sourceCommandReports[agent]
	if !ok {
		return false
	}
	_, ok = reports[report]
	return ok
}
