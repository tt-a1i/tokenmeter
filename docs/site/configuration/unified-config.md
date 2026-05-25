# Unified Configuration

TokenMeter is moving toward a unified user configuration file.

This page documents the v1.x roadmap shape.

Some options may still be command-line only in current builds.

Use this page as the contract direction for future config work.

The goal is to reduce scattered flags and source-specific setup.

The proposed file is:

```text
~/.tokenmeter/config.json
```

Legacy installs may still have data under `~/.agmon`.

Do not move legacy files manually unless a migration command tells you to.

## Why Unified Config

TokenMeter has several configuration layers today.

CLI flags are explicit and script-friendly.

Environment variables are useful for source discovery.

Statusline has its own file.

Budgets and webhooks have their own stores.

A unified config gives users one place for stable defaults.

It also makes setup easier to document.

It should not remove CLI flags.

Flags should continue to override config for one command.

## Proposed Shape

The v1.x model has three levels.

`defaults` applies to all commands.

`commands` applies to a named command family.

`sources` applies to source adapters.

Example:

```json
{
  "defaults": {
    "timezone": "Asia/Shanghai",
    "mode": "auto",
    "offline": false,
    "no_color": false
  },
  "commands": {
    "blocks": {
      "session_length": "5h",
      "token_limit": 500000
    },
    "statusline": {
      "burn_rate_display": "emoji-text",
      "context_low_threshold": 0.5,
      "context_medium_threshold": 0.75
    }
  },
  "sources": {
    "opencode": {
      "data_dir": "~/.local/share/opencode"
    },
    "qwen": {
      "data_dir": "~/.qwen"
    }
  }
}
```

This is forward-looking documentation.

Check release notes before relying on every key.

## Defaults Level

`defaults.timezone` maps to `--timezone`.

`defaults.mode` maps to `--mode`.

`defaults.offline` maps to `--offline`.

`defaults.no_color` maps to `--no-color`.

Defaults should be conservative.

They should not surprise scripts.

They should not hide data.

They should be easy to override with flags.

For example:

```bash
tm daily --timezone UTC
```

should override a config timezone for that run.

## Commands Level

Command config stores stable preferences for command families.

`commands.blocks.session_length` maps to `--session-length`.

`commands.blocks.token_limit` maps to `--token-limit`.

`commands.statusline.burn_rate_display` controls statusline burn-rate display.

Valid burn-rate display modes are:

```text
off
emoji
text
emoji-text
```

Statusline context thresholds are ratios.

`0.5` means fifty percent.

Keep the low threshold below the medium threshold.

Command config should not replace report filters.

Date windows like `--since` and `--until` are usually better as command-line flags.

## Sources Level

Source config is intended to replace repeated environment exports.

Each key should match a source command name.

Examples:

```json
{
  "sources": {
    "amp": { "data_dir": "~/.local/share/amp" },
    "droid": { "sessions_dir": "~/.factory/sessions" },
    "copilot": { "otel_file": "~/.copilot/otel/usage.jsonl" }
  }
}
```

The source level should mirror existing environment variables.

For example, `sources.amp.data_dir` corresponds to `AMP_DATA_DIR`.

`sources.droid.sessions_dir` corresponds to `DROID_SESSIONS_DIR`.

`sources.copilot.otel_file` corresponds to `COPILOT_OTEL_FILE_EXPORTER_PATH`.

Environment variables should remain useful in CI.

Config should make daily workstation use simpler.

## Precedence

Expected precedence from highest to lowest:

1. Explicit CLI flags.
2. Environment variables.
3. Unified config.
4. Built-in defaults.

This keeps scripts predictable.

It keeps existing environment-based source discovery working.

It lets users define defaults without losing one-off overrides.

When debugging, print the command and relevant environment variables first.

Then inspect the config file.

## Validation

The config file should be valid JSON.

Unknown keys should be ignored or warned about.

Type errors should be actionable.

Paths should support `~` expansion.

Comma-separated roots may remain environment-only unless the config schema defines arrays.

For config arrays, prefer:

```json
{
  "sources": {
    "kilo": {
      "data_dirs": ["~/.local/share/kilo", "/Volumes/archive/kilo"]
    }
  }
}
```

Avoid storing secrets in this file.

TokenMeter source adapters read local usage files.

They should not need provider API keys for normal reports.

## Migration Notes

Keep existing environment variables for now.

Move only stable preferences into config.

Good candidates are timezone, color, offline mode, block length, and token limit.

Good source candidates are non-standard data roots.

Avoid putting transient date ranges into config.

Avoid putting one-off project filters into config.

When v1.x unified config lands, release notes should list supported keys.

Until then, treat this document as roadmap guidance.

## Troubleshooting

If config seems ignored, check whether the current build supports unified config.

If a flag behaves differently, remember flags should win.

If an environment variable wins, unset it and rerun.

If JSON fails to parse, validate the file with `jq`.

If a source path is wrong, run the source-specific command with `--json`.

If statusline ignores display settings, check `~/.tokenmeter/statusline.json` too.

If docs and behavior differ, prefer command help and release notes for the installed version.

Report mismatches with the TokenMeter version and config snippet.
