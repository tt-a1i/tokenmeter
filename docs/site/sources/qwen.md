# Qwen Source

Qwen is a batch-only TokenMeter source.

Use it for local Qwen CLI chat logs.

The adapter scans project chat JSONL files.

It does not require Qwen to be running.

It does not call a remote billing API.

It is available through all-source reports and `tm qwen`.

## What is Qwen

Qwen CLI is an AI coding command-line tool.

It stores project chat history under the Qwen data root.

Those chat files can contain token accounting.

TokenMeter reads the local JSONL rows and normalizes them.

Qwen is batch-only in TokenMeter.

New rows appear when a report scans sources.

Use the source command to validate one Qwen install.

Use all-source reports for normal accounting.

## Data Location

Default root:

```text
~/.qwen
```

Environment override:

```bash
QWEN_DATA_DIR=/path/to/qwen tm qwen daily
```

Expected path shape:

```text
<root>/projects/<project>/chats/<file>.jsonl
```

Point the override at the Qwen root.

Do not point it at a project subdirectory unless that is the root layout you maintain.

Use absolute paths in CI.

## File Format

Qwen usage is read from JSONL.

Each chat file contains line-delimited JSON events.

The parser extracts token counts from usage-bearing events.

The parser extracts timestamps from event fields.

Project identity can come from the path.

Session identity can come from the chat file and payload.

Model identity is preserved when present.

Malformed lines may be skipped.

## TokenMeter 实现细节

Collector:

```text
internal/collector/qwen.go
```

The collector is batch-only.

It backs `tm qwen`.

It honors `QWEN_DATA_DIR`.

It scans `projects/*/chats/*.jsonl`.

It uses local JSONL as source truth.

It does not watch Qwen files continuously.

It does not infer tokens from text.

It does not merge provider invoices.

Key limitation: rows without usage fields do not contribute totals.

Key limitation: model labels depend on Qwen event fields.

Key limitation: copied project directories can duplicate scans.

## Examples

Daily Qwen usage:

```bash
tm qwen daily
```

Qwen sessions:

```bash
tm qwen session
```

Custom root:

```bash
QWEN_DATA_DIR=/tmp/qwen tm qwen daily --json
```

Monthly Qwen report:

```bash
tm qwen monthly --since 20260501
```

Model breakdown:

```bash
tm qwen daily --breakdown
```

## Troubleshooting

If no rows appear, check `projects/*/chats/*.jsonl`.

If your data root is custom, set `QWEN_DATA_DIR`.

If rows are present but models are blank, inspect event model fields.

If token totals are zero, inspect usage fields.

If parsing fails, check for truncated JSONL lines.

If costs are missing, run with `--mode calculate`.

If the date window is wrong, pass `--timezone`.

If scanning is slow, use `--since`.

If duplicate rows appear, avoid scanning copied project roots.

Use `tm qwen session --json --jq '.rows[0]'` to inspect one row.
