# JSON Output

TokenMeter commands can emit machine-readable JSON.

Use JSON output for scripts, dashboards, CI checks, and handoff artifacts.

The shared flag is:

```bash
tm daily --json
```

The JSON shape is intentionally stable at the command boundary.

Fields may be omitted when a command has no value for them.

Do not assume every adapter can populate every field.

Prefer checking for field existence in automation.

## Supported Commands

The usage report commands support JSON output.

Common examples:

```bash
tm daily --json
tm weekly --json
tm monthly --json
tm session --json
tm blocks --json
```

Source-specific commands use the same report machinery:

```bash
tm opencode daily --json
tm gemini session --json
tm qwen monthly --json
```

Operational commands can also expose JSON when they declare it.

Examples include:

```bash
tm healthcheck --json
tm version --check --json
```

When in doubt, run the command with `--help`.

## Top-Level Shape

Report JSON uses command-specific envelope keys.

Aggregate commands use the bucket name as the row key.

Session commands use `sessions`.

Blocks commands use `blocks`.

Every report envelope also includes `totals`.

Daily output looks like:

```json
{
  "daily": [],
  "totals": {
    "inputTokens": 0,
    "outputTokens": 0,
    "cacheCreationTokens": 0,
    "cacheReadTokens": 0,
    "totalTokens": 0,
    "totalCost": 0
  }
}
```

Weekly output uses `weekly`.

Monthly output uses `monthly`.

Session output uses `sessions`.

Blocks output uses `blocks`.

`totals` contains aggregate values when the command provides them.

## Row Fields

Daily, weekly, and monthly rows can include:

| Field | Meaning |
| --- | --- |
| `date` | Bucket date for daily reports. |
| `week` | ISO-like week bucket for weekly reports. |
| `month` | Month bucket for monthly reports. |
| `modelsUsed` | Models seen in the bucket. |
| `inputTokens` | Prompt or input tokens. |
| `outputTokens` | Completion or output tokens. |
| `cacheCreationTokens` | Tokens written to cache. |
| `cacheReadTokens` | Tokens read from cache. |
| `totalTokens` | Combined token total. |
| `totalCost` | Cost in USD when known or calculated. |

Aggregate rows do not currently expose a `source` field.

Use a source-specific command when you need one adapter's rows.

Use `--breakdown` when you need model-level rows.

Without `--breakdown`, rows are usually aggregated at the bucket level.

With `--breakdown`, rows can include `modelBreakdowns`.

Each breakdown entry includes `model`, token fields, and `totalCost`.

## Session Fields

Session JSON can include:

| Field | Meaning |
| --- | --- |
| `sessionId` | TokenMeter or adapter session identifier. |
| `projectPath` | Workspace path or source project label when known. |
| `lastActivity` | Last usage timestamp. |
| `modelsUsed` | Models seen in the session. |
| `inputTokens` | Prompt or input tokens. |
| `outputTokens` | Completion or output tokens. |
| `cacheCreationTokens` | Tokens written to cache. |
| `cacheReadTokens` | Tokens read from cache. |
| `totalTokens` | Token total for the session. |
| `totalCost` | Displayed or calculated cost. |

Session ids can come from the original agent.

Session ids can also be synthesized from file paths when the source lacks a durable id.

Do not treat ids from different sources as globally meaningful outside TokenMeter.

Use the command name and `sessionId` when joining source-specific data.

## Blocks Fields

`tm blocks --json` describes five-hour billing windows.

Common fields:

| Field | Meaning |
| --- | --- |
| `period` | Human-readable block time window. |
| `modelsUsed` | Models seen in the block. |
| `inputTokens` | Prompt or input tokens. |
| `outputTokens` | Completion or output tokens. |
| `cacheCreationTokens` | Tokens written to cache. |
| `cacheReadTokens` | Tokens read from cache. |
| `totalTokens` | Tokens inside the block. |
| `totalCost` | Cost inside the block. |
| `status` | Block status, including token-limit status when enabled. |
| `token_limit` | Limit provided by `--token-limit` when present. |
| `usage_pct` | Percent of `token_limit` used when present. |
| `projection` | Estimated final values for active blocks when available. |

`projection` can include:

| Field | Meaning |
| --- | --- |
| `totalTokens` | Estimated final token count. |
| `totalCost` | Estimated final cost. |
| `remainingTimeSeconds` | Seconds left in the block. |

Projection fields are estimates.

They depend on elapsed time and observed usage.

Short sessions can produce noisy burn rates.

Use projection as an alerting signal, not as an invoice.

## Pricing Mode

The `--mode` flag changes cost interpretation.

```bash
tm daily --json --mode auto
tm daily --json --mode display
tm daily --json --mode calculate
```

`auto` is the default.

`display` prefers recorded cost from the source.

`calculate` recalculates from model pricing when possible.

Rows can have tokens but zero cost when pricing data is unavailable.

Use `--offline` to prevent runtime pricing refresh.

## Date Filters

Use date filters to keep JSON small:

```bash
tm daily --json --since 20260501 --until 20260525
```

The date format is `YYYYMMDD`.

The command timezone affects bucket boundaries.

Use:

```bash
tm daily --json --timezone Asia/Shanghai
```

Project filters are useful for daemon-backed sources:

```bash
tm session --json --project /Users/admin/code/agmon
```

Batch adapters may only populate project when their source data exposes it.

## Using `--jq`

TokenMeter can post-filter JSON with `--jq`.

This is useful when a script only needs one field.

List row count:

```bash
tm daily --json --jq '.daily | length'
```

Show total cost:

```bash
tm daily --json --jq '.totals.totalCost'
```

Pick expensive sessions:

```bash
tm session --json --jq '.sessions[] | select(.totalCost > 1)'
```

Project only selected fields:

```bash
tm session --json --jq '.sessions[] | {sessionId, totalTokens, totalCost}'
```

Read daily buckets:

```bash
tm daily --json --jq '.daily[] | {date, totalTokens, totalCost}'
```

The expression runs after TokenMeter has produced JSON.

If `jq` is not installed, the command can fail.

Keep production expressions small and explicit.

## Source-Specific JSON

Every adapter command accepts the same shared flags.

Examples:

```bash
tm amp daily --json
tm droid session --json
tm goose monthly --json --breakdown
tm copilot daily --json
```

Source-specific output is easier to debug than combined output.

It proves that path discovery works.

It also limits row volume.

Use combined `tm daily --json` when you need the whole workstation view.

Use `tm daily --json --no-scan` when you only want TokenMeter's SQLite database.

Use `tm opencode daily --json` instead of filtering combined output by source.

The current aggregate JSON schema is bucket-oriented rather than source-oriented.

## Stability Notes

JSON output is designed for automation.

New fields may be added over time.

Existing field meanings should stay stable.

Consumers should ignore unknown fields.

Consumers should tolerate missing optional fields.

Prefer numeric comparisons on numeric fields.

Prefer ISO timestamps for time parsing.

Avoid scraping table output when JSON exists.

Use source-specific commands in tests.

Pin TokenMeter versions for strict CI assertions.
