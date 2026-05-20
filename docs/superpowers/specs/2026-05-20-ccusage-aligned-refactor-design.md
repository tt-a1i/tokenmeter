# TokenMeter — ccusage 对齐重构（设计稿）

**日期**：2026-05-20
**作者**：tt-a1i（决策） · Orchestrator（spec 整理）
**状态**：草稿，待 user 审阅
**范围**：v1.0（本 spec 焦点）+ v1.1/v1.2 roadmap

---

## 1. 背景与动机

TokenMeter 与 [ccusage](https://github.com/ryoppippi/ccusage) 解决同类问题（AI 编码 agent 本地 token / 费用观测），但走两条完全不同的路：

| | TokenMeter（现状） | ccusage |
|---|---|---|
| 形态 | Bubbletea TUI + Web | 纯 CLI（无 TUI） |
| 数据流 | daemon + SQLite 长驻 | 每次 CLI 重扫 JSONL（stateless） |
| 数据源 | Claude + Codex（2 个） | 15 个 agent |
| Pricing | 手维护 | 嵌入 LiteLLM `model_prices_and_context_window.json` |
| 5h block | 无 | `blocks` 命令 + burn rate + projection |
| Statusline | 无 | `ccusage statusline` 接 Claude Code 状态行协议 |

User 反馈：
1. 现有 TUI 不够好看，投入产出比低
2. ccusage 风格的命令面更清爽，CLI + Web 两条产品形态足够
3. 希望借鉴 ccusage 的核心能力但保留 daemon + 实时性优势

本重构目标：**保留 TokenMeter 在实时性 / 工具调用粒度 / Web Dashboard 上的优势，砍掉 TUI，把 CLI 完全换成 ccusage 风格的命令面，并把 ccusage 的几项核心产品能力（5h blocks / statusline / LiteLLM pricing）迁移过来。**

---

## 2. 决策记录

本节记录 brainstorming 阶段 user 已拍板的决策。每条都是后续设计与实施的硬约束。

| # | 决策 | User 选择 |
|---|---|---|
| D1 | TUI 是否保留 | **砍掉** |
| D2 | daemon + SQLite 是否保留 | **保留**（daemon 写、CLI 读、Web 走 daemon） |
| D3 | CLI 命令面风格 | **完全抹平 ccusage 风格**（不双轨并存） |
| D4 | 实施节奏 | **分 3 阶段递进**（v1.0 / v1.1 / v1.2） |
| D5 | v1.0 scope 含哪些 ccusage 能力 | **5h blocks + statusline + LiteLLM pricing**（多 adapter 推到 v1.1） |
| D6 | `tm` 不带参数默认行为 | **显示 help 文本** |
| D7 | CLI 子命令数据源 | **直读 SQLite**（read-only, WAL）；不依赖 daemon 在运行 |
| D8 | 现有命令处理方式 | **保留作为 deprecated alias** 一个 major 周期，v2.0 再移除 |
| D9 | v1.0 是否同步动 Web 后端 | **不动**，Web 留 v1.2 一次性重做 |
| D10 | 5h block 时长是否可配置 | **加 `--session-length` flag**，默认 5h |
| D11 | statusline 默认 quota | **不设默认**；未配置 `~/.tokenmeter/statusline.json` 时显示数字但不染色 |
| D12 | CI 周期刷新 pricing 方式 | **每周自动开 PR**，作者手动 review + merge |

---

## 3. 架构变更（v1.0 完成后）

```
Claude Code hooks ──→ tm emit ──→ Unix socket ──┐
                                                │
Codex JSONL 日志 ──→ CodexWatcher ──→ daemon ───┤   (采集，写入)
                                                │
Claude JSONL ──→ ClaudeLogWatcher ──────────────┘
                                                │
                                                ▼
                  ~/.tokenmeter/data/tokenmeter.db (SQLite)
                                                │
                            ┌───────────────────┴───────────────────┐
                            ▼ (read-only, WAL)                      ▼
                  tm <subcommand>  (CLI, 直读)              tm web (HTTP, daemon)
                  daily/blocks/statusline/…                  Dashboard (v1.2 重做)
```

### 3.1 保留（继续维护）

| 组件 | 改动 |
|---|---|
| `internal/daemon/` | 无功能改动；socket 协议不动 |
| `internal/storage/` | **新增 read API**：`ListBlocks`、`ActiveBlock`、`BurnRate`、`Projection` |
| `internal/collector/claude.go`, `codex.go` | 无功能改动 |
| `internal/collector/pricing.go` | **改造**：底层数据源切换到 LiteLLM embed + 可选 fetch（详 §4.4） |
| `internal/event/` | 不动 |
| `internal/web/` | v1.0 不动；v1.2 重做 |
| `cmd/tm/main.go` 内的 `runEmit`/`runDaemon`/`runSetup`/`runUninstall` | 不动 |

### 3.2 删除（v1.0 一次性）

- `internal/tui/` 整包
- `go.mod` 中相关 dep：`bubbletea`、`lipgloss`、`glamour`、`termenv`、`runewidth`（确认无其他使用者）
- `cmd/tm/main.go` 中 TUI 启动逻辑（`run()` 默认进 TUI 的路径）

> 注：现有 `cost/report/status/top` 命令**入口不删**，按 D8 保留为 deprecated alias 直到 v2.0。本节"删除"仅指 TUI 相关代码。

### 3.3 新增

| 路径 | 职责 |
|---|---|
| `internal/blocks/` | 5h session block 切分、burn rate、projection、active block 探测 |
| `internal/statusline/` | Claude Code statusline JSON 协议（stdin → 渲染 → stdout） |
| `internal/pricing/litellm.go` | LiteLLM 数据加载 + 在线刷新 + 200k 阶梯 + fast multiplier |
| `internal/pricing/litellm-snapshot.json` | go:embed 编译时快照 |
| `cmd/tm/cli/` | 新 CLI 命令注册（ccusage 风格平铺，参考 `rust/crates/ccusage/src/cli.rs`） |
| `docs/MIGRATION-v1.0.md` | 旧命令 → 新命令对照表，给老用户看 |

---

## 4. v1.0 详细设计

### 4.1 CLI 命令面

#### 主命令（ccusage 风格平铺）

```
tm                       # 无参数 → 打印 help 文本（D6）
tm daily                 # 按天聚合（所有 source）
tm weekly                # 按周聚合
tm monthly               # 按月聚合
tm session [<id>]        # 按 session 聚合，或显示单个 session 详情
tm blocks                # 5h session blocks 表
tm blocks --active       # 仅显示当前活跃 block（含 burn rate、projection）
tm blocks --session-length 1h   # 自定义切分时长（默认 5h，对应 Claude API 限流窗口）
tm statusline            # Claude Code statusline provider（stdin → stdout）
tm claude daily          # 仅 Claude 数据源
tm codex daily           # 仅 Codex 数据源

tm web                   # 启 Web Dashboard（不变）
tm daemon                # 仅启 daemon（不变）
tm setup                 # 写入 ~/.claude/settings.json hooks（不变）
tm uninstall             # 卸载（不变）
tm emit                  # Claude hook 接收端（不变）
tm version               # 版本（不变）
tm help [<command>]      # 帮助
```

#### 共享 flag（参考 ccusage `SharedArgs`）

| Flag | 默认 | 说明 |
|---|---|---|
| `--since YYYYMMDD` | 无 | 起始日期 |
| `--until YYYYMMDD` | 无 | 结束日期 |
| `--json` | false | JSON 输出 |
| `--mode auto\|calculate\|display` | auto | 费用模式（详 §4.4） |
| `--order asc\|desc` | asc | 排序方向 |
| `--breakdown` | false | 按 model 拆分明细 |
| `--offline` | false | 仅用 embed pricing，不联网 |
| `--timezone TZ` | system | 时区（影响日期分桶） |
| `--jq EXPR` | 无 | JSON 输出过 jq |
| `--config PATH` | 无 | 配置文件路径 |

#### 现有命令 → 新命令映射（D8：保留为 deprecated alias）

老命令仍可用，但 stderr 打印：
```
warning: `tm cost` is deprecated and will be removed in v2.0. Use `tm daily` instead.
```

| 旧 | 新 |
|---|---|
| `tm cost` | `tm daily --today` |
| `tm cost week` | `tm weekly` |
| `tm cost today` | `tm daily --today` |
| `tm report` | `tm session` 或 `tm daily`（按 args） |
| `tm report --weekly` | `tm weekly` |
| `tm report --monthly` | `tm monthly` |
| `tm status` | `tm blocks --active`（业务接近） |
| `tm top` | `tm blocks --active` |
| `tm tag <id> "txt"` | 保留原命令但移到 `tm session --tag`（v1.0 双形式都接受） |
| `tm clean [days]` | 保留原命令（管理类，不归到聚合命令下） |
| `tm watch` | 保留原命令（实时事件流，与聚合查询不同维度） |
| `tm init` | 保留原命令（首次设置向导） |
| `tm doctor` | 保留原命令（自检/修复，运维类） |

> tag/clean/watch/init/doctor **不是 deprecated**，是因为它们语义上与 ccusage 不重合（ccusage 没有这些能力，TokenMeter 独有），保留原名。

### 4.2 5h Session Blocks（参考 ccusage `blocks.rs`）

#### 定义

一个 **session block** 是一段连续活跃的时间窗口，由"5 小时静默断开"切分。每个 block 起点向下 floor 到整点。

```go
type SessionBlock struct {
    StartTime    time.Time            // floor 到整点
    EndTime      time.Time            // StartTime + 5h
    ActualEnd    *time.Time           // 最后一个 entry 的 timestamp（block 关闭后才有）
    IsActive     bool                 // now < EndTime && now - last_entry < 5h
    IsGap        bool                 // 静默 gap（不含 entries）
    Entries      []EntryRef           // block 内所有 token usage 记录引用
    TokenCounts  TokenCounts          // 聚合 token
    Cost         float64              // 聚合费用
    Models       []string             // 用到的 model 列表
    BurnRate     *BurnRate            // 仅 active block 有
    Projection   *Projection          // 仅 active block 有
}

type BurnRate struct {
    TokensPerMinute float64
    CostPerHour     float64
}

type Projection struct {
    TotalTokens   uint64   // 按 burn rate 推算到 block 结束的总 token
    TotalCost     float64
    RemainingTime time.Duration
}
```

#### 算法（移植自 ccusage `identify_session_blocks`）

```
1. 按 timestamp 升序排序所有 LoadedEntry
2. 初始化 current_start = nil, current_entries = []
3. 对每个 entry：
   a. 若 current_start 不空：
      - since_start = entry.ts - current_start
      - since_last = entry.ts - current_entries.last().ts
      - 若 since_start > 5h 或 since_last > 5h：
        - 关闭当前 block（push 到结果）
        - 若 since_last > 5h，插入一个 gap block
        - current_start = floor_to_hour(entry.ts)
      - 否则继续
   b. 若 current_start 空：current_start = floor_to_hour(entry.ts)
   c. current_entries.append(entry)
4. 收尾：把最后的 current_entries 关成一个 block
```

#### Active block 判定

```
is_active = (now < block.EndTime) && (now - last_entry.ts < 5h)
```

#### Burn rate 计算

```
elapsed_minutes = (last_entry.ts - block.StartTime).Minutes()
burn_rate.TokensPerMinute = block.TokenCounts.Total / elapsed_minutes
burn_rate.CostPerHour     = block.Cost / elapsed_minutes * 60
```

#### Projection（active block）

```
remaining = block.EndTime - now
projection.TotalTokens = block.TokenCounts.Total + uint64(burn_rate.TokensPerMinute * remaining.Minutes())
projection.TotalCost   = block.Cost + (burn_rate.CostPerHour * remaining.Hours())
```

#### 存储

`SessionBlock` **不入库**，每次 `tm blocks` 调用时从 `token_usage` 表实时聚合。原因：
- 数据已在 `token_usage`，重算成本可控（典型用户 100k 行）
- 5h 切分依赖"现在时间"，写库会让数据时变
- 测试更容易（纯函数）

#### CLI 输出

```
$ tm blocks
PERIOD              MODELS                       TOKENS       COST    STATUS
2026-05-20 14:00    sonnet, opus                 1.23M     $12.34    ACTIVE
2026-05-20 09:00    sonnet                         567K      $5.67    closed
2026-05-19 22:00    -                              -            -    gap (3.2h)
...

$ tm blocks --active
Current 5h block (started 2026-05-20 14:00, ends 16:00 [38m remaining])
  Tokens: 1,234,567   Cost: $12.34
  Burn rate: 32,400 tok/min ($0.81/hour)
  Projection: ~1.85M tokens, ~$18.50 by 16:00
  Models: sonnet (78%), opus (22%)
```

### 4.3 `tm statusline` 子命令

#### 协议（Claude Code statusline provider）

**输入**（stdin，JSON）：
```json
{
  "model_id": "claude-sonnet-4-6",
  "session_id": "126b5856-...",
  "cwd": "/Users/admin/code/foo",
  "transcript_path": "/Users/admin/.claude/projects/.../session.jsonl"
}
```

**输出**（stdout，单行，可含 ANSI 颜色）：
```
🤖 sonnet  ▎ $12.34 / 5h (38m left, on track)  ▎ 1.2M tok
```

#### 渲染逻辑

```
1. 读 stdin JSON
2. 查 active block（直读 SQLite）
3. 渲染：
   - model 短名
   - 当前 block 已花费 / 5h 上限（参考 user 配置或固定 $30 默认 quota）
   - 剩余分钟数 + 是否 on track（burn rate 推算下不会超额）
   - block 总 token
4. 颜色：
   - on track（projection < quota） → 绿
   - approaching（projection 80%–100% quota） → 黄
   - over quota（projection > quota） → 红
```

#### 配置（`~/.tokenmeter/statusline.json`，可选）

```json
{
  "quota_usd": 30.0,
  "format": "compact|detailed",
  "color": true
}
```

**默认行为**（D11）：配置文件不存在时 statusline 仍工作，但**不染色**——仅输出 `🤖 model ▎ $cost (5h) ▎ tokens` 这种中性格式。`tm setup --statusline` 会引导 user 写入配置（含 quota）。

#### 与 Claude Code 集成

`tm setup --statusline` 会向 `~/.claude/settings.json` 写入：
```json
{
  "statusline": {
    "command": "tm statusline"
  }
}
```

### 4.4 LiteLLM Pricing 集成

#### 数据源

- **嵌入**：通过 `go:generate` 触发的脚本（`scripts/refresh-pricing.go`）拉 [`BerriAI/litellm/model_prices_and_context_window.json`](https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json)，过滤只保留 `claude-*`、`anthropic.*`、`gpt-*`、`openai/*` 等相关 model（参考 ccusage `build.rs:is_embedded_model`），压缩写入 `internal/pricing/litellm-snapshot.json`，提交进 git。`go build` **不依赖网络**——它读已 commit 的 snapshot（`//go:embed`）。`go generate ./internal/pricing/...` 是开发者 / CI 显式触发。
- **运行时**：默认尝试在线刷新（HTTP GET 同一 URL，10s timeout）；失败回退 embed；`--offline` 跳过在线刷新

#### 字段

```go
type Pricing struct {
    Input             float64
    Output            float64
    CacheCreate       float64
    CacheRead         float64
    InputAbove200K    *float64   // tiered: 200k+ context
    OutputAbove200K   *float64
    CacheCreateAbove200K *float64
    CacheReadAbove200K   *float64
    FastMultiplier    float64    // Codex fast vs standard
    MaxInputTokens    uint64
}
```

#### CostMode（参考 ccusage `CostMode`）

| Mode | 行为 |
|---|---|
| `display` | 直接信任 JSONL 里的 `costUSD` 字段（Claude 自己写的）；如该字段缺失则该行 cost 显示为 0 |
| `calculate` | 永远从 token + pricing 重算 |
| `auto`（默认） | 有 `costUSD` 字段就 display，没就 calculate |

> 解决"Claude 算 vs TokenMeter 算"对不上的问题。
>
> Codex JSONL **不含 `costUSD` 字段**，因此 `auto` 模式下 Codex 数据永远走 calculate 路径；`display` 模式下 Codex cost 为 0（行为预期，文档说明）。

#### 迁移

- 现有 `internal/collector/pricing.go` 表替换为 LiteLLM snapshot
- 现有 unit tests 用 fixture pricing，不影响
- 用户感知：可能某些已记录 session 的 `total_cost_usd` 与 LiteLLM 不一致（已落库），**不回填**；新 session 用新 pricing

#### CI

`.github/workflows/ci.yml` 加一步：
```yaml
- name: Regenerate pricing snapshot
  run: go generate ./internal/pricing/...
- name: Check pricing snapshot in sync
  run: git diff --exit-code internal/pricing/litellm-snapshot.json
```

每周 GitHub Action（新增 `pricing-refresh.yml`，cron `0 6 * * 1` UTC 周一早 6 点）自动跑 `go generate ./internal/pricing/...`，若有 diff 则**自动开 PR**（用 `peter-evans/create-pull-request` action），作者人工 review + merge 后入库。**不 push 到 main**，避免静默变价。

### 4.5 TUI 删除

#### 文件清单

完全删除：
- `internal/tui/` 整包（model.go / view_*.go / *_test.go ...）
- `cmd/tm/main.go` 中 `runTUI()` 和默认进 TUI 的逻辑

`go.mod` 移除：
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/glamour`
- `github.com/muesli/termenv`
- `github.com/mattn/go-runewidth`
- 间接依赖运行 `go mod tidy` 清理

#### 默认入口行为变更

`tm`（无参）：
- **旧**：检查 daemon → 启 daemon → 进 TUI
- **新**：打印 help 文本（含 `tm daily` / `tm blocks` 等示例）

`tm daemon` 行为不变（用户可显式启 daemon）。

---

## 5. v1.1 Roadmap：多 Adapter

参考 ccusage `adapter/` 目录平铺式（不抽 trait，每个 adapter 独立）。

待支持（按社区热度优先级）：
1. OpenCode
2. Amp
3. Gemini CLI
4. GitHub Copilot CLI
5. Goose
6. Codebuff
7. Hermes
8. Kilo / Kimi / Qwen / Droid / OpenClaw / pi（次优）

每个 adapter 约 0.5–1 天，含 fixture 测试。新增：
- `internal/collector/<name>.go`
- `internal/collector/<name>_test.go`
- `internal/collector/<name>_fixtures/` 测试数据

CLI：每个 adapter 对应 `tm <name>` 子命令（如 `tm opencode daily`）。

---

## 6. v1.2 Roadmap：Web 打磨

- 同步新增 blocks / statusline 等 API endpoint
- Web UI 增加 5h block 视图
- 性能优化（参考已有 perf milestones）
- a11y / 移动端响应式
- 具体内容 v1.0 ship 后再 brainstorm

---

## 7. 兼容性与迁移

### 7.1 数据库 Schema

**不变更**。`token_usage` / `sessions` / `tool_calls` / `agents` / `file_changes` 五张表保持现状。blocks 是计算视图（不持久化）。

### 7.2 Hook 兼容

- `tm emit` 不变（Claude hook 接收端）
- 旧的 `agmon emit` hook 识别保持（已有 legacy 检测代码）
- 已注册 hooks 重写：`tm setup` 重新跑一次即可

### 7.3 CLI 兼容

- 旧命令 `tm cost` / `tm report` / `tm status` / `tm top` 保留为 deprecated alias，stderr 打印迁移提示
- v2.0 移除（提前一个 major 周期通知）

### 7.4 配置文件

- `~/.tokenmeter/` 数据目录不变
- 新增 `~/.tokenmeter/statusline.json`（可选）
- 新增 `~/.tokenmeter/pricing-cache.json`（运行时刷新缓存）

### 7.5 Release Notes 模板

```markdown
## v1.0.0 - Breaking Changes

### Removed
- TUI（`tm` 默认不再进交互界面）

### Deprecated（v2.0 移除）
- `tm cost` → `tm daily`
- `tm report` → `tm session` / `tm weekly` / `tm monthly`
- `tm status` → `tm blocks --active`
- `tm top` → `tm blocks --active`

### Added
- `tm daily` / `tm weekly` / `tm monthly` / `tm session` / `tm blocks` / `tm statusline`
- 5h session blocks + burn rate + projection
- Claude Code statusline provider 集成
- LiteLLM pricing 数据源（自动周期更新）
- `--mode auto/calculate/display` 费用模式

### Unchanged
- daemon、SQLite schema、Web Dashboard、Hook 集成
```

---

## 8. 测试策略

### 8.1 单元测试

| 包 | 测试要点 |
|---|---|
| `internal/blocks/` | identify_session_blocks 边界（空、单 entry、跨 5h、含 gap、active 判定、floor_to_hour） |
| `internal/blocks/` | burn rate / projection 数值正确性（fixture 时间） |
| `internal/statusline/` | stdin JSON 解析 + 输出格式 + 颜色阈值 + 配置加载 |
| `internal/pricing/` | LiteLLM JSON 加载 + 200k tiered + fast multiplier + offline fallback |
| `internal/pricing/` | CostMode auto/calculate/display 三种路径 |
| `cmd/tm/cli/` | 每个子命令 happy path + flag 解析 + JSON 输出 |
| `cmd/tm/cli/` | Deprecated alias 触发 stderr 警告且仍然工作 |

### 8.2 集成测试

- `cmd/tm/e2e_test.go` 新增：从空 SQLite 跑完整 `tm daily` / `tm blocks --active` / `tm statusline` 流程
- 模拟 5h 块边界的 fixture（多个 entry 跨 5h 静默）

### 8.3 兼容性测试

- `cmd/tm/cli_test.go` 中现有 `TestRunSetupReplacesLegacyAgmonHook` 必须仍然通过
- 新增：`TestDeprecatedAliasStillWorks` 验证 `tm cost` / `tm report` / `tm status` / `tm top` 可调用 + 打印 deprecation warning

### 8.4 Snapshot / Golden

- `tm daily --json` / `tm blocks --json` / `tm statusline` 三个命令产 fixture-driven golden file，避免输出格式回归

---

## 9. 风险与缓解

| 风险 | 级别 | 缓解 |
|---|---|---|
| 老用户脚本断（migration 不彻底） | M | deprecated alias 保留至 v2.0 + MIGRATION.md + release notes |
| LiteLLM 价格表与已落库费用不一致 | L | 不回填；新 session 用新表；release notes 说明 |
| 5h block 切分边界 bug 影响计费判断 | M | 大量 fixture 测试 + 与 ccusage 同样输入对比 |
| Statusline 渲染慢（每次 prompt 都跑） | M | 直读 SQLite + WAL；目标 < 50ms；加 benchmark |
| TUI 删除遗漏依赖 | L | `go mod tidy` + CI typecheck |
| Web v1.0 不动但用户预期 | L | release notes 写明 v1.0 仅 CLI 改造，v1.2 才动 Web |
| Build 时拉 LiteLLM 失败 | M | 仓库内 `litellm-snapshot.json` 作为 always-present fallback；offline build 不依赖网络 |

---

## 10. Open Questions / 待 user 反馈

所有 brainstorming 阶段产生的 Open Question 均已解决（见 §2 决策记录 D10–D12）。如实施过程中发现新 ambiguity，回到本节追加。

---

## 11. 实施粒度建议（给 writing-plans 用）

按依赖关系排序的 work items（粗粒度，writing-plans 阶段细化）：

1. **新增 storage read API**（`ListBlocks`、`ActiveBlock` 等查询）
2. **新增 `internal/blocks/`**（纯函数算法 + 单测）
3. **改造 `internal/collector/pricing.go`** 接 LiteLLM + `--mode` 支持
4. **新增 `internal/statusline/`**（stdin/stdout 协议）
5. **新增 `cmd/tm/cli/` 命令注册**，把 `daily/weekly/monthly/session/blocks/statusline` 实现
6. **添加 deprecated alias**（旧命令 → 新命令调用 + stderr warning）
7. **删除 `internal/tui/`** + `go mod tidy`
8. **改 `cmd/tm/main.go` 默认行为**（无参 → help）
9. **更新文档** README / CHANGELOG / MIGRATION.md
10. **新增 CI workflow**：每周自动刷新 pricing snapshot
