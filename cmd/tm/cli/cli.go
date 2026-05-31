// Package cli implements the v1.0 ccusage-style command surface.
package cli

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	tmconfig "github.com/tt-a1i/tokenmeter/internal/config"
)

// Shared holds CLI flags common to every subcommand. See spec §4.1.
type Shared struct {
	Since                  string
	Until                  string
	JSON                   bool
	Mode                   string
	ModeSet                bool
	Speed                  string
	Order                  string
	OrderSet               bool
	StartOfWeek            string
	Breakdown              bool
	Offline                bool
	Timezone               string
	Project                string
	NoColor                bool
	Compact                bool
	Instances              bool
	JQ                     string
	Config                 string
	ID                     string
	ContextLowThreshold    int
	ContextMediumThreshold int
	BurnRateDisplay        string
	CostSource             string
	Cache                  bool
	NoCache                bool
	RefreshInterval        int
	Debug                  bool
	DebugSamples           string
	SingleThread           bool
	Color                  bool
	All                    bool
	ProjectAliases         string
	TokenLimit             string
	SessionLength          time.Duration
	Active                 bool
	Recent                 bool
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
	if command == "statusline" && !hasAnyFlag(args, "--offline", "-O", "--no-offline") {
		s.Offline = true
	}
	s.ModeSet = hasExplicitValueFlag(args, "--mode", "-m")
	s.OrderSet = hasExplicitValueFlag(args, "--order", "-o")
	fs := flag.NewFlagSet("shared", flag.ContinueOnError)
	fs.StringVar(&s.Since, "since", s.Since, "start date YYYYMMDD or YYYY-MM-DD")
	fs.StringVar(&s.Since, "s", s.Since, "alias for --since")
	fs.StringVar(&s.Until, "until", s.Until, "end date YYYYMMDD or YYYY-MM-DD")
	fs.StringVar(&s.Until, "u", s.Until, "alias for --until")
	fs.BoolVar(&s.JSON, "json", s.JSON, "emit JSON instead of table")
	fs.BoolVar(&s.JSON, "j", s.JSON, "alias for --json")
	fs.StringVar(&s.Mode, "mode", defaultString(s.Mode, "auto"), "cost mode: auto | calculate | display")
	fs.StringVar(&s.Mode, "m", defaultString(s.Mode, "auto"), "alias for --mode")
	fs.StringVar(&s.Speed, "speed", defaultString(s.Speed, "auto"), "Codex pricing speed tier: auto | standard | fast")
	orderDefault := defaultOrder(command, s.Order)
	fs.StringVar(&s.Order, "order", orderDefault, "sort order: asc | desc")
	fs.StringVar(&s.Order, "o", orderDefault, "alias for --order")
	fs.StringVar(&s.StartOfWeek, "start-of-week", s.StartOfWeek, "weekly start day")
	fs.StringVar(&s.StartOfWeek, "w", s.StartOfWeek, "alias for --start-of-week")
	fs.BoolVar(&s.Breakdown, "breakdown", s.Breakdown, "break down by model")
	fs.BoolVar(&s.Breakdown, "b", s.Breakdown, "alias for --breakdown")
	fs.BoolVar(&s.Offline, "offline", s.Offline, "skip online pricing refresh")
	fs.BoolVar(&s.Offline, "O", s.Offline, "alias for --offline")
	var noOffline bool
	fs.BoolVar(&noOffline, "no-offline", false, "enable online pricing refresh")
	fs.StringVar(&s.Timezone, "timezone", s.Timezone, "timezone for date bucketing")
	fs.StringVar(&s.Timezone, "z", s.Timezone, "alias for --timezone")
	fs.StringVar(&s.Project, "project", s.Project, "filter by workspace path")
	fs.StringVar(&s.Project, "p", s.Project, "alias for --project")
	fs.BoolVar(&s.NoColor, "no-color", s.NoColor, "disable color")
	fs.BoolVar(&s.Color, "color", s.Color, "enable color")
	fs.BoolVar(&s.Compact, "compact", s.Compact, "use compact table layout")
	fs.BoolVar(&s.Instances, "instances", s.Instances, "show project instances")
	fs.StringVar(&s.JQ, "jq", s.JQ, "post-filter JSON via jq expression")
	fs.StringVar(&s.JQ, "q", s.JQ, "alias for --jq")
	fs.StringVar(&s.Config, "config", s.Config, "config file path")
	fs.StringVar(&s.ID, "id", s.ID, "session id for session detail")
	if shortIIsSessionID(command, args) {
		fs.StringVar(&s.ID, "i", s.ID, "alias for --id")
	} else {
		fs.BoolVar(&s.Instances, "i", s.Instances, "alias for --instances on daily reports")
	}
	fs.IntVar(&s.ContextLowThreshold, "context-low-threshold", 0, "statusline context warning threshold percent")
	fs.IntVar(&s.ContextMediumThreshold, "context-medium-threshold", 0, "statusline context danger threshold percent")
	fs.StringVar(&s.BurnRateDisplay, "burn-rate-display", "", "statusline burn-rate display: off | emoji | text | emoji-text")
	fs.StringVar(&s.BurnRateDisplay, "visual-burn-rate", "", "alias for --burn-rate-display")
	fs.StringVar(&s.BurnRateDisplay, "B", "", "alias for --visual-burn-rate")
	fs.StringVar(&s.CostSource, "cost-source", s.CostSource, "statusline cost source: auto | ccusage | cc | both")
	fs.BoolVar(&s.Cache, "cache", s.Cache, "statusline: enable cache")
	fs.BoolVar(&s.NoCache, "no-cache", s.NoCache, "statusline: disable cache")
	fs.IntVar(&s.RefreshInterval, "refresh-interval", s.RefreshInterval, "statusline refresh interval seconds")
	fs.BoolVar(&s.Debug, "debug", s.Debug, "enable debug logging")
	fs.BoolVar(&s.Debug, "d", s.Debug, "alias for --debug")
	fs.StringVar(&s.DebugSamples, "debug-samples", s.DebugSamples, "debug sample path or count")
	fs.BoolVar(&s.SingleThread, "single-thread", s.SingleThread, "disable parallel scans")
	fs.BoolVar(&s.All, "all", s.All, "compatibility flag accepted by aggregate commands")
	fs.StringVar(&s.ProjectAliases, "project-aliases", s.ProjectAliases, "project alias JSON or JSON file path")
	fs.StringVar(&s.TokenLimit, "token-limit", s.TokenLimit, "blocks token limit: positive integer or max")
	fs.StringVar(&s.TokenLimit, "t", s.TokenLimit, "alias for --token-limit")
	var sessionLengthStr string
	fs.StringVar(&sessionLengthStr, "session-length", "5h", "duration of one session block (e.g. 5h, 1h30m)")
	fs.StringVar(&sessionLengthStr, "n", "5h", "alias for --session-length")
	fs.BoolVar(&s.Active, "active", false, "blocks: show only the active 5h window")
	fs.BoolVar(&s.Active, "a", false, "alias for --active")
	fs.BoolVar(&s.Recent, "recent", false, "blocks: show recent blocks")
	fs.BoolVar(&s.Recent, "r", false, "alias for --recent")
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
	if noOffline {
		s.Offline = false
	}
	if s.Color {
		s.NoColor = false
	}
	if sessionLengthStr != "" {
		d, err := parseSessionLength(sessionLengthStr)
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
	if s.StartOfWeek != "" {
		if _, err := parseWeekday(s.StartOfWeek); err != nil {
			return Shared{}, nil, err
		}
	}
	switch s.BurnRateDisplay {
	case "", "off", "emoji", "text", "emoji-text":
	default:
		return Shared{}, nil, fmt.Errorf("invalid --burn-rate-display %q (want off, emoji, text, or emoji-text)", s.BurnRateDisplay)
	}
	switch s.CostSource {
	case "", "auto", "ccusage", "cc", "both":
	default:
		return Shared{}, nil, fmt.Errorf("invalid --cost-source %q (want auto, ccusage, cc, or both)", s.CostSource)
	}
	return s, rest, nil
}

func shortIIsSessionID(command string, args []string) bool {
	if command == "session" {
		return true
	}
	if command == "claude" || command == "codex" {
		return firstPositional(args) == "session"
	}
	return false
}

func firstPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if flagTakesValue(arg) && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

func flagTakesValue(arg string) bool {
	name := strings.SplitN(arg, "=", 2)[0]
	switch name {
	case "--config", "--since", "-s", "--until", "-u", "--mode", "-m", "--order", "-o",
		"--start-of-week", "-w", "--timezone", "-z", "--project", "-p", "--jq", "-q",
		"--id", "-i", "--burn-rate-display", "--visual-burn-rate", "-B", "--cost-source",
		"--refresh-interval", "--debug-samples", "--project-aliases", "--token-limit", "-t",
		"--session-length", "-n", "--speed", "--context-low-threshold",
		"--context-medium-threshold", "--cpu-profile":
		return true
	default:
		return false
	}
}

func parseWeekday(raw string) (time.Weekday, error) {
	switch strings.ToLower(raw) {
	case "", "sunday":
		return time.Sunday, nil
	case "monday":
		return time.Monday, nil
	case "tuesday":
		return time.Tuesday, nil
	case "wednesday":
		return time.Wednesday, nil
	case "thursday":
		return time.Thursday, nil
	case "friday":
		return time.Friday, nil
	case "saturday":
		return time.Saturday, nil
	default:
		return time.Sunday, fmt.Errorf("invalid --start-of-week %q (want sunday, monday, tuesday, wednesday, thursday, friday, or saturday)", raw)
	}
}

func parseSessionLength(raw string) (time.Duration, error) {
	if d, err := time.ParseDuration(raw); err == nil {
		return d, nil
	}
	if strings.ContainsAny(raw, "hms") {
		return 0, fmt.Errorf("invalid duration %q", raw)
	}
	hours, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if hours <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return time.Duration(hours * float64(time.Hour)), nil
}

func defaultString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func defaultOrder(command, configured string) string {
	if configured != "" {
		return configured
	}
	if command == "session" {
		return "desc"
	}
	return "asc"
}

func hasExplicitValueFlag(args []string, long, short string) bool {
	for _, arg := range args {
		if arg == long || arg == short || strings.HasPrefix(arg, long+"=") || strings.HasPrefix(arg, short+"=") {
			return true
		}
	}
	return false
}

func hasAnyFlag(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return true
			}
		}
	}
	return false
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
