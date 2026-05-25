package cli

import (
	"fmt"
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

// bucketFromName maps the sub-command string (rest[0] when an adapter source
// drives the route) onto the cli Bucket enum. Defaults to daily.
func bucketFromName(s string) Bucket {
	switch s {
	case "weekly":
		return BucketWeekly
	case "monthly":
		return BucketMonthly
	case "session":
		return BucketSession
	default:
		return BucketDaily
	}
}

// Route parses argv into a Command. argv[0] is the subcommand name; any
// remaining tokens are passed through ParseShared so the shared flag set
// is honored uniformly across subcommands.
func Route(argv []string) (Command, error) {
	if len(argv) == 0 {
		return Command{
			Name:          "daily",
			AggregateArgs: AggregateArgs{Bucket: BucketDaily},
		}, nil
	}
	name := argv[0]
	tail := argv[1:]
	shared, rest, err := ParseShared(tail)
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
		bucket := bucketFromName(bucketName)
		cmd.Name = "adapter"
		cmd.Source = name
		cmd.AggregateArgs = AggregateArgs{Shared: shared, Bucket: bucket}
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
		args := SessionArgs{Shared: shared}
		if len(rest) > 0 {
			args.SessionID = rest[0]
		}
		cmd.SessionArgs = args
	case "blocks":
		cmd.BlocksArgs = BlocksArgs{
			Shared:        shared,
			SessionLength: shared.SessionLength,
			Active:        shared.Active,
			Now:           time.Now(),
		}
	case "statusline":
		// Args parsed inline; main wires stdin/stdout.
	case "pricing":
		if len(rest) == 0 || rest[0] != "refresh" {
			return Command{Name: "help"}, fmt.Errorf("unknown pricing command")
		}
		cmd.Name = "pricing:refresh"
	case "help", "":
		cmd.Name = "help"
	default:
		return Command{Name: "help"}, fmt.Errorf("unknown command: %s", name)
	}
	return cmd, nil
}
