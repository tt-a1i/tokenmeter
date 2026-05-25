# File Churn

`tm analyze --file-churn` reports which files and directories change most often.

It is a v1.2 feature introduced in commit `caaa4ce`.

ccusage does not track file-level activity.

TokenMeter can use its local `file_changes` table.

That makes the report useful for code review, refactoring, and workflow health.

The report answers three questions.

Which files changed most?

Which directory prefixes are churn hotspots?

How has file activity changed over time?

## What It Reports

The command reads local daemon-captured file change events.

It does not inspect Git history.

It does not require a remote repository.

It uses rows from `file_changes`.

It joins sessions only when needed by the storage layer.

The report has three sections.

Top Changed Files ranks individual files.

File Churn Hotspots groups paths by directory prefix.

Daily file changes shows the recent daily trend.

Run the default report:

```bash
tm analyze --file-churn
```

The default analyze range is the last 30 days.

Use all available local data:

```bash
tm analyze --file-churn --range all
```

Use a fixed range:

```bash
tm analyze --file-churn --since 20260501 --until 20260525
```

Use JSON:

```bash
tm analyze --file-churn --json
```

## Why It Is Useful

Token totals show how much work happened.

File churn shows where work concentrated.

A high-churn file may be central to a feature.

It may also be unstable.

It may be a shared abstraction with too much responsibility.

It may be a test fixture that every change touches.

Churn helps review risk.

A small diff in a high-churn file deserves attention.

A huge token bill with low file churn can indicate analysis rather than implementation.

A low token bill with high file churn can indicate mechanical edits.

## Basic Commands

Show the report for the last month:

```bash
tm analyze --file-churn
```

Limit top changed files:

```bash
tm analyze --file-churn --limit 10
```

Show all history:

```bash
tm analyze --file-churn --range all
```

Use a release window:

```bash
tm analyze --file-churn --since 20260501 --until 20260525
```

Use compact rendering:

```bash
tm analyze --file-churn --compact
```

Combine with tool failures:

```bash
tm analyze --tool-errors --file-churn
```

Save machine-readable output:

```bash
tm analyze --file-churn --range all --json > file-churn.json
```

## Output Section: Top Changed Files

This section ranks individual file paths.

The primary sort key is change count.

The secondary sort key is distinct session count.

The final tie-breaker is file path.

The normal table columns are:

| Column | Meaning |
| --- | --- |
| Rank | Position after sorting. |
| File | File path from `file_changes.file_path`. |
| Changes | Number of captured file change events. |
| Sessions | Number of distinct sessions that touched the file. |
| First Seen | First date in the selected range. |
| Last Seen | Last date in the selected range. |
| Mode | Per-change-type counts. |

The compact table omits `Sessions` and `Mode`.

That keeps narrow terminals readable.

The `Mode` column is built from `change_type`.

Common values include `edit`.

Common values include `write`.

Common values include `read`.

Common values include `create`.

Common values include `delete`.

The exact set depends on captured event data.

Mode counts are sorted by count descending.

Mode ties are sorted by name.

Example:

```text
Mode
edit:12 write:2 create:1
```

That means most activity was incremental editing.

## Output Section: File Churn Hotspots

This section groups changed files by directory prefix.

It answers where churn concentrates.

The v1.2 command uses depth 2 internally.

That means a path like `internal/storage/db.go` groups under `internal/storage`.

The table columns are:

| Column | Meaning |
| --- | --- |
| Hotspot Path | Directory prefix used for grouping. |
| Changes | Total file change events under that prefix. |
| Files | Number of distinct files under that prefix. |
| Top File | Basename of the highest-churn file in that prefix. |

The primary sort key is change count.

The secondary sort key is file count.

The final tie-breaker is path.

Hotspot grouping helps separate one noisy file from a noisy directory.

If `Changes` is high and `Files` is 1, inspect the single top file.

If both `Changes` and `Files` are high, inspect the package or module boundary.

## Depth Interpretation

The storage helper accepts a depth.

The CLI currently calls it with depth 2.

The examples below explain how the helper behaves.

For `internal/storage/db.go`:

| Depth | Hotspot path |
| --- | --- |
| 1 | `internal` |
| 2 | `internal/storage` |
| 3 | `internal/storage` |

Depth never includes the file name.

It only groups by directory prefixes.

For `cmd/tm/main.go`:

| Depth | Hotspot path |
| --- | --- |
| 1 | `cmd` |
| 2 | `cmd/tm` |
| 3 | `cmd/tm` |

