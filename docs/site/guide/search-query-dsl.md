# Search Query DSL

`tm search` searches local TokenMeter activity.

v1.2 adds an advanced filter DSL in commit `b6d00ad`.

The command can combine keyword matching with structured filters.

It searches tool parameters.

It searches tool results.

It searches captured file paths.

It returns the newest hits first.

The feature is built on local SQLite data.

When FTS5 is available, keyword-only search can use the FTS index.

When advanced filters are present, TokenMeter builds parameterized SQL.

The old `tm search keyword` behavior remains available.

## Basic Usage

Search for a keyword:

```bash
tm search "permission denied"
```

Limit results:

```bash
tm search "permission denied" --limit 10
```

Render JSON:

```bash
tm search "permission denied" --json
```

Search failed Bash calls:

```bash
tm search "exit code" tool:Bash status:failed
```

Search Codex failures:

```bash
tm search status:failed platform:codex
```

Search expensive sessions:

```bash
tm search cost:>5
```

Search large-token sessions:

```bash
tm search tokens:>100000
```

Search a date window:

```bash
tm search "timeout" since:2026-05-01 until:2026-05-25
```

## Query DSL Reference

Filters are written as `key:value`.

Plain words are keywords.

All filters are combined with AND.

All keywords are also combined with AND.

The parser splits on whitespace.

Use shell quotes around the whole query when you need spaces.

| Syntax | Meaning | Example |
| --- | --- | --- |
| `keyword` | Full-text or LIKE keyword match. | `tm search timeout` |
| `tool:NAME` | Exact tool name. | `tm search tool:Bash` |
| `session:PREFIX` | Session id prefix. | `tm search session:abc123` |
| `status:ok` | Successful tool calls. | `tm search status:ok` |
| `status:failed` | Failed tool calls. | `tm search status:failed` |
| `status:all` | Do not filter by status. | `tm search status:all` |
| `platform:claude` | Claude sessions only. | `tm search platform:claude` |
| `platform:codex` | Codex sessions only. | `tm search platform:codex` |
| `platform:all` | Do not filter by platform. | `tm search platform:all` |
| `cost:>N` | Session cost greater than N USD. | `tm search cost:>5` |
| `cost:<N` | Session cost less than N USD. | `tm search cost:<1` |
| `cost:N` | Shorthand for `cost:>N` in v1.2. | `tm search cost:5` |
| `tokens:>N` | Session tokens greater than N. | `tm search tokens:>100000` |
| `tokens:<N` | Session tokens less than N. | `tm search tokens:<10000` |
| `tokens:N` | Shorthand for `tokens:>N` in v1.2. | `tm search tokens:100000` |
| `since:DATE` | Hits on or after date. | `tm search since:2026-05-01` |
| `until:DATE` | Hits through date. | `tm search until:2026-05-25` |

Dates use `YYYY-MM-DD`.

This differs from `tm analyze --since`, which uses `YYYYMMDD`.

`until` includes the whole day.

Internally it becomes `< next-day midnight`.

Unknown filters are rejected.

Empty filter values are rejected.

Invalid status values are rejected.

Invalid platform values are rejected.

Invalid ranges are rejected.

## Keyword Search

Keyword-only search keeps the old behavior.

Example:

```bash
tm search "gofmt"
```

When there are no advanced filters, TokenMeter can call `SearchHits`.

That path uses FTS5 when available.

It falls back to LIKE when FTS5 is unavailable.

It also falls back to LIKE for wildcard-sensitive input.

The result kinds can include:

`tool_param`.

`tool_result`.

`file`.

File hits search `file_changes.file_path`.

Tool parameter hits search `tool_calls.params_summary`.

Tool result hits search `tool_calls.result_summary`.

## Tool Filter

`tool:NAME` filters to one exact tool name.

Example:

```bash
tm search "exit code" tool:Bash
```

The match is exact.

`tool:bash` and `tool:Bash` are different if the stored tool name differs.

Tool filtering applies to tool call rows.

When `tool:` is present, file path hits are excluded.

That prevents file rows from bypassing a tool-only query.

Use the tool names that appear in `tm analyze --tool-errors`.

See [Tool Error Analysis](./tool-error-analysis.md) for failure-focused ranking.

## Session Filter

`session:PREFIX` matches session ids by prefix.

Example:

```bash
tm search "timeout" session:abc123
```

