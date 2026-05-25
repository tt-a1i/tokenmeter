# Goose Source

Goose is a batch-only TokenMeter source.

Use it for local Goose session accounting.

The adapter reads Goose SQLite databases.

It supports several common Goose data roots.

It does not require Goose to be running.

It can be queried directly with `tm goose`.

## What is Goose

Goose is an AI agent for local development workflows.

It stores sessions in a local database.

Different installs and forks can place that database in different user data directories.

TokenMeter searches standard roots and honors an override.

Goose is not hook-backed in TokenMeter.

Reports are refreshed by scanning the database.

Use direct commands to validate database discovery.

Use all-source reports for normal usage accounting.

## Data Location

Primary environment override:

```bash
GOOSE_PATH_ROOT=/path/to/goose tm goose daily
```

Common database name:

```text
sessions.db
```

Common candidate paths include:

```text
$GOOSE_PATH_ROOT/data/sessions/sessions.db
~/.local/share/goose/data/sessions/sessions.db
~/Library/Application Support/goose/data/sessions/sessions.db
```

Some Block fork installs use a similar Goose-compatible layout.

TokenMeter checks known candidates.

Set `GOOSE_PATH_ROOT` when auto-discovery misses your install.

## File Format

Goose usage is read from SQLite.

The database stores session rows.

The adapter maps those rows into TokenMeter usage records.

Timestamps come from database fields.

Session identity comes from Goose session identifiers.

Model identity is preserved when present.

Token counts are read from Goose usage columns.

Recorded costs can be used when the database contains them.

## TokenMeter 实现细节

Collector:

```text
internal/collector/goose.go
```

The collector is batch-only.

It backs `tm goose`.

It searches default Goose roots.

It honors `GOOSE_PATH_ROOT`.

It opens `sessions.db` read-only.

It does not mutate Goose state.

It does not run Goose migrations.

It does not use cloud billing endpoints.

Key limitation: unsupported Goose schema versions may need adapter updates.

Key limitation: missing token columns cannot be reconstructed.

Key limitation: cost calculation requires recognizable model ids.

## Examples

Daily Goose totals:

```bash
tm goose daily
```

Goose session rows:

```bash
tm goose session --json
```

Custom root:

```bash
GOOSE_PATH_ROOT=/tmp/goose tm goose daily
```

Monthly Goose usage:

```bash
tm goose monthly --breakdown
```

All-source report:

```bash
tm daily
```

## Troubleshooting

If no rows appear, locate `sessions.db`.

If auto-discovery misses it, set `GOOSE_PATH_ROOT`.

If the database is locked, close Goose and retry.

If schema errors appear, note the Goose version.

If models are empty, inspect the database row.

If costs are zero, try `--mode calculate`.

If totals are too broad, narrow with `--since`.

If you maintain multiple Goose profiles, run one root at a time.

If the report must not refresh pricing, pass `--offline`.

Use `tm goose session --json --jq '.rows[0]'` for quick inspection.
