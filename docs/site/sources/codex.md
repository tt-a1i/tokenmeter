# Codex Source

Codex integration is JSONL-log based.

Codex does not provide the same hook interface as Claude Code.

TokenMeter watches local Codex session files instead.

The watcher turns Codex records into the same unified event model used by the daemon.

That keeps downstream storage, reports, and web views consistent.

## Data Locations

The default Codex home is `~/.codex`.

TokenMeter watches session files under `sessions`.

It also considers archived session locations when configured by the watcher.

The pricing code reads `CODEX_HOME` when resolving Codex config for speed tier detection.

If `CODEX_HOME` is unset, it falls back to `~/.codex`.

## What It Captures

Codex JSONL records can include session metadata.

They can include response items.

They can include token count event messages.

Function calls become tool start events.

Function call outputs become tool end events.

Token count events become token usage rows.

Session metadata can provide working directory and model context.

The watcher keeps in-memory offsets per file.

It can defer empty session starts until useful data appears.

It garbage-collects orphaned pending file changes after a TTL.

## Cost And Speed Tier

Codex cost depends on service tier.

TokenMeter supports `--speed auto`, `--speed standard`, and `--speed fast`.

The default is `auto`.

Auto mode reads Codex `config.toml` and checks `service_tier`.

Values containing `fast` or `priority` select fast pricing.

Other values fall back to standard pricing.

Use explicit speed when you want stable report behavior:

```bash
tm daily --speed standard
tm daily --speed fast
```

## Common Commands

Run all-source daily reporting:

```bash
tm daily
```

Run SQLite-only reporting:

```bash
tm daily --no-scan
```

Run session reporting:

```bash
tm session
```

Open visual details:

```bash
tm web
```

## Troubleshooting

Check that Codex has written local JSONL files.

Check `CODEX_HOME` if your sessions are not under `~/.codex`.

Use `--since` and `--until` to narrow large directories.

Use `--json` to inspect raw model and token fields.

Use `tm doctor` for database and daemon health.
