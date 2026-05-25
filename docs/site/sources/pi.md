# pi-agent Source

pi-agent is a batch-only TokenMeter source.

Use it for local pi-agent session logs.

The adapter scans JSONL session files.

It does not depend on a daemon.

It does not use network APIs.

It is available through all-source reports and `tm pi`.

## What is pi-agent

pi-agent is an AI coding agent that records local session events.

The records are stored under a sessions directory.

Each session can contain token usage events.

TokenMeter reads those local events and converts them into report rows.

pi-agent is historical in TokenMeter.

New rows appear after the next report scan.

Source-specific commands are useful for path validation.

All-source commands include pi-agent when the files exist.

## Data Location

Default root:

```text
~/.pi/agent/sessions
```

Environment override:

```bash
PI_AGENT_DIR=/path/to/sessions tm pi daily
```

The adapter scans recursively.

Expected files:

```text
*.jsonl
```

Point the environment variable at the sessions directory.

Do not point it at a single JSONL file.

Use absolute paths in scripts.

## File Format

pi-agent usage is read from JSONL.

Each line is a JSON event.

The adapter reads lines independently.

Malformed lines may be skipped.

Usage lines must include token information.

Timestamps are extracted from event fields.

Session identity is derived from event data and path context.

Model names are preserved when present.

## TokenMeter 实现细节

Collector:

```text
internal/collector/pi.go
```

The collector is batch-only.

It backs `tm pi`.

It recursively scans the configured session root.

It converts JSONL events into TokenMeter usage rows.

It does not watch the directory.

It does not infer token counts from content.

It does not merge cloud-side invoices.

It uses source ids to avoid duplicate rows inside a scan.

Key limitation: event schemas must include usage fields.

Key limitation: partial JSONL writes can be unreadable until the agent completes the line.

Key limitation: model breakdown requires model fields in the event.

## Examples

Daily pi-agent usage:

```bash
tm pi daily
```

pi-agent session report:

```bash
tm pi session
```

Restrict to a month:

```bash
tm pi monthly --since 20260501 --until 20260531
```

Custom sessions root:

```bash
PI_AGENT_DIR=/tmp/pi-sessions tm pi daily --json
```

Filter JSON output:

```bash
tm pi session --json --jq '.rows[] | select(.total_tokens > 0)'
```

## Troubleshooting

If no rows appear, confirm the root contains JSONL files.

If files exist but rows are empty, inspect whether usage fields are present.

If a file is still being written, rerun after pi-agent finishes the session.

If dates are grouped unexpectedly, pass `--timezone`.

If costs are zero, use calculated mode.

If calculated mode is still zero, confirm model names match pricing data.

If the scan is too slow, use `--since`.

If the wrong directory is scanned, set `PI_AGENT_DIR`.

If duplicate local copies exist, scan only one root.

Use all-source `tm daily` when you want pi-agent plus other agents.
