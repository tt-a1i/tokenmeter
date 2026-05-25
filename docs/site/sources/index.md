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

| Source | Command | Default data location or override |
| --- | --- | --- |
| Claude Code | built-in SQLite path | Hooks plus `~/.claude/projects/` JSONL logs. |
| Codex | built-in SQLite path | `~/.codex/sessions` and archived sessions. |
| OpenCode | `tm opencode` | `OPENCODE_DATA_DIR` or `~/.local/share/opencode`. |
| Amp | `tm amp` | `AMP_DATA_DIR` or `~/.local/share/amp`. |
| Gemini CLI | `tm gemini` | `GEMINI_DATA_DIR` or `~/.gemini/tmp`. |
| GitHub Copilot CLI | `tm copilot` | `COPILOT_OTEL_FILE_EXPORTER_PATH`. |
| Goose | `tm goose` | `GOOSE_PATH_ROOT` or standard Goose data roots. |
| Codebuff | `tm codebuff` | `CODEBUFF_DATA_DIR` or channel config roots. |
| Hermes Agent | `tm hermes` | `HERMES_HOME` or `~/.hermes`. |
| Kilo | `tm kilo` | `KILO_DATA_DIR` or `~/.local/share/kilo`. |
| Kimi | `tm kimi` | `KIMI_DATA_DIR` or `~/.kimi`. |
| OpenClaw | `tm openclaw` | `OPENCLAW_DIR` or OpenClaw-compatible roots. |
| pi-agent | `tm pi` | `PI_AGENT_DIR` or `~/.pi/agent/sessions`. |
| Droid | `tm droid` | `DROID_SESSIONS_DIR` or `~/.factory/sessions`. |
| Qwen | `tm qwen` | `QWEN_DATA_DIR` or `~/.qwen`. |

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

The first detailed source pages cover Claude Code, Codex, and OpenCode.

Additional source pages will be added incrementally.

For now, use this index as the supported source matrix.
