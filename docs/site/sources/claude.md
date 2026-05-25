# Claude Code Source

Claude Code is TokenMeter's primary hook-driven source.

It contributes real-time tool and session events through Claude hooks.

It contributes token usage through local JSONL transcript logs.

The two paths complement each other.

Hooks are immediate.

JSONL logs carry token truth.

## What It Captures

TokenMeter hooks receive Claude Code lifecycle and tool events.

The configured hook events are:

| Hook event | Purpose |
| --- | --- |
| `SessionStart` | Starts a session and main agent row. |
| `SessionEnd` | Ends a session. |
| `Stop` | Supplies transcript path and stop metadata. |
| `PreToolUse` | Starts a tool call. |
| `PostToolUse` | Completes a tool call. |
| `PostToolUseFailure` | Completes a failed tool call. |
| `SubagentStart` | Starts a subagent row. |
| `SubagentStop` | Ends a subagent row. |

The daemon correlates pre and post tool events by tool use id.

That produces duration and status fields.

Subagent events preserve hierarchy.

File changes can be attached when tool payloads expose paths.

## Token Logs

Claude hook payloads do not include token counts.

TokenMeter scans Claude JSONL logs under `~/.claude/projects/`.

Assistant messages with usage fields become token rows.

The watcher processes recent files.

It stores read offsets so restarts do not double count the same lines.

Token rows use `source_id` for deduplication.

## Setup

Run:

```bash
tm setup
```

This writes hook entries into `~/.claude/settings.json`.

If you prefer a guided flow, run:

```bash
tm init
```

Use `tm doctor --fix` when hooks are missing or malformed.

Use `tm uninstall` to remove TokenMeter hooks.

## Operational Notes

Keep `tm daemon` running if you want hook events recorded immediately.

`tm web` can also start the embedded daemon and watchers.

If no Claude rows appear, inspect `~/.claude/settings.json`.

Then run `tm doctor`.

Then check `tm logs --emit` for hook errors.

The daemon socket is local to the current user.

The database stays on disk under the TokenMeter app directory.

## Related Commands

Use `tm daily --no-scan` to see only daemon-backed Claude and Codex rows.

Use `tm session` to inspect session totals.

Use `tm watch --types tool,token` to watch live Claude activity.

Use `tm web` for message and tool browsing.
