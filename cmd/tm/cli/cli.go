// Package cli implements the v1.0 ccusage-style command surface.
package cli

import (
	"flag"
	"fmt"
	"time"

	tmconfig "github.com/tt-a1i/tokenmeter/internal/config"
)

// Shared holds CLI flags common to every subcommand. See spec §4.1.
type Shared struct {
	Since                  string
	Until                  string
	JSON                   bool
	Mode                   string
	Speed                  string
	Order                  string
	Breakdown              bool
	Offline                bool
	Timezone               string
	Project                string
	NoColor                bool
	Compact                bool
	Instances              bool
	JQ                     string
	Config                 string
	ContextLowThreshold    int
	ContextMediumThreshold int
	BurnRateDisplay        string
	ProjectAliases         string
	TokenLimit             string
	SessionLength          time.Duration
	Active                 bool
	CPUProfile             string
	NoScan                 bool
}

// ParseShared extracts shared flags from args. Returns the remaining
// non-flag tokens for the subcommand to dispatch on.
func ParseShared(args []string) (Shared, []string, error) {
	return ParseSharedForCommand(inferCommand(args), args)
}

func ParseSharedForCommand(command string, args []string) (Shared, []string, error) {
	var s Shared
	if err := applyConfigDefaults(&s, command, args); err != nil {
		return Shared{}, nil, err
	}
	fs := flag.NewFlagSet("shared", flag.ContinueOnError)
	fs.StringVar(&s.Since, "since", s.Since, "start date YYYYMMDD")
	fs.StringVar(&s.Until, "until", s.Until, "end date YYYYMMDD")
	fs.BoolVar(&s.JSON, "json", s.JSON, "emit JSON instead of table")
	fs.StringVar(&s.Mode, "mode", defaultString(s.Mode, "auto"), "cost mode: auto | calculate | display")
	fs.StringVar(&s.Speed, "speed", defaultString(s.Speed, "auto"), "Codex pricing speed tier: auto | standard | fast")
	fs.StringVar(&s.Order, "order", defaultString(s.Order, "asc"), "sort order: asc | desc")
	fs.BoolVar(&s.Breakdown, "breakdown", s.Breakdown, "break down by model")
	fs.BoolVar(&s.Offline, "offline", s.Offline, "skip online pricing refresh")
	fs.StringVar(&s.Timezone, "timezone", s.Timezone, "timezone for date bucketing")
	fs.StringVar(&s.Project, "project", s.Project, "filter by workspace path")
	fs.BoolVar(&s.NoColor, "no-color", s.NoColor, "disable color")
	fs.BoolVar(&s.Compact, "compact", s.Compact, "use compact table layout")
	fs.BoolVar(&s.Instances, "instances", s.Instances, "show project instances")
	fs.StringVar(&s.JQ, "jq", s.JQ, "post-filter JSON via jq expression")
	fs.StringVar(&s.Config, "config", s.Config, "config file path")
	fs.IntVar(&s.ContextLowThreshold, "context-low-threshold", 0, "statusline context warning threshold percent")
	fs.IntVar(&s.ContextMediumThreshold, "context-medium-threshold", 0, "statusline context danger threshold percent")
	fs.StringVar(&s.BurnRateDisplay, "burn-rate-display", "", "statusline burn-rate display: off | emoji | text | emoji-text")
	fs.StringVar(&s.ProjectAliases, "project-aliases", s.ProjectAliases, "project alias JSON or JSON file path")
	fs.StringVar(&s.TokenLimit, "token-limit", s.TokenLimit, "blocks token limit: positive integer or max")
	var sessionLengthStr string
	fs.StringVar(&sessionLengthStr, "session-length", "5h", "duration of one session block (e.g. 5h, 1h30m)")
	fs.BoolVar(&s.Active, "active", false, "blocks: show only the active 5h window")
	fs.StringVar(&s.CPUProfile, "cpu-profile", "", "write CPU profile to file (hidden)")
	fs.BoolVar(&s.NoScan, "no-scan", false, "skip batch adapter scans; use only SQLite (Claude+Codex)")
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
	switch s.Speed {
	case "auto", "standard", "fast":
	default:
		return Shared{}, nil, fmt.Errorf("invalid --speed %q (want auto, standard, or fast)", s.Speed)
	}
	switch s.BurnRateDisplay {
	case "", "off", "emoji", "text", "emoji-text":
	default:
		return Shared{}, nil, fmt.Errorf("invalid --burn-rate-display %q (want off, emoji, text, or emoji-text)", s.BurnRateDisplay)
	}
	return s, rest, nil
}

func defaultString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func applyConfigDefaults(s *Shared, command string, args []string) error {
	path := scanConfigFlag(args)
	cfg, err := tmconfig.LoadWithOptions(tmconfig.Options{ExplicitPath: path})
	if err != nil {
		return err
	}
	d := cfg.EffectiveDefaults(command)
	s.Since = d.Since
	s.Until = d.Until
	if d.JSON != nil {
		s.JSON = *d.JSON
	}
	if d.Offline != nil {
		s.Offline = *d.Offline
	}
	s.Timezone = d.Timezone
	if d.Mode != "" {
		s.Mode = d.Mode
	}
	if d.Order != "" {
		s.Order = d.Order
	}
	if d.Breakdown != nil {
		s.Breakdown = *d.Breakdown
	}
	s.Project = d.Project
	if d.NoColor != nil {
		s.NoColor = *d.NoColor
	}
	if d.Speed != "" {
		s.Speed = d.Speed
	}
	if d.Compact != nil {
		s.Compact = *d.Compact
	}
	s.TokenLimit = d.TokenLimit
	return nil
}

func scanConfigFlag(args []string) string {
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if len(arg) > len("--config=") && arg[:len("--config=")] == "--config=" {
			return arg[len("--config="):]
		}
	}
	return ""
}

func inferCommand(args []string) string {
	for _, arg := range args {
		if arg == "" || arg[0] == '-' {
			continue
		}
		return arg
	}
	return ""
}
