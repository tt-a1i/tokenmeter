# Webhooks

TokenMeter can send webhook notifications from the daemon.

Webhook configuration lives in the app directory.

The current file name is:

```text
~/.tokenmeter/webhooks.json
```

Legacy installs may resolve through `~/.agmon/`.

The daemon loads the file at startup.

It can reload configuration through `tm reload`.

## Events

Supported event names include:

| Event | Meaning |
| --- | --- |
| `budget_warn` | A budget crossed the warn threshold. |
| `budget_over` | A budget crossed the over threshold. |
| `session_high_cost` | A session exceeded the configured high-cost threshold. |
| `tool_failure_rate` | Tool failures crossed the configured percentage threshold. |
| `daemon_started` | Daemon startup or reload notification. |
| `daemon_lost_events` | Daemon dropped events during shutdown or overload. |
| `webhook_test` | Manual test payload. |

Use `*` in an endpoint event list to receive every event.

## Config Shape

Example:

```json
{
  "endpoints": [
    {
      "url": "https://example.com/hook",
      "events": ["budget_warn", "budget_over"],
      "format": "json",
      "retry": {
        "max_attempts": 3,
        "initial_backoff_seconds": 2
      },
      "thresholds": {
        "session_high_cost_usd": 5,
        "tool_failure_rate_pct": 20
      }
    }
  ]
}
```

Formats include `json`, `slack`, and `discord`.

Empty URL is invalid.

Unsupported format is rejected.

## CLI Commands

List configured endpoints:

```bash
tm webhook list
```

Send a test webhook:

```bash
tm webhook test https://example.com/hook
```

Replay failed webhook deliveries:

```bash
tm webhook replay
```

## Delivery Behavior

Webhook HTTP timeout is short.

Retry settings are per endpoint.

Failed deliveries can be written to a dead-letter log.

Replay reads failed delivery records and attempts them again.

Malformed reloads keep the previous good config.

## TODO

Future docs should show Slack and Discord payload examples.

They should also document the exact dead-letter file path and retention policy.
