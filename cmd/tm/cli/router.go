package cli

import (
	"fmt"
	"time"
)

// Command is a routed command — a Name plus the resolved args struct for the
// caller to dispatch on.
type Command struct {
	Name          string
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
		return Command{Name: "help"}, nil
	}
	name := argv[0]
	tail := argv[1:]
	shared, rest, err := ParseShared(tail)
	if err != nil {
		return Command{}, err
	}
	cmd := Command{Name: name, Shared: shared, Rest: rest}
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
			SessionLength: 5 * time.Hour, // default; --session-length wired in Task 23
			Active:        false,
			Now:           time.Now(),
		}
		for _, r := range rest {
			if r == "--active" {
				cmd.BlocksArgs.Active = true
			}
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
