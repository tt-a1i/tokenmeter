# Source Index

TokenMeter can report usage from the daemon-backed SQLite database and from batch-only source adapters.

Claude Code and Codex are the primary real-time sources.

The other sources are scanned when report commands run.

`tm daily` scans all installed supported sources by default.

Use `--no-scan` to restrict reports to the TokenMeter SQLite database.

Use source-specific commands when debugging one adapter.

Example:

```bash
tm opencode daily
tm gemini session
tm qwen monthly --json
```

## Supported Sources

| Source | Command | Shape | Summary |
| --- | --- | --- | --- |
| [Claude Code](./claude.md) | built-in SQLite path | JSONL + SQLite | Hook events plus `~/.claude/projects/` transcripts for Claude Code usage. |
| [Codex](./codex.md) | built-in SQLite path | JSONL + SQLite | Local Codex session files and TokenMeter daemon rows. |
| [OpenCode](./opencode.md) | `tm opencode` | SQLite + JSON | `OPENCODE_DATA_DIR` or `~/.local/share/opencode`, including `opencode.db`. |
| [Amp](./amp.md) | `tm amp` | JSON | `AMP_DATA_DIR` or `~/.local/share/amp/threads/*.json`. |
| [Gemini CLI](./gemini.md) | `tm gemini` | JSON / JSONL | `GEMINI_DATA_DIR` or `~/.gemini/tmp` local Gemini usage files. |
| [GitHub Copilot CLI](./copilot.md) | `tm copilot` | OTEL JSONL | `COPILOT_OTEL_FILE_EXPORTER_PATH` points at the exported telemetry file. |
| [Goose](./goose.md) | `tm goose` | SQLite | `GOOSE_PATH_ROOT` or standard Goose roots containing `sessions.db`. |
| [Codebuff](./codebuff.md) | `tm codebuff` | JSON | `CODEBUFF_DATA_DIR` or channel roots with `chat-messages.json`. |
| [Hermes Agent](./hermes.md) | `tm hermes` | SQLite | `HERMES_HOME` or `~/.hermes` state database. |
| [Kilo](./kilo.md) | `tm kilo` | SQLite | `KILO_DATA_DIR` or `~/.local/share/kilo/kilo.db`. |
| [Kimi](./kimi.md) | `tm kimi` | JSONL + JSON | `KIMI_DATA_DIR` or `~/.kimi/sessions/**/wire.jsonl`, plus optional config. |
| [OpenClaw](./openclaw.md) | `tm openclaw` | JSONL | `OPENCLAW_DIR` or compatible roots with normal, deleted, and reset JSONL files. |
| [pi-agent](./pi.md) | `tm pi` | JSONL | `PI_AGENT_DIR` or `~/.pi/agent/sessions/**/*.jsonl`. |
| [Droid](./droid.md) | `tm droid` | JSON + JSONL | `DROID_SESSIONS_DIR` or `~/.factory/sessions`, including sidecar JSONL model fallback. |
| [Qwen](./qwen.md) | `tm qwen` | JSONL | `QWEN_DATA_DIR` or `~/.qwen/projects/*/chats/*.jsonl`. |

## What A Source Must Provide

TokenMeter only reports usage records it can read locally.

A useful source record needs a timestamp.

It needs a session identity.

It should include model identity when available.

It should include token counts or recorded cost.

TokenMeter does not call private cloud billing APIs.

It does not infer token counts from raw transcript length.

When a source carries token counts but not costs, TokenMeter estimates cost from pricing data.

When a source carries a positive recorded cost, the adapter can preserve it.

## Debugging Source Coverage

Start with `tm daily --json`.

Check whether the source appears in the returned rows.

Then run the source-specific command.

Use environment variables to point at a fixture or alternate data directory.

Use `--since` and `--until` to reduce the scan window.

Use `--no-scan` to prove whether the row came from SQLite or a batch adapter.

If a source has multiple local roots, prefer comma-separated env vars where the adapter supports them.

## Pages In This Section

Every supported source has a dedicated page.

Start with the page for the adapter you are debugging.

Each page lists the default data location.

Each page lists the environment override.

Each page lists the file format.

Each page points to the TokenMeter collector implementation.

Each page includes command examples.

Each page includes troubleshooting notes.

Use the shape column above to identify whether a source is file-based, SQLite-based, or OTEL-based.
