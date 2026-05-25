# Tool Error Analysis

`tm analyze --tool-errors` turns local tool call history into a failure report.

It is a v1.2 feature introduced in commit `5ec2cb3`.

ccusage focuses on token and cost accounting.

TokenMeter can also inspect the operational reliability of local agent tools.

That means you can answer questions like these.

Which tool fails most often?

Is a failure pattern repeated across sessions?

Did the failure rate spike today?

Is a rarely used tool broken by configuration?

Is a hot tool failing because of API rate limits?

## What It Reports

The command reads the local SQLite store.

It uses rows from `tool_calls`.

It only needs local data.

It does not call a remote service.

The report has three sections.

Top Failing Tools ranks tools that have failures.

Error Pattern Groups groups repeated result summaries.

Daily failure rate shows the recent trend.

Run the default report:

```bash
tm analyze --tool-errors
```

The default analyze range is the last 30 days.

Use all available data:

```bash
tm analyze --tool-errors --range all
```

Use one week:

```bash
tm analyze --tool-errors --range week
```

Use an explicit date range:

```bash
tm analyze --tool-errors --since 20260501 --until 20260525
```

Dates for `--since` and `--until` use `YYYYMMDD`.

`--until` is inclusive for that day.

The implementation converts it to the next day internally.

## Why It Is Useful

Cost alone cannot explain slow or frustrating sessions.

A session can be cheap and still waste time.

A tool can fail repeatedly before a user notices the pattern.

Tool error analysis surfaces those patterns.

One local review found `WebFetch` at a 28.9 percent failure rate.

That is not a token-cost problem.

It is an agent reliability problem.

The output points to the affected tool.

It also shows whether failures share a normalized pattern.

That helps decide whether the fix is configuration, permissions, prompt shape, or upstream availability.

## Basic Commands

Use the default month window:

```bash
tm analyze --tool-errors
```

Use all local history:

```bash
tm analyze --tool-errors --range all
```

Use a fixed window for a release review:

```bash
tm analyze --tool-errors --since 20260501 --until 20260525
```

Render JSON:

```bash
tm analyze --tool-errors --json
```

Use compact table rendering:

```bash
tm analyze --tool-errors --compact
```

Combine with file churn in the same command:

```bash
tm analyze --tool-errors --file-churn
```

`--limit N` is accepted by `tm analyze`.

In v1.2, it controls the file churn top-file limit.

The tool-error top tools list is fixed at 10.

The error pattern minimum count is fixed at 3.

Treat this as a current implementation detail.

## Output Section: Top Failing Tools

This section is ranked by failure count.

If two tools have the same failure count, total call count breaks the tie.

Tool name breaks any remaining tie.

The table columns are:

| Column | Meaning |
| --- | --- |
| Rank | Position after sorting. |
| Tool | The tool name from `tool_calls.tool_name`. |
| Total Calls | All calls for that tool in the selected range. |
| Failures | Calls with `status = fail`. |
| Failure Rate | `Failures / Total Calls * 100`. |
| Top Error Pattern | Most common normalized failure pattern for that tool. |

Rows with zero failures are omitted.

That keeps the report focused.

A high failure rate on a low-volume tool can mean configuration drift.

A high failure rate on a high-volume tool usually deserves immediate attention.

Example interpretation:

```text
Rank  Tool      Total Calls  Failures  Failure Rate  Top Error Pattern
1     WebFetch  83           24        28.9%         fetch failed with status N
2     Bash      240          13        5.4%          exit code N: go test ./...
```

`WebFetch` is a reliability issue.

`Bash` may be normal if tests are intentionally run during development.

## Output Section: Error Pattern Groups

This section groups repeated failure summaries.

It is not grouped by exact raw string.

The implementation normalizes paths, numbers, casing, and whitespace.

Then it hashes the normalized string.

Only groups with at least 3 failures are shown.

The table columns are:

| Column | Meaning |
| --- | --- |
| Pattern | Normalized key, truncated to the table width. |
| Count | Number of failed tool calls in the group. |
| Tools | Sorted list of tools that emitted the pattern. |
| Sample Sessions | Up to 5 session ids that show the pattern. |

This section is useful when one failure appears under several tools.

For example, `Bash` and `Read` may both expose the same missing file problem.

The group points to the shared root cause.

If the section says `none`, common causes are:

The selected range has too few failures.

The failures have empty `result_summary` values.

The failures are all unique.

The minimum group threshold is 3.

## Output Section: Daily Failure Rate

This section shows a per-day trend.

The renderer prints the last 14 displayed days.

Each line includes a date.

Each line includes a ten-cell bar.

Each line includes a percentage.

Example:

```text
2026-05-21 ███░░░░░░░ 28.9%
2026-05-22 ░░░░░░░░░░ 0.0%
```

The bar is based on the failure percentage.

