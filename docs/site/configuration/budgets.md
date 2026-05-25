# Budgets

TokenMeter has a local monthly budget system.

Budgets are stored in the SQLite database.

They can apply to one platform or all platforms.

The CLI exposes list, set, usage, and delete operations.

The web dashboard can also show budget metrics.

The daemon checks budget transitions in the background.

## Commands

List budgets:

```bash
tm budget list
```

Create or update a budget:

```bash
tm budget set "Claude monthly" 100 --platform claude
```

Check usage:

```bash
tm budget usage 1
```

Delete a budget:

```bash
tm budget delete 1
```

## Platform Scope

The `--platform` value can be `claude` or `codex`.

Omitting platform means all platforms.

The list output displays empty platform as `all`.

Use platform-specific budgets when one tool has a strict monthly ceiling.

Use all-platform budgets when you care about total local AI agent spend.

## Status Rules

Budget status is calculated from usage percentage.

Below 80 percent is `ok`.

80 percent or higher is `warn`.

100 percent or higher is `over`.

The daemon uses transitions to trigger webhook events.

That avoids spamming the same warning repeatedly.

## What Counts Toward Usage

Budget usage comes from stored token usage costs.

That includes Claude and Codex rows recorded by the daemon.

It can include batch source rows once they are merged into report flows.

Cost accuracy depends on model pricing and source-provided cost.

Use `--mode calculate` or `--mode display` in reports when you need to audit cost assumptions.

## Web Dashboard

The web API exposes budget list and per-budget usage.

The dashboard can show budget chips and settings.

Use the CLI for repeatable setup.

Use the web UI for quick inspection.

## TODO

Future docs should add screenshots from the dashboard.

They should also include a policy example for teams that share a local machine.
