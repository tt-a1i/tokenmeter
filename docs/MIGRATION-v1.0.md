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

## v1.0.1 changes

v1.0.1 finishes the ccusage-alignment work the v1.0 changelog promised
"in v1.0.1". Almost everything is additive — two notes deserve highlight.

### `--until` semantics — potentially breaking

- **v1.0.0:** `--until 20260520` resolved to `< 2026-05-20 00:00 UTC`
  (half-open). Activity on 2026-05-20 itself was **excluded**.
- **v1.0.1:** `--until 20260520` resolves to `< 2026-05-21 00:00 UTC`
  (inclusive close to end-of-day). Activity on 2026-05-20 is now
  **included**. Same wire format, different semantics — ccusage's
  default.

If a v1.0.0 script depended on the half-open behavior, subtract one day
from the date when upgrading. Most users want the new behavior.

### JSON schema — newly enforced

- **v1.0.0:** `--json` was accepted but ignored; output was still the
  hand-rolled tabwriter table.
- **v1.0.1:** `--json` switches to a top-level envelope with camelCase
  field names:
  - `daily` / `weekly` / `monthly` wrap their rows under those keys.
  - `sessions` wraps session rows.
  - `blocks` wraps block rows.
  - `totals` carries grand totals alongside each list.
  - Per-row fields: `date` / `week` / `month` / `sessionId` / `period`,
    `modelsUsed`, `inputTokens` / `outputTokens` / `cacheCreationTokens`
    / `cacheReadTokens` / `totalTokens`, `totalCost`, and (when
    `--breakdown` is set) `modelBreakdowns[]`. Block rows additionally
    carry `status` and an optional `projection` object.
  - Full schema lives in
    `docs/superpowers/specs/2026-05-20-v1.0.1-ccusage-alignment-part1-design.md`
    §Phase B.4.

Pipe targets such as `jq`, `dasel`, or downstream automation that already
worked against ccusage's JSON can be pointed at `tm` with no further
adapter.

### Color autodetection

`--no-color` actually takes effect now. v1.0.1 also adds isatty
autodetection plus the `NO_COLOR` and `FORCE_COLOR` environment variables
(see https://no-color.org/). Resolution order: JSON output → `--no-color`
flag → `NO_COLOR` env → `FORCE_COLOR` env → TTY check → off.

## v1.0.2 changes

v1.0.2 pushes the daily / weekly / monthly / session aggregation down to
SQL and ships two correctness fixes that affect how costs are reported.
The user-facing JSON / table envelopes are unchanged.

### `Mode=auto` correctness restored (post-push-down regression fix)

- **v1.0.1:** every entry was re-priced before aggregation. Codex rows
  with `cost_usd = 0` would fall back to a calculated cost, so a mixed
  day (Claude row with cost + Codex row with zero cost) reported the
  sum of both.
- **v1.0.2-rc:** aggregation moved to SQL, the `cost_usd` SUM was
  inspected once per bucket, and ModeAuto only fell back when the
  bucket SUM itself was zero. Mixed days silently dropped the Codex
  share — a clean ~18% under-report on the local test DB ($57 769 vs
  $69 284 with `--breakdown`).
- **v1.0.2:** cli now transparently flips `filter.Breakdown=true`
  whenever `Mode=auto` is requested without an explicit `--breakdown`,
  walks the per-(bucket, model) rows so each can independently trigger
  the zero-cost fallback, then folds back to one render row per bucket.
  The view is identical to v1.0.1; only the cost numbers match
  `--breakdown` now.

No action required. `tm daily --mode auto` after v1.0.2 will agree with
`tm daily --breakdown --mode auto` to the cent.

### `Mode=calculate` precision (small approximation, opt-in)

- **v1.0.1:** cli re-priced every entry before aggregation.
- **v1.0.2:** cli re-prices each SQL aggregate row. For single-model
  buckets the result is identical. For mixed-model buckets in the
  non-breakdown view, the recalculate uses the first model in the
  bucket's distinct model list as a unit-price proxy — accuracy is
  within $0.01 for typical workloads. Pass `--breakdown` for exact
  per-model recompute.

### `blocks --breakdown` is now wired

`tm blocks --breakdown` was accepted but ignored in v1.0.1. v1.0.2
emits `└─ <model>` rows under each block (boxed table) and a
`modelBreakdowns[]` array per block (JSON). Cost share is approximated
by token-occupancy because block cost is only persisted at the block
level.

### `session --json` exposes `projectPath` and `lastActivity`

New fields, populated straight from SQL `MAX(sessions.cwd)` and
`MAX(token_usage.timestamp)`. Scripts that hard-code the JSON key list
should either ignore unknown fields or explicitly select the ones they
need.

### `--timezone <IANA>` DST limitation

`--timezone <IANA>` resolves the offset at query time. For DST regions
(e.g. `America/Los_Angeles`), historical rows that crossed a DST
boundary may be off by 1 hour. Use UTC or a fixed-offset zone when DST
accuracy matters.
