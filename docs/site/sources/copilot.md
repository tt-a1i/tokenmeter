# GitHub Copilot CLI Source

GitHub Copilot CLI is a batch-only TokenMeter source.

Use it when Copilot CLI exports local OpenTelemetry usage.

The adapter reads OTEL JSONL.

It does not query GitHub billing.

It does not use TokenMeter hooks.

It is available through all-source reports and `tm copilot`.

## What is GitHub Copilot CLI

GitHub Copilot CLI is a command-line coding assistant.

It can export telemetry to a local file.

That local telemetry can include token usage signals.

TokenMeter reads the file when the OTEL exporter path is configured.

This source is explicit by design.

TokenMeter does not guess a default Copilot telemetry path.

You provide the file path with an environment variable.

Reports scan the file during command execution.

## Data Location

Required environment variable:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=/path/to/copilot-otel.jsonl
```

Example:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=$HOME/.copilot/otel/usage.jsonl tm copilot daily
```

The variable should point at a JSONL file.

It should not point at a directory.

Keep the exporter writing line-delimited JSON.

Rotate files carefully if you rely on historical reports.

## File Format

Copilot usage is read from OTEL JSONL.

Each line is a JSON OpenTelemetry record.

The adapter extracts usage attributes from those records.

Timestamps come from OTEL event timestamps.

Session or request identity comes from telemetry attributes when available.

Model identity comes from telemetry attributes when available.

Token counts come from telemetry usage attributes.

Malformed or incomplete records may be skipped.

## TokenMeter 实现细节

Collector:

```text
internal/collector/copilot.go
```

The collector is batch-only.

It backs `tm copilot`.

It requires `COPILOT_OTEL_FILE_EXPORTER_PATH`.

It reads one local JSONL telemetry file.

It does not discover Copilot logs automatically.

It does not call GitHub APIs.

It does not infer tokens from prompts.

It preserves fields that can be mapped to TokenMeter rows.

Key limitation: missing env var means no Copilot rows.

Key limitation: telemetry attributes must include usage data.

Key limitation: file rotation can hide older rows from direct scans.

## Examples

Daily Copilot usage:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=$HOME/.copilot/otel/usage.jsonl tm copilot daily
```

Copilot sessions:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=$HOME/.copilot/otel/usage.jsonl tm copilot session
```

JSON output:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=/tmp/copilot.jsonl tm copilot daily --json
```

All sources:

```bash
COPILOT_OTEL_FILE_EXPORTER_PATH=/tmp/copilot.jsonl tm daily
```

Filter rows:

```bash
tm copilot session --json --jq '.rows[] | {model,total_tokens,cost_usd}'
```

## Troubleshooting

If no rows appear, confirm the environment variable is set.

If the file is empty, confirm Copilot CLI is exporting telemetry.

If the path is a directory, point it at the JSONL file.

If lines fail to parse, inspect the OTEL JSONL format.

If token totals are zero, confirm telemetry includes usage attributes.

If models are blank, inspect telemetry model attributes.

If costs are zero, use `--mode calculate`.

If old data disappears, check telemetry rotation.

If reports are slow, use `--since`.

Use `--offline` when pricing refresh should be disabled.
