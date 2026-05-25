# Kimi Source

Kimi is a batch-only TokenMeter source.

Use it for local Kimi agent session usage.

The adapter scans session wire logs.

It can read model defaults from Kimi config.

It does not use hooks.

It is available through all-source reports and `tm kimi`.

## What is Kimi

Kimi is an AI coding agent with local session storage.

Its session logs are organized by group and session.

The most useful usage data lives in `wire.jsonl`.

TokenMeter reads those local wire events.

It can also use Kimi config to fill model context.

Kimi is not real-time inside TokenMeter.

Report scans discover new usage.

Use direct source commands for path validation.

## Data Location

Default root:

```text
~/.kimi
```

Environment override:

```bash
KIMI_DATA_DIR=/path/to/kimi tm kimi daily
```

The override may contain comma-separated roots.

Expected session log shape:

```text
<root>/sessions/<group>/<session>/wire.jsonl
```

Optional model context:

```text
<root>/config.json
```

Point the override at the Kimi root.

## File Format

Kimi usage is read from JSONL.

Each `wire.jsonl` line is parsed as an event.

The adapter extracts token counts from usage events.

Session identity is derived from the path and payload.

Group identity can be preserved as project context.

Model identity comes from event data when present.

If events omit model, config can provide fallback context.

Malformed JSONL lines may be skipped.

## TokenMeter 实现细节

Collector:

```text
internal/collector/kimi.go
```

The collector is batch-only.

It backs `tm kimi`.

It scans `sessions/*/*/wire.jsonl`.

It reads `config.json` when useful.

It honors `KIMI_DATA_DIR`.

It supports multiple roots.

It does not infer token counts from text.

It does not call Kimi cloud APIs.

Key limitation: model fallback depends on local config accuracy.

Key limitation: incomplete JSONL lines can be skipped.

Key limitation: rows without model may not price cleanly.

## Examples

Daily Kimi usage:

```bash
tm kimi daily
```

Kimi session breakdown:

```bash
tm kimi session
```

Custom root:

```bash
KIMI_DATA_DIR=/tmp/kimi tm kimi daily --json
```

Multiple roots:

```bash
KIMI_DATA_DIR=/work/kimi,/personal/kimi tm kimi monthly
```

Filter session rows:

```bash
tm kimi session --json --jq '.rows[] | {session_id,total_tokens,model}'
```

## Troubleshooting

If no rows appear, check for `sessions/*/*/wire.jsonl`.

If the default root is wrong, set `KIMI_DATA_DIR`.

If model is blank, inspect the event and `config.json`.

If token totals are zero, confirm the JSONL events include usage.

If parsing fails, look for truncated lines.

If costs are zero, use `--mode calculate`.

If pricing still fails, check the model id spelling.

If multiple roots are scanned, watch for copied sessions.

If reports are slow, add `--since`.

Use all-source `tm daily --breakdown` to compare Kimi with other agents.
