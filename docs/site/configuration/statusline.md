# Statusline

`tm statusline` is a Claude Code statusline provider.

It reads one JSON object from stdin.

It writes one compact line to stdout.

Claude Code can call it when configured through statusline settings.

The command reuses TokenMeter's block calculation.

That keeps the statusline aligned with `tm blocks --active`.

## Basic Usage

Normally Claude Code runs this command for you.

Manual testing looks like this:

```bash
echo '{"model_id":"claude-sonnet-4-6","session_id":"demo","cwd":"/tmp/demo"}' | tm statusline
```

Claude Code's current hook input can also be used directly:

```bash
echo '{"session_id":"demo","cwd":"/tmp/demo","transcript_path":"/tmp/demo.jsonl","model":{"id":"claude-sonnet-4-6","display_name":"Claude Sonnet 4.6"},"cost":{"total_cost_usd":1.23}}' | tm statusline
```

If no active block exists, the output says there is no active block.

If a block exists, the output includes model, session cost, today's cost, active block cost, remaining time, and token count.

## Config File

The statusline config file is:

```text
~/.tokenmeter/statusline.json
```

Legacy installs may resolve through `~/.agmon/`.

Missing config is valid.

Missing config means no quota and no color.

Current fields:

```json
{
  "quota_usd": 25,
  "format": "compact",
  "color": true
}
```

`quota_usd` enables budget comparison inside the line.

`format` is intended for compact or detailed display.

`color` enables ANSI color when a quota is configured.

`--no-color` disables color at the command layer.

## Cost Mode

`tm statusline` accepts shared flags.

Statusline defaults to offline pricing refresh behavior, matching ccusage.

Use `--no-offline` when statusline may refresh runtime pricing.

`--mode auto` is the default for active block cost calculation.

`--mode display` prefers stored cost.

`--mode calculate` recalculates when pricing data is available.

`--cost-source auto` is the default for session cost display.

`--cost-source cc` uses the hook's `cost.total_cost_usd`.

`--cost-source ccusage` uses TokenMeter-calculated session cost.

`--cost-source both` displays both values.

The statusline uses the same pricing configuration path as report commands.

## Relationship To Blocks

The statusline reports the active five-hour block.

The default block length is five hours.

`tm blocks --active` is the debugging companion command.

If the statusline looks wrong, compare it with:

```bash
tm blocks --active
```

## TODO

Future docs should include the exact Claude Code statusline installation snippet.

They should also document any threshold fields if those become part of the runtime config.

For now, treat `quota_usd`, `format`, and `color` as the stable file-level fields.
