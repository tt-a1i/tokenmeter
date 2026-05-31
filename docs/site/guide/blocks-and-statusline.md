# Blocks and Statusline

TokenMeter models active work in five-hour billing blocks.

The block view is designed for daily operator awareness.

It answers three questions.

How much has this block cost?

How fast is the block burning?

Where will the block land if current pace continues?

The statusline reuses the same block calculation.

That keeps the one-line prompt signal aligned with `tm blocks --active`.

## Five-Hour Blocks

The default block length is five hours.

That matches common AI coding usage windows.

Run:

```bash
tm blocks
```

Show only the current block:

```bash
tm blocks --active
```

Use a custom length:

```bash
tm blocks --session-length 4h
```

The active block is calculated from recent usage.

It is not a server-side billing object.

It is a local reporting window.

Rows can include daemon-backed TokenMeter usage.

Rows can also include batch adapter usage when scans are enabled.

Use `--no-scan` when you want only TokenMeter's SQLite rows.

## Burn Rate

Burn rate is observed usage divided by elapsed block time.

Cost burn rate is usually shown as USD per hour.

Token burn rate can be shown when token totals are available.

Run:

```bash
tm blocks --active --json
```

Current JSON uses the same block envelope as the report renderer.

Look for fields like:

```json
{
	  "blocks": [
	    {
	      "id": "2026-05-25T10:00:00Z",
	      "startTime": "2026-05-25T10:00:00Z",
	      "endTime": "2026-05-25T15:00:00Z",
	      "isActive": true,
	      "tokenCounts": {
	        "inputTokens": 60000,
	        "outputTokens": 30000,
	        "cacheCreationInputTokens": 0,
	        "cacheReadInputTokens": 30000
	      },
	      "totalTokens": 120000,
	      "costUSD": 3.10,
	      "models": ["claude-sonnet-4-6"],
	      "burnRate": {
	        "tokensPerMinute": 2400,
	        "costPerHour": 1.55
	      },
	      "projection": {
	        "totalTokens": 220000,
	        "totalCost": 5.70,
	        "remainingMinutes": 90
	      }
	    }
	  ]
}
```

The table and statusline can display burn-rate wording.

The JSON contract exposes totals, burn-rate, and projection fields.

Burn rate is volatile early in a block.

A short expensive request can spike the rate.

A long idle period can lower it.

Use burn rate as a trend signal.

Do not treat it as a final cost.

## Projection

Projection estimates the final block value.

It uses observed pace and remaining time.

Run:

```bash
tm blocks --active
```

JSON output can expose projected cost and projected tokens:

```bash
tm blocks --active --json
```

Projection is most useful after the block has enough elapsed time.

It is least useful immediately after the first request.

It depends on the same pricing mode as normal reports.

Use calculated pricing when source costs are incomplete:

```bash
tm blocks --active --mode calculate
```

Use offline mode when the command must not refresh pricing:

```bash
tm blocks --active --offline
```

## Token Limits

`--token-limit` adds a token budget to the block view.

Example:

```bash
tm blocks --active --token-limit 500000
```

The limit is local.

It does not stop an agent.

It helps display remaining capacity and projection risk.

The value should be a whole token count.

Use the same value in scripts and statusline configuration when you need consistency.

When the limit is present, JSON output can include limit-aware fields.

Those fields let automation alert before a block exhausts its token budget.

The limit applies to the displayed block calculation.

It is not persisted unless a config layer stores it.

## Statusline Basics

`tm statusline` is intended for Claude Code statusline integration.

It reads one JSON object from stdin.

It writes one compact line to stdout.

Manual test:

```bash
echo '{"session_id":"demo","cwd":"/tmp/demo","model_id":"claude-sonnet"}' | tm statusline
```

Claude Code's current statusline hook shape uses nested model data and may include hook cost:

```bash
echo '{"session_id":"demo","cwd":"/tmp/demo","transcript_path":"/tmp/demo.jsonl","model":{"id":"claude-sonnet-4-6","display_name":"Claude Sonnet 4.6"},"cost":{"total_cost_usd":1.23}}' | tm statusline
```

