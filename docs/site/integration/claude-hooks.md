# Claude Hooks Integration

TokenMeter integrates with Claude Code through hooks.

The hook integration is installed by `tm setup`.

The hook command is `tm emit`.

Claude Code writes hook payload JSON to stdin.

`tm emit` forwards that payload to the local TokenMeter daemon socket.

The daemon converts it into the internal event model.

## Installed Events

`tm setup` installs entries for:

| Event | Why TokenMeter needs it |
| --- | --- |
| `SessionStart` | Create the session and main agent. |
| `SessionEnd` | Close the session. |
| `Stop` | Capture transcript path and stop metadata. |
| `PreToolUse` | Start a tool call. |
| `PostToolUse` | Complete a successful tool call. |
| `PostToolUseFailure` | Complete a failed tool call. |
| `SubagentStart` | Track subagent lifecycle. |
| `SubagentStop` | Track subagent lifecycle. |

Each event is configured with a matcher entry.

The matcher is broad enough to catch all relevant hook payloads.

The inner hook command points at the current `tm` binary.

## Settings File

Claude Code settings live at:

```text
~/.claude/settings.json
```

TokenMeter expects valid JSON.

If the file is invalid, setup stops and asks you to repair it.

This protects existing Claude Code configuration.

Setup removes older TokenMeter or agmon hook commands before adding current ones.

That keeps repeated setup idempotent.

## Runtime Flow

Claude Code starts a hook.

The hook executes `tm emit`.

`tm emit` reads stdin.

It parses the hook event.

It sends a JSON frame to the daemon socket.

The daemon writes storage rows.

The daemon broadcasts events to subscribers.

`tm watch` can show those events.

The Web Dashboard can refresh from the same local data.

## Troubleshooting

Run:

```bash
tm doctor
```

If hooks are missing, run:

```bash
tm doctor --fix
```

Check hook logs:

```bash
tm logs --emit
```

Remove hooks:

```bash
tm uninstall
```

Then run `tm setup` again if you want a clean reinstall.

## TODO

Future docs should include a redacted before/after `settings.json` example.

They should also document how Claude Code statusline setup differs from event hooks.