Each filled cell is roughly 10 percentage points.

Any non-zero rate gets at least one filled cell.

Gap days are preserved.

That makes a flatline meaningful.

## JSON Schema

Use `--json` for automation:

```bash
tm analyze --tool-errors --range all --json
```

The top-level key is `tool_errors`.

The shape is:

```json
{
  "tool_errors": {
    "top_tools": [
      {
        "tool": "WebFetch",
        "total": 83,
        "failures": 24,
        "failure_rate": 28.915662650602407,
        "top_pattern": "fetch failed with status N"
      }
    ],
    "patterns": [
      {
        "normalized_key": "no such file or directory: /users/.../aN.go",
        "sample_result": "No such file or directory: /Users/admin/a1.go",
        "count": 3,
        "tools": ["Bash", "Read"],
        "sessions": ["session-a", "session-b", "session-c"]
      }
    ],
    "daily": [
      {
        "date": "2026-05-21",
        "total": 83,
        "failures": 24,
        "failure_rate": 28.915662650602407
      }
    ]
  }
}
```

`top_tools[].tool` is the original tool name.

`top_tools[].total` is the number of calls in range.

`top_tools[].failures` is the number of failed calls in range.

`top_tools[].failure_rate` is a percentage, not a fraction.

`top_tools[].top_pattern` can be empty when result summaries are empty.

`patterns[].normalized_key` is the grouping key.

`patterns[].sample_result` is one original raw result summary.

`patterns[].count` is the number of failures in the group.

`patterns[].tools` is sorted alphabetically.

`patterns[].sessions` is capped at 5 session ids.

`daily[].date` is UTC day formatted as `YYYY-MM-DD`.

`daily[].total` is all tool calls on that day.

`daily[].failures` is failed tool calls on that day.

`daily[].failure_rate` is a percentage.

## Pattern Normalize Algorithm

The normalizer is implemented by `NormalizeToolErrorPattern`.

It uses these steps.

1. Convert the result summary to lowercase.

2. Replace digit runs with `N`.

3. Normalize Unix-like paths.

4. Collapse whitespace to a single space.

5. Trim leading and trailing whitespace.

6. Truncate the normalized key to the first 80 bytes.

The path rule keeps short paths unchanged.

Longer paths keep the first and last path segment.

Example:

```text
/Users/admin/code/agmon/internal/storage/tool_errors.go
```

becomes:

```text
/users/.../tool_errors.go
```

Example input:

```text
Exit code 1: gofmt /Users/admin/code/agmon/internal/storage/tool_errors.go:123
```

Normalized output:

```text
exit code N: gofmt /users/.../tool_errors.go:N
```

This is intentionally lossy.

It groups repeated classes of failure.

It is not meant to preserve exact forensic detail.

Use session detail or raw SQLite rows when exact paths matter.

## Reading Signals

High failure rate plus high call volume is usually urgent.

High failure rate plus low call volume is often setup-specific.

Low failure rate plus high count can still be annoying.

Repeated patterns across tools often indicate file system or permission issues.

Repeated patterns in one tool often indicate prompt shape or tool-specific configuration.

Failures with empty `result_summary` cannot be grouped.

That is a data quality limitation.

If pattern groups are `none` but top tools show failures, inspect raw tool details.

## Operational Tips

Run the report before changing automation.

Save JSON output in release review artifacts.

Compare `--range week` with `--range all` to separate new regressions from old baseline noise.

Use `--since` and `--until` for incidents.

Do not panic over intentional failed `Bash` calls.

Some agents learn through failed shell commands.

Look for changes in rate and repeated pattern text.

Treat `Edit` or `Write` failures as workflow friction.

Treat `WebFetch` failures as network, auth, or external site reliability signals.

Treat `Read` failures as path, workspace, or stale context signals.

Pair this report with `tm search status:failed`.

That lets you inspect the underlying failed calls.

Pair it with `tm analyze --file-churn`.

That helps reveal whether failures cluster near hot files.

## Troubleshooting

If the report is empty, confirm the daemon has captured tool calls.

Run:

```bash
tm search status:failed --limit 5
```

If the search is empty, the database may not contain failed tool calls.

If the pattern table is empty, result summaries may be missing.

Old databases can have failed rows without useful summaries.

If a date range returns nothing, check the date format.

Analyze date flags use `YYYYMMDD`.

Search date filters use `YYYY-MM-DD`.

Those are different command surfaces.

If a tool appears unexpectedly, remember that tool names come from captured local events.

They are not normalized across every agent provider.

## See Also

Use [Webhook Anomaly Detection](./webhook-anomaly-detection.md) for `usage_regression` alerts.

Use [Search Query DSL](./search-query-dsl.md) to inspect failed tool calls.

Use [File Churn](./file-churn.md) when failures are related to repeated edits.