The filter uses `LIKE 'prefix%'`.

It is escaped before binding.

This is useful after copying a session id from another report.

Use more characters when the prefix is ambiguous in your own workflow.

## Status Filter

`status:failed` maps to stored tool status `fail`.

`status:ok` maps to stored tool status `success`.

`status:all` means no status filter.

Examples:

```bash
tm search status:failed
tm search "permission" status:failed
tm search "created" status:ok
```

Status filters apply to tool call rows.

File hits are included only when status is empty or `all`.

That means `status:failed` will not return file path hits.

Use this for incident review.

Use it with `tool:` when one tool is noisy.

## Platform Filter

`platform:claude` restricts hits to Claude sessions.

`platform:codex` restricts hits to Codex sessions.

`platform:all` means no platform filter.

Examples:

```bash
tm search "apply_patch" platform:codex
tm search "WebFetch" platform:claude
```

Platform values come from stored sessions.

They are not the same as source adapter names.

For source-specific docs, start with the [Sources index](../sources/).

## Source-Aware Search

Search filters operate on stored sessions and tool/file rows.

They do not use every source adapter name as a `platform:` value.

Most source pages still help you choose search terms.

Use the source guide to learn file locations, tool names, and likely captured text.

Then use `tm search` to inspect local rows.

Start here:

| Source | Guide | Search idea |
| --- | --- | --- |
| Claude Code | [Claude Code](../sources/claude.md) | `tm search status:failed platform:claude` |
| Codex | [Codex](../sources/codex.md) | `tm search status:failed platform:codex` |
| OpenCode | [OpenCode](../sources/opencode.md) | Search imported session text or file paths. |
| Amp | [Amp](../sources/amp.md) | Search thread-derived text after import. |
| Droid | [Droid](../sources/droid.md) | Search sidecar-informed model/session text. |
| Codebuff | [Codebuff](../sources/codebuff.md) | Search project path fragments. |
| Hermes | [Hermes](../sources/hermes.md) | Search SQLite-backed session text. |
| pi-agent | [pi-agent](../sources/pi.md) | Search session path or exported text. |
| Goose | [Goose](../sources/goose.md) | Search SQLite-backed session text. |
| Kilo | [Kilo](../sources/kilo.md) | Search local database content. |
| Kimi | [Kimi](../sources/kimi.md) | Search session path fragments. |
| OpenClaw | [OpenClaw](../sources/openclaw.md) | Search compatible root names. |
| Qwen | [Qwen](../sources/qwen.md) | Search project chat paths. |
| GitHub Copilot CLI | [GitHub Copilot CLI](../sources/copilot.md) | Search OTEL text fragments. |
| Gemini CLI | [Gemini CLI](../sources/gemini.md) | Search chat text or path fragments. |

Use this section as orientation.

The exact searchable rows depend on what has been ingested into the local database.

## Cost Filter

`cost:>N` filters on `sessions.total_cost_usd > N`.

`cost:<N` filters on `sessions.total_cost_usd < N`.

`cost:N` is currently shorthand for `cost:>N`.

It is not exact equality in v1.2.

Examples:

```bash
tm search cost:>5
tm search "retry" cost:<1
tm search tool:Bash cost:>10
```

Cost is session-level.

A cheap individual tool result inside an expensive session still matches `cost:>5`.

Use this as a session triage filter.

Do not use it as per-call cost accounting.

## Token Filter

`tokens:>N` filters on total session tokens.

`tokens:<N` filters below a threshold.

`tokens:N` is currently shorthand for `tokens:>N`.

The total is:

Input tokens.

Output tokens.

Cache read tokens.

Cache creation tokens.

Examples:

```bash
tm search tokens:>100000
tm search "schema" tokens:<20000
tm search status:failed tokens:>50000
```

Token filters are session-level.

They are useful for finding large sessions.

They do not isolate the exact call that spent the tokens.

## Date Filters

`since:DATE` filters by hit timestamp.

`until:DATE` filters by hit timestamp.

Dates use local parsing for `YYYY-MM-DD`.

The stored comparison is formatted for SQLite query time.

Examples:

```bash
tm search "error" since:2026-05-01
tm search "error" until:2026-05-25
tm search "error" since:2026-05-01 until:2026-05-25
```

`until` is inclusive.

The query uses `< DATE+1`.

This matches user expectations for whole-day windows.

