# Webhook Anomaly Detection

TokenMeter v1.2 can send anomaly webhook events from the daemon.

The first implementation landed in commit `ae5f4bf`.

Cooldown configuration was completed in commit `4c415ea`.

This guide covers the two anomaly event types.

It covers payload shape.

It covers unified config fields.

It covers cooldown behavior.

It covers practical testing.

## What It Does

The daemon periodically checks local usage history.

It compares today's cost against recent daily baseline cost.

It compares recent tool failure rates against prior baseline rates.

When a threshold is crossed, it dispatches webhook events.

The new anomaly events are:

`cost_spike`.

`usage_regression`.

These events are separate from budget alerts.

They are also separate from the older `tool_failure_rate` alert.

Budget alerts compare spend to configured budgets.

Anomaly alerts compare current behavior to recent local history.

## Why This Exists

Static thresholds are useful.

They are not enough.

A project can double its daily cost without crossing a monthly budget.

A tool can become flaky before the overall failure rate looks dramatic.

Anomaly webhooks catch those changes early.

They are designed for operator attention.

They are not designed to block the agent.

They do not enforce budgets.

They notify configured endpoints.

## Event Type: cost_spike

`cost_spike` compares today's cost to the historical daily average.

The lookback window is 7 days in v1.2.

The default threshold ratio is 2.0.

That means today must be at least 2x the baseline average.

The baseline requires enough history.

The storage query requires at least 7 historical days.

If history is incomplete, no event is sent.

If historical total cost is zero, no event is sent.

If today's cost is below threshold, no event is sent.

Example event meaning:

Today cost is 12.00 USD.

The 7-day average is 4.00 USD.

The ratio is 3.0.

The threshold is 2.0.

The daemon sends `cost_spike`.

## Event Type: usage_regression

`usage_regression` compares the last hour of tool failures to baseline failure rates.

The baseline window is the prior 7 days.

The default minimum recent failure count is 10.

The default ratio threshold is 2.0.

That means recent failure rate must be at least 2x the baseline rate.

The tool must have enough recent failures.

The baseline tool must exist.

The baseline failure rate must be above zero.

If the baseline is empty, no event is sent.

If the recent hour has too few failures, no event is sent.

If the ratio is below threshold, no event is sent.

This alert pairs well with [Tool Error Analysis](./tool-error-analysis.md).

Use the guide to inspect patterns after the webhook fires.

## Detection Cycle

The daemon starts an anomaly sweep loop.

The loop runs every 5 minutes.

Each sweep reads the current endpoint snapshot.

Each endpoint can subscribe to specific events.

If an endpoint wants `cost_spike`, the daemon checks daily cost spike.

If an endpoint wants `usage_regression`, the daemon checks tool failure regression.

Endpoints that do not include the event are skipped.

Use `*` in an endpoint event list to receive every event.

The sweep is local.

It does not fetch remote pricing.

It uses costs and tool calls already stored in the database.

## Payload Wrapper

All JSON webhook events use a common wrapper.

The wrapper fields are:

| Field | Meaning |
| --- | --- |
| `event` | Event name such as `cost_spike`. |
| `timestamp` | Dispatch timestamp. |
| `cost_spike` | Cost spike payload, when present. |
| `usage_regressions` | Usage regression items, when present. |

Other webhook event types can use other wrapper fields.

For anomaly events, focus on `cost_spike` and `usage_regressions`.

## cost_spike Payload Schema

Example:

```json
{
  "event": "cost_spike",
  "timestamp": "2026-05-25T10:05:00Z",
  "cost_spike": {
    "date": "2026-05-25",
    "today_cost": 12.0,
    "baseline_cost": 4.0,
    "ratio": 3.0,
    "threshold_ratio": 2.0,
    "lookback_days": 7
  }
}
```

`date` is the UTC day under evaluation.

`today_cost` is total cost for that day so far.

`baseline_cost` is the average daily cost over the lookback window.

`ratio` is `today_cost / baseline_cost`.

`threshold_ratio` is the endpoint threshold used for the decision.

`lookback_days` is currently 7.

## usage_regression Payload Schema

Example:

```json
{
  "event": "usage_regression",
  "timestamp": "2026-05-25T10:05:00Z",
  "usage_regressions": [
    {
      "tool": "WebFetch",
      "recent_calls": 83,
      "recent_failures": 24,
      "recent_failure_rate": 28.9,
      "baseline_calls": 320,
      "baseline_failures": 23,
      "baseline_failure_rate": 7.2,
      "ratio": 4.0,
      "threshold_ratio": 2.0,
      "min_failures": 10
    }
  ]
}
```

`tool` is the tool name.

`recent_calls` is the last-hour call count.

`recent_failures` is the last-hour failure count.

`recent_failure_rate` is a percentage.

`baseline_calls` is the prior baseline call count.

`baseline_failures` is the prior baseline failure count.

