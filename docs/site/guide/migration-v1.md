# Migrating From v0.x To v1.x

TokenMeter v1 changed the default interaction model.

The v0.x interactive terminal UI is gone.

The v1 command surface is CLI-first.

The Web Dashboard remains available through `tm web`.

The migration is mostly about replacing muscle memory and scripts.

## The Main Change

Before v1, running `tm` entered a terminal UI.

In v1, running `tm` is equivalent to `tm daily`.

That means it prints a report and exits.

It does not open tabs.

It does not capture keyboard navigation.

It does not auto-present Dashboard, Messages, Tool Calls, or Stats views.

## Common Replacements

| v0.x habit | v1.x replacement |
| --- | --- |
| `tm` for the overview | `tm daily` or just `tm` |
| Dashboard tab | `tm daily`, `tm weekly`, `tm monthly`, or `tm web` |
| Messages tab | `tm web` session detail |
| Tool Calls tab | `tm web`, `tm search`, or `tm watch --types tool` |
| Stats tab | `tm analyze` or `tm web` |
| Live terminal view | `tm watch` |
| Active cost view | `tm blocks --active` |

## Script Migration

Replace `tm cost` with `tm daily`.

Replace weekly reports with `tm weekly`.

Replace monthly reports with `tm monthly`.

Replace session text reports with `tm session`.

Replace `tm status` or `tm top` with `tm blocks --active`.

Use `--json` for machine-readable output.

Use `--jq` when the script needs a filtered field.

## Web Dashboard Migration

The Web Dashboard is the closest replacement for rich browsing.

Run:

```bash
tm web
```

Use it for charts, session detail, search, budgets, and exports.

It is a separate HTTP process.

It reads the same local SQLite database.

It does not require the old terminal UI.

## Data Migration

The local database remains under `~/.tokenmeter/data/tokenmeter.db`.

Legacy installs may keep reading `~/.agmon/` until `~/.tokenmeter/` exists.

The migration does not require deleting old data.

Back up before major version upgrades if the data matters.

Use:

```bash
tm backup
```

## More Detail

The canonical migration table lives in the existing document:

[Migrating to TokenMeter v1.0](../../MIGRATION-v1.0.md)

Use that page when updating older shell snippets.

This VitePress page gives the high-level model.
