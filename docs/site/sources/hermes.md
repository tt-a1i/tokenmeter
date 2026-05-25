# Hermes Source

Hermes is a batch-only TokenMeter source.

Use it for local Hermes Agent usage stored in its state database.

The adapter reads SQLite.

It does not use hooks.

It does not call a remote account API.

It can be scanned through all-source reports or queried with `tm hermes`.

## What is Hermes

Hermes Agent is an AI coding agent with local state.

Its local home contains a state database.

That database can include session, model, token, and cost information.

TokenMeter reads those rows and normalizes them into usage buckets.

Hermes is historical from TokenMeter's perspective.

Rows appear after the next scan.

Use direct commands when validating a Hermes install.

Use all-source reports for daily accounting.

## Data Location

Default root:

```text
~/.hermes
```

Environment override:

```bash
HERMES_HOME=/path/to/hermes tm hermes daily
```

The override may contain comma-separated homes.

The expected database is under the Hermes home.

Common name:

```text
state.db
```

Point `HERMES_HOME` at the directory that owns the database.

Do not point it at a copied SQL dump.

## File Format

Hermes usage is read from SQLite.

The adapter opens the local state database read-only.

Rows are converted into TokenMeter usage records.

Timestamps are taken from Hermes fields.

Session identity is taken from Hermes fields when present.

Model identity is preserved when present.

Token counts are mapped from Hermes usage columns.

Positive recorded costs can be displayed directly.

## TokenMeter 实现细节

Collector:

```text
internal/collector/hermes.go
```

The collector is batch-only.

It backs `tm hermes`.

It supports one or more homes through `HERMES_HOME`.

It reads SQLite rather than JSON files.

It uses the local Hermes database as source truth.

It does not modify the database.

It does not vacuum or migrate Hermes data.

It may skip rows whose usage fields are incomplete.

Key limitation: schema drift can break collection until TokenMeter adds support.

Key limitation: model breakdown depends on database model columns.

Key limitation: cost calculations require recognized model names.

## Examples

Daily Hermes usage:

```bash
tm hermes daily
```

Hermes sessions:

```bash
tm hermes session
```

Custom home:

```bash
HERMES_HOME=/tmp/hermes tm hermes daily --json
```

Multiple homes:

```bash
HERMES_HOME=/work/hermes,/personal/hermes tm hermes monthly
```

Model breakdown:

```bash
tm hermes daily --breakdown
```

## Troubleshooting

If no rows appear, confirm `state.db` exists.

If the database is locked, close Hermes and rerun.

If the home is custom, set `HERMES_HOME`.

If schema errors appear, capture the Hermes version and database table names.

If model names are blank, inspect the source database row.

If costs are zero, try `--mode calculate`.

If the report should be deterministic, use `--offline`.

If multiple homes duplicate data, scan only the active home.

If a date is unexpected, pass `--timezone`.

Use `tm hermes session --json --jq '.rows | length'` to verify row counts.