## Composition Rules

Filters combine with AND.

This query requires every condition:

```bash
tm search "exit code" tool:Bash status:failed platform:claude since:2026-05-01
```

It means:

The body must contain `exit`.

The body must contain `code`.

The tool name must equal `Bash`.

The status must be failed.

The platform must be Claude.

The hit timestamp must be on or after 2026-05-01.

There is no OR syntax in v1.2.

Run separate searches for OR cases.

There is no quoted phrase parser inside the DSL.

Shell quotes group command arguments.

TokenMeter still splits the query string on whitespace.

## JSON Output

Use:

```bash
tm search "exit code" tool:Bash --json
```

The JSON shape is:

```json
{
  "query": {
    "keywords": ["exit", "code"],
    "tool": "Bash",
    "status": "failed",
    "cost_min": 5,
    "since": "2026-05-01T00:00:00+08:00"
  },
  "results": [
    {
      "session_id": "abc123",
      "session_name": "main",
      "platform": "claude",
      "kind": "tool_result",
      "excerpt": "output: exit code 1",
      "timestamp": "2026-05-21T10:00:00Z"
    }
  ]
}
```

`query.keywords` is omitted when empty.

`query.tool` is omitted when unset.

`query.session` is omitted when unset.

`query.status` is omitted when unset.

`query.platform` is omitted when unset.

`query.cost_min` is set by `cost:>N` and `cost:N`.

`query.cost_max` is set by `cost:<N`.

`query.tokens_min` is set by `tokens:>N` and `tokens:N`.

`query.tokens_max` is set by `tokens:<N`.

`query.since` and `query.until` are timestamps.

`results[].session_id` is the full session id.

`results[].session_name` is git branch, cwd basename, or short session id.

`results[].platform` is the stored platform.

`results[].kind` is `tool_param`, `tool_result`, or `file`.

`results[].excerpt` is trimmed display text.

`results[].timestamp` is the hit timestamp.

## SQL Injection Safety

Advanced filters are not string-concatenated into SQL values.

The implementation builds WHERE clauses with placeholders.

Filter values are passed as bound arguments.

LIKE patterns are escaped.

The test suite includes an injection-style tool filter:

```text
Bash' OR 1=1 --
```

That value must not match ordinary `Bash` rows.

This matters because search is often used on copied error text.

You should still quote shell arguments correctly.

Shell quoting happens before TokenMeter sees the query.

## Examples Gallery

Find failed Bash results since May 1:

```bash
tm search 'error' tool:Bash status:failed since:2026-05-01
```

Find high-cost sessions with large token totals:

```bash
tm search cost:>5 tokens:>100000
```

Find failed Codex tool calls:

```bash
tm search status:failed platform:codex
```

Find file path hits near storage code:

```bash
tm search internal/storage since:2026-05-01
```

Find WebFetch failures:

```bash
tm search WebFetch status:failed
```

Find a session by prefix:

```bash
tm search session:abc123
```

Find cheap successful activity:

```bash
tm search status:ok cost:<1
```

## Migration Notes

Existing keyword search still works.

Scripts using `tm search keyword` do not need to change.

`--limit` keeps the same role.

`--json` wraps parsed query details alongside results.

The new DSL only activates when terms contain recognized `key:value` filters.

Unknown filters now produce errors.

That is intentional.

It prevents typos from silently becoming keywords.

If you want to search for literal text that contains a colon, quote it and inspect behavior.

If the prefix before the colon is not a supported filter, v1.2 rejects it.

## Troubleshooting

If a query returns too few results, remove filters one at a time.

Start with the keyword.

Then add date filters.

Then add status or tool filters.

If file hits disappear, check whether `tool:` or `status:failed` is present.

Those filters focus the query on tool calls.

If `platform:` rejects a value, remember v1.2 supports `claude`, `codex`, and `all`.

If dates fail to parse, use `YYYY-MM-DD`.

If cost or token filters fail, remove commas.

Use `tokens:>100000`, not `tokens:>100,000`.

If FTS5 is unavailable, keyword-only search falls back to LIKE.

Advanced filters use the advanced SQL path.

## See Also

Use [Tool Error Analysis](./tool-error-analysis.md) for ranked failure reports.

Use [File Churn](./file-churn.md) to understand file-level activity before searching paths.

Use the [Sources index](../sources/) to understand which adapters can contribute local rows.
