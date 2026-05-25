<p align="center">
  <img src="https://img.shields.io/badge/TokenMeter-AI%20Agent%20%E7%94%A8%E9%87%8F%E4%BB%AA%E8%A1%A8%E7%9B%98-7C3AED?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIiBzdHJva2U9IndoaXRlIiBzdHJva2Utd2lkdGg9IjIiPjxwYXRoIGQ9Ik0xMyAyTDMgMTRoOWwtMSA4IDEwLTEyaC05bDEtOHoiLz48L3N2Zz4=&logoColor=white" alt="TokenMeter" height="28">
</p>

<h1 align="center">TokenMeter</h1>

<p align="center">
  <strong>AI 编码 Agent 的本地用量仪表盘</strong>
</p>

<p align="center">
  <a href="https://github.com/tt-a1i/tokenmeter/releases"><img src="https://img.shields.io/github/v/release/tt-a1i/tokenmeter?style=flat-square&color=7C3AED&label=version" alt="版本"></a>
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <a href="https://github.com/tt-a1i/tokenmeter/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-22c55e?style=flat-square" alt="许可证"></a>
  <img src="https://img.shields.io/badge/%E5%B9%B3%E5%8F%B0-macOS%20%7C%20Linux%20%7C%20Windows-6B7280?style=flat-square" alt="平台">
  <img src="https://img.shields.io/badge/Claude%20Code-%E5%B7%B2%E6%94%AF%E6%8C%81-F59E0B?style=flat-square" alt="Claude Code">
  <img src="https://img.shields.io/badge/Codex-%E5%B7%B2%E6%94%AF%E6%8C%81-22C55E?style=flat-square" alt="Codex">
</p>

<p align="center">
  <a href="./README_EN.md">English</a>
</p>

---

> 用 CLI 报表和本地 Web Dashboard 监控 Claude Code、Codex 以及其他 AI 编码 Agent 的 Token 消耗、费用、工具调用和会话详情。
<p align="center">
  <img width="711" alt="image" src="https://github.com/user-attachments/assets/b1dc6609-868e-4c24-bfc0-73baa9c81432" />
</p>

<p align="center">
  <img width="711" alt="工具调用" src="https://github.com/user-attachments/assets/1fcc7162-0af7-49c3-93f4-2e947c531549" />
</p>

<p align="center">
<img width="711" alt="image" src="https://github.com/user-attachments/assets/90389a23-352d-4159-ad78-aed5a0b1a54a" />
</p>


## 功能

### Multi-source aggregation

- **Claude Code / Codex 实时采集** — hooks、JSONL watcher 和 SQLite 统一到同一套报表。
- **15 个本地 source** — Claude Code、Codex、OpenCode、Amp、Gemini CLI、GitHub Copilot CLI、Goose、Codebuff、Hermes、Kilo、Kimi、OpenClaw、pi-agent、Droid、Qwen。
- **OpenCode SQLite 支持** — 自动读取新版 `opencode.db`，同时保留 JSON 文件兼容。
- **Droid sidecar fallback** — Droid session 主 JSON 缺 model 时，会从 sidecar JSONL 回填。

### TokenMeter 独有实时能力（相对 ccusage）

- **Live event stream** — daemon 通过 Unix socket 接收 Claude hook 事件，并广播给 `tm watch` / Web Dashboard。
- **Web Dashboard** — `tm web` 提供费用趋势、热力图、模型/工具分布、会话详情和对话回顾。
- **Budget alerts + webhooks** — 月度预算、阈值提醒和 webhook endpoint 都在本地配置。
- **Tool error pattern analysis** — `tm analyze --tool-errors` 聚合失败工具、错误片段和高风险会话，这是 ccusage 没有的诊断视角。

### Accurate pricing

- **LiteLLM 运行期定价同步** — `tm pricing refresh` 更新 24 小时缓存；离线报告只使用本地缓存 / embedded fallback。
- **Codex speed tier** — `--speed auto|standard|fast` 支持 Codex 定价层；`auto` 会读取 `~/.codex/config.toml` 的 `service_tier`。
- **模型感知估算** — Opus / Sonnet / Haiku / GPT-5 / GPT-4.1 等模型按 token 类型拆分成本。

### Configuration and ergonomics

- **统一配置** — `tm config show` / `tm config init` 管理 `~/.tokenmeter/config.json`，并兼容 legacy `pricing.json` / `webhooks.json`。
- **窄终端 compact 表格** — `tm daily --compact` 在小窗口下输出更紧凑的汇总。
- **Blocks token limit** — `tm blocks --token-limit 100000` 显示 5 小时窗口进度条和硬阈值。
- **项目别名归一化** — `tm daily --instances --project-aliases '{"core":["/Users/admin/code/core"]}'` 可把多个路径归并到同一项目名。
- **单二进制** — `tm setup` 注入 hooks 后即可采集，核心功能无外部服务依赖。

