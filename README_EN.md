<p align="center">
  <img src="https://img.shields.io/badge/TokenMeter-AI%20Agent%20Usage%20Meter-7C3AED?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIiBzdHJva2U9IndoaXRlIiBzdHJva2Utd2lkdGg9IjIiPjxwYXRoIGQ9Ik0xMyAyTDMgMTRoOWwtMSA4IDEwLTEyaC05bDEtOHoiLz48L3N2Zz4=&logoColor=white" alt="TokenMeter" height="28">
</p>

<h1 align="center">TokenMeter</h1>

<p align="center">
  <strong>Local usage meter for AI coding agents</strong>
</p>

<p align="center">
  <a href="https://github.com/tt-a1i/tokenmeter/releases"><img src="https://img.shields.io/github/v/release/tt-a1i/tokenmeter?style=flat-square&color=7C3AED&label=version" alt="Version"></a>
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <a href="https://github.com/tt-a1i/tokenmeter/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-22c55e?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-6B7280?style=flat-square" alt="Platform">
  <img src="https://img.shields.io/badge/Claude%20Code-supported-F59E0B?style=flat-square" alt="Claude Code">
  <img src="https://img.shields.io/badge/Codex-supported-22C55E?style=flat-square" alt="Codex">
</p>

<p align="center">
  <a href="./README.md">中文文档</a>
</p>

---

> Monitor token consumption, costs, tool calls, and file changes across Claude Code, Codex, and other AI coding agents with CLI reports and a local Web Dashboard.

<p align="center">
  <img width="732" alt="Dashboard" src="https://github.com/user-attachments/assets/06664199-5860-484c-818c-0b3257313dde" />
</p>

<p align="center">
  <img width="711" alt="Tool Calls" src="https://github.com/user-attachments/assets/32d70f5b-e6ab-48be-98c0-12209ddcd621" />
</p>

## Features

- **Multi-platform** — Claude Code + Codex in one unified view
- **Token tracking** — input, output, cache creation, cache read — per session, per model
- **Cost estimation** — model-aware pricing (Opus / Sonnet / Haiku / GPT-5 / GPT-4.1)
- **Cost trends** — Web Dashboard charts, heatmaps, and model/tool breakdowns show daily spend at a glance
- **Tool call traces** — name, params, result, duration, success/failure status
- **Conversation messages** — browse user prompts within each session, with `/` search
- **Session tags** — `tm tag <id> "note"` to label sessions for easy recall
- **Time range stats** — Today / Week / Month / All token & cost aggregation
- **Share recaps** — `tm share [session]` creates a compact Markdown session recap for sharing or handoff
- **Live updates** — daemon broadcasts events to `tm watch` and the Web Dashboard
- **Single binary** — run `tm setup` to install hooks, then collect locally without external services

## Supported Platforms

| Platform | Integration | How |
|----------|-------------|-----|
| **Claude Code** | Hooks + JSONL log watcher | `tm setup` auto-injects hooks into `~/.claude/settings.json` |
| **Codex** | JSONL log watcher | Automatic — polls `~/.codex/sessions/` |

## Install

### Quick Install (recommended)

```bash
curl -sL https://raw.githubusercontent.com/tt-a1i/tokenmeter/main/install.sh | sh
```

### Homebrew Cask

Available only when the release pipeline is configured with a Homebrew tap repository and `HOMEBREW_TAP_GITHUB_TOKEN`.
See [docs/release.md](docs/release.md) for release prerequisites.

```bash
brew install --cask tt-a1i/tap/tm
```

_The Homebrew tap repository is being set up; for now please use the quick-install script above, `go install`, or download from GitHub Releases._

### Go Install

```bash
go install github.com/tt-a1i/tokenmeter/cmd/tm@latest
```

### From source

```bash
git clone https://github.com/tt-a1i/tokenmeter.git
cd tokenmeter
make install
```

## Quick Start

```bash
tm setup             # one-time: register Claude hooks
tm daemon &          # background collector
tm daily             # today's summary across all sources
tm blocks --active   # what's happening in the current 5-hour window
tm web               # browser dashboard (separate process)
```

