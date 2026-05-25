# Environment Variables

TokenMeter is mostly configured through files and CLI flags.

Environment variables are still important for source discovery and offline operation.

This page lists the variables that affect reports.

It also notes variables that users often ask about but that are not currently runtime knobs.

## TokenMeter Variables

| Variable | Purpose |
| --- | --- |
| `TOKENMETER_OFFLINE` | When truthy, runtime LiteLLM pricing refresh is disabled. |
| `HOME` | Used to resolve `~/.tokenmeter`, `~/.agmon`, Claude logs, Codex logs, and default source roots. |
| `HOMEDRIVE` / `HOMEPATH` | Windows home resolution through Go's user home logic. |

`TOKENMETER_OFFLINE` is also effectively covered by the CLI `--offline` flag for report commands.

The runtime pricing cache is stored in the TokenMeter app directory.

There is no documented `TM_DATABASE` override in the current command path.

The database path comes from `internal/storage.DefaultDBPath()`.

That resolves through the app directory helper.

By default, the database is `~/.tokenmeter/data/tokenmeter.db`.

Legacy installs may use `~/.agmon/data/agmon.db`.

## Source Variables

| Variable | Source | Notes |
| --- | --- | --- |
| `OPENCODE_DATA_DIR` | OpenCode | Comma-separated roots; supports SQLite and JSON stores. |
| `AMP_DATA_DIR` | Amp | Overrides Amp data root. |
| `GEMINI_DATA_DIR` | Gemini CLI | Overrides Gemini data root. |
| `COPILOT_OTEL_FILE_EXPORTER_PATH` | GitHub Copilot CLI | Points at the OTEL JSONL export file. |
| `CODEBUFF_DATA_DIR` | Codebuff | Overrides Codebuff data root. |
| `HERMES_HOME` | Hermes Agent | Comma-separated homes are supported. |
| `KILO_DATA_DIR` | Kilo | Comma-separated roots are supported. |
| `KIMI_DATA_DIR` | Kimi | Comma-separated roots are supported. |
| `OPENCLAW_DIR` | OpenClaw | Overrides OpenClaw root. |
| `PI_AGENT_DIR` | pi-agent | Overrides pi-agent sessions root. |
| `DROID_SESSIONS_DIR` | Droid | Comma-separated session roots are supported. |
| `QWEN_DATA_DIR` | Qwen | Overrides Qwen data root. |
| `GOOSE_PATH_ROOT` | Goose | Overrides Goose data root. |
| `CODEX_HOME` | Codex pricing/config | Used for Codex `config.toml` speed tier detection. |

## Offline Pricing

Use the flag:

```bash
tm daily --offline
```

Or set:

```bash
TOKENMETER_OFFLINE=1 tm daily
```

Offline mode skips online pricing refresh.

Embedded pricing and any existing cache remain usable.

This is useful in CI, air-gapped machines, or deterministic tests.

## Date And Project Filters

Date and project filters are CLI flags, not environment variables.

Use `--since YYYYMMDD`.

Use `--until YYYYMMDD`.

Use `--timezone Asia/Shanghai`.

Use `--project /path/to/workspace`.

## TODO

Future documentation should add per-source examples for every adapter.

It should also document any new database path override if one is added.

Until then, avoid relying on undocumented env vars.
