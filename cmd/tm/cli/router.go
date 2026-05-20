package cli

import (
	"fmt"
	"time"
)

// Command is a routed command — a Name plus the resolved args struct for the
// caller to dispatch on.
type Command struct {
	Name          string
	Alias         string // original argv[0] when arrived via a deprecated alias; "" otherwise
	Shared        Shared
	BlocksArgs    BlocksArgs
	AggregateArgs AggregateArgs
	SessionArgs   SessionArgs
	Rest          []string
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
	case "help", "":
		cmd.Name = "help"
	default:
		return Command{Name: "help"}, fmt.Errorf("unknown command: %s", name)
	}
	return cmd, nil
}