## 支持平台

| 平台 | 接入方式 | 说明 |
|------|---------|------|
| **Claude Code** | Hooks + JSONL 日志监听 | `tm setup` 自动注入 hooks 到 `~/.claude/settings.json` |
| **Codex** | JSONL 日志监听 | 自动轮询 `~/.codex/sessions/` |
| **OpenCode** | SQLite + JSON 扫描 | 支持新版 `opencode.db` 和旧 JSON 文件 |
| **Amp** | JSON 扫描 | 读取本地 thread JSON |
| **Gemini CLI** | JSON / JSONL 扫描 | 读取本地 Gemini usage 文件 |
| **GitHub Copilot CLI** | OTEL JSONL | 读取 Copilot telemetry exporter 输出 |
| **Goose** | SQLite 扫描 | 读取 Goose `sessions.db` |
| **Codebuff** | JSON 扫描 | 读取 channel 内 `chat-messages.json` |
| **Hermes / Kilo** | SQLite 扫描 | 读取各自本地状态库 |
| **Kimi / OpenClaw / pi-agent / Qwen** | JSONL 扫描 | 读取本地 session transcript |
| **Droid** | JSON + JSONL 扫描 | sidecar JSONL 可补齐缺失 model |

## 安装

### 一键安装（推荐）

```bash
curl -sL https://raw.githubusercontent.com/tt-a1i/tokenmeter/main/install.sh | sh
```

### Homebrew Cask

仅当 release 流水线配置了 Homebrew tap 仓库和 `HOMEBREW_TAP_GITHUB_TOKEN` 时可用。
发布细节见 [docs/release.md](docs/release.md)。

```bash
brew install --cask tt-a1i/tap/tm
```

_Homebrew tap 仓库准备中；目前请用上面的一键安装脚本、`go install` 或 GitHub Releases 直装。_

### Go Install

```bash
go install github.com/tt-a1i/tokenmeter/cmd/tm@latest
```

### 从源码构建

```bash
git clone https://github.com/tt-a1i/tokenmeter.git
cd tokenmeter
make install
```

## 快速开始

```bash
tm setup                                           # 首次运行：注册 Claude hooks
tm daily --compact                                 # 今天所有来源的紧凑汇总
tm pricing refresh                                 # 从 LiteLLM 同步定价（首次需要联网）
tm config show                                     # 查看统一配置和 legacy merge 结果
tm config init                                     # 初始化 ~/.tokenmeter/config.json
tm blocks --token-limit 100000                     # 当前 5 小时窗口 + token 阈值进度
tm daily --instances --project-aliases '{"core":["/Users/admin/code/core"]}'
tm analyze --tool-errors                           # 工具失败模式分析
tm web                                             # 浏览器 dashboard（独立进程）
```

### Offline mode

```bash
tm pricing refresh --offline  # offline mode: exit 1 when no refresh is attempted
# pricing refresh skipped: offline mode
```

`--offline` 用于确认命令不会联网；它不是 quickstart 的 happy path。

正常使用 Claude Code 或 Codex，TokenMeter 在后台自动采集所有数据。完整命令对照参见 [docs/MIGRATION-v1.0.md](docs/MIGRATION-v1.0.md)。

TokenMeter v1.1 还支持 13 个 batch-only 数据源：OpenCode、Amp、Gemini CLI、GitHub
Copilot CLI、Goose、Codebuff、Hermes、Kilo、Kimi、OpenClaw、pi-agent、Droid、Qwen。
`tm daily` 默认扫所有已安装的 agent，加 `--no-scan` 退回到仅 SQLite (Claude + Codex)。
具体每个 source 的子命令和数据路径见 [docs/MIGRATION-v1.1.md](docs/MIGRATION-v1.1.md)。

## 命令

| 命令 | 说明 |
|------|------|
| `tm` | 默认等同于 `tm daily`，显示每日 Token / 费用汇总 |
| `tm daemon` | 仅启动 daemon |
| `tm daily --compact` / `tm weekly` / `tm monthly` | 按日 / 周 / 月汇总所有来源，支持窄终端 compact 表格 |
| `tm daily --instances --project-aliases JSON` | 展开项目实例，并用 alias 归一化工作区路径 |
| `tm session [id]` | 按 session 展示明细，可传 id 过滤 |
| `tm blocks [--active] [--token-limit N\|max]` | 5 小时窗口、burn rate、projection 和 token 阈值进度 |
| `tm statusline` | Claude Code statusline provider，支持 context 阈值和 burn-rate 展示 |
| `tm analyze --tool-errors` | 汇总工具失败模式、错误片段和高风险会话 |
| `tm watch [opts]` | 从 daemon socket 流式输出事件 |
| `tm share [session]` | 生成可分享的 Markdown 会话战报 |
| `tm export [opts]` | CSV / JSON 导出 |
| `tm web [--port N]` | 启动 Web Dashboard（默认端口 8370） |
| `tm clean [days]` | 清理 N 天前的历史数据（默认 7 天） |
| `tm tag <id> [text]` | 给会话打标签（省略 text 则清除） |
| `tm budget <subcommand>` | 管理预算 |
| `tm webhook <subcommand>` | 管理 webhook endpoint |
| `tm config <show\|path\|init>` | 查看、定位或初始化统一配置 |
| `tm pricing refresh` | 刷新 LiteLLM 运行期定价缓存 |
| `tm setup` | 配置 Claude Code hooks |
| `tm uninstall` | 卸载 hooks 并停止 daemon |
| `tm version` | 显示版本 |