The statusline reports the active block.

It can include model, session cost, today's cost, active block cost, remaining time, token count, burn rate, and context state.

If no active block exists, it reports that state.

The command accepts shared flags where relevant.

Debug statusline math with:

```bash
tm blocks --active
```

## Statusline Config File

The statusline file is:

```text
~/.tokenmeter/statusline.json
```

Legacy installs may resolve through `~/.agmon/`.

Missing config is valid.

Start with:

```json
{
  "quota_usd": 25,
  "format": "compact",
  "color": true
}
```

`quota_usd` enables budget comparison.

`format` controls compact or detailed display.

`color` enables ANSI color when useful.

`--no-color` disables color from the command line.

## Burn Rate Display

Recent statusline builds support four burn-rate display modes.

Use them to tune how much noise appears in the prompt.

Mode list:

| Mode | Behavior |
| --- | --- |
| `off` | Hide burn-rate signal. |
| `emoji` | Show only a compact visual indicator. |
| `text` | Show textual burn-rate state. |
| `emoji-text` | Show both indicator and text. |

When burn rate is shown, the line includes cost per hour plus the selected visual status.

## Cost Source

`--cost-source` controls the session cost displayed in the line:

| Mode | Behavior |
| --- | --- |
| `auto` | Prefer Claude Code hook cost, then TokenMeter-calculated session cost. |
| `cc` | Use only hook cost from `cost.total_cost_usd`. |
| `ccusage` | Use TokenMeter-calculated session cost. |
| `both` | Show hook cost and TokenMeter-calculated cost side by side. |

Example:

```json
{
  "burn_rate_display": "emoji-text"
}
```

Use `off` for quiet environments.

Use `emoji` for narrow terminals.

Use `text` when logs must be readable without color or symbols.

Use `emoji-text` when the statusline is the primary usage monitor.

## Context Thresholds

Statusline can classify context pressure with thresholds.

Use low and medium thresholds to separate normal, caution, and high-pressure states.

Example:

```json
{
  "context_low_threshold": 0.50,
  "context_medium_threshold": 0.75
}
```

The values are ratios.

`0.50` means fifty percent.

`0.75` means seventy-five percent.

Keep the low threshold below the medium threshold.

If thresholds are omitted, TokenMeter uses built-in defaults.

Context classification depends on model context information.

When context capacity is unknown, the statusline may omit or downgrade the context signal.

## Practical Workflows

Start a work session and watch the active block:

```bash
tm blocks --active
```

Keep a machine-readable watch in another terminal:

```bash
tm blocks --active --json
```

Set a temporary token limit:

```bash
tm blocks --active --token-limit 300000
```

Compare table and statusline:

```bash
echo '{"session_id":"demo","cwd":"'$PWD'"}' | tm statusline
tm blocks --active
```

Use `--timezone` when block dates cross midnight:

```bash
tm blocks --timezone Asia/Shanghai
```

## Troubleshooting

If the statusline looks stale, compare it with `tm blocks --active`.

If the block is empty, confirm the daemon database has rows or scans are enabled.

If cost is zero, try `--mode calculate`.

If network refresh is not allowed, run reports with `--offline`.

If burn rate is noisy, wait until more time has elapsed.

If token limit output is missing, confirm `--token-limit` is passed.

If colors are unwanted, pass `--no-color`.

If context state is missing, confirm model context metadata is available.

If statusline config is ignored, check the file path and JSON syntax.

If all-source scans make statusline slow, prefer daemon-backed rows for statusline use.

## Automation Notes

Prefer JSON for alerting.

Read `active` before alerting on a block.

Check projected fields only when they exist.

Check token-limit fields only when a limit is configured.

Avoid hard-failing when model context is unknown.

Keep statusline output short.

Use full reports for deep analysis.

Treat projection as an early warning.

Treat final daily totals as accounting.

Keep human prompt signals less noisy than CI alerts.
