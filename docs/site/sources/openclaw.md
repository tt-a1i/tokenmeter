# OpenClaw Source

OpenClaw is a batch-only TokenMeter source.

Use it for local OpenClaw-compatible session logs.

The adapter scans JSONL transcripts.

It also recognizes deleted and reset JSONL variants.

It supports multiple compatible roots.

It is available through `tm openclaw`.

## What is OpenClaw

OpenClaw is an AI coding agent with local transcript files.

Some compatible tools use related names and roots.

TokenMeter scans those roots for session JSONL.

The adapter reads local usage events.

It does not depend on a running OpenClaw process.

It does not call a remote API.

New rows appear after report scans.

Direct commands help isolate OpenClaw issues.

## Data Location

Primary environment override:

```bash
OPENCLAW_DIR=/path/to/openclaw tm openclaw daily
```

Default root:

```text
~/.openclaw
```

Additional compatible roots can include:

```text
~/.clawdbot
~/.moltbot
~/.moldbot
```

Expected files include:

```text
*.jsonl
*.jsonl.deleted.*
*.jsonl.reset.*
```

Use `OPENCLAW_DIR` when the active root is non-standard.

## File Format

OpenClaw usage is read from JSONL.

Each line is a JSON event.

The adapter handles normal JSONL files.

It also scans deleted and reset variants when present.

Token usage must be recorded in the event.

Model identity is preserved when present.

Session identity is derived from file and event context.

Malformed lines can be skipped.

## TokenMeter 实现细节

Collector:

```text
internal/collector/openclaw.go
```

The collector is batch-only.

It backs `tm openclaw`.

It searches OpenClaw and compatible roots.

It honors `OPENCLAW_DIR`.

It reads transcript variants that preserve historical usage.

It does not reconstruct deleted text.

It does not infer tokens from transcript text.

It does not call cloud billing APIs.

Key limitation: reset and deleted variants can require careful deduplication.

Key limitation: model breakdown depends on model fields.

Key limitation: partial files can produce partial results.

## Examples

Daily OpenClaw usage:

```bash
tm openclaw daily
```

Session report:

```bash
tm openclaw session
```

Custom root:

```bash
OPENCLAW_DIR=/tmp/openclaw tm openclaw daily --json
```

Monthly totals:

```bash
tm openclaw monthly --breakdown
```

Filter rows:

```bash
tm openclaw session --json --jq '.rows[] | select(.source == "openclaw")'
```

## Troubleshooting

If no rows appear, check for JSONL files under the root.

If you use a compatible fork, confirm the root name.

If auto-discovery misses it, set `OPENCLAW_DIR`.

If models are blank, inspect source events.

If token totals are zero, confirm events include usage fields.

If duplicate-looking rows appear, check copied reset or deleted files.

If costs are missing, use calculated mode.

If dates are unexpected, pass `--timezone`.

If scanning is slow, narrow the window with `--since`.

Use `tm daily --no-scan` to exclude OpenClaw from all-source reports.
