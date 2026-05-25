package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/tt-a1i/tokenmeter/cmd/tm/cli"
	"github.com/tt-a1i/tokenmeter/internal/blocks"
	"github.com/tt-a1i/tokenmeter/internal/collector"
	"github.com/tt-a1i/tokenmeter/internal/event"
	"github.com/tt-a1i/tokenmeter/internal/statusline"
	"github.com/tt-a1i/tokenmeter/internal/storage"
	_ "modernc.org/sqlite"
)

func TestE2EV12Smoke(t *testing.T) {
	t.Run("GroupA_OpenCodeSQLite", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		dir := t.TempDir()
		writeV12OpenCodeJSONMessage(t, dir, "dup", `{
			"id": "msg-dup",
			"sessionID": "json-session",
			"modelID": "json-model",
			"time": {"created": 1767312000000},
			"tokens": {"input": 1, "output": 2},
			"cost": 0.01
		}`)
		writeV12OpenCodeDB(t, filepath.Join(dir, "opencode.db"), "msg-dup", "db-session", `{
			"id": "msg-dup",
			"sessionID": "db-session",
			"modelID": "db-model",
			"time": {"created": 1767312000000},
			"tokens": {"input": 10, "output": 20},
			"cost": 0.20
		}`)
		t.Setenv("OPENCODE_DATA_DIR", dir)
		v12HermeticAdapterEnv(t, dir)
		setTestHome(t, home)

		out, err := v12Dispatch(t, "opencode", "daily", "--json", "--offline", "--no-color")
		if err != nil {
			t.Fatalf("tm opencode daily: %v\n%s", err, out)
		}
		v12AssertContains(t, out, `"db-model"`)
		v12AssertContains(t, out, `"inputTokens": 10`)
		if strings.Contains(out, "json-model") {
			t.Fatalf("SQLite row should win duplicate JSON row:\n%s", out)
		}
	})

	t.Run("GroupA_DroidSidecar", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		dir := t.TempDir()
		settings := `{"providerLockTimestamp":"2026-05-01T00:00:00Z","tokenUsage":{"inputTokens":100,"outputTokens":50}}`
		if err := os.WriteFile(filepath.Join(dir, "sidecar.settings.json"), []byte(settings), 0o600); err != nil {
			t.Fatalf("write settings: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sidecar.jsonl"), []byte(`{"message":"Model: Claude-Sonnet-4-[Anthropic]"}`+"\n"), 0o600); err != nil {
			t.Fatalf("write sidecar: %v", err)
		}
		t.Setenv("DROID_SESSIONS_DIR", dir)
		v12HermeticAdapterEnv(t, dir)
		setTestHome(t, home)

		out, err := v12Dispatch(t, "droid", "daily", "--json", "--offline", "--no-color")
		if err != nil {
			t.Fatalf("tm droid daily: %v\n%s", err, out)
		}
		v12AssertContains(t, out, `"claude-sonnet-4"`)
		v12AssertContains(t, out, `"inputTokens": 100`)
	})

	t.Run("GroupB_PricingOffline", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		setTestHome(t, home)
		cmd := exec.Command(os.Args[0], "-test.run", "^TestE2EV12SmokePricingOfflineHelperProcess$", "--")
		cmd.Env = append(os.Environ(),
			"TM_E2E_HELPER=pricing-offline",
			"HOME="+home,
			"TOKENMETER_HOME="+filepath.Join(home, ".tokenmeter"),
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("pricing refresh --offline error = %v, want exit status 1\nstdout:%s\nstderr:%s", err, stdout.String(), stderr.String())
		}
		if got := exitErr.ExitCode(); got != 1 {
			t.Fatalf("pricing refresh --offline exit = %d, want 1\nstdout:%s\nstderr:%s", got, stdout.String(), stderr.String())
		}
		combined := strings.ToLower(stdout.String() + stderr.String())
		v12AssertContains(t, combined, "offline mode")
	})

	t.Run("GroupB_CodexSpeedAuto", func(t *testing.T) {
		home := t.TempDir()
		setTestHome(t, home)
		t.Setenv("CODEX_HOME", "")
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(`service_tier = "fast"`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := collector.SetCodexSpeedMode("auto"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = collector.SetCodexSpeedMode("auto") })
		if got := collector.ResolveCodexPricingSpeed(); got != collector.CodexSpeedFast {
			t.Fatalf("auto speed=%v want fast", got)
		}
		stdIn, stdOut, stdCache := collector.CodexPricingForSpeed("gpt-5.4", collector.CodexSpeedStandard)
		fastIn, fastOut, fastCache := collector.CodexPricingForSpeed("gpt-5.4", collector.CodexSpeedFast)
		if fastIn <= stdIn || fastOut <= stdOut || fastCache <= stdCache {
			t.Fatalf("fast pricing should exceed standard: std=(%v,%v,%v) fast=(%v,%v,%v)", stdIn, stdOut, stdCache, fastIn, fastOut, fastCache)
		}
	})

	t.Run("GroupC_BlocksTokenLimit", func(t *testing.T) {
		var out bytes.Buffer
		err := cli.RunBlocks(context.Background(), &out, cli.BlocksArgs{
			Shared:        cli.Shared{JSON: true, TokenLimit: "100"},
			SessionLength: 5 * time.Hour,
			Now:           v12Time("2026-05-20T12:00:00Z"),
		}, v12UsageLoader{entries: []storage.TokenUsageEntry{
			{SessionID: "block-smoke", Timestamp: v12Time("2026-05-20T10:00:00Z"), Model: "claude", InputTokens: 80, CostUSD: 1.0},
		}})
		if err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, out.String(), `"token_limit": 100`)
		v12AssertContains(t, out.String(), `"usage_pct": 80`)
		v12AssertContains(t, out.String(), `"status": "WARN"`)
	})

	t.Run("GroupC_BlocksANSI", func(t *testing.T) {
		home, db := v12HomeDB(t)
		seedCLISession(t, db, "v12-blocks-ansi", event.PlatformClaude, "/repo/agmon", "main", time.Now().Add(-time.Hour), 80, 0, 1)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		run := func(t *testing.T, forceColor bool) string {
			t.Helper()
			cmd := exec.Command(os.Args[0], "-test.run", "^TestE2EV12SmokeBlocksHelperProcess$", "--")
			env := append(os.Environ(),
				"TM_E2E_HELPER=blocks",
				"HOME="+home,
				"TOKENMETER_HOME="+filepath.Join(home, ".tokenmeter"),
				"NO_COLOR=",
				"TERM=xterm-256color",
				"FORCE_COLOR=",
			)
			if forceColor {
				env[len(env)-1] = "FORCE_COLOR=1"
			}
			cmd.Env = env
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				t.Fatalf("blocks helper: %v\nstdout:%s\nstderr:%s", err, stdout.String(), stderr.String())
			}
			return stdout.String()
		}
		t.Run("NonTTYDefaultNoANSI", func(t *testing.T) {
			out := run(t, false)
			if strings.Contains(out, "\x1b[") {
				t.Fatalf("non-tty output should not contain ANSI escapes:\n%q", out)
			}
		})
		t.Run("ForceColorANSI", func(t *testing.T) {
			out := run(t, true)
			v12AssertContains(t, out, "\x1b[33mWARN\x1b[0m")
		})
	})

	t.Run("GroupD_UnifiedConfigDefaults", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tokenmeter.json")
		if err := os.WriteFile(path, []byte(`{"defaults":{"order":"desc"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got, _, err := cli.ParseSharedForCommand("daily", []string{"daily", "--config", path})
		if err != nil {
			t.Fatal(err)
		}
		if got.Order != "desc" {
			t.Fatalf("defaults.order not applied: %+v", got)
		}
	})

	t.Run("GroupD_UnifiedConfigCommands", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tokenmeter.json")
		if err := os.WriteFile(path, []byte(`{"defaults":{"breakdown":false},"commands":{"daily":{"breakdown":true}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got, _, err := cli.ParseSharedForCommand("daily", []string{"daily", "--config", path})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Breakdown {
			t.Fatalf("commands.daily.breakdown should override default false: %+v", got)
		}
	})

	t.Run("GroupD_ConfigShowPath", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		path := filepath.Join(t.TempDir(), "tokenmeter.json")
		if err := os.WriteFile(path, []byte(`{"defaults":{"order":"desc"},"pricing":{"cacheTTLHours":72}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		setTestHome(t, home)
		out, err := v12Dispatch(t, "config", "show", "--config", path)
		if err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, out, `"order": "desc"`)
		v12AssertContains(t, out, `"cacheTTLHours": 72`)
	})

	t.Run("GroupE_ProjectAliases", func(t *testing.T) {
		var out bytes.Buffer
		loader := v12UsageLoader{entries: []storage.TokenUsageEntry{
			{SessionID: "a", Timestamp: v12Time("2026-05-20T10:00:00Z"), CWD: "/repo/agmon", Model: "claude", InputTokens: 100, CostUSD: 1},
			{SessionID: "b", Timestamp: v12Time("2026-05-20T11:00:00Z"), CWD: "/repo/agmon-feature", Model: "claude", InputTokens: 200, CostUSD: 2},
		}}
		err := cli.RunAggregate(context.Background(), &out, cli.AggregateArgs{
			Shared: cli.Shared{JSON: true, Instances: true, ProjectAliases: `{"agmon":["/repo/agmon","/repo/agmon-feature"]}`},
			Bucket: cli.BucketDaily,
		}, loader)
		if err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, out.String(), `"project": "agmon"`)
		v12AssertContains(t, out.String(), `"inputTokens": 300`)
	})

	t.Run("GroupF_AnalyzeToolErrors", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		now := time.Now().Add(-24 * time.Hour)
		seedAnalyzeCLISession(t, db, "v12-tool-errors", event.PlatformClaude, now, "sonnet", 1, "a.go")
		for i := 0; i < 3; i++ {
			callID := "v12-tool-fail-" + string(rune('a'+i))
			if _, err := db.InsertToolCallStart(callID, "agent-v12-tool-errors", "v12-tool-errors", "Bash", "{}", now.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := db.UpdateToolCallEnd(callID, "exit code 1: missing file 123", event.StatusFail, 1, now.Add(time.Duration(i)*time.Minute+time.Second)); err != nil {
				t.Fatal(err)
			}
		}
		setTestHome(t, home)
		withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--tool-errors", "--json"})
		out := captureStdout(t, func() {
			if err := runAnalyze(); err != nil {
				t.Fatal(err)
			}
		})
		v12AssertContains(t, out, `"tool_errors"`)
		v12AssertContains(t, out, `"top_tools"`)
		v12AssertContains(t, out, `"patterns"`)
		v12AssertContains(t, out, `"daily"`)
	})

	t.Run("GroupF_AnalyzeFileChurn", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		now := time.Now().Add(-24 * time.Hour)
		seedAnalyzeCLISession(t, db, "v12-file-churn", event.PlatformClaude, now, "sonnet", 1, "internal/storage/db.go")
		for i := 0; i < 2; i++ {
			if err := db.InsertFileChange("v12-file-churn", "internal/storage/db.go", event.FileEdit, now.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
		setTestHome(t, home)
		withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--file-churn", "--json"})
		out := captureStdout(t, func() {
			if err := runAnalyze(); err != nil {
				t.Fatal(err)
			}
		})
		v12AssertContains(t, out, `"file_churn"`)
		v12AssertContains(t, out, `"top_files"`)
		v12AssertContains(t, out, `"hotspots"`)
		v12AssertContains(t, out, `"daily"`)
	})

	t.Run("GroupF_AnalyzeCombined", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		now := time.Now().Add(-24 * time.Hour)
		seedAnalyzeCLISession(t, db, "v12-combined", event.PlatformClaude, now, "sonnet", 1, "cmd/tm/main.go")
		if _, err := db.InsertToolCallStart("v12-combined-fail", "agent-v12-combined", "v12-combined", "Bash", "{}", now); err != nil {
			t.Fatal(err)
		}
		if err := db.UpdateToolCallEnd("v12-combined-fail", "exit code 1", event.StatusFail, 1, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		setTestHome(t, home)
		withArgs(t, []string{"tokenmeter", "analyze", "--range", "all", "--tool-errors", "--file-churn", "--json"})
		out := captureStdout(t, func() {
			if err := runAnalyze(); err != nil {
				t.Fatal(err)
			}
		})
		v12AssertContains(t, out, `"tool_errors"`)
		v12AssertContains(t, out, `"file_churn"`)
	})

	t.Run("GroupF_SearchAdvanced", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.Local)
		seedCLISession(t, db, "v12-search", event.PlatformClaude, "/repo/agmon", "main", now, 1000, 100, 2)
		if _, err := db.InsertToolCallStart("v12-search-call", "agent-v12-search", "v12-search", "Bash", "error from smoke", now); err != nil {
			t.Fatal(err)
		}
		setTestHome(t, home)
		withArgs(t, []string{"tokenmeter", "search", "error", "tool:Bash", "cost:>1", "since:2026-05-01", "--json"})
		out := captureStdout(t, func() {
			if err := runSearch(); err != nil {
				t.Fatal(err)
			}
		})
		v12AssertContains(t, out, `"query"`)
		v12AssertContains(t, out, `"tool": "Bash"`)
		v12AssertContains(t, out, `"results"`)
		v12AssertContains(t, out, `"session_id": "v12-search"`)
	})

	t.Run("GroupF_AnomalyWebhook", func(t *testing.T) {
		t.Skip("deferred to v1.3: cmd/tm package cannot call internal/daemon checkAnomalies or unexported webhook payload types without adding production test hooks; internal/daemon anomaly tests cover cost_spike and usage_regression directly")
	})

	t.Run("GroupG_StatuslineFlags", func(t *testing.T) {
		now := v12Time("2026-05-20T12:00:00Z")
		block := &blocks.SessionBlock{
			StartTime: now.Add(-30 * time.Minute),
			EndTime:   now.Add(4*time.Hour + 30*time.Minute),
			IsActive:  true,
			Tokens:    blocks.TokenCounts{Input: 60000, Output: 30000},
			Cost:      1.50,
			BurnRate:  &blocks.BurnRate{TokensPerMinute: 3000, CostPerHour: 3.0},
			Models:    []string{"claude-sonnet-4-6"},
		}
		input := statusline.Input{ModelID: "claude-sonnet-4-6", ContextWindow: &statusline.ContextWindow{TotalInputTokens: 85_000, ContextWindowSize: 100_000}}
		var emoji bytes.Buffer
		if err := statusline.Render(&emoji, input, block, statusline.Config{BurnRateDisplay: "emoji", Color: true, ContextLowThreshold: 50, ContextMediumThreshold: 80}, now); err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, emoji.String(), "📈")
		v12AssertContains(t, emoji.String(), "\x1b[31m85%\x1b[0m")
		var off bytes.Buffer
		if err := statusline.Render(&off, input, block, statusline.Config{BurnRateDisplay: "off", ContextLowThreshold: 50, ContextMediumThreshold: 80}, now); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(off.String(), "📈") || strings.Contains(off.String(), "🔥") {
			t.Fatalf("burn-rate-display off should hide emoji: %q", off.String())
		}
	})

	t.Run("GroupH_LegacyConfigFallback", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		base := filepath.Join(home, ".tokenmeter")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "pricing.json"), []byte(`{"codex":[{"match":["legacy-model"],"inputPerMillion":1,"outputPerMillion":2}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		setTestHome(t, home)
		out, err := v12Dispatch(t, "config", "show")
		if err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, out, `"legacy-model"`)
	})

	t.Run("GroupH_NoConfigZeroValue", func(t *testing.T) {
		home, db := v12HomeDB(t)
		defer db.Close()
		setTestHome(t, home)
		out, err := v12Dispatch(t, "config", "show")
		if err != nil {
			t.Fatal(err)
		}
		v12AssertContains(t, out, `"defaults": {}`)
		v12AssertContains(t, out, `"pricing": {}`)
	})
}

func TestE2EV12SmokePricingOfflineHelperProcess(t *testing.T) {
	if os.Getenv("TM_E2E_HELPER") != "pricing-offline" {
		return
	}
	os.Args = []string{"tm", "pricing", "refresh", "--offline"}
	main()
}

func TestE2EV12SmokeBlocksHelperProcess(t *testing.T) {
	if os.Getenv("TM_E2E_HELPER") != "blocks" {
		return
	}
	if os.Getenv("FORCE_COLOR") != "" {
		text.EnableColors()
	}
	os.Args = []string{"tm", "blocks", "--token-limit", "100"}
	main()
}

type v12UsageLoader struct {
	entries []storage.TokenUsageEntry
	aggRows []storage.AggregateUsageRow
}

func (l v12UsageLoader) ListUsageForBlocksFiltered(_ context.Context, _, _ time.Time, _ string) ([]storage.TokenUsageEntry, error) {
	return l.entries, nil
}

func (l v12UsageLoader) AggregateUsage(_ context.Context, f storage.AggregateFilter) ([]storage.AggregateUsageRow, error) {
	if len(l.aggRows) > 0 {
		return l.aggRows, nil
	}
	byBucket := map[string]*storage.AggregateUsageRow{}
	order := []string{}
	for _, e := range l.entries {
		bucket := e.Timestamp.UTC().Format("2006-01-02")
		row := byBucket[bucket]
		if row == nil {
			row = &storage.AggregateUsageRow{Bucket: bucket, Project: filepath.Base(e.CWD)}
			byBucket[bucket] = row
			order = append(order, bucket)
		}
		row.InputTokens += e.InputTokens
		row.OutputTokens += e.OutputTokens
		row.CacheCreateTokens += e.CacheCreationInputTokens
		row.CacheReadTokens += e.CacheReadInputTokens
		row.Cost += e.CostUSD
		row.Models = v12AppendUniqueString(row.Models, e.Model)
	}
	out := make([]storage.AggregateUsageRow, 0, len(order))
	for _, bucket := range order {
		out = append(out, *byBucket[bucket])
	}
	_ = f
	return out, nil
}

func v12HomeDB(t *testing.T) (string, *storage.DB) {
	t.Helper()
	home := t.TempDir()
	db := openHomeDB(t, home)
	return home, db
}

func v12Dispatch(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(t, func() {
		err = runCLIDispatch(args)
	})
	return out, err
}

func v12HermeticAdapterEnv(t *testing.T, except string) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "no-such-source")
	envs := []string{
		"OPENCODE_DATA_DIR", "AMP_DATA_DIR", "GEMINI_DATA_DIR", "COPILOT_OTEL_FILE_EXPORTER_PATH",
		"GOOSE_PATH_ROOT", "CODEBUFF_DATA_DIR", "HERMES_HOME", "KILO_DATA_DIR", "KIMI_DATA_DIR",
		"OPENCLAW_DIR", "PI_AGENT_DIR", "DROID_SESSIONS_DIR", "QWEN_DATA_DIR",
	}
	for _, env := range envs {
		if os.Getenv(env) == "" {
			t.Setenv(env, empty)
		}
	}
	_ = except
}

func writeV12OpenCodeDB(t *testing.T, path, id, sessionID, data string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE message (id TEXT, session_id TEXT, data TEXT)`); err != nil {
		t.Fatalf("create message table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, session_id, data) VALUES (?, ?, ?)`, id, sessionID, data); err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func writeV12OpenCodeJSONMessage(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "storage", "message", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir opencode message dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "message.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write opencode message: %v", err)
	}
}

func v12AssertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("output missing %q:\n%s", want, got)
	}
}

func v12AppendUniqueString(values []string, next string) []string {
	if next == "" {
		return values
	}
	for _, existing := range values {
		if existing == next {
			return values
		}
	}
	return append(values, next)
}

func v12Time(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return t
}