`baseline_failure_rate` is a percentage.

`ratio` is recent failure rate divided by baseline failure rate.

`threshold_ratio` is the endpoint threshold.

`min_failures` is the endpoint minimum failure count.

## Unified Config

Anomaly webhooks live under `webhooks`.

Use `~/.tokenmeter/config.json`.

Legacy `webhooks.json` is still supported for migration.

Example:

```json
{
  "webhooks": {
    "anomaly_cooldown": {
      "cost_spike_hours": 24,
      "usage_regression_minutes": 60
    },
    "endpoints": [
      {
        "url": "https://example.com/tokenmeter",
        "events": ["cost_spike", "usage_regression"],
        "format": "json",
        "thresholds": {
          "cost_spike_ratio": 2.0,
          "regression_failure_count_min": 10,
          "regression_ratio_min": 2.0
        }
      }
    ]
  }
}
```

Supported formats include `json`.

Supported formats include `slack`.

Supported formats include `discord`.

An empty URL is invalid.

An unsupported format is rejected.

## Threshold Fields

Thresholds are per endpoint.

The fields are:

| Field | Default | Meaning |
| --- | --- | --- |
| `cost_spike_ratio` | `2.0` | Today cost divided by baseline cost. |
| `regression_failure_count_min` | `10` | Minimum recent failures for usage regression. |
| `regression_ratio_min` | `2.0` | Recent failure rate divided by baseline failure rate. |

Thresholds can coexist with older fields.

Older fields include `session_high_cost_usd`.

Older fields include `tool_failure_rate_pct`.

An endpoint can subscribe to both old and new events.

Example:

```json
{
  "url": "https://example.com/all-alerts",
  "events": ["budget_over", "cost_spike", "usage_regression"],
  "format": "json",
  "thresholds": {
    "session_high_cost_usd": 5,
    "tool_failure_rate_pct": 20,
    "cost_spike_ratio": 2.5,
    "regression_failure_count_min": 12,
    "regression_ratio_min": 3.0
  }
}
```

## Cooldown Fields

Cooldowns are global to webhook anomaly config.

They are not per endpoint in v1.2.

The fields are:

| Field | Default | Meaning |
| --- | --- | --- |
| `cost_spike_hours` | `24` | Suppress repeat cost spike alerts for the same date. |
| `usage_regression_minutes` | `60` | Suppress repeat usage regression alerts for similar tool ratio buckets. |

The `cost_spike` cooldown key includes the date.

The `usage_regression` cooldown key includes the tool and floored ratio.

Cooldown state is in memory.

Daemon restart clears cooldown state.

This is a known v1.2 limitation.

It keeps the implementation simple.

It also means duplicate alerts can happen after restart.

## Testing Webhooks

Use a temporary endpoint while testing.

webhook.site is convenient for manual testing.

httpbin can also receive JSON posts.

Add the endpoint to config.

Then reload the daemon:

```bash
tm reload
```

List configured endpoints:

```bash
tm webhook list
```

Send a manual test payload:

```bash
tm webhook test https://example.com/hook
```

The manual test sends `webhook_test`.

It does not force anomaly detection.

To test anomaly detection itself, use a development database with controlled history.

The internal test suite covers `cost_spike` and `usage_regression`.

Production users should validate endpoint delivery first.

Then wait for natural anomaly conditions.

## Common Gotchas

`cost_spike` needs historical data.

If the last 7 days are incomplete, no cost spike event is sent.

`cost_spike` needs non-zero baseline cost.

If the baseline is zero, no event is sent.

`usage_regression` needs baseline failures.

If the baseline failure rate is zero, no event is sent.

`usage_regression` needs the minimum recent failure count.

The default is 10 failures in the last hour.

Cooldown can suppress events.

Check `anomaly_cooldown` before assuming detection failed.

Daemon restart clears cooldown.

That can make testing appear inconsistent.

Endpoint event lists are filters.

An endpoint with only `budget_over` will not receive anomaly events.

Use `events: ["*"]` for broad testing.

## Operational Advice

Start with default thresholds.

Raise `cost_spike_ratio` if your usage is naturally bursty.

Lower it only after you have enough baseline history.

Raise `regression_failure_count_min` for noisy tools.

Lower it for high-value tools where even a few failures matter.

Keep cooldowns enabled for chat endpoints.

Use JSON endpoints for automation.

Use Slack or Discord endpoints for human notification.

Record anomaly events during release windows.

Pair `usage_regression` with `tm analyze --tool-errors`.

Pair `cost_spike` with `tm daily --breakdown`.

Use `tm search status:failed since:YYYY-MM-DD` to inspect related failure rows.

## See Also

Use [Tool Error Analysis](./tool-error-analysis.md) after a `usage_regression` alert.

Use [Search Query DSL](./search-query-dsl.md) to inspect failed calls.

Use [Configuration: Webhooks](../configuration/webhooks.md) for the broader webhook feature.

