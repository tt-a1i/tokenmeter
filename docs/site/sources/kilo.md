# Kilo Source

Kilo is a batch-only TokenMeter source.

Use it for local Kilo usage stored in SQLite.

The adapter reads Kilo data roots.

It supports comma-separated roots.

It does not require a live Kilo process.

It can be scanned through all-source reports or `tm kilo`.

## What is Kilo

Kilo is an AI coding assistant with local state.

Its usage data is stored in a local database.

TokenMeter reads that database and normalizes usage.

Kilo is historical from TokenMeter's point of view.

It does not emit TokenMeter hook events.

New rows are found when report commands scan sources.

Use `tm kilo` to isolate Kilo while debugging.

Use `tm daily` to include it in normal totals.

## Data Location

Default root:

```text
~/.local/share/kilo
```

Environment override:

```bash
KILO_DATA_DIR=/path/to/kilo tm kilo daily
```

The override may contain comma-separated roots.

Common database name:

```text
kilo.db
```

Point the override at the directory that owns the database.

Use one root per profile unless you intentionally combine profiles.

## File Format

Kilo usage is read from SQLite.

The adapter opens the local database read-only.

Usage rows are converted into TokenMeter rows.

Timestamps are preserved from the database.

Session identifiers are preserved when available.

Model identifiers are preserved when available.

Token counts come from Kilo usage columns.

Cost can be displayed or calculated depending on available fields.

## TokenMeter 实现细节

Collector:

```text
internal/collector/kilo.go
```

The collector is batch-only.

It backs `tm kilo`.

It honors `KILO_DATA_DIR`.

It supports multiple roots.

It reads SQLite rather than JSONL.

It does not write to Kilo's database.

It does not infer missing token counts.

It does not call a provider billing API.

Key limitation: schema changes may require collector updates.

Key limitation: rows without model ids can be counted but not priced accurately.

Key limitation: copied databases can duplicate rows if scanned together.

## Examples

Daily Kilo usage:

```bash
tm kilo daily
```

Kilo sessions:

```bash
tm kilo session
```

Custom root:

```bash
KILO_DATA_DIR=/tmp/kilo tm kilo daily --json
```

Multiple roots:

```bash
KILO_DATA_DIR=/work/kilo,/home/kilo tm kilo monthly
```

Breakdown by model:

```bash
tm kilo daily --breakdown
```

## Troubleshooting

If no rows appear, confirm `kilo.db` exists.

If a custom install is used, set `KILO_DATA_DIR`.

If the database is locked, close Kilo and retry.

If costs are missing, run with `--mode calculate`.

If models are blank, inspect the source database.

If rows look duplicated, avoid scanning copied roots together.

If the date window is too wide, add `--since`.

If pricing should be deterministic, add `--offline`.

If schema errors appear, capture the Kilo version.

Use `tm kilo session --json` for raw row inspection.