> v0.x 历史形态中，`tm` 会进入 Bubbletea TUI。v1.0 起 TUI 已移除，`tm` 默认显示 daily 报表。旧命令迁移见 [docs/MIGRATION-v1.0.md](docs/MIGRATION-v1.0.md)。

## Web Dashboard

```bash
tm web              # 打开 http://localhost:8370
tm web --port 9000  # 自定义端口
```

浏览器面板功能：

- 费用面积图（Canvas 绘制，hover 显示详情，点击按天筛选会话）
- 模型费用占比 + 工具调用排行
- 会话列表（搜索、排序、费用色标）
- 会话详情（对话消息、工具时间线、文件变更、Agent 层级）
- 深色/浅色模式切换（自动检测系统偏好）
- 键盘导航（`j`/`k` 选择、`Enter` 打开、`/` 搜索、`←`/`→` 切换、`Esc` 返回、`?` 帮助）

## 架构

交互版架构图展示了完整数据流。下面是组件职责速查：

- **Daemon** — 通过 Unix socket 接收 Claude hook 事件，存入 SQLite，实时广播给 `tm watch` / Web
- **Claude hooks** — `PreToolUse` / `PostToolUse` / `SessionStart` / `SessionEnd` 等 8 个事件
- **日志监听器** — Claude watcher 扫描 `~/.claude/projects/` 的 JSONL 提取 token；Codex watcher 轮询 `~/.codex/sessions/`，内存去重
- **CLI** — `daily` / `weekly` / `monthly` / `session` / `blocks` / `statusline` 等命令读取 SQLite 或本地 source 日志输出报表
- **Web** — 独立 HTTP 服务 + 嵌入式 SPA，读取 SQLite，提供 REST API 与费用报表

> 交互版架构图（主题切换 + PNG/SVG 导出）：[`docs/architecture.html`](docs/architecture.html)
>
> ASCII 版速写：
>
> ```
> Claude Code hooks ──→ tm emit ──→ Unix socket ─┐
> Claude JSONL 日志 ──→ ClaudeLogWatcher ───────────┤
> Codex  JSONL 日志 ──→ CodexWatcher ───────────────┘
>                                                    ▼
>                                              tm daemon
>                                                    │
>                                          SQLite (~/.tokenmeter/data/tokenmeter.db)
>                                                    │
>                                    tm daily/session/blocks  ◄─────┴─────►  tm web
> ```

## 数据存储

```
~/.tokenmeter/
├── data/tokenmeter.db    # SQLite 数据库
├── tokenmeter.sock       # Unix domain socket（0600，仅当前用户可连接）
├── emit.log              # hook emit 错误日志（自动 10MB 截断）
└── daemon.pid            # PID 锁文件
```

从旧版升级时，如果本机已有 `~/.agmon/` 且尚未创建 `~/.tokenmeter/`，TokenMeter 会继续读取旧目录，避免历史数据丢失。

## 升级提示（v0.7.0+）

- **每日费用按本地时区分桶** — 之前按 UTC 0 点切日，UTC+8 用户上午看到的"今天"实际从昨天下午 4 点开始；v0.7 起 dashboard / Web 图表按本机日历日分桶。历史总额不变，**单日柱状值会因为切桶边界变化**。
- **首次启动会建索引** — 老库 reopen 时新建 3 个时间索引（token_usage / tool_calls / file_changes 的 timestamp）。几十万行的库可能多花几秒到十几秒，一次性。
- **Unix socket 现为 0600** — 之前是 0644，本机其他用户可能注入伪事件；新版 chmod 仅当前用户可 connect。无需手动操作。

## 配置 / 环境变量

- `INSTALL_DIR`（仅 `install.sh`）— 覆盖默认 `/usr/local/bin` 安装位置。
- HTTP 端口通过 `tm web --port N` 指定；Web Dashboard `?limit=N` query 参数可拉取 200 以外的 session 数量（cap 1000）。

## 卸载

```bash
tm uninstall        # 移除 hooks，停止 daemon
rm -rf ~/.tokenmeter        # 删除所有数据
```

## 许可证

[MIT](LICENSE)
