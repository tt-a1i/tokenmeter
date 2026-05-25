# Amp Source

Amp is a batch-only TokenMeter source.

Use it when you want usage reports for local Amp conversations.

The adapter reads Amp's local thread files.

It does not call an Amp cloud API.

It runs when all-source reports scan installed agents.

It can also be queried directly through `tm amp`.

## What is Amp

Amp is an AI coding agent with local conversation storage.

It writes thread data under its user data directory.

Those files can include model names, token counts, timestamps, and message identity.

TokenMeter treats those records as historical usage evidence.

Amp is not daemon-backed in TokenMeter.

There are no TokenMeter hooks for Amp.

Fresh rows appear after the next report scan.

Use the source command when debugging Amp alone.

## Data Location

Default root:

```text
~/.local/share/amp
```

Environment override:

```bash
AMP_DATA_DIR=/path/to/amp tm amp daily
```

The adapter scans:

```text
<root>/threads/*.json
```

Set the environment variable when Amp is installed in a non-standard home.

Use an absolute path in scripts.

Keep the root at the directory that contains `threads`.

Do not point it at a single thread file.

## File Format

Amp usage is read from JSON files.

The expected shape is one thread per file.

TokenMeter extracts message-level usage when present.

It uses thread and message identifiers for stable source ids.

It uses timestamps from the thread payload.

It maps model strings directly when Amp records them.

Recorded costs are preserved only when the source data contains them.

Otherwise TokenMeter can calculate cost from model pricing.

## TokenMeter 实现细节

Collector:

```text
internal/collector/amp.go
```

The collector is batch-only.

It participates in `tm daily`, `tm weekly`, `tm monthly`, and `tm session`.

It also backs direct commands under `tm amp`.

It reads local JSON files synchronously during report generation.

It deduplicates rows by source identity.

It does not watch the Amp directory continuously.

It does not infer tokens from prose length.

If a thread lacks token fields, that thread may not contribute token totals.

If a thread lacks model fields, model breakdown rows may show an empty or unknown model.

## Examples

Daily Amp totals:

```bash
tm amp daily
```

Amp sessions as JSON:

```bash
tm amp session --json
```

Only recent Amp usage:

```bash
tm amp daily --since 20260501
```

All sources including Amp:

```bash
tm daily
```

Debug a custom root:

```bash
AMP_DATA_DIR=/tmp/amp-fixture tm amp session --json
```

## Troubleshooting

If no rows appear, confirm the root contains `threads`.

If no files are found, check `AMP_DATA_DIR`.

If dates look wrong, pass `--timezone`.

If costs are zero, try `--mode calculate`.

If pricing should not refresh, pass `--offline`.

If model names are empty, inspect the original thread JSON.

If duplicate rows appear in exports, report the source ids.

If the scan is slow, narrow it with `--since`.

If you only want daemon-backed TokenMeter rows, run `tm daily --no-scan`.

Use `tm amp session --json --jq '.rows[0]'` to inspect the first parsed row.
