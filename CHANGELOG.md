# Changelog

All notable changes to TokenMeter are tracked here. Versions follow semver.
The "Unreleased" section captures work merged but not yet tagged.

## v1.2.0 — TBD

> Compared with v1.1.0.

### Headline (TL;DR)

v1.2.0 turns TokenMeter from a ccusage-aligned local usage reporter into a broader local observability tool for AI coding agents. It closes the largest v1.1 parity gaps: runtime pricing, Codex speed tiers, OpenCode SQLite, Droid sidecar metadata, responsive tables, statusline controls, blocks token limits, project aliases, and unified config. It also goes beyond ccusage with tool error analysis, file churn reporting, anomaly webhooks, and an advanced FTS5 search DSL.

### ✨ Aligned with ccusage (9 P0/P1 features)

- **LiteLLM runtime pricing sync with offline fallback** — runtime cache refresh via `tm pricing refresh`; report commands can keep network access disabled with local cache / embedded fallback; live LiteLLM schemas with mixed numeric/string fields now parse and the cache loop has end-to-end verification. ([e0ba1c4](https://github.com/tt-a1i/tokenmeter/commit/e0ba1c4), [718d2e1](https://github.com/tt-a1i/tokenmeter/commit/718d2e1), [2b38b20](https://github.com/tt-a1i/tokenmeter/commit/2b38b20))
- **Codex `--speed` tier support** — `auto`, `standard`, and `fast`, including `~/.codex/config.toml` `service_tier` detection. ([e0ba1c4](https://github.com/tt-a1i/tokenmeter/commit/e0ba1c4))
- **OpenCode SQLite loader** — reads modern `opencode.db` and channel databases before JSON fallbacks. ([9e78818](https://github.com/tt-a1i/tokenmeter/commit/9e78818))
- **Droid sidecar JSONL model fallback** — fills missing session model names from adjacent sidecar logs. ([9e78818](https://github.com/tt-a1i/tokenmeter/commit/9e78818))
- **Responsive terminal tables with `--compact`** — daily, weekly, monthly, session, and blocks output adapt to narrow terminals. ([b10272d](https://github.com/tt-a1i/tokenmeter/commit/b10272d))
- **Statusline context thresholds and burn-rate display modes** — configurable context warning/danger thresholds plus `off`, `emoji`, `text`, and `emoji-text` burn-rate views. ([b10272d](https://github.com/tt-a1i/tokenmeter/commit/b10272d))
- **`tm blocks --token-limit` progress bar** — annotates 5-hour billing windows with usage percentage and status. ([992ebd1](https://github.com/tt-a1i/tokenmeter/commit/992ebd1))
- **Project alias grouping flags** — `--instances` and `--project-aliases` normalize multi-worktree reporting. ([7215719](https://github.com/tt-a1i/tokenmeter/commit/7215719))
- **Unified config model** — `~/.tokenmeter/config.json`, `tm config show/path/init`, legacy pricing/webhook merge, defaults, per-command overrides, and presence-aware bools. ([2c8b307](https://github.com/tt-a1i/tokenmeter/commit/2c8b307), [892de23](https://github.com/tt-a1i/tokenmeter/commit/892de23), [4c415ea](https://github.com/tt-a1i/tokenmeter/commit/4c415ea))

### 🚀 Beyond ccusage (4 features)

- **Tool error pattern analysis** — `tm analyze --tool-errors` ranks failing tools, groups repeated error patterns, and shows a daily failure-rate trend sparkline for the last 14 displayed days; v1.2 now defaults this view to the current project, with `--all-projects`, `--project NAME`, and `--project-aliases` for explicit scope control. ([5ec2cb3](https://github.com/tt-a1i/tokenmeter/commit/5ec2cb3), [12e8d93](https://github.com/tt-a1i/tokenmeter/commit/12e8d93))
- **File churn heatmap** — `tm analyze --file-churn` reports top changed files with mode breakdown, depth-2 directory hotspots such as `internal/storage`, and a daily file-change sparkline; it now defaults to current project scope with the same analyze project flags. ([caaa4ce](https://github.com/tt-a1i/tokenmeter/commit/caaa4ce), [12e8d93](https://github.com/tt-a1i/tokenmeter/commit/12e8d93))
- **Webhook anomaly detection** — `cost_spike` and `usage_regression` events with cooldown controls. ([ae5f4bf](https://github.com/tt-a1i/tokenmeter/commit/ae5f4bf), [4c415ea](https://github.com/tt-a1i/tokenmeter/commit/4c415ea))
- **FTS5 advanced search query DSL** — `tool:`, `session:`, `cost:`, `tokens:`, `since:`, `until:`, `status:`, and `platform:` filters for `tm search`. ([b6d00ad](https://github.com/tt-a1i/tokenmeter/commit/b6d00ad))

### 📚 Documentation

- **VitePress documentation site** — docs skeleton, source pages, guides, and sidebar coverage under `docs/site/`. ([585b343](https://github.com/tt-a1i/tokenmeter/commit/585b343), [97e2734](https://github.com/tt-a1i/tokenmeter/commit/97e2734))
- **Front-page docs refresh** — README and docs home now highlight Wave 2 capabilities such as config, compact output, project aliases, and tool error analysis. ([432baab](https://github.com/tt-a1i/tokenmeter/commit/432baab))
- **Documentation accuracy** — README and CLAUDE.md no longer claim the removed Bubbletea TUI is the current default. ([e13a38c](https://github.com/tt-a1i/tokenmeter/commit/e13a38c))
- **Release preparation docs** — v1.2 changelog and release notes draft were added for tag preparation. ([7b16df5](https://github.com/tt-a1i/tokenmeter/commit/7b16df5))

### 🐛 Bug fixes

- Project alias resolution is deterministic; duplicate paths no longer depend on Go map iteration order. ([3b34305](https://github.com/tt-a1i/tokenmeter/commit/3b34305))
- Leading global flags such as `--config`, `--no-color`, and `--offline` are no longer rejected before the subcommand. ([3b34305](https://github.com/tt-a1i/tokenmeter/commit/3b34305))
- README quickstart no longer includes `tm pricing refresh --offline` in the happy path, because that command intentionally returns exit 1 when no refresh is attempted. ([1dbd7df](https://github.com/tt-a1i/tokenmeter/commit/1dbd7df))
- Render color tests no longer depend on ambient terminal color state; the test now isolates go-pretty color initialization before checking token-limit status colors. ([6aa6e1a](https://github.com/tt-a1i/tokenmeter/commit/6aa6e1a))
- `tm config` and appdir resolution now honor `TOKENMETER_HOME` with precedence over current and legacy config locations. ([53e221f](https://github.com/tt-a1i/tokenmeter/commit/53e221f))
- LiteLLM runtime pricing accepts number or numeric-string schema fields, tolerates null/empty values, rejects fractional integer fields, and skips documentation-only `sample_spec` entries. ([718d2e1](https://github.com/tt-a1i/tokenmeter/commit/718d2e1))

### 🧹 Internal

- Help regression tests now cover daily, weekly, monthly, session, blocks, statusline, analyze, and advanced search help text. ([5ef334f](https://github.com/tt-a1i/tokenmeter/commit/5ef334f), [bd28965](https://github.com/tt-a1i/tokenmeter/commit/bd28965), [b6d00ad](https://github.com/tt-a1i/tokenmeter/commit/b6d00ad))
- Wave 2 and Wave 3 acceptance reviews are archived under `.hive/wave2-review.md` and `.hive/wave3-review.md`. ([6df4dc9](https://github.com/tt-a1i/tokenmeter/commit/6df4dc9), [2ba4bd8](https://github.com/tt-a1i/tokenmeter/commit/2ba4bd8))
- v1.2 smoke coverage validates key CLI flows before release, with tightened release expectations. ([62ea6c3](https://github.com/tt-a1i/tokenmeter/commit/62ea6c3), [1796905](https://github.com/tt-a1i/tokenmeter/commit/1796905))
- v1.2 prerelease verification report captures final release readiness evidence. ([4e86ef9](https://github.com/tt-a1i/tokenmeter/commit/4e86ef9))

### ⚠️ Breaking changes

- `commands.<name>.field=false` now truly overrides `defaults.field=true` for presence-aware boolean config fields. In v1.1-style behavior, explicit false was effectively indistinguishable from unset. Review any unified config that used explicit false only as documentation. ([4c415ea](https://github.com/tt-a1i/tokenmeter/commit/4c415ea))

### 📦 Migration from v1.1 → v1.2

1. Run `tm config init` to create a documented unified config skeleton.
2. Run `tm config show` to inspect the effective config and legacy pricing/webhook merge.
3. Review existing `pricing.json` and `webhooks.json`; migrate durable settings into `~/.tokenmeter/config.json` when ready.
4. Check any `commands.<name>` overrides that explicitly set boolean fields to `false`; they now take effect.
5. Use `tm pricing refresh` once online to seed the LiteLLM runtime pricing cache.
6. Try `tm daily --compact` in narrow terminals and `tm blocks --token-limit max` for 5-hour block progress.
7. Use `tm daily --instances --project-aliases '{"repo":["/path/a","/path/b"]}'` to normalize multiple worktrees.
8. Use `tm analyze --tool-errors` and `tm analyze --file-churn` to inspect current-project reliability and churn hotspots; add `--all-projects` for whole-database release audits.
9. Update automation that passed `--config` after the command only if you want to standardize on leading global flags; both positions are supported.

### 📋 Known limitations / follow-up

- `sources.<name>` schema exists but is not yet applied to adapter/source-specific defaults.
- `session-length` is not in unified config defaults; continue using `--session-length`.
- Anomaly webhook cooldown is in-memory; daemon restart resets cooldown state.
- Tool error pattern grouping depends on captured `result_summary`; old databases may have failures without summaries.
- Droid sidecar JSONL model fallback scans only the first 500 lines as a large-log heuristic.
- OpenCode SQLite open, query, and scan errors are silently skipped for batch resilience; verbose diagnostics are a follow-up.

### Acknowledgements

- 李诫 / 颜真卿 / 公孙弘 delivered v1.2 through parallel implementation, review, and documentation passes.

## v1.1.0 — 2026-05-22

### Added — 13 new data sources (ccusage parity)
- **OpenCode** (`tm opencode daily`) — `~/.local/share/opencode/` / `OPENCODE_DATA_DIR`
- **Amp** (`tm amp daily`) — `~/.local/share/amp/threads/*.json` / `AMP_DATA_DIR`
- **Gemini CLI** (`tm gemini daily`) — `~/.gemini/tmp/chats/*` / `GEMINI_DATA_DIR`
- **GitHub Copilot CLI** (`tm copilot daily`) — `~/.copilot/otel/*.jsonl` / `COPILOT_OTEL_FILE_EXPORTER_PATH`
- **Goose** (`tm goose daily`) — `~/.local/share/goose/sessions/<db>` (SQLite) / `GOOSE_PATH_ROOT`
- **Codebuff** (`tm codebuff daily`) — `~/.config/<channel>/projects/...` / `CODEBUFF_DATA_DIR`
- **Hermes** (`tm hermes daily`) — `~/.hermes/state.db` (SQLite) / `HERMES_HOME`
- **Kilo** (`tm kilo daily`) — `~/.local/share/kilo/<db>` (SQLite) / `KILO_DATA_DIR`
- **Kimi** (`tm kimi daily`) — `~/.kimi/sessions/...` / `KIMI_DATA_DIR`
- **OpenClaw** (`tm openclaw daily`) — `~/.openclaw/`, `~/.clawdbot/`, `~/.moltbot/`, `~/.moldbot/` / `OPENCLAW_DIR`
- **pi-agent** (`tm pi daily`) — `~/.pi/agent/sessions/...` / `PI_AGENT_DIR`
- **Droid** (`tm droid daily`) — `~/.factory/sessions/*.settings.json` / `DROID_SESSIONS_DIR`
- **Qwen** (`tm qwen daily`) — `~/.qwen/projects/<project>/chats/*.jsonl` / `QWEN_DATA_DIR`

All 13 sources support `daily` / `weekly` / `monthly` / `session` buckets and honor
`--since` / `--until` / `--json` / `--breakdown` / `--order` / `--mode` /
`--timezone` / `--no-color`.

### Changed
- `tm daily` (no source) now scans the 13 new batch-only sources in addition
  to the SQLite (Claude + Codex) store. Missing log directories are silently
  skipped (fast path: `< 5 ms`). Pass `--no-scan` to restore the v1.0.x
  SQLite-only behavior for scripts that depend on the old default.

### Internal
- Adapter pattern is flat: each new source has `internal/collector/<name>.go`
  exporting `Load<Name>Entries(ctx, opts)`. No `Adapter` trait — matches ccusage
  upstream choice (different per-agent log formats make a shared interface
  leaky). See `docs/MIGRATION-v1.1.md` for migration notes.
- daemon, SQLite schema, and Claude / Codex collection paths unchanged.

### Known limitations / known differences vs ccusage upstream
- **Reasoning tokens fold into OutputTokens** for Gemini / Goose / Hermes /
  Kilo / Copilot / Qwen / Pi. `UsageEntry` has no dedicated extra-total slot;
  totals match ccusage but `outputTokens` is inflated by the reasoning count.
  Per-bucket / per-model breakdowns are unaffected.
- **OpenCode SQLite-only installs are not visible in v1.1**. The v1.1 adapter
  reads the file-tree format only; a SQLite-only OpenCode store (newer
  install) needs the v1.1.1 follow-up. Workaround: pin OpenCode to a build
  that still writes the file tree.
- **Copilot OTEL cross-source dedup not implemented**. ccusage suppresses
  lower-priority OTEL spans (chat > inference > agent_turn > agent_summary)
  when they overlap by `trace_id` / `response_id`; the v1.1 Go adapter emits
  every `gen_ai.usage.*` row it finds, so production layouts with overlapping
  span types may double-count. Track for v1.1.x.
- **Codebuff `runState.sessionState.mainAgentState.messageHistory` fallback
  not implemented**. A rare branch ccusage uses when `metadata.usage` and
  `metadata.codebuff.usage` are both missing.
- **Droid sidecar `.jsonl` model fallback** is not yet wired. When the model
  is missing from the session JSON, the row is dropped (ccusage tries an
  adjacent log file for the inferred model first).
- **Costs are recomputed at the cli pricing layer**. Most adapters set
  `CostUSD = 0` and let `--mode auto / calculate` reach `pricing.Resolve`;
  ccusage carries provider-native costs (credits, USD) directly on the
  source row. `--mode display` therefore shows `$0` for batch adapters with
  no native USD cost — switch to `--mode auto` (default) or `--mode calculate`
  for non-zero values.
- **`pricing.Resolve` provider/model candidate fallback** is single-key only.
  ccusage tries `<model>` then `qwen/<model>` then `alibaba/<model>` etc.;
  v1.1 looks up the verbatim model name only. Track for v1.1.x.
- **`--project <path>`** is honored by Claude / Codex (sessions.cwd join).
  Most batch adapters carry no reliable workspace path in their logs and
  silently ignore `--project`. The per-source support matrix lives in
  `docs/MIGRATION-v1.1.md`.

## v1.0.3 — 2026-05-21

### Performance
- `token_usage(timestamp DESC)` covering index + JOIN elision for daily/weekly/
  monthly buckets. SQL planner now uses the covering index instead of nested-loop
  via the session index.
  - session warm 1.92s → 1.16s (**1.66×** — biggest single win)
  - weekly warm 1.02s → 0.88s (1.16×)
  - daily warm 0.95s → 0.85s (1.12×)
  - monthly warm 1.01s → 0.90s (1.12×)
  - blocks unchanged (raw timeline scan, not push-down)
- Index size cost: **+46 MB / +8%** on 549 MB DB.
- See `docs/perf/v1.0.3-baseline.md` for benchmark methodology + EXPLAIN
  QUERY PLAN verification.

### Internal
- daily/weekly/monthly skip the `JOIN sessions` because the daemon invariant
  guarantees `session_id` has an upstream sessions row (PRAGMA foreign_keys
  defaults to OFF; REFERENCES is declarative-only). Session bucket retains
  the JOIN to read `MAX(s.cwd)` for projectPath.

## v1.0.2 — 2026-05-21

### Performance
- SQL push-down for `daily` / `weekly` / `monthly` / `session`:
  daily warm 1.22s → 0.99s (1.23×); weekly/monthly ~1.14×; session ~1.06×.
  blocks unchanged (5h identification needs raw timeline).
- See `docs/perf/v1.0.2-baseline.md` for full pprof + benchmark breakdown
  and v1.0.3 optimization candidates (covering index est. 30–50% further win).

### Fixed
- `Mode=auto` 在 non-breakdown 路径下不再 silent 丢失零成本 entries 的 fallback
  recompute。v1.0.1 是 per-entry fallback；push-down 后 bucket SUM 会掩盖个体零成本
  行。修复：cli 在 Mode=auto && !user-Breakdown 时透明强制 storage filter.Breakdown=true，
  渲染时按 bucket 折叠回单行（用户视图不变）但 cost 按 (bucket, model) 重算。混合天
  cost 可恢复 ~$11k 量级真实金额（实测本地 DB 由 $57769 修正为 $69284）。

### Added
- `blocks --breakdown` 嵌套显示 per-model 子行（v1.0.1 known limitation 补齐）。
- `session --json` 输出含 `projectPath` / `lastActivity` 字段（来自 SQL
  `MAX(s.cwd)` / `MAX(u.timestamp)`）。

### Known limitations (carry-forward)
- `--timezone <IANA>` 用查询时刻的 UTC offset；DST 区跨 DST 边界的历史数据可能
  ±1h 误分桶。Workaround：UTC 或固定 offset alias。

## v1.0.1 — 2026-05-20

### Added
- `--json` actually takes effect on `tm daily` / `tm weekly` / `tm monthly` / `tm session` / `tm blocks`. The envelope is camelCase with a top-level wrapper (`daily` / `weekly` / `monthly` / `sessions` / `blocks` + `totals`) aligned with ccusage v20.
- `--breakdown` actually renders per-model subrows for `tm daily` / `tm weekly` / `tm monthly` / `tm session`. Boxed output uses a `└─ <model>` nest; JSON adds a `modelBreakdowns[]` field on each row.
- `--order desc` reverses the row order across daily / weekly / monthly / session.
- `--mode auto|calculate|display` now applies for every cost-aware subcommand. `calculate` rewrites cost from `tokens × pricing.Resolve(model)`; `auto` only recomputes when the source row's cost is zero; `display` passes through unchanged.
- `--timezone <IANA>` reroutes timestamp bucketing through the named zone (e.g. `--timezone Asia/Shanghai` puts 23:30 UTC into the next day).
- `--project <path>` filters all subcommands by `sessions.cwd`.
- `--no-color` actually suppresses ANSI escapes, with autodetection for non-TTY writers plus `NO_COLOR` and `FORCE_COLOR` environment variable support (`FORCE_COLOR` wins on TTYs, `NO_COLOR` wins everywhere else).
- Table output aligned with ccusage: 8 / 9 column rounded box with TOTAL footer, Models cyan, token columns yellow, COST red, block STATUS (`ACTIVE` green / `gap` faint / `closed` dim).
- Empty range prints `(no data in range)` instead of a bare header.
- `--cpu-profile <path>` hidden flag writes a Go pprof CPU profile for `tm <subcommand>` invocations (developer tool — not advertised in `tm help`).

### Changed
- `tm` with no subcommand now runs `daily` (matching ccusage). `tm help`, `tm -h`, `tm --help` still surface the help text.
- `--until YYYYMMDD` is now an inclusive close to the end of that day. Internally the parsed timestamp is bumped +24h before being compared with the loader's `<= until`, so an entry at 2026-05-20 23:59 is included when you ask for `--until 20260520`. Previously a half-open interval that silently dropped same-day data. See `docs/MIGRATION-v1.0.md`.

### Performance
- v1.0.1 baseline: `docs/perf/v1.0.1-baseline.md`. On a 1,587-session / 445,904-row / 549 MB local DB, warm `tm daily` runs at ~820 ms; cold ~2.4 s. Top hotspot is SQLite page reads via `syscall.Pread` (71.4% flat). v1.0.2 optimization candidates documented but not committed: `token_usage(timestamp DESC)` index, SQL-side `GROUP BY` push-down for the daily/weekly/monthly path, selective `SELECT` when `Mode != calculate`.

## v1.0.0 — 2026-05-20

### Breaking
- TUI removed. `tm` (no arguments) now prints help instead of entering an interactive interface.

### Deprecated (will be removed in v2.0)
- `tm cost` → use `tm daily`
- `tm report` → use `tm session` / `tm weekly` / `tm monthly`
- `tm status` → use `tm blocks --active`
- `tm top` → use `tm blocks --active`

### Added
- `tm daily`, `tm weekly`, `tm monthly`, `tm session` — ccusage-style aggregate reports.
- `tm blocks`, `tm blocks --active` — 5-hour session block view with burn rate and projection.
- `tm statusline` — Claude Code statusline provider (`stdin JSON → stdout line`).
- `--session-length` flag to customize block window.
- LiteLLM-sourced pricing snapshot with weekly auto-refresh PR.
- `--mode auto|calculate|display` cost mode.
- `--since` / `--until` / `--project` now apply to `tm daily` / `tm weekly` / `tm monthly` / `tm session` / `tm blocks` (previously silently ignored).

### Limitations (to be addressed in v1.0.1)
- `--json` is honored by `tm blocks` but not yet by `tm daily` / `tm weekly` / `tm monthly` / `tm session`.
- `--breakdown` (model breakdown) and `--order desc` are not wired through aggregate output.
- `--mode auto|calculate|display` defaults to `display`; the calculate path is implemented in `internal/pricing.Apply` but not yet selected at the CLI layer.

## v0.8.3 — 2026-05-19 — Cross-platform CI fixes

v0.8.2 functionally shipped to `main`, but its `ci.yml` `release` job
was skipped because the Linux and Windows test matrices uncovered
cross-platform fixture issues that were invisible on darwin. v0.8.3
fixes every one of them so the same v0.8.2 feature set is the first
release to publish six-platform artifacts.

### Linux test fixtures

- **`gofmt cmd/tm/cmd_help.go`** (`5c792c8`) — restores the spacing
  regression introduced in the `f8204f8` budget seeAlso entry.
- **`TestFDSnapshotReturnsList`** (`f59bcd7`) opens a canary file so
  the assertion no longer depends on stdin/stdout/stderr surviving
  Linux's `/proc/self/fd` noise filter (pipes and `/dev/null` are
  filtered, so the snapshot returned zero elements on the runner).
- **`TestRemoteSubscriberCleanupOnSlowConsumer`** (`a6189b7`) inflates
  the broadcast payload to ~4 KB so 200 events overflow Linux's 208 KB
  unix-socket sndbuf. macOS sndbuf is ~8 KB, so the test already
  overflowed there; the test now exercises the cleanup path on both
  platforms.

### Windows test infrastructure

- **`internal/daemon/socket_windows.go`** (`0c97b71`) — real socket
  lock using `os.O_CREATE | os.O_EXCL` on a sidecar `.lock` file plus
  `tasklist`-based stale-PID detection. Previously a no-op stub, so
  two Windows daemons could share the same port without complaint and
  `TestStartRefusesIfSocketAlreadyLive` could never observe
  `ErrExist`.
- **`daemonPIDRunning` split** (`0c97b71`) into
  `cmd/tm/doctor_pid_unix.go` (`syscall.Signal(0)` probe) and
  `doctor_pid_windows.go` (`tasklist` check), so `tm doctor` no longer
  always reports the daemon as stale on Windows.
- **`captureStdout` reader goroutine** (`0c97b71`) is now started
  before the captured function runs, draining the pipe while the
  function writes. Fixes `windows-latest` hangs when `runDoctor`
  output exceeded Windows's smaller anonymous pipe buffer.
- **Six narrower fixture adjustments** (`06f29f6`):
  `TestDefaultLogPath` uses `setTestHome` for full env coverage;
  `TestRunReload*` gated to `!windows` (Windows reload is a Unix-only
  stub); `TestRunTopRespectsInterval` and
  `TestFTS5BackfillsOnUpgrade` skip on Windows (runner timing /
  SQLite WAL release); `TestRunWebGenerateToken` tolerates the
  Windows file-mode mapping; `TestHandleCostsTodayIncludesLocal
  EarlyMorning` no longer slips its fallback seed into yesterday.
- **Tightened `TestRunTopOnceProducesSnapshot`** (`06f29f6`) seeds
  from now−3h to now−2m so the test passes regardless of when the
  wall clock fires.

### CI plumbing

- **`go test` timeout raised from 180s to 600s** (`62ba70b`); the
  `windows-latest` runner needs the extra headroom under `-race`,
  while `ubuntu-latest` still finishes well under a minute.

### Production code touched

- New `internal/daemon/socket_windows.go` (real lock).
- New `cmd/tm/doctor_pid_unix.go` / `cmd/tm/doctor_pid_windows.go`
  (split daemon-PID probe).

Everything else in v0.8.3 is `_test.go` fixtures or YAML config; no
behavior change for end users on macOS or Linux.

## v0.8.2 — 2026-05-18

A polish-and-stabilize release: web dashboard gets a visual refresh, five
auto-generated insight cards plus a cost-forecast endpoint, a calendar
heatmap and cost sunburst, real Windows support across daemon and tests,
the PWA picks up versioned caches and an update banner, and the release
pipeline is consolidated onto a single `ci.yml` path so tag pushes stop
failing on duplicate goreleaser runs.

### Highlights

- **Web dashboard visual upgrade** (`d314297`) — glassmorphism cards with
  `backdrop-filter: blur(...) saturate(180%)`, spring hover lift
  (`translateY(-2px)` + accent glow), `tabular-nums` on every numeric
  metric, ambient radial-gradient body background gated by
  `prefers-reduced-motion` and high-contrast, and an SSE value-flash so
  changed numbers briefly highlight without flicker.
- **Auto-generated insight cards** (`e3e0846` API + `61d5bff` UI) —
  `GET /api/insights?range=today|week|month|all` returns up to five
  insights (`peak_day`, `top_tool`, `model_mix_shift`, `cost_anomaly`,
  `rhythm`), each with `{id, kind, title, body, value, metadata}`. The
  frontend renders glass cards with per-insight dismiss persisted to
  localStorage; dismiss resets automatically when the underlying value
  changes.
- **Cost forecast endpoint** (`208c68e`) — `GET /api/forecast?period=...`
  returns `spent_to_date`, `burn_rate_per_day` (7-day rolling),
  `projected_total`, `projected_remaining`, `confidence`
  (`low|medium|high`), `trend` (`up|down|stable`), and
  `vs_previous_period`. Two DB round-trips, no shared state, panic-safe.
- **Calendar heatmap + cost sunburst** (`b955750`) — 13-week
  GitHub-style activity heatmap with quantile-tiered cells
  (`--heatmap-q0..q4`, dark / high-contrast aware) plus a 2-ring sunburst
  (platform → model) with keyboard navigation. Both have SR-only
  fallbacks.
- **Audit-driven polish** (`1e0a74d`) — eight of the top-ten web audit
  findings landed: `setPref` no longer redraws charts for non-theme
  changes (A2), nine input elements gained `aria-label` (C1), session
  detail Back restores scroll position (B1), a global
  `:where(...):focus-visible` outline (C4), `applyDashboardData` wraps
  `renderChart` / `renderModels` / `renderTools` in `requestIdleCallback`
  (A1), `.heatmap-section` / `.sunburst-card` / `.ic-charts-section`
  pick up `content-visibility: auto` (E3 + A5), tap targets land on
  `min-height: 36px` with a desktop override at 768px (B7), and hero
  metrics span the full row at 640px (B6). Chart consolidation (E1) and
  virtual-scroll Map diff (A3) are intentionally postponed.
- **PWA update banner** (`1e0a74d`) — `controllerchange` listener and
  `SKIP_WAITING` postMessage path so new SW versions surface a glass
  banner ("新版本可用 / 立即刷新 / ×") instead of forcing a manual
  reload.
- **PWA infrastructure** (`6411ab7`) — `sw.js` rewritten with versioned
  cache pool (`tm-v2-static` / `tm-v2-api`), API GET on network-first
  with a 5s timeout and cached-JSON fallback marked
  `X-TokenMeter-Cache: hit`, static assets stale-while-revalidate gated
  on `request.destination`, SSE pass-through for `/api/events`. New
  `icon-maskable.svg` for Android Adaptive Icon and a `manifest.json`
  with `short_name "TM"` plus Today / Sessions / Analytics shortcuts.

### Real Windows support

- **Workspace filter uses `path.Clean`** (`2db25dc`) — previously
  `filepath.Clean` silently converted `/foo` to `\foo` on Windows,
  breaking workspace-filtered session lists.
- **Daemon picks OS-assigned TCP ports on Windows via `:0`** (`0dea537`)
  — fixes bind conflicts when tests restart the daemon rapidly.
- **Test fixtures cover `USERPROFILE` / `HOMEDRIVE` / `HOMEPATH`**
  (`38eb2d2`) so `t.Setenv("HOME", ...)` actually isolates on Windows
  where `os.UserHomeDir()` reads `USERPROFILE`.
- **Three tests skip on Windows** (`babf734`) for documented
  environmental differences: modernc SQLite WAL release timing, Windows
  ACL semantics for `os.Chmod(0o555)`, and GitHub Actions Windows runner
  I/O latency too slow for the existing p99 budget.

### Release pipeline

- **`.github/workflows/release.yml` removed** (`69d98be`) — it
  duplicated `ci.yml`'s `release` job and caused every tag push to fail
  at the second goreleaser invocation. `ci.yml` is now the single
  canonical release path (lint + cross-platform test matrix +
  goreleaser with conditional `--skip=homebrew`).
- **`.goreleaser.yml` on v2 schema** (`d844360`) — `archives.formats`
  array, `homebrew_casks` replacing `brews` (with `binaries: ["tm"]`
  instead of a Ruby `install` block), and a `skip_upload` template
  using `index .Env` that gracefully no-ops when
  `HOMEBREW_TAP_GITHUB_TOKEN` is absent (`6e8c145`).
- **Brew install command** updated to `brew install --cask tt-a1i/tap/tm`
  (`1a536cc`) across `README.md`, `README_EN.md`, `docs/design.md`, and
  `CLAUDE.md`.

### Documentation

- **`docs/api.md`** (`c47bff7`) — 868 lines covering all 17 HTTP
  endpoints in a consistent five-section template (description,
  parameters, response, errors, curl example, source location).

### Preserved (no migration needed)

- Module path `github.com/tt-a1i/tokenmeter`
- Data paths `~/.tokenmeter/`, `tokenmeter.db`, `tokenmeter.sock`,
  `tokenmeter.log`
- Prometheus metric namespace `tokenmeter_*`
- localStorage keys and Service Worker caches — legacy
  `tokenmeter-static-v2` / `tokenmeter-api-v1` pools are auto-evicted on
  first `tm-v2-*` activation (no user action required).

### Documentation fixes

Audit-driven (`f8204f8`):

- `tm web --help` no longer lists a non-existent `--host` flag and now
  documents the real authentication flags (`--token`, `--no-auth`,
  `--generate-token`).
- `tm version --help` documents `--json` for machine-readable output.
- `tm budget --help` includes a `tm budget delete <id>` example.
- README.md / README_EN.md command tables: `tm cost [period]` lists the
  six accepted values (today / week / month / 3month / year / all)
  instead of the stale `[today|week]`.
- README_EN.md gains rows for `tm web`, `tm report --weekly`, and
  `tm report --monthly` (previously English readers couldn't discover
  the web dashboard entry point or weekly/monthly reports).

## v0.8.1 — 2026-05-17 — Fix Windows build

- Split `cmd/tm/reload` into `reload_unix.go` (build tag `!windows`) and
  a Windows stub returning a clear error. Restores cross-platform builds
  for goreleaser windows_amd64 / arm64 and CI Vet on windows-latest.
- No functional change on macOS / Linux versus v0.8.0.

## v0.8.0 — 2026-05-17 — Rename to tm with auto-setup; web flicker fix

### Highlights
- **Binary renamed** `tokenmeter` → `tm`. `cmd/tokenmeter/` moved to `cmd/tm/`
  so `go install github.com/tt-a1i/tokenmeter/cmd/tm@latest` produces a
  short `tm` binary directly.
- **Auto-setup on first run** — `tm` / `tm daemon` / `tm web` silently
  inject Claude Code hooks into `~/.claude/settings.json` on first
  invocation. Manual `tm setup` retained for repair. Legacy
  `tokenmeter emit` and `agmon emit` hook entries are detected as
  already-installed; explicit `tm setup` rewrites them.
- **Web dashboard flicker fixed** — full dashboard repaints (6 metric
  cards, 3 Canvas charts, sessions list, sparkline) on every SSE
  `token_usage` event are now coalesced into a 200ms window. In-memory
  totals (allS / lastStats / lastCosts) still update synchronously so
  optimistic state remains accurate.
- **goreleaser v2 migration** — `archives.formats` (array form),
  `homebrew_casks` replacing `brews`, `skip_upload` template
  gracefully no-ops when `HOMEBREW_TAP_GITHUB_TOKEN` is absent.
  Brew install command is now `brew install --cask tt-a1i/tap/tm` (cask
  tap pending activation).
- **CI / Docker / release pipelines** updated for renamed binary.

### Preserved (no migration needed)
- Module path `github.com/tt-a1i/tokenmeter`
- Data paths `~/.tokenmeter/`, `tokenmeter.db`, `tokenmeter.sock`,
  `tokenmeter.log`
- Prometheus metric namespace `tokenmeter_*`
- localStorage keys and Service Worker cache names — onboarding state
  and offline cache survive the upgrade

## v0.7.0 — 2026-05-17

This release rolls up a multi-round hardening pass covering security,
correctness, time-zone behavior, observability, and test/benchmark
infrastructure. No new user-facing features; everything is reliability,
safety, and developer ergonomics.

### Security

- **Unix socket now mode `0600`** — `~/.tokenmeter/tokenmeter.sock` and the
  subscriber socket are chmod'd to owner-only after `Listen`. Previously
  the default umask left them at `0644`, which on Linux/macOS lets any
  local user inject fake hook events into the daemon. Same-host attackers
  can no longer poison your usage data without already running as your UID.
  A µs-level TOCTOU window between Listen and Chmod is documented; for
  strict guarantees wrap the daemon start in `syscall.Umask(0o077)`.
- **HTTP server hardening** — `tokenmeter web` now sets
  `ReadHeaderTimeout=5s` / `ReadTimeout=30s` / `WriteTimeout=30s` /
  `IdleTimeout=60s`, and SIGINT/SIGTERM triggers `srv.Shutdown(ctx)` so
  in-flight requests drain before the process exits. Slowloris-style hangs
  are no longer possible.
- **`/api/session/{id}` no longer leaks internal errors** — ambiguous
  prefix matches return a 400 with a user-friendly message; any other DB
  error (SQL syntax, table names, driver internals) becomes a generic
  500 with the detail logged server-side only.

### Correctness — time zones

- **All daily aggregates now bucket by local time** — `DATE(timestamp,
  'localtime')` plus matching local-time `from`/`to` boundaries in
  `web/`, `tui/`, and `cmd/`. Stored timestamps remain UTC; the change is
  purely query-side. For a UTC+8 user, "Today's cost" now matches the
  local calendar instead of starting at UTC midnight (= local 08:00).
- **`GetDailyCostsBetween` is now inclusive of endDay** — the previous
  half-open `[from, to)` semantics dropped today's partial bar when
  callers passed `to = now` mid-day.
- **`GetFirstTokenDate` returns a local-day anchor** — `range=all` no
  longer shows an empty first-day bucket from UTC/local offset.

### Correctness — data integrity

- **`parseTimestamp` returns `(time.Time, bool)`** instead of falling
  back to `time.Now()`. Malformed Codex/Claude log timestamps no longer
  pollute today's cost — affected events are dropped instead.
- **`truncate` is rune-safe** — tool params/results truncated mid-rune no
  longer produce invalid UTF-8 in storage (Chinese, emoji, etc.).
- **`bufio.Scanner` buffer raised to 16MB** in `extractPatchFileChanges`
  — apply_patch bodies with lines >64KB (minified JS, base64) no longer
  silently drop the rest of the patch.
- **Watcher truncation/rotation detected** — when a Claude/Codex JSONL
  file shrinks since the last scan, the watcher resets the byte offset
  to 0 and re-reads. `source_id` UNIQUE indexes prevent double-counting.
- **`codex pendingFileChanges` map now has a 2-hour TTL GC** running
  every ~30s — orphaned `function_call` entries (codex died mid-call)
  no longer leak the map.
- **Schema migrations are gated by `PRAGMA user_version`** — the legacy
  `normalizeTimeColumns` full-table scan no longer runs on every daemon
  restart. Set once, skipped forever.
- **`MarkStaleSessionsEnded(2h)` now runs on a 1-hour ticker** — a
  daemon running for days no longer accumulates `active` zombies.
- **`addColumnIfMissing` uses `PRAGMA table_info`** instead of string-
  matching the driver's error message.

### Performance

- **3 new time indexes** on hot aggregation paths:
  `idx_token_usage_ts`, `idx_tool_calls_start`, `idx_file_changes_ts`.
  First upgrade reopens an old DB will spend a few seconds building
  these; subsequent INSERTs are virtually free to maintain.
- **`/api/sessions?limit=N` query parameter** (capped at 1000). Default
  remains 200; the `/api/stats` `total_sessions` count now uses the
  same filter as `ListSessions` so the dashboard and stats numbers
  agree.
- **11 benchmarks added** as performance regression baselines:
  `broadcast` (~62 ns/op), `processEvent TokenUsage` (~140 µs/op),
  `ParseClaudeFileEvents`, `extractPatchFileChanges`, `truncate`,
  `GetDailyCostsBetween` (~4 ms / 500 rows), `GetCostBetween`,
  `GetModelCostBreakdown`, `ListSessions`, `GetActiveSessionCount`,
  `UpdateSessionTokens`. Future regressions surface immediately.

### Observability

- **Daemon `Stats()` counters** — `dropped_broadcasts`,
  `dropped_shutdown`, `duplicate_tool_starts` atomic counters surfaced
  in the daemon Stop log so operators can spot slow subscribers,
  shutdown-window event loss, or Pre-hook re-emit anomalies.
- **`ProcessExternalEventAsync` uses a two-stage send** — non-blocking
  first try; falls back to blocking with the `done` channel as a
  backstop. The previous random-select made shutdown-window event loss
  more likely than it had to be.
- **Second Ctrl+C force-quits** — `tokenmeter daemon` and `tokenmeter
  web` now install a watchdog goroutine that calls `os.Exit(130)` if a
  user impatiently presses Ctrl+C while graceful shutdown is in
  progress.
- **`emit.log` for `tokenmeter emit` errors** — the hook entry point
  logs to `~/.tokenmeter/emit.log` (10MB self-truncate) instead of
  stderr, so error chatter never pollutes Claude Code's hook
  stdout/stderr parsing.

### Refactors

- **`claude_log.go` extracts `parseClaudeLogTokenEvent`** — single
  source of truth for assistant-message token parsing, shared by both
  the parallel initial scan and the incremental processFile loop.
- **`HTTP Server` built in `NewServer`** — `Start()`/`Shutdown(ctx)`
  pair is now race-free at the cost of moving mux registration out of
  Start.
- **`storage.ErrAmbiguousSessionPrefix` sentinel** — callers use
  `errors.Is` to split user-input errors (400) from system errors (500)
  without parsing error strings.
- **`view_messages.go` chunked rendering** — no longer infinite-loops
  on extremely narrow terminals; preserves user content at the
  `len(remaining) == maxCols` boundary where the previous length-based
  detection mistook a truncated chunk for a complete one.

### Test infrastructure

- **~78 new tests** across all packages — boundary cases, regression
  guards, contract tests, and a TUI keyboard handler harness covering
  17 key events (q / Tab / Shift-Tab / j / k / G / [ / ] / / / Esc /
  Enter / t / p / s / c / r and several window/tick/refresh messages).
- **Coverage gains** (Round 0 → final):
  - cmd/tokenmeter: 33.7% → 53.3%
  - internal/appdir: 87.0% → 95.7%
  - internal/collector: 71.2% → 83.4%
  - internal/daemon: 62.9% → 77.8%
  - internal/report: 80.2% → 88.4%
  - internal/storage: 77.0% → 83.9%
  - internal/tui: 49.2% → 70.9%
  - internal/web: 62.2% → 73.0%

### Upgrade notes

- **First reopen of an existing database** will create three new time
  indexes; expect a few seconds of extra startup time on a database
  with hundreds of thousands of token rows. One-time cost.
- **`tokenmeter web` dashboard "today" / "this week" / "this month"
  values may shift by hours after upgrading** if the host is not in
  UTC — the buckets now align with local calendar days instead of UTC
  midnight. Totals over longer periods are unchanged; only the
  per-bucket numbers move.
- **Existing 0644 socket files from older daemons are removed and
  recreated at 0600** on the next start. No manual action required.