For `/Users/admin/code/agmon/main.go`:

| Depth | Hotspot path |
| --- | --- |
| 1 | `/Users` |
| 2 | `/Users/admin` |
| 3 | `/Users/admin/code` |

Absolute path roots are preserved.

That makes multi-root local activity easier to distinguish.

It also means private home directory names can appear in JSON output.

Be careful before sharing raw reports.

## Output Section: Daily File Changes

This section shows a daily sparkline-style trend.

The renderer prints the last 14 displayed days.

Each line has a date.

Each line has a ten-cell bar.

Each line has the change count.

Example:

```text
2026-05-21 █████░░░░░ 42
2026-05-22 ██░░░░░░░░ 18
```

The bar is relative to the largest displayed day.

A day with no changes is all empty cells.

Gap days are preserved.

Cross-month ranges are handled.

This makes the trend useful for release windows.

## JSON Schema

Use JSON for automation:

```bash
tm analyze --file-churn --json
```

The top-level key is `file_churn`.

The shape is:

```json
{
  "file_churn": {
    "top_files": [
      {
        "path": "internal/storage/db.go",
        "changes": 3,
        "sessions": 2,
        "first_seen": "2026-05-10T10:00:00Z",
        "last_seen": "2026-05-10T12:00:00Z",
        "mode_counts": {
          "edit": 2,
          "create": 1
        }
      }
    ],
    "hotspots": [
      {
        "path": "internal/storage",
        "changes": 4,
        "files": 2,
        "top_file": "db.go"
      }
    ],
    "daily": [
      {
        "date": "2026-05-10T00:00:00Z",
        "changes": 7
      }
    ]
  }
}
```

`top_files[].path` is the exact captured file path.

`top_files[].changes` is event count.

`top_files[].sessions` is distinct session count.

`top_files[].first_seen` is the first timestamp in range.

`top_files[].last_seen` is the last timestamp in range.

`top_files[].mode_counts` maps change type to count.

`hotspots[].path` is the grouped directory prefix.

`hotspots[].changes` is total events in that group.

`hotspots[].files` is distinct files in that group.

`hotspots[].top_file` is a basename, not a full path.

`daily[].date` is a UTC day timestamp.

`daily[].changes` is event count for that day.

## Reading Signals

High churn and high session count means many separate tasks touched the file.

That often marks an architectural hotspot.

High churn and low session count means one task edited the file repeatedly.

That often marks an implementation struggle.

High read counts can be normal for reference files.

High edit counts in a test fixture may be expected.

High write counts in generated files may be noise.

High churn under `internal/storage` can indicate schema or query instability.

High churn under CLI command files can indicate help or flag drift.

Use the mode mix before deciding whether churn is risky.

## Cross-Project Caveat

In v1.2, file churn is database-wide for the selected time range.

It is not filtered by project path.

That matters if one TokenMeter database contains several repositories.

The top file list can mix projects.

The hotspot list can mix projects.

Absolute path roots help reveal this.

Relative paths may collide across repositories.

Use a narrower date range when reviewing one project.

Use separate databases when you need strict project isolation.

Pair churn results with session detail before making release decisions.

## Practical Review Flow

Start with:

```bash
tm analyze --file-churn --range all --limit 20
```

Look for top files that appear in many sessions.

Then inspect recent search hits:

```bash
tm search internal/storage/db.go since:2026-05-01
```

Use the [Search Query DSL](./search-query-dsl.md) to combine path keywords with dates.

Then inspect failed tools:

```bash
tm analyze --tool-errors --since 20260501 --until 20260525
```

If a hot file also correlates with tool failures, review that area first.

If a hot file has no failures and low session count, it may just be normal implementation work.

## Troubleshooting

If the report is empty, the database may not contain file change events.

File churn depends on local event capture.

Batch source adapters may not populate `file_changes`.

If paths look absolute, they came from the captured event.

If paths look relative, they also came from the captured event.

TokenMeter does not rewrite every path to repository-relative form.

If `--limit` seems ignored, check which section you are reading.

`--limit` controls top changed files.

Hotspots use a fixed limit of 20 in v1.2.

If daily trend shows zeros, check the selected range.

Analyze date flags use `YYYYMMDD`.

Search date filters use `YYYY-MM-DD`.

## See Also

Use [Tool Error Analysis](./tool-error-analysis.md) to find failing tools near hot files.

Use [Search Query DSL](./search-query-dsl.md) to inspect specific file paths.

Use [Blocks and Statusline](./blocks-and-statusline.md) to correlate churn with active billing windows.

