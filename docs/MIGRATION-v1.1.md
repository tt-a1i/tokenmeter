# Migrating to TokenMeter v1.1

## TL;DR

v1.1 ships 13 new data source adapters. The only behavior change for existing
users is **`tm daily` (no source) now scans batch adapters by default**. If
your scripts depend on `tm daily` returning only Claude + Codex data, add
`--no-scan` to restore v1.0.x behavior.

## New subcommands

Each of these 13 sources now has full `daily / weekly / monthly / session`
bucket support, identical to `tm claude daily` and `tm codex daily`:

| Source | Subcommand prefix | Log location | Env override |
|---|---|---|---|
| OpenCode | `tm opencode` | `~/.local/share/opencode/` | `OPENCODE_DATA_DIR` |
| Amp | `tm amp` | `~/.local/share/amp/threads/*.json` | `AMP_DATA_DIR` |
| Gemini CLI | `tm gemini` | `~/.gemini/tmp/chats/*` | `GEMINI_DATA_DIR` |
| Copilot CLI | `tm copilot` | `~/.copilot/otel/*.jsonl` | `COPILOT_OTEL_FILE_EXPORTER_PATH` |
| Goose (SQLite) | `tm goose` | `~/.local/share/goose/sessions/<db>` | `GOOSE_PATH_ROOT` |
| Codebuff | `tm codebuff` | `~/.config/<channel>/projects/...` | `CODEBUFF_DATA_DIR` |
| Hermes (SQLite) | `tm hermes` | `~/.hermes/state.db` | `HERMES_HOME` |
| Kilo (SQLite) | `tm kilo` | `~/.local/share/kilo/<db>` | `KILO_DATA_DIR` |
| Kimi | `tm kimi` | `~/.kimi/sessions/...` | `KIMI_DATA_DIR` |
| OpenClaw | `tm openclaw` | `~/.openclaw/`, `~/.clawdbot/`, `~/.moltbot/`, `~/.moldbot/` | `OPENCLAW_DIR` |
| pi-agent | `tm pi` | `~/.pi/agent/sessions/...` | `PI_AGENT_DIR` |
| Droid | `tm droid` | `~/.factory/sessions/*.settings.json` | `DROID_SESSIONS_DIR` |
| Qwen | `tm qwen` | `~/.qwen/projects/<project>/chats/*.jsonl` | `QWEN_DATA_DIR` |

## `tm daily` behavior change

**v1.0.x**: `tm daily` queries only SQLite (Claude + Codex).
**v1.1.0**: `tm daily` queries SQLite *and* scans every batch adapter listed
above. Adapters whose log directory does not exist are silently skipped
(`< 5 ms` overhead per missing source).

If you have a script that expects only Claude + Codex rows, use one of:
- `tm daily --no-scan` — explicitly skip batch adapters
- `tm claude daily` and `tm codex daily` — query each source individually

## Data is not migrated into SQLite

Batch adapter data is *not* written into TokenMeter's SQLite store. Each
`tm <source> daily` invocation re-scans the source's logs. This matches
ccusage's stateless model. The daemon, SQLite schema, and Claude / Codex
collection paths are unchanged from v1.0.x.

## Project filter (`--project`)

`--project <path>` matches `sessions.cwd` for Claude / Codex. Most batch
adapters cannot map workspace paths reliably from their logs and silently
ignore `--project`. The matrix below records what each source actually
honors:

| Source | `--project` filter | Notes |
|---|---|---|
| Claude | yes | exact match against `sessions.cwd` |
| Codex | yes | exact match against `sessions.cwd` |
| OpenCode | yes | per-project subdir layout |
| Codebuff | yes | per-project subdir layout |
| Qwen | yes | per-project subdir layout (strict `projects/<project>/chats/`) |
| Amp | no | threads have no workspace metadata |
| Gemini CLI | no | session logs have no workspace path |
| Copilot CLI | no | OTEL spans carry interaction context, not workspace |
| Goose | no | SQLite store has no workspace column |
| Hermes | no | same |
| Kilo | no | same |
| Kimi | no | session logs are flat |
| OpenClaw | no | filtered out at the source level |
| pi-agent | no | session logs are flat |
| Droid | no | session settings.json carries no workspace |

## Known limitations / known differences vs ccusage upstream

These deltas accumulated during the 13-adapter implementation. They do not
break v1.1 itself but are worth knowing when comparing against ccusage's
own output:

- **Reasoning tokens fold into `OutputTokens`** for Gemini / Goose / Hermes /
  Kilo / Copilot / Qwen / Pi. UsageEntry has no dedicated extra-total slot;
  totals match ccusage but `outputTokens` is inflated by the reasoning count.
  Per-bucket / per-model breakdowns are unaffected.
- **OpenCode SQLite-only installs are not visible in v1.1**. The adapter
  reads the file-tree format only; a SQLite-only OpenCode store (newer
  install) needs the v1.1.1 follow-up. Workaround: pin OpenCode to a build
  that still writes the file tree.
- **Copilot OTEL cross-source dedup not implemented**. ccusage suppresses
  lower-priority OTEL spans (chat > inference > agent_turn > agent_summary)
  when they overlap by trace_id / response_id. v1.1 emits every
  `gen_ai.usage.*` row it finds, so production layouts with overlapping span
  types may double-count.
- **Codebuff `runState` fallback not implemented**. A rare branch ccusage
  uses when `metadata.usage` and `metadata.codebuff.usage` are both missing.
- **Droid sidecar `.jsonl` model fallback** is not yet wired. When a session
  JSON is missing the `model` field, the row is dropped (ccusage looks in
  adjacent jsonl for the inferred model first).
- **Costs are recomputed at the cli pricing layer**. Most batch adapters set
  `CostUSD = 0` and rely on `--mode auto / calculate` to reach
  `pricing.Resolve`. `--mode display` therefore reports `$0` for adapters
  with no native USD cost — switch to `--mode auto` (default) or
  `--mode calculate` to see priced output.
- **`pricing.Resolve` provider/model candidate fallback** is single-key
  only. ccusage tries `<model>` then `qwen/<model>` then `alibaba/<model>`
  etc.; v1.1 looks up the verbatim model name only. Track for v1.1.x.
