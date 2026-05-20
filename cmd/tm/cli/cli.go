// Package cli implements the v1.0 ccusage-style command surface.
package cli

import (
	"flag"
	"fmt"
	"time"
)

// Shared holds CLI flags common to every subcommand. See spec §4.1.
type Shared struct {
	Since         string
	Until         string
	JSON          bool
	Mode          string
	Order         string
	Breakdown     bool
	Offline       bool
	Timezone      string
	Project       string
	NoColor       bool
	JQ            string
	Config        string
	SessionLength time.Duration
}

// ParseShared extracts shared flags from args. Returns the remaining
// non-flag tokens for the subcommand to dispatch on.
func ParseShared(args []string) (Shared, []string, error) {
	var s Shared
	fs := flag.NewFlagSet("shared", flag.ContinueOnError)
	fs.StringVar(&s.Since, "since", "", "start date YYYYMMDD")
	fs.StringVar(&s.Until, "until", "", "end date YYYYMMDD")
	fs.BoolVar(&s.JSON, "json", false, "emit JSON instead of table")
	fs.StringVar(&s.Mode, "mode", "auto", "cost mode: auto | calculate | display")
	fs.StringVar(&s.Order, "order", "asc", "sort order: asc | desc")
	fs.BoolVar(&s.Breakdown, "breakdown", false, "break down by model")
	fs.BoolVar(&s.Offline, "offline", false, "skip online pricing refresh")
	fs.StringVar(&s.Timezone, "timezone", "", "timezone for date bucketing")
	fs.StringVar(&s.Project, "project", "", "filter by workspace path")
	fs.BoolVar(&s.NoColor, "no-color", false, "disable color")
	fs.StringVar(&s.JQ, "jq", "", "post-filter JSON via jq expression")
	fs.StringVar(&s.Config, "config", "", "config file path")
	var sessionLengthStr string
	fs.StringVar(&sessionLengthStr, "session-length", "5h", "duration of one session block (e.g. 5h, 1h30m)")
	// Allow flags to be interspersed with positionals: stdlib flag.Parse stops
	// at the first non-flag token, so we drive it in a loop and accumulate the
	// real positionals separately.
	var rest []string
	remaining := args
	for {
		if err := fs.Parse(remaining); err != nil {
			return Shared{}, nil, fmt.Errorf("parse flags: %w", err)
		}
		tail := fs.Args()
		if len(tail) == 0 {
			break
		}
		rest = append(rest, tail[0])
		remaining = tail[1:]
	}
	if sessionLengthStr != "" {
		d, err := time.ParseDuration(sessionLengthStr)
		if err != nil {
			return Shared{}, nil, fmt.Errorf("invalid --session-length: %w", err)
		}
		s.SessionLength = d
	}
	return s, rest, nil
}
