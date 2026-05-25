# CLI Reference

TokenMeter v1 uses a ccusage-style command surface.

Running `tm` without arguments routes to `daily`.

Shared flags are parsed before command dispatch.

Some commands read the TokenMeter SQLite database.

Some commands scan batch-only agent sources.

Some commands manage local configuration files.

## Usage Commands

| Command | Purpose |
| --- | --- |
| `tm` | Default daily token and cost summary. |
| `tm daily` | Daily token and cost buckets. |
| `tm daily --compact` | Daily summary with compact columns for narrow terminals. |
| `tm daily --instances --project-aliases JSON` | Expand project instances and normalize workspace paths under aliases. |
| `tm weekly` | ISO-week token and cost buckets. |
| `tm monthly` | Monthly token and cost buckets. |
| `tm session [id]` | Per-session breakdown, optionally filtered by id prefix. |
| `tm blocks [--active] [--token-limit N\|max]` | Five-hour session blocks with burn rate, projection, and token-limit progress. |
| `tm statusline` | Claude Code statusline provider; reads stdin JSON and writes one line. |
| `tm share [session]` | Markdown session recap for sharing or handoff. |

## Run Modes

| Command | Purpose |
| --- | --- |
| `tm daemon` | Run the daemon in the foreground. |
| `tm web [--port N]` | Start the local Web Dashboard. |
| `tm watch [opts]` | Stream live daemon events to stdout. |

## Analysis Commands

| Command | Purpose |
| --- | --- |
| `tm analyze` | Usage insights and activity analysis. |
| `tm analyze --tool-errors` | Tool failure pattern analysis grouped by tool, error fragment, and session. |
| `tm search <query>` | Search tool calls and file paths. |
| `tm compare <a> <b>` | Compare two sessions. |
| `tm export [opts]` | CSV or JSON export. |

## Maintenance Commands

| Command | Purpose |
| --- | --- |
| `tm clean [days]` | Remove old sessions. |
| `tm compact [--full]` | Run SQLite optimize or full VACUUM. |
| `tm checkpoint` | Run an immediate WAL checkpoint. |
| `tm backup [path]` | Snapshot the database. |
| `tm restore <path>` | Restore from a snapshot. |
| `tm reload` | Ask the daemon to reload configuration. |
| `tm pricing refresh [--offline]` | Refresh the LiteLLM runtime pricing cache, or use fallback-only mode when offline. |
| `tm logs [--follow]` | Print daemon or hook logs. |
| `tm healthcheck [--json]` | Check database and daemon liveness. |
| `tm emit` | Hook-facing event receiver; normally not run manually. |

## Configuration Commands

| Command | Purpose |
| --- | --- |
| `tm setup` | Configure Claude Code hooks. |
| `tm init` | Interactive setup wizard. |
| `tm doctor [--fix]` | Diagnose and optionally repair local setup. |
| `tm completion <shell>` | Generate shell completion. |
| `tm update` | Download and install the latest release. |
| `tm version [--check]` | Print version and optionally check for updates. |
| `tm tag <id> [text]` | Set or clear a session note. |
| `tm config <show\|path\|init>` | Show, locate, or initialize unified config. |
| `tm budget <subcommand>` | Manage monthly budgets. |
| `tm webhook <subcommand>` | Manage webhook endpoints. |

## Source Commands

Each batch adapter can be used as a top-level command.

The accepted adapter commands are `amp`, `opencode`, `gemini`, `copilot`, `goose`, `codebuff`, `hermes`, `kilo`, `kimi`, `openclaw`, `pi`, `droid`, and `qwen`.

Each adapter supports daily, weekly, monthly, and session buckets through the shared parser.

If no bucket is supplied, the adapter defaults to daily.

Example:

```bash
tm opencode daily
tm gemini session --json
```

## Shared Flags

| Flag | Meaning |
| --- | --- |
| `--since YYYYMMDD` | Start date. |
| `--until YYYYMMDD` | End date. |
| `--json` | Emit JSON instead of a table. |
| `--mode auto|calculate|display` | Cost mode. |
| `--speed auto|standard|fast` | Codex pricing speed tier. |
| `--order asc|desc` | Sort order. |
| `--breakdown` | Include model breakdown rows. |
| `--offline` | Skip online pricing refresh. |
| `--timezone ZONE` | Date bucketing timezone. |
| `--project PATH` | Filter by workspace path. |
| `--instances` | Show project instances instead of only canonical project names. |
| `--project-aliases JSON\|PATH` | Normalize workspace paths with inline JSON or a JSON file. |
| `--no-color` | Disable color. |
| `--compact` | Use compact table layout. |
| `--jq EXPR` | Post-filter JSON through jq. |
| `--config PATH` | Config file path placeholder for commands that support it. |
| `--session-length 5h` | Session block duration. |
| `--active` | Blocks: show only active window. |
| `--token-limit N\|max` | Blocks: annotate usage against a token limit. |
| `--context-low-threshold PCT` | Statusline: context warning threshold percent. |
| `--context-medium-threshold PCT` | Statusline: context danger threshold percent. |
| `--burn-rate-display MODE` | Statusline: burn-rate display, one of `off`, `emoji`, `text`, `emoji-text`. |
| `--no-scan` | Skip batch adapter scans and use only SQLite. |

## Deprecated Aliases

`cost`, `report`, `status`, and `top` remain as deprecated aliases.

They are retained for compatibility until v2.0.

Prefer `daily`, `session`, `weekly`, `monthly`, and `blocks --active` in new scripts.
