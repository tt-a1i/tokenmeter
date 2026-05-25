# OpenCode Source

OpenCode is a batch adapter source.

It is scanned when all-source reports run.

It can also be queried directly with `tm opencode`.

The current adapter supports both SQLite and JSON message stores.

That matters because modern OpenCode installs may keep usage data in `opencode.db`.

## Data Roots

Set `OPENCODE_DATA_DIR` to override root discovery.

The variable can contain a comma-separated list of roots.

If it is unset, TokenMeter uses `~/.local/share/opencode` when that directory exists.

Each root is scanned independently.

Duplicate messages are collapsed during one run.

## SQLite Support

The adapter looks for:

```text
<root>/opencode.db
<root>/opencode-<channel>.db
```

It reads the SQLite `message` table.

It parses message `data` JSON.

Database rows are loaded before JSON files.

When a database row and a JSON file represent the same message, the database row wins.

This aligns OpenCode handling with ccusage-style behavior.

It prevents under-counting installs that no longer write all usage into `storage/message`.

## JSON Store Support

The adapter also scans:

```text
<root>/storage/message/**/*.json
```

JSON files are read recursively.

The parser extracts token and model fields.

It uses message id when present for deduplication.

It synthesizes a fallback key when id is absent.

## Field Mapping

| OpenCode field | TokenMeter field |
| --- | --- |
| `tokens.input` | input tokens |
| `tokens.output` | output tokens |
| `tokens.cache.write` | cache creation tokens |
| `tokens.cache.read` | cache read tokens |
| `tokens.total` | fallback output tokens when parts are empty |
| `modelID` | model |
| `time.created` | timestamp |
| `sessionID` | session id |
| positive `cost` | recorded cost |

If recorded cost is zero or missing, all-source auto mode can re-price from model tokens.

## Commands

Run OpenCode daily:

```bash
tm opencode daily
```

Run OpenCode sessions:

```bash
tm opencode session --json
```

Run all sources, including OpenCode:

```bash
tm daily
```

## Troubleshooting

Point `OPENCODE_DATA_DIR` at the root that contains `opencode.db`.

Use comma-separated roots if you have multiple OpenCode profiles.

Check that the SQLite file contains a `message` table.

Check that JSON message files exist if you rely on file storage.

Use `--since` to limit scans during debugging.
