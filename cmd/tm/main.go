package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/appdir"
	"github.com/tt-a1i/tokenmeter/internal/collector"
	"github.com/tt-a1i/tokenmeter/internal/daemon"
	"github.com/tt-a1i/tokenmeter/internal/event"
	"github.com/tt-a1i/tokenmeter/internal/report"
	"github.com/tt-a1i/tokenmeter/internal/storage"
	"github.com/tt-a1i/tokenmeter/internal/web"
)

var version = "dev"

var tokenmeterHookNames = []string{
	"SessionStart", "SessionEnd", "Stop",
	"PreToolUse", "PostToolUse", "PostToolUseFailure",
	"SubagentStart", "SubagentStop",
}

type daemonMetricsProvider struct {
	daemon *daemon.Daemon
	db     *storage.DB
}

func (p daemonMetricsProvider) DaemonStats() (int64, int64, int64) {
	if p.daemon == nil {
		return 0, 0, 0
	}
	return p.daemon.Stats()
}

func (p daemonMetricsProvider) BudgetUsageAll() ([]web.BudgetMetric, error) {
	budgets, err := p.db.ListBudgets()
	if err != nil {
		return nil, err
	}
	result := make([]web.BudgetMetric, 0, len(budgets))
	for _, budget := range budgets {
		used, limit, err := p.db.GetBudgetUsage(budget.ID)
		if err != nil {
			return nil, err
		}
		percent := 0.0
		if limit > 0 {
			percent = used / limit * 100
		}
		result = append(result, web.BudgetMetric{
			Name:     budget.Name,
			Platform: budget.Platform,
			UsedUSD:  used,
			LimitUSD: limit,
			Percent:  percent,
		})
	}
	return result, nil
}

func mustOpenDB() *storage.DB {
	db, err := storage.Open(storage.DefaultDBPath())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	return db
}

