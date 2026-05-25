# Droid Source

Droid is a batch-only TokenMeter source.

Use it for usage stored by Factory Droid sessions.

The adapter reads local session metadata and companion logs.

It does not require a running Droid process.

It does not call a remote billing API.

It runs during all-source scans and through `tm droid`.

## What is Droid

Droid is an AI development agent that stores session artifacts on disk.

The local session directory includes structured metadata.

Recent TokenMeter support also reads sidecar JSONL when present.

That sidecar helps recover model information.

It is especially useful when the main session metadata leaves the model blank.

Droid is batch-only in TokenMeter.

There are no Droid hooks.

Reports refresh after each scan.

## Data Location

Default root:

```text
~/.factory/sessions
```

Environment override:

```bash
DROID_SESSIONS_DIR=/path/to/sessions tm droid daily
```

The override may contain comma-separated roots.

Each root should contain Droid session directories or files.

The adapter reads session settings files.

It also looks for sidecar JSONL session material.

Keep the root at the sessions directory.

Do not point it at `~/.factory` unless your layout requires that.

## File Format

Droid records are a mix of structured local files.

Session metadata commonly appears as JSON settings files.

Sidecar records appear as JSONL.

The JSONL sidecar can carry model names.

TokenMeter uses that sidecar as a model fallback.

That means a usage row can still have a model when the primary session metadata omits it.

Token counts are read from fields Droid stores locally.

TokenMeter does not count raw transcript characters.

## TokenMeter 实现细节

Collector:

```text
internal/collector/droid.go
```

The collector is batch-only.

It is available through the `droid` adapter command.

It reads local files each time a report scans sources.

It honors `DROID_SESSIONS_DIR`.

It supports multiple session roots.

It handles missing model fields by checking sidecar JSONL.

That fallback aligns Droid with the newer OpenCode and ccusage-style data coverage.

Key limitation: source data must contain token accounting.

Key limitation: incomplete session folders may be skipped.

Key limitation: sidecar model fallback improves labels but cannot create missing token counts.

## Examples

Daily Droid totals:

```bash
tm droid daily
```

Droid sessions:

```bash
tm droid session
```

JSON output for automation:

```bash
tm droid session --json
```

Custom session root:

```bash
DROID_SESSIONS_DIR=/tmp/droid-sessions tm droid daily
```

Multiple roots:

```bash
DROID_SESSIONS_DIR=/old/droid,/new/droid tm droid monthly
```

## Troubleshooting

If models are blank, confirm sidecar JSONL files exist beside the session data.

If rows are missing, confirm the directory is `~/.factory/sessions`.

If a custom path is used, export `DROID_SESSIONS_DIR`.

If only some sessions appear, check for partial or interrupted Droid runs.

If token totals are zero, inspect the source files for token fields.

If costs are zero but tokens exist, use `--mode calculate`.

If a report is too broad, add `--since`.

If you need one active window, use `tm blocks --active` instead of source session output.

If you want to prove the row came from Droid, run `tm droid daily --json`.

Use `--jq` to inspect source-specific rows.
