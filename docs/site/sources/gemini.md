# Gemini CLI Source

Gemini CLI is a batch-only TokenMeter source.

Use it for local Gemini CLI usage files.

The adapter scans JSON and JSONL files.

It does not depend on a running Gemini CLI process.

It does not call Google billing APIs.

It is available through all-source reports and `tm gemini`.

## What is Gemini CLI

Gemini CLI is a command-line assistant backed by Gemini models.

It writes local temporary or session usage data.

Those files can contain token counts and model names.

TokenMeter scans the local files and normalizes the usage.

Gemini is batch-only in TokenMeter.

New files are picked up when reports run.

Use `tm gemini` to inspect Gemini alone.

Use `tm daily` to include Gemini in combined totals.

## Data Location

Default root:

```text
~/.gemini/tmp
```

Environment override:

```bash
GEMINI_DATA_DIR=/path/to/gemini/tmp tm gemini daily
```

The adapter looks for local Gemini usage files.

Expected formats include:

```text
*.json
*.jsonl
```

Point the override at the directory containing the files.

Use an absolute path in automation.

## File Format

Gemini usage can be read from JSON.

Gemini usage can also be read from JSONL.

JSON files may contain structured session records.

JSONL files are parsed line by line.

Token counts come from Gemini usage fields.

Model names come from Gemini model fields.

Timestamps come from the source payload.

Malformed files or lines may be skipped.

## TokenMeter 实现细节

Collector:

```text
internal/collector/gemini.go
```

The collector is batch-only.

It backs `tm gemini`.

It honors `GEMINI_DATA_DIR`.

It scans local JSON and JSONL material.

It does not watch the directory continuously.

It does not call Gemini APIs.

It does not infer token counts from transcript text.

It maps source records into daily, weekly, monthly, and session reports.

Key limitation: files without usage fields cannot be counted.

Key limitation: model labels depend on source fields.

Key limitation: temporary cleanup by Gemini can remove historical rows.

## Examples

Daily Gemini usage:

```bash
tm gemini daily
```

Gemini sessions:

```bash
tm gemini session
```

Custom root:

```bash
GEMINI_DATA_DIR=/tmp/gemini tm gemini daily --json
```

Monthly report:

```bash
tm gemini monthly --breakdown
```

Filtered JSON:

```bash
tm gemini session --json --jq '.rows[] | {session_id,model,total_tokens}'
```

## Troubleshooting

If no rows appear, check `~/.gemini/tmp`.

If the default root is wrong, set `GEMINI_DATA_DIR`.

If Gemini cleaned temporary files, older usage may no longer be available.

If models are blank, inspect the JSON payload.

If token totals are zero, confirm usage fields exist.

If parsing fails, check for truncated JSONL lines.

If costs are zero, use `--mode calculate`.

If pricing should not refresh, pass `--offline`.

If reports are slow, add `--since`.

Use `tm daily --no-scan` to exclude Gemini from combined reports.