func main() {
	args := normalizeTopLevelArgs(os.Args[1:])
	if len(args) < 1 {
		if err := runCLIDispatch(nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "daemon":
		ensureHooksInstalled()
		runDaemon()
	case "reload":
		if err := runReload(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "emit":
		runEmit()
	case "setup":
		runSetup()
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "uninstall":
		runUninstall()
	case "share":
		runShare()
	case "export":
		if err := runExport(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "compare":
		if err := runCompare(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "search":
		if err := runSearch(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "budget":
		if err := runBudget(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "webhook":
		if err := runWebhook(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "analyze":
		if err := runAnalyze(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "watch":
		if err := runWatch(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "healthcheck":
		if err := runHealthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	case "logs":
		if err := runLogs(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "checkpoint":
		if err := runCheckpoint(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "completion":
		if err := runCompletion(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "backup":
		if err := runBackup(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "restore":
		if err := runRestore(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "doctor":
		if err := runDoctor(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "compact":
		if err := runCompact(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "clean":
		runClean()
	case "tag":
		runTag()
	case "web":
		ensureHooksInstalled()
		if err := runWeb(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "update":
		runUpdate()
	case "version", "-v", "-V", "--version":
		if err := runVersion(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printHelp()
	case "daily", "weekly", "monthly", "session", "blocks", "statusline", "pricing", "config",
		"cost", "report", "status", "top",
		"claude", "codex",
		"amp", "codebuff", "copilot", "droid", "gemini", "goose", "hermes",
		"kilo", "kimi", "openclaw", "opencode", "pi", "qwen":
		if maybePrintCmdHelp(args[0], args[1:]) {
			return
		}
		if err := runCLIDispatch(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprint(os.Stderr, unknownCommandHelpMessage(args[0]))
		printHelp()
		os.Exit(1)
	}
}

func normalizeTopLevelArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	args = normalizeLegacyTopLevelAgentArgs(args)
	var globals []string
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case isTopLevelValueFlag(arg):
			if i+1 >= len(args) {
				return append([]string(nil), args...)
			}
			globals = append(globals, arg, args[i+1])
			i += 2
		case isTopLevelValueFlagWithEquals(arg):
			globals = append(globals, arg)
			i++
		case isTopLevelBoolFlag(arg):
			globals = append(globals, arg)
			i++
		case isTopLevelBoolFlagWithEquals(arg):
			globals = append(globals, arg)
			i++
		default:
			out := []string{arg}
			if len(globals) > 0 {
				out = append(out, globals...)
			}
			out = append(out, args[i+1:]...)
			return out
		}
	}
	if len(globals) > 0 {
		command := "daily"
		if topLevelArgsIncludeSessionID(globals) {
			command = "session"
		}
		out := []string{command}
		out = append(out, globals...)
		return out
	}
	return append([]string(nil), args...)
}

func normalizeLegacyTopLevelAgentArgs(args []string) []string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if isTopLevelValueFlag(arg) && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		agent, report, ok := splitLegacyTopLevelAgentCommand(arg)
		if !ok {
			return args
		}
		out := make([]string, 0, len(args)+1)
		out = append(out, args[:i]...)
		out = append(out, agent, report)
		out = append(out, args[i+1:]...)
		return out
	}
	return args
}

func splitLegacyTopLevelAgentCommand(arg string) (string, string, bool) {
	agent, report, ok := strings.Cut(arg, ":")
	if !ok || !topLevelAgentReportSupported(agent, report) {
		return "", "", false
	}
	return agent, report, true
}

func topLevelAgentReportSupported(agent, report string) bool {
	switch agent {
	case "claude":
		switch report {
		case "daily", "weekly", "monthly", "session", "blocks", "statusline":
			return true
		}
	case "codex":
		switch report {
		case "daily", "monthly", "session":
			return true
		}
	case "opencode":
		switch report {
		case "daily", "weekly", "monthly", "session":
			return true
		}
	case "amp", "codebuff", "copilot", "droid", "gemini", "goose", "hermes", "kilo", "kimi", "openclaw", "pi", "qwen":
		switch report {
		case "daily", "monthly", "session":
			return true
		}
	}
	return false
}

func topLevelArgsIncludeSessionID(args []string) bool {
	for _, arg := range args {
		if arg == "--id" || arg == "-i" || strings.HasPrefix(arg, "--id=") || strings.HasPrefix(arg, "-i=") {
			return true
		}
	}
	return false
}

func isTopLevelValueFlag(arg string) bool {
	switch arg {
	case "--config", "--since", "-s", "--until", "-u", "--mode", "-m", "--order", "-o",
		"--start-of-week", "-w",
		"--timezone", "-z", "--project", "-p", "--jq", "-q", "--id", "-i",
		"--burn-rate-display", "--visual-burn-rate", "-B", "--cost-source",
		"--refresh-interval", "--debug-samples", "--project-aliases",
		"--token-limit", "-t", "--session-length", "-n", "--speed",
		"--context-low-threshold", "--context-medium-threshold", "--cpu-profile":
		return true
	default:
		return false
	}
}

func isTopLevelValueFlagWithEquals(arg string) bool {
	for _, name := range []string{
		"--config", "--since", "-s", "--until", "-u", "--mode", "-m", "--order", "-o",
		"--start-of-week", "-w",
		"--timezone", "-z", "--project", "-p", "--jq", "-q", "--id", "-i",
		"--burn-rate-display", "--visual-burn-rate", "-B", "--cost-source",
		"--refresh-interval", "--debug-samples", "--project-aliases",
		"--token-limit", "-t", "--session-length", "-n", "--speed",
		"--context-low-threshold", "--context-medium-threshold", "--cpu-profile",
	} {
		if strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func isTopLevelBoolFlag(arg string) bool {
	switch arg {
	case "--json", "-j", "--breakdown", "-b", "--offline", "-O", "--no-offline",
		"--debug", "-d",
		"--no-color", "--color", "--compact", "--instances", "--cache", "--no-cache",
		"--single-thread", "--all", "--active", "-a", "--recent", "-r",
		"--no-scan":
		return true
	default:
		return false
	}
}

func isTopLevelBoolFlagWithEquals(arg string) bool {
	for _, name := range []string{
		"--json", "-j", "--breakdown", "-b", "--offline", "-O", "--no-offline",
		"--debug", "-d",
		"--no-color", "--color", "--compact", "--instances", "--cache", "--no-cache",
		"--single-thread", "--all", "--active", "-a", "--recent", "-r",
		"--no-scan",
	} {
		if strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

// runCLIDispatch routes argv to the new ccusage-aligned cli subcommands
// (daily / weekly / monthly / session / blocks / statusline). It opens the
// storage DB once and reuses a single time.Now() reading across the
// statusline adapter + Run call so block boundary math stays consistent.
func runCLIDispatch(argv []string) error {
	cmd, err := cli.Route(argv)
	if err != nil {
		return err
	}
	if cmd.Shared.CPUProfile != "" {
		f, err := os.Create(cmd.Shared.CPUProfile)
		if err != nil {
			return fmt.Errorf("create cpu profile: %w", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("start cpu profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}
	ctx := context.Background()
	if err := cli.SetCodexSpeedMode(cmd.Shared.Speed); err != nil {
		return err
	}
	if cmd.Name == "pricing:refresh" {
		return cli.RunPricingRefresh(ctx, os.Stdout, cmd.Shared.Offline, cmd.Shared.Config)
	}
	switch cmd.Name {
	case "config:show", "config:path", "config:init":
		return cli.RunConfig(ctx, os.Stdout, strings.TrimPrefix(cmd.Name, "config:"), cmd.Shared.Config)
	}
	if err := cli.ConfigureRuntimePricing(ctx, cmd.Shared.Offline, cmd.Shared.Config); err != nil {
		return err
	}
	db := mustOpenDB()
	defer db.Close()
	now := time.Now()
	switch cmd.Name {
	case "daily", "weekly", "monthly", "session-all":
		if cmd.Shared.NoScan || cmd.AggregateArgs.Platform != "" {
			return cli.RunAggregate(ctx, os.Stdout, cmd.AggregateArgs, db)
		}
		return cli.RunAggregateAllSource(ctx, os.Stdout, cmd.AggregateArgs, db, cli.AllAdapters)
	case "session":
		return cli.RunSession(ctx, os.Stdout, cmd.SessionArgs, db)
	case "blocks":
		return cli.RunBlocks(ctx, os.Stdout, cmd.BlocksArgs, db)
	case "statusline":
		adapter := cli.NewActiveBlockAdapter(db, 5*time.Hour, now)
		cfgPath := appdir.Path("statusline.json")
		return cli.RunStatusline(ctx, os.Stdin, os.Stdout, adapter, cfgPath, now,
			cli.WithStatuslineOptions(cli.StatuslineOptions{
				NoColor:                cmd.Shared.NoColor,
				Mode:                   cmd.Shared.Mode,
				ModeSet:                cmd.Shared.ModeSet,
				CostSource:             cmd.Shared.CostSource,
				Cache:                  cmd.Shared.Cache,
				NoCache:                cmd.Shared.NoCache,
				RefreshInterval:        cmd.Shared.RefreshInterval,
				Debug:                  cmd.Shared.Debug,
				Timezone:               cmd.Shared.Timezone,
				ContextLowThreshold:    cmd.Shared.ContextLowThreshold,
				ContextMediumThreshold: cmd.Shared.ContextMediumThreshold,
				BurnRateDisplay:        cmd.Shared.BurnRateDisplay,
				ConfigPath:             cmd.Shared.Config,
			}))
	case "deprecated:daily", "deprecated:session", "deprecated:blocks-active":
		return cli.RunDeprecatedAlias(ctx, os.Stdout, os.Stderr, cmd.Alias, cmd.Rest, db)
	case "adapter":
		return cli.RunAdapter(ctx, os.Stdout, cmd.AggregateArgs, cmd.Source)
	default:
		return fmt.Errorf("internal: unhandled cli dispatch for %q", cmd.Name)
	}
}

func latestOrRequestedSession(db *storage.DB, args []string) (storage.SessionRow, bool) {
	if len(args) > 2 {
		s, found, err := db.GetSessionByIDPrefix(args[2])
		if err != nil {
			log.Fatalf("lookup session: %v", err)
		}
		if !found {
			log.Fatalf("session not found: %s", args[2])
		}
		return s, true
	}

	sessions, err := db.ListSessions()
	if err != nil {
		log.Fatalf("list sessions: %v", err)
	}
	if len(sessions) == 0 {
		return storage.SessionRow{}, false
	}
	return sessions[0], true
}

func runDaemon() {
	if maybePrintCmdHelp("daemon", os.Args[2:]) {
		return
	}
	if err := daemon.EnsureNotRunning(); err != nil {
		log.Fatalf("%v", err)
	}

	cleanupLogs, err := daemon.SetupLogFile()
	if err != nil {
		log.Fatalf("setup daemon log file: %v", err)
	}
	defer cleanupLogs()
	log.Printf("daemon log file enabled")

	db := mustOpenDB()
	defer db.Close()

	sockPath := daemon.DefaultSocketPath()
	d := daemon.New(db, sockPath)
	if err := d.Start(); err != nil {
		log.Fatalf("start daemon: %v", err)
	}
	daemon.WritePID()
	defer daemon.RemovePID()

	// Start Codex watcher (async emit decouples file parsing from DB writes)
	codexWatcher := collector.NewCodexWatcher(func(ev event.Event) {
		d.ProcessExternalEventAsync(ev)
	})
	collector.RegisterCodexWatcher(codexWatcher)
	codexWatcher.Start()
	defer codexWatcher.Stop()

	// Start Claude log watcher
	claudeLogWatcher := collector.NewClaudeLogWatcher(func(ev event.Event) {
		d.ProcessExternalEventAsync(ev)
	}, collector.WithClaudeTokenUsageDeleteFunc(func(sourceID string) error {
		return db.DeleteTokenUsageBySourceID(context.Background(), sourceID)
	}))
	claudeLogWatcher.Start()
	defer claudeLogWatcher.Stop()

	fmt.Printf("tm daemon running (socket: %s)\n", sockPath)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-sigCh
		if sig == syscall.SIGHUP {
			d.ReloadConfig()
			continue
		}
		break
	}
	fmt.Fprintln(os.Stderr, "\nstopping daemon... (press Ctrl+C again to force quit)")

	// Watchdog goroutine: if Stop hangs, a second signal force-exits.
	go func() {
		for {
			sig := <-sigCh
			if sig == syscall.SIGHUP {
				d.ReloadConfig()
				continue
			}
			fmt.Fprintln(os.Stderr, "force quit")
			os.Exit(130)
		}
	}()

	// Stop watchers first so no new events are sent to the batch channel,
	// then stop daemon which drains remaining events before cleanup.
	claudeLogWatcher.Stop()
	codexWatcher.Stop()
	d.Stop()
	fmt.Println("daemon stopped")
}

func runEmit() {
	if maybePrintCmdHelp("emit", os.Args[2:]) {
		return
	}
	// Redirect log output to a dedicated file so emit errors don't pollute
	// Claude Code's hook stderr parsing. Open in append mode; if it fails,
	// silence logging entirely to avoid leaking anything to Claude's stderr.
	logPath := appdir.PathFor("emit.log", "emit.log")
	// Self-truncate at ~10MB so a crash loop can't fill the disk. emit.log is
	// a transient failure log, not an audit trail — losing history is fine.
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 10*1024*1024 {
		_ = os.Truncate(logPath, 0)
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		defer f.Close()
		log.SetOutput(f)
	} else {
		log.SetOutput(io.Discard)
	}

	if err := runEmitWithReader(daemon.DefaultSocketPath(), os.Stdin); err != nil {
		log.Printf("run emit: %v", err)
		// Exit 0 so Claude Code never treats hook failure as a tool failure.
		os.Exit(0)
	}
}

func runEmitWithReader(sockPath string, r io.Reader) error {
	hookEvent, err := collector.ParseClaudeHook(r)
	if err != nil {
		return err
	}

	// Use ClaudeHookToEvents which produces properly correlated events
	// using tool_use_id from Claude Code for Pre/Post matching
	events := collector.ClaudeHookToEvents(hookEvent)

	for _, ev := range events {
		if err := collector.EmitEvent(sockPath, ev); err != nil {
			// Daemon not running, silently fail
			return err
		}
	}
	return nil
}

// tm setup hook format:
// Claude Code settings.json uses: [{ "matcher": "", "hooks": [{ "type": "command", "command": "..." }] }]
func runSetup() {
	if maybePrintCmdHelp("setup", os.Args[2:]) {
		return
	}
	if err := installHooks(false); err != nil {
		log.Fatalf("setup failed: %v", err)
	}
}

// installHooks writes the tm emit hooks into ~/.claude/settings.json.
// When silent is true, prints nothing to stdout (used by auto-setup).
// Returns an error instead of calling log.Fatalf so callers can decide.
func installHooks(silent bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	claudeDir := filepath.Join(home, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("settings.json contains invalid JSON: %v\nPlease fix %s before running setup", err, settingsPath)
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	tokenmeterPath, _ := os.Executable()
	if tokenmeterPath == "" {
		tokenmeterPath = "tm"
	}
	quoted := tokenmeterPath
	if strings.ContainsAny(tokenmeterPath, " \t\"") {
		quoted = `"` + strings.ReplaceAll(tokenmeterPath, `"`, `\"`) + `"`
	}
	emitCmd := quoted + " emit"

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
	}
	for _, hookName := range tokenmeterHookNames {
		removeTokenMeterHook(hooks, hookName)
		addHookEntry(hooks, hookName, emitCmd)
	}
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	if !silent {
		fmt.Println("✓ Claude Code hooks configured")
		fmt.Printf("  Settings: %s\n", settingsPath)
		fmt.Printf("  Command:  %s\n", emitCmd)
		fmt.Printf("  Events:   %s\n", strings.Join(tokenmeterHookNames, ", "))
		fmt.Println()
		fmt.Println("Run `tm` to start monitoring.")
	}
	return nil
}

// ensureHooksInstalled is the auto-setup hook called by long-running entry points
// (TUI, daemon, web). It silently installs hooks pointing at this binary if no
// tm/tokenmeter emit hook is already registered in ~/.claude/settings.json.
// Skips entirely if Claude Code is not installed (no ~/.claude dir).
func ensureHooksInstalled() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	claudeDir := filepath.Join(home, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		s := string(data)
		if strings.Contains(s, "tm emit") || strings.Contains(s, "tokenmeter emit") {
			return
		}
	}
	if err := installHooks(true); err != nil {
		fmt.Fprintf(os.Stderr, "[tm] auto-setup skipped: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[tm] Claude Code hooks installed → %s\n", settingsPath)
}

// addHookEntry adds a TokenMeter hook entry in the correct Claude Code format:
// [{ "matcher": "", "hooks": [{ "type": "command", "command": "..." }] }]
func addHookEntry(hooks map[string]any, hookName, emitCmd string) {
	tokenmeterHook := map[string]any{
		"type":    "command",
		"command": emitCmd,
	}

	matcherEntry := map[string]any{
		"matcher": "",
		"hooks":   []any{tokenmeterHook},
	}

	existing, ok := hooks[hookName].([]any)
	if ok {
		// Check if this exact hook already exists in any matcher entry.
		for _, entry := range existing {
			if entryMap, ok := entry.(map[string]any); ok {
				if innerHooks, ok := entryMap["hooks"].([]any); ok {
					for _, h := range innerHooks {
						if hm, ok := h.(map[string]any); ok {
							if cmd, ok := hm["command"].(string); ok && cmd == emitCmd {
								return // already installed
							}
						}
					}
				}
			}
		}
		hooks[hookName] = append(existing, matcherEntry)
	} else {
		hooks[hookName] = []any{matcherEntry}
	}
}

func runUninstall() {
	if maybePrintCmdHelp("uninstall", os.Args[2:]) {
		return
	}
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		var settings map[string]any
		if json.Unmarshal(data, &settings) == nil {
			if hooks, ok := settings["hooks"].(map[string]any); ok {
				for _, hookName := range tokenmeterHookNames {
					removeTokenMeterHook(hooks, hookName)
				}
				settings["hooks"] = hooks
				out, _ := json.MarshalIndent(settings, "", "  ")
				os.WriteFile(settingsPath, out, 0o644)
			}
		}
	}

	if running, pid := daemon.IsRunning(); running {
		proc, err := os.FindProcess(pid)
		if err == nil {
			proc.Signal(syscall.SIGTERM)
			fmt.Printf("✓ Stopped daemon (pid %d)\n", pid)
		}
	}

	fmt.Println("✓ Removed Claude Code hooks")
	fmt.Println()
	fmt.Printf("Data preserved at %s/\n", appdir.Base())
	fmt.Printf("To remove all data: rm -rf %s\n", appdir.Base())
}

// removeTokenMeterHook removes TokenMeter entries from the nested hook format.
func removeTokenMeterHook(hooks map[string]any, hookName string) {
	existing, ok := hooks[hookName].([]any)
	if !ok {
		return
	}

	var filtered []any
	for _, entry := range existing {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}

		innerHooks, ok := entryMap["hooks"].([]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}

		// Filter out TokenMeter hooks from this matcher entry. During the rename,
		// old "agmon emit" hooks are also treated as TokenMeter hooks.
		var cleanHooks []any
		for _, h := range innerHooks {
			if hm, ok := h.(map[string]any); ok {
				if cmd, ok := hm["command"].(string); ok && isTokenMeterEmitCommand(cmd) {
					continue
				}
			}
			cleanHooks = append(cleanHooks, h)
		}

		if len(cleanHooks) > 0 {
			entryMap["hooks"] = cleanHooks
			filtered = append(filtered, entryMap)
		}
		// If no hooks left in this matcher entry, drop the whole entry
	}

	if len(filtered) > 0 {
		hooks[hookName] = filtered
	} else {
		delete(hooks, hookName)
	}
}

func isTokenMeterEmitCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if !strings.HasSuffix(cmd, " emit") {
		return false
	}
	exe := strings.TrimSuffix(cmd, " emit")
	exe = strings.Trim(exe, `"'`)
	base := filepath.Base(exe)
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimSuffix(base, ".test")
	return base == "tm" || base == "tokenmeter" || base == "agmon"
}

func runShare() {
	if maybePrintCmdHelp("share", os.Args[2:]) {
		return
	}
	db := mustOpenDB()
	defer db.Close()

	target, ok := latestOrRequestedSession(db, os.Args)
	if !ok {
		fmt.Println("No sessions recorded.")
		return
	}

	toolStats, _ := db.ListToolStats(target.SessionID)
	fileChanges, _ := db.ListFileChanges(target.SessionID)
	fmt.Print(report.SessionShareMarkdown(target, toolStats, fileChanges, time.Now().UTC()))
}

func runWeb() error {
	if maybePrintCmdHelp("web", os.Args[2:]) {
		return nil
	}
	webOpts, err := parseWebOptions(os.Args[2:])
	if err != nil {
		return err
	}
	if webOpts.generateToken {
		token, path, err := writeGeneratedWebToken()
		if err != nil {
			return fmt.Errorf("generate web token: %w", err)
		}
		fmt.Printf("Token written to %s\n", path)
		fmt.Printf("Set request header: Authorization: Bearer %s\n", token)
		return nil
	}
	authToken, err := resolveWebAuthToken(webOpts)
	if err != nil {
		return err
	}

	db := mustOpenDB()
	defer db.Close()

	sockPath := daemon.DefaultSocketPath()

	// Start embedded daemon if not already running, so web dashboard has live data.
	var d *daemon.Daemon
	if running, _ := daemon.IsRunning(); !running {
		d = daemon.New(db, sockPath)
		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "start daemon: %v\n", err)
		} else {
			daemon.WritePID()
			defer daemon.RemovePID()
			defer d.Stop()

			// Start Codex watcher
			codexWatcher := collector.NewCodexWatcher(func(ev event.Event) {
				d.ProcessExternalEventAsync(ev)
			})
			collector.RegisterCodexWatcher(codexWatcher)
			codexWatcher.Start()
			defer codexWatcher.Stop()

			// Start Claude log watcher
			claudeLogWatcher := collector.NewClaudeLogWatcher(func(ev event.Event) {
				d.ProcessExternalEventAsync(ev)
			}, collector.WithClaudeTokenUsageDeleteFunc(func(sourceID string) error {
				return db.DeleteTokenUsageBySourceID(context.Background(), sourceID)
			}))
			claudeLogWatcher.Start()
			defer claudeLogWatcher.Stop()

			fmt.Println("daemon + watchers started (live data collection)")
		}
	} else {
		fmt.Println("daemon already running (connecting to existing)")
	}

	opts := []web.ServerOption{
		web.WithEventSocketPath(sockPath),
		web.WithBuildVersion(version),
		web.WithAuthToken(authToken),
	}
	if d != nil {
		opts = append(opts, web.WithMetricsProvider(daemonMetricsProvider{daemon: d, db: db}))
	}
	srv := web.NewServer(db, webOpts.port, opts...)
	fmt.Printf("TokenMeter web dashboard: http://localhost:%s\n", webOpts.port)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	for {
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("web server: %w", err)
			}
			return nil
		case sig := <-sigCh:
			if sig == syscall.SIGHUP {
				if d != nil {
					d.ReloadConfig()
				}
				continue
			}
			fmt.Println("\nshutting down web server... (press Ctrl+C again to force quit)")
			// Watchdog: second signal force-quits if Shutdown hangs.
			go func() {
				for {
					sig := <-sigCh
					if sig == syscall.SIGHUP {
						if d != nil {
							d.ReloadConfig()
						}
						continue
					}
					fmt.Fprintln(os.Stderr, "force quit")
					os.Exit(130)
				}
			}()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
			if err := <-errCh; err != nil {
				return fmt.Errorf("web server shutdown: %w", err)
			}
			fmt.Println("web server stopped")
			return nil
		}
	}
}

func runClean() {
	if maybePrintCmdHelp("clean", os.Args[2:]) {
		return
	}
	days := 7
	if len(os.Args) > 2 {
		d, err := strconv.Atoi(os.Args[2])
		if err != nil || d <= 0 {
			fmt.Fprintf(os.Stderr, "Invalid days: %q (must be a positive number)\n", os.Args[2])
			os.Exit(1)
		}
		days = d
	}

	db := mustOpenDB()
	defer db.Close()

	n, err := db.CleanOldSessions(days)
	if err != nil {
		log.Fatalf("clean: %v", err)
	}
	if n == 0 {
		fmt.Printf("No sessions older than %d days to remove.\n", days)
	} else {
		fmt.Printf("Removed %d session(s) older than %d days.\n", n, days)
	}
}

func runTag() {
	if maybePrintCmdHelp("tag", os.Args[2:]) {
		return
	}
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: tm tag <session-id> [text]\n")
		fmt.Fprintf(os.Stderr, "  Set a tag:   tm tag abc123 \"refactoring auth\"\n")
		fmt.Fprintf(os.Stderr, "  Clear tag:   tm tag abc123\n")
		os.Exit(1)
	}

	db := mustOpenDB()
	defer db.Close()

	prefix := os.Args[2]
	s, found, err := db.GetSessionByIDPrefix(prefix)
	if err != nil {
		log.Fatalf("lookup session: %v", err)
	}
	if !found {
		log.Fatalf("session not found: %s", prefix)
	}

	tag := ""
	if len(os.Args) > 3 {
		tag = strings.Join(os.Args[3:], " ")
	}

	if err := db.SetSessionTag(s.SessionID, tag); err != nil {
		log.Fatalf("set tag: %v", err)
	}

	if tag == "" {
		fmt.Printf("Cleared tag for session %s\n", shortSessionID(s.SessionID))
	} else {
		fmt.Printf("Tagged session %s: %s\n", shortSessionID(s.SessionID), tag)
	}
}

func shortSessionID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}

type helpSection struct {
	title    string
	commands []helpCommand
}

type helpCommand struct {
	name string
	desc string
}

var helpSections = []helpSection{
	{"Setup & installation", []helpCommand{
		{"setup", "Configure Claude Code hooks"},
		{"uninstall", "Remove hooks and stop daemon"},
		{"init", "Interactive setup wizard"},
		{"doctor [--fix]", "Self-diagnostic and auto-repair"},
		{"completion <shell>", "Generate shell completion (bash|zsh|fish)"},
		{"update", "Update to latest release"},
		{"version [--check]", "Show version (and check for updates)"},
		{"help", "Show this help"},
	}},
	{"Run modes", []helpCommand{
		{"daemon", "Run daemon (foreground)"},
		{"web [--port N]", "Web dashboard at http://localhost:N"},
		{"watch [opts]", "Stream live events to stdout"},
	}},
	{"Usage summary (ccusage-aligned)", []helpCommand{
		{"daily", "Daily token / cost summary (default when no command)"},
		{"weekly", "Weekly summary by ISO week"},
		{"monthly", "Monthly summary"},
		{"session [<id>]", "Per-session breakdown, optionally one session"},
		{"blocks [--active]", "5-hour session blocks + burn rate / projection"},
		{"statusline", "Claude Code statusline provider (stdin JSON → stdout line)"},
		{"share [session]", "Shareable Markdown session recap"},
	}},
	{"Analysis", []helpCommand{
		{"analyze [--range]", "Usage insights with heatmap"},
		{"search <query>", "Search tool calls and file paths"},
		{"compare <a> <b>", "Diff two sessions"},
		{"export [opts]", "CSV/JSON export"},
	}},
	{"Maintenance", []helpCommand{
		{"clean [days]", "Remove sessions older than N days (default 7)"},
		{"compact [--full]", "PRAGMA optimize (or full VACUUM)"},
		{"checkpoint", "WAL truncate immediately"},
		{"backup [path]", "Snapshot database via VACUUM INTO"},
		{"restore <path>", "Restore from snapshot"},
		{"reload", "Send SIGHUP to running daemon"},
		{"logs [--follow]", "Tail daemon log"},
		{"healthcheck", "DB + daemon liveness probe"},
		{"emit", "Emit event from hook (reads stdin)"},
	}},
	{"Configuration", []helpCommand{
		{"tag <id> [text]", "Set/clear session note"},
		{"config <subcommand>", "Show, locate, or initialize config"},
		{"pricing refresh", "Refresh LiteLLM pricing cache"},
		{"budget <subcommand>", "Manage budgets: list, set, delete, usage"},
		{"webhook <subcommand>", "Manage webhooks: list, test, replay"},
	}},
	{"Deprecated (removed in v2.0)", []helpCommand{
		{"cost", "use 'tm daily' instead"},
		{"report", "use 'tm session' / 'tm weekly' / 'tm monthly' instead"},
		{"status", "use 'tm blocks --active' instead"},
		{"top", "use 'tm blocks --active' instead"},
	}},
}

func printHelp() {
	fmt.Printf("TokenMeter v%s — AI coding agent usage meter\n\n", version)
	fmt.Println("Usage: tm <command> [args...]")
	fmt.Println()

	width := helpCommandWidth(helpSections)
	for _, section := range helpSections {
		fmt.Printf("▎%s\n", section.title)
		for _, cmd := range section.commands {
			fmt.Printf("  %-*s  %s\n", width, cmd.name, cmd.desc)
		}
		fmt.Println()
	}

	fmt.Println("▎Examples")
	for _, example := range []helpCommand{
		{"tm", "Default — show daily token / cost summary"},
		{"tm daily --breakdown", "Per-model nested rows under each day"},
		{"tm daily --json | jq .totals", "CamelCase JSON envelope for scripts"},
		{"tm daily --order desc", "Newest day first"},
		{"tm daily --until 20260520", "Inclusive close — covers all of 2026-05-20"},
		{"tm blocks --active", "Live 5h block with burn rate"},
		{"tm daily --timezone Asia/Shanghai", "Bucket days in a non-UTC zone"},
		{`tm budget set "Monthly" 100 --platform claude`, "Create a Claude monthly budget"},
	} {
		fmt.Printf("  %-*s  # %s\n", width, example.name, example.desc)
	}
	fmt.Println()
	fmt.Println("Run 'tm <command> --help' for command-specific options (if available).")
	fmt.Println("Source: https://github.com/tt-a1i/tokenmeter")
}

func helpCommandWidth(sections []helpSection) int {
	width := 0
	for _, section := range sections {
		for _, cmd := range section.commands {
			if len(cmd.name) > width {
				width = len(cmd.name)
			}
		}
	}
	return width
}
