# Getting Started

This guide gets TokenMeter from zero to a useful daily report in about five minutes.

It assumes you already have the `tm` binary available on your PATH.

If you do not, start with the installation page first.

TokenMeter is local-first.

It writes its own data under `~/.tokenmeter/`.

It can also read a legacy `~/.agmon/` directory if that exists and `~/.tokenmeter/` does not.

## 1. Register Claude Hooks

Run setup once:

```bash
tm setup
```

This updates `~/.claude/settings.json`.

The setup command adds TokenMeter hook commands for Claude Code events.

The hook command is `tm emit`.

Claude Code sends JSON to that command on stdin.

`tm emit` forwards the event to the local daemon socket.

If setup reports invalid JSON in the Claude settings file, fix that file first.

TokenMeter will not overwrite a malformed settings file.

## 2. Start The Daemon

Start the daemon in a foreground terminal:

```bash
tm daemon
```

The daemon listens on the TokenMeter socket.

It receives hook events.

It writes sessions, agents, tool calls, token rows, file changes, budgets, and cache tables into SQLite.

It also runs log watchers for Claude and Codex when started by the web mode.

For a first manual check, foreground mode keeps errors visible.

For day-to-day use, your process manager can keep it running.

## 3. Use Your Agent Normally

Open Claude Code or Codex and work as usual.

Claude Code contributes hook events and JSONL token logs.

Codex contributes JSONL session logs.

Batch-only sources are scanned when report commands run.

You do not need to keep a terminal UI open.

The v1 CLI is report-oriented.

## 4. Print The Daily Report

Run:

```bash
tm daily
```

Or use the default command:

```bash
tm
```

Both paths print a daily token and cost summary.

By default, `tm daily` scans all installed supported sources.

Use `--no-scan` to read only the TokenMeter SQLite database.

Use `--json` when piping into another program.

Use `--jq '.totals'` when you want JSON filtered through `jq`.

## 5. Open The Dashboard

Run:

```bash
tm web
```

The web server uses an embedded SPA.

It exposes REST APIs and dashboard views.

It can show cost trends, heatmaps, model breakdowns, tools, sessions, budgets, and exports.

Use `tm web --port 9000` if port `8370` is already taken.

## 6. Watch Live Events

Run:

```bash
tm watch
```

`tm watch` subscribes to daemon events.

Use `--types tool,token` to focus the stream.

Use `--session abc123` to narrow to one session prefix.

## Next Steps

Read the CLI reference for every command.

Read the source pages if one agent does not appear in reports.

Read budgets and webhooks when you want cost guardrails.
