# Migrating to TokenMeter v1.0

## TL;DR

| What you ran before | Run now |
|---|---|
| `tm` (entered TUI) | `tm daily`, `tm blocks`, `tm web` — pick one |
| `tm cost`, `tm cost today` | `tm daily` |
| `tm cost week` | `tm weekly` |
| `tm report` | `tm session [<id>]` |
| `tm report --weekly` | `tm weekly` |
| `tm report --monthly` | `tm monthly` |
| `tm status` | `tm blocks --active` |
| `tm top` | `tm blocks --active` |

Old commands still work in v1.0; they print a deprecation warning to stderr.
They will be removed in v2.0.

## Data

Your existing `~/.tokenmeter/data/tokenmeter.db` is preserved verbatim — no
schema changes. Hooks registered in `~/.claude/settings.json` continue to work.

## New things you can do

- **5-hour blocks**: `tm blocks` shows your activity in 5-hour windows that
  match Claude API's rate-limit cadence. `tm blocks --active` shows burn rate
  + projection for the current window.
- **Claude Code statusline**: `tm setup --statusline` writes a hook so Claude
  Code's status line shows `🤖 sonnet ▎ $X.XX (5h, Nm left) ▎ X.XK tok` in real
  time. Configure quota in `~/.tokenmeter/statusline.json`.
- **LiteLLM pricing**: model prices auto-refresh weekly via PR.

## Known v1.0 limitations

The following shared flags are documented in `tm <cmd> --help` but **not yet
honored** by `tm daily` / `tm weekly` / `tm monthly` / `tm session` in v1.0:

- `--json` — table output is the only mode in v1.0; JSON support is planned for v1.0.1
- `--mode auto|calculate|display` — display mode is the default behavior
- `--breakdown` — model breakdown to be added in v1.0.1
- `--order desc` — output is always ascending in v1.0

`tm blocks` honors `--json` and `--active`. `tm <cmd> --since YYYYMMDD --until YYYYMMDD` now works for daily/weekly/monthly/session/blocks, and `--project <workspace>` filters by workspace path on all five.
