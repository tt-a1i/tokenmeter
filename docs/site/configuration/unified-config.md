# Unified Configuration

TokenMeter uses a unified JSON config file for settings that should survive across commands.

The default path is:

```text
~/.tokenmeter/config.json
```

You can inspect the effective path with:

```bash
tm config path
```

You can print the loaded config with:

```bash
tm config show
```

You can create an example file with:

```bash
tm config init
```

Use `--config PATH` when a command should read a specific config file.

The environment variable `TOKENMETER_CONFIG` can also point at a config file.

## Search Order

TokenMeter resolves config paths in this order:

1. Explicit `--config PATH`.
2. `TOKENMETER_CONFIG`.
3. Local `.tokenmeter/tokenmeter.json` in the current working directory.
4. Global `~/.tokenmeter/config.json`.

The first existing file wins.

If no file exists, commands use built-in defaults.

`tm config path` still prints the path that would be used.

Legacy installs may also have files under `~/.agmon`.

Do not move legacy files manually unless a migration command tells you to.

## Current Effective Scope (v1.x Phase 1)

The unified config schema is larger than the first runtime integration.

Current effective behavior:

| Area | Status | Notes |
| --- | --- | --- |
| Search path | ✅ Effective | `--config`, `TOKENMETER_CONFIG`, local config, then global config. |
| Legacy pricing merge | ✅ Effective | Missing unified pricing can fall back to legacy `pricing.json`. |
| Legacy webhook merge | ✅ Effective | Missing unified webhooks can fall back to legacy `webhooks.json`. |
| Pricing runtime sync | ✅ Effective | `pricing.runtimeSyncURL` and `pricing.cacheTTLHours` are read by runtime pricing. |
| Webhook endpoints | ✅ Effective | `webhooks.endpoints` and endpoint thresholds are part of the loaded model. |
| Statusline | ⏳ Partially effective | Unified `statusline` fields are being wired through the v1.x config follow-up. |
| `defaults` | ⏳ Not fully effective | Schema exists; broad CLI default application is part of the follow-up. |
| `commands.<name>` | ⏳ Not fully effective | Schema exists; per-command defaults are part of the follow-up. |
| `sources` | ⏳ Not effective | Schema exists as nested defaults/commands; source data-dir aliases are not current schema fields. |

For strict automation, prefer explicit CLI flags until the relevant section is marked effective.

## Schema Shape

The current Go schema is defined in `internal/config/config.go`.

Mirror the JSON tags from the structs, not field names guessed from CLI flags.

Top-level shape:

```json
{
  "$schema": "https://tokenmeter.dev/config-schema.json",
  "defaults": {},
  "commands": {},
  "pricing": {},
  "webhooks": {},
  "statusline": {},
  "sources": {}
}
```

All sections are optional.

Unknown keys may be ignored by the current loader.

Keep config files valid JSON.

## Defaults

`defaults` stores shared CLI-style defaults.

Current JSON tags:

```json
{
  "defaults": {
    "since": "20260501",
    "until": "20260525",
    "json": false,
    "offline": false,
    "timezone": "Asia/Shanghai",
    "mode": "auto",
    "order": "asc",
    "breakdown": false,
    "project": "/Users/admin/code/agmon",
    "noColor": false,
    "speed": "auto",
    "compact": true,
    "token_limit": "500000"
  }
}
```

Important spelling details:

`noColor` is camelCase.

`token_limit` is snake_case in the current struct tag.

There is no `sessionLength` field in the current `Defaults` struct.

There is no `session_length` field in the current `Defaults` struct.

Use `--session-length` explicitly until config support for that setting exists.

## Commands

`commands` maps a command name to the same `Defaults` shape.

Example:

```json
{
  "commands": {
    "daily": {
      "timezone": "Asia/Shanghai",
      "compact": true
    },
    "blocks": {
      "token_limit": "500000"
    }
  }
}
```

This section is intended for stable per-command preferences.

It should not replace one-off date filters in scripts.

CLI flags should remain the highest-precedence override.

## Pricing

`pricing` stores runtime pricing configuration and optional override rules.

Example:

```json
{
  "pricing": {
    "runtimeSyncURL": "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json",
    "cacheTTLHours": 24,
    "codex": [
      {
        "match": ["gpt-5.4"],
        "inputPerMillion": 2.5,
        "outputPerMillion": 15,
        "cacheCreatePerMill": 0,
        "cacheReadPerMill": 0.25,
        "fastMultiplier": 2
      }
    ]
  }
}
```

`runtimeSyncURL` is camelCase.

`cacheTTLHours` is camelCase.

Pricing rule fields use camelCase.

Rules with negative rates are invalid.

`match` is required for override rules.

## Webhooks

`webhooks` stores endpoint configuration.

Example:

```json
{
  "webhooks": {
    "endpoints": [
      {
        "url": "https://example.test/tokenmeter",
        "events": ["budget_warn", "session_high_cost"],
        "format": "json",
        "retry": {
          "max_attempts": 3,
          "initial_backoff_seconds": 5
        },
        "thresholds": {
          "session_high_cost_usd": 10,
          "tool_failure_rate_pct": 20
        }
      }
    ]
  }
}
```

Endpoint `retry` and `thresholds` fields currently use snake_case.

Keep webhook secrets out of this file when possible.

Use environment-level secret management for shared machines.

## Statusline

`statusline` stores statusline display preferences.

Example:

```json
{
  "statusline": {
    "quota_usd": 25,
    "format": "compact",
    "color": "true",
    "context_low_threshold": 50,
    "context_medium_threshold": 80,
    "burn_rate_display": "emoji-text"
  }
}
```

Statusline fields currently use snake_case.

`color` is a string in the current unified config model.

Accepted burn-rate display modes are:

```text
off
emoji
text
emoji-text
```

Keep the low context threshold below the medium context threshold.

## Sources

`sources` maps a source name to nested defaults and command overrides.

Current schema:

```json
{
  "sources": {
    "opencode": {
      "defaults": {
        "json": true
      },
      "commands": {
        "daily": {
          "since": "20260501"
        }
      }
    }
  }
}
```

This is not a data-directory schema.

Do not use `data_dir`, `sessions_dir`, or `otel_file` under `sources` unless a later release adds those fields.

For source discovery today, continue using the documented environment variables such as `OPENCODE_DATA_DIR`, `DROID_SESSIONS_DIR`, and `COPILOT_OTEL_FILE_EXPORTER_PATH`.

## Precedence

Expected precedence is:

1. Explicit CLI flags.
2. Environment variables for config path and source discovery.
3. Unified config.
4. Built-in defaults.

This keeps scripts predictable.

It also lets workstation users set durable defaults.

When debugging, print the command line first.

Then inspect relevant environment variables.

Then run `tm config path`.

Then run `tm config show`.

## Troubleshooting

If config seems ignored, confirm the current TokenMeter build supports the section you are using.

If JSON fails to parse, validate it with `jq`.

If `tm config show` is empty, check `tm config path`.

If pricing overrides do not apply, check `pricing` field names for camelCase.

If webhooks do not load, check endpoint nesting under `webhooks.endpoints`.

If source roots do not change, use source environment variables instead of `sources`.

If statusline settings do not appear, compare your installed version with the statusline config follow-up release.

If docs and behavior differ, prefer command help and release notes for the installed version.
