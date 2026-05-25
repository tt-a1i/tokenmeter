---
layout: home

hero:
  name: TokenMeter
  text: AI 编码 Agent 的本地用量仪表盘
  tagline: 用 CLI 报表和本地 Web Dashboard 监控 Claude Code、Codex 以及其他 AI 编码 Agent 的 token、费用、工具调用和会话详情。
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: CLI Reference
      link: /guide/cli-reference

features:
  - title: Real-time daemon
    details: Claude hooks and JSONL watchers feed a local daemon that writes SQLite and broadcasts live events.
  - title: Multi-source reports
    details: Daily, weekly, monthly, session, and block reports can merge Claude, Codex, and batch-only agent sources.
  - title: Web Dashboard
    details: The embedded web app provides charts, session detail, search, budgets, and export paths without a hosted service.
  - title: Budget alerts
    details: Monthly budgets, webhook events, and statusline quota hints help catch high-cost sessions before they surprise you.
---

# Overview

TokenMeter is a local observability tool for AI coding agents.

It tracks token usage, estimated cost, tool calls, file changes, and session metadata.

The current v1 command surface is a CLI-first workflow.

Running `tm` without arguments is the same as running `tm daily`.

The old v0.x interactive terminal UI has been removed.

Use `tm web` when you want a visual dashboard.

Use `tm watch` when you want a live event stream in the terminal.

Use the report commands when you want scriptable output.

## Quickstart

```bash
tm setup
tm daily
tm web
```

`tm setup` writes Claude Code hooks into `~/.claude/settings.json`.

`tm daily` scans installed sources and prints the daily token and cost summary.

`tm web` starts the browser dashboard, usually on port `8370`.

## What To Read First

Start with the getting started guide if you are installing TokenMeter for yourself.

Read installation when you need release artifact, Go install, or source-build details.

Read CLI reference when you are wiring TokenMeter into scripts.

Read source pages when one adapter is not finding your local agent logs.

Read configuration pages when you need offline pricing, statusline quota, budgets, or webhooks.

## Local-first Model

TokenMeter stores data under `~/.tokenmeter/` by default.

Existing legacy installs may continue using `~/.agmon/` until a new TokenMeter directory exists.

The main database is `data/tokenmeter.db`.

The daemon uses a local socket.

No hosted service is required for normal operation.