Use Claude Code or Codex normally — TokenMeter captures everything in the background. See [docs/MIGRATION-v1.0.md](docs/MIGRATION-v1.0.md) for the full old→new command mapping.

TokenMeter v1.1 adds 13 batch-only data sources: OpenCode, Amp, Gemini CLI,
GitHub Copilot CLI, Goose, Codebuff, Hermes, Kilo, Kimi, OpenClaw, pi-agent,
Droid, Qwen. `tm daily` scans every installed agent by default; pass
`--no-scan` to restore the SQLite-only behavior (Claude + Codex only).
Per-source subcommands and log paths are in [docs/MIGRATION-v1.1.md](docs/MIGRATION-v1.1.md).

## Commands

| Command | Description |
|---------|-------------|
| `tm` | Same as `tm daily`; shows the daily token / cost summary |
| `tm daemon` | Start daemon only |
| `tm daily` / `tm weekly` / `tm monthly` | Aggregate all sources by day / week / month |
| `tm session [id]` | Per-session breakdown, optionally filtered by id |
| `tm blocks [--active]` | 5-hour blocks, burn rate, and projection |
| `tm statusline` | Claude Code statusline provider |
| `tm watch [opts]` | Stream daemon events from the socket |
| `tm share [session]` | Shareable Markdown session recap |
| `tm export [opts]` | CSV / JSON export |
| `tm web [--port N]` | Start Web Dashboard (default port 8370) |
| `tm clean [days]` | Remove sessions older than N days (default: 7) |
| `tm tag <id> [text]` | Tag a session with a note (omit text to clear) |
| `tm budget <subcommand>` | Manage budgets |
| `tm webhook <subcommand>` | Manage webhook endpoints |
| `tm setup` | Configure Claude Code hooks |
| `tm uninstall` | Remove hooks and stop daemon |
| `tm version` | Show version |

> In the historical v0.x shape, `tm` entered a Bubbletea TUI. The TUI was removed in v1.0; `tm` now defaults to the daily report. See [docs/MIGRATION-v1.0.md](docs/MIGRATION-v1.0.md) for the old-to-new command mapping.

## Architecture

The interactive architecture diagram shows the full data flow. Component cheat sheet:

- **Daemon** — receives Claude hook events over a Unix socket, persists them to SQLite, and broadcasts live events to `tm watch` / Web
- **Claude hooks** — 8 events: `PreToolUse`, `PostToolUse`, `SessionStart`, `SessionEnd`, etc.
- **Log watchers** — Claude watcher scans JSONL under `~/.claude/projects/` for tokens; Codex watcher polls `~/.codex/sessions/` with in-memory deduplication
- **CLI** — `daily` / `weekly` / `monthly` / `session` / `blocks` / `statusline` read SQLite or local source logs and render reports
- **Web** — standalone HTTP server + embedded SPA, reads SQLite, serves REST API and cost reports

> Interactive diagram (theme toggle + PNG/SVG export): [`docs/architecture.html`](docs/architecture.html)
>
> ASCII sketch:
>
> ```
> Claude Code hooks ──→ tm emit ──→ Unix socket ─┐
> Claude JSONL logs ──→ ClaudeLogWatcher ───────────┤
> Codex  JSONL logs ──→ CodexWatcher ───────────────┘
>                                                    ▼
>                                              tm daemon
>                                                    │
>                                          SQLite (~/.tokenmeter/data/tokenmeter.db)
>                                                    │
>                                    tm daily/session/blocks  ◄─────┴─────►  tm web
> ```

## Data Storage

```
~/.tokenmeter/
├── data/tokenmeter.db    # SQLite database
├── tokenmeter.sock       # Unix domain socket
└── daemon.pid       # PID lock file
```

When upgrading from the old name, TokenMeter continues to read `~/.agmon/` if it exists and `~/.tokenmeter/` has not been created yet, so existing history remains available.

## Uninstall

```bash
tm uninstall        # remove hooks, stop daemon
rm -rf ~/.tokenmeter        # remove all data
```

## License

[MIT](LICENSE)
