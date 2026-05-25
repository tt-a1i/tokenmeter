# Codebuff Source

Codebuff is a batch-only TokenMeter source.

Use it when you want reports for local Codebuff or Manicode chat data.

The adapter scans stored chat message JSON.

It does not rely on TokenMeter hooks.

It does not contact Codebuff services.

It runs during all-source reports and direct `tm codebuff` commands.

## What is Codebuff

Codebuff is an AI coding assistant with local project chat history.

Some installs use the Manicode channel name.

Those histories are grouped by channel, project, chat, and messages.

TokenMeter reads the local records that include usage.

It groups them into daily, weekly, monthly, and session views.

Codebuff is not real-time inside TokenMeter.

Run a report again to pick up new chat files.

Use source-specific commands while validating paths.

## Data Location

Default root:

```text
~/.config/manicode
```

Environment override:

```bash
CODEBUFF_DATA_DIR=/path/to/codebuff-root tm codebuff daily
```

Common scan shape:

```text
<channel>/projects/<project>/chats/<chat>/chat-messages.json
```

The configured root should contain channel directories.

If your install uses a different channel, point at the parent root.

If your data lives under multiple profiles, scan one root at a time.

TokenMeter does not require the project to still exist on disk.

## File Format

Codebuff usage is read from JSON.

The important file is `chat-messages.json`.

Each chat can contain multiple message records.

The parser extracts timestamps.

It extracts token counts when the message records include them.

It extracts model identity when recorded.

It derives project and session identity from the path and payload.

Recorded costs are preserved only when the data includes them.

## TokenMeter 实现细节

Collector:

```text
internal/collector/codebuff.go
```

The collector is batch-only.

It is exposed through `tm codebuff`.

It scans the configured Codebuff data root during report generation.

It maps each chat to a TokenMeter session.

It uses local files as the source of truth.

It does not infer missing tokens.

It does not merge remote account billing.

It may skip malformed chat JSON.

Key limitation: non-standard channel layouts may need `CODEBUFF_DATA_DIR`.

Key limitation: message rows without usage fields cannot become usage rows.

Key limitation: model breakdown depends on model fields in the JSON.

## Examples

Daily Codebuff usage:

```bash
tm codebuff daily
```

Codebuff session breakdown:

```bash
tm codebuff session
```

Monthly totals:

```bash
tm codebuff monthly --since 20260101
```

Custom root:

```bash
CODEBUFF_DATA_DIR=/tmp/manicode tm codebuff daily --json
```

All sources:

```bash
tm daily --breakdown
```

## Troubleshooting

If no data appears, check for `chat-messages.json`.

If the default root is wrong, set `CODEBUFF_DATA_DIR`.

If the adapter sees chats but no tokens, inspect message usage fields.

If model is empty, inspect the chat JSON for model fields.

If a project name looks odd, remember that path segments can be used as fallback identity.

If costs are missing, use calculated mode with pricing data.

If scanning too much data is slow, add `--since`.

If you need machine-readable rows, add `--json`.

If you need to filter the JSON, add `--jq`.

If you only want TokenMeter's daemon database, use `--no-scan` on all-source reports.
