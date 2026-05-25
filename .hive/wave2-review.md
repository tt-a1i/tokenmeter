# Wave 2 Acceptance Review

> Reviewer: 公孙弘  ·  时间: 2026-05-25  ·  范围: 7 commit (9e78818 → 2c8b307)

## 总评

Wave 2 的核心代码改动基本达成“向 ccusage 对齐”的目标：OpenCode SQLite、Droid sidecar、runtime pricing、Codex speed、响应式表格、blocks token limit、统一 config loader 都有对应实现和测试，且 7 个 commit 在各自 HEAD 上 `go test ./...` / `go vet ./...` 均通过。行为验证也确认 `tm --help`、`tm pricing refresh --offline`、`tm config show/path`、`tm blocks --token-limit 100`、`tm statusline --help` 都能运行到预期路径。

本轮阻塞点不在核心计算路径，而在用户可见文档和 help 同步：旧 TUI 文档仍有一份未清理，新增 flags 在命令 help 和 docs/site CLI reference 中不可发现，统一 config 文档与 2c8b307 的真实 JSON schema 不一致。按 Hive 约束，本次未启动子代理；这是主线程 degraded review，但关键证据已直接核查。

## 逐 commit Findings

### 9e78818 李诫 P0-3+P1-1

- ✅ ACCEPT
- 变更范围：`internal/collector/droid.go`, `internal/collector/droid_test.go`, `internal/collector/opencode.go`, `internal/collector/opencode_test.go`，范围匹配 OpenCode SQLite + Droid sidecar。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
- 覆盖判断：
  - OpenCode 覆盖 SQLite-only、SQLite 优先于重复 JSON、JSON fallback、channel DB 等路径。
  - Droid 覆盖 settings model 优先、sidecar model fallback、settings/sidecar 都缺 model 时 drop row。
- Findings:
  - 无 P0/P1。
  - [P2] `loadOpencodeDBEntries` 对 SQLite open/query/scan 错误采用 silent skip。作为 batch adapter 可接受，但后续若要改善可在 `--json` debug 或 verbose 模式暴露 per-root warning，避免纯 SQLite 用户只看到空结果。

### e0ba1c4 颜真卿 P0-1+P0-2

- ✅ ACCEPT
- 变更范围：runtime pricing、Codex speed、CLI routing/help/pricing path，范围合理。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - `/tmp/tm-review pricing refresh --offline` 输出 `pricing refresh skipped: offline mode`，exit 1，符合 offline refresh 不联网且失败提示的语义。
- 覆盖判断：
  - runtime pricing 覆盖 fresh cache、expired cache background refresh、offline skip、network failure fallback。
  - Codex speed 覆盖 auto 读取 `CODEX_HOME/config.toml`、explicit standard、explicit fast、fast tier cost multiplier。
- Findings:
  - 无 P0/P1。
  - [P2] help 中 runtime/shared flags 仍是分散维护，后续 b10272d/992ebd1 暴露了漏同步问题；建议把 shared flags help 从 `cli.ParseShared` 生成或集中复用。

### e13a38c 公孙弘 P0-4

- ⚠️ NEEDS-FIX
- 变更范围：`README.md`, `README_EN.md`, `CLAUDE.md`, `docs/*.html`，没有越过当时派单范围。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - 自审 grep：
    - `rg -n -i 'TUI|Bubbletea' README.md README_EN.md CLAUDE.md docs/*.html docs/*.md | grep -v -i 'migration\|v0.x\|historical\|removed\|deprecated'`
    - 仍命中 `docs/design.md` 多处非历史上下文。
- Findings:
  - [P1] `docs/design.md:5`, `docs/design.md:16`, `docs/design.md:23`, `docs/design.md:57-88`, `docs/design.md:110` 仍描述已移除的 TUI/Bubbletea 路线。触发条件：读者打开 `docs/design.md` 会看到 `tm` 启动 TUI、`tm tui`、四视图和 Bubbletea 技术栈，和当前 CLI-first 现实冲突。最小修复：把 `docs/design.md` 标为 `v0.x historical design`，或按 e13a38c 的口径改成 CLI daily / `tm watch` / `tm web` 架构。

### b10272d 李诫 P1-3+P1-5

- ⚠️ NEEDS-FIX
- 变更范围：render width/boxed 和 statusline flags/config tests，范围合理。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - `/tmp/tm-review statusline --help` 可运行，但未列出 context/burn-rate flags。
- 覆盖判断：
  - `ParseShared` 覆盖 `--compact`, `--context-low-threshold`, `--context-medium-threshold`, `--burn-rate-display`。
  - statusline 覆盖 flag overrides config。
  - render 覆盖 terminal width fallback/compact columns。
- Findings:
  - [P1] `cmd/tm/cmd_help.go:133-141`、`cmd/tm/cmd_help.go:199-205` 没同步 b10272d 新增的 `--compact`、`--context-low-threshold`、`--context-medium-threshold`、`--burn-rate-display`。真实解析在 `cmd/tm/cli/cli.go:51-57`，但 `tm daily --help` 只展示 `--speed/--offline`，`tm statusline --help` 完全不展示新增控制项。最小修复：为 daily/weekly/monthly/session 增补 `--compact`，为 statusline 增补 context 和 burn-rate flags，并加 help snapshot 或 command help test。

### 585b343 公孙弘 P1-7

- ✅ ACCEPT
- 变更范围：仅 `docs/site/**`，符合文档站骨架任务。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - sidebar link 逐项对照当前 `docs/site` 文件存在；`index.md` 明确 `tm` 等价 `tm daily`，没有恢复 TUI 误导。
- 覆盖判断：
  - VitePress package/config/sidebar/index/core guides/source stubs 都落地。
  - 未安装 node_modules / 未 build，符合派单约束，但也意味着没有 VitePress runtime link/build 验证。
- Findings:
  - 无 P0/P1。
  - [P2] 585b343 本身没有 build 验证；后续 CI 接入 VitePress 后应补 `npm install`/`vitepress build` 检查，避免 sidebar 或 markdown 语法问题延迟暴露。

### 992ebd1 李诫 P1-4

- ⚠️ NEEDS-FIX
- 变更范围：blocks CLI、blocks token-limit logic、render JSON/boxed，范围合理。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - `/tmp/tm-review blocks --token-limit 100` 在空数据下输出 `(no data in range)`，无崩溃。
- 覆盖判断：
  - 覆盖 `--token-limit max`、JSON `token_limit/usage_pct`、WARN 状态、render progress/status 字段。
- Findings:
  - [P1] `cmd/tm/cmd_help.go:190-196` 未同步 `--token-limit`。真实解析在 `cmd/tm/cli/cli.go:58`，但 `tm blocks --help` 只展示 `--active`；高价值功能不可发现。最小修复：补 `--token-limit N|max`、`--session-length`、`--json/--breakdown` 等 blocks 常用 shared flags，并用 command help test 锁住。

### 2c8b307 颜真卿 P1-2 一阶段

- ⚠️ NEEDS-FIX
- 变更范围：config model/loader、pricing/webhook legacy merge、CLI `config` 子命令、routing，范围合理。
- 验证：
  - `go test ./...` exit 0
  - `go vet ./...` exit 0
  - `/tmp/tm-review config show` 输出 zero-value unified config。
  - `/tmp/tm-review config path` 输出 `~/.tokenmeter/config.json` 解析后的路径。
- 覆盖判断：
  - 覆盖 explicit/env/local/global 搜索优先级、legacy pricing/webhooks fallback、malformed JSON、save/load。
  - 一阶段只实现 config model 和 pricing/webhook 接入；`defaults` / `commands` / `sources` 尚未实际套用到 CLI shared flags 或 source discovery。
- Findings:
  - [P1] docs/site 没同步真实 config API：`docs/site/guide/cli-reference.md:59-72` 缺 `tm config <show|path|init>`；`docs/site/configuration/unified-config.md:57-80` 使用 `no_color`, `session_length`, `token_limit`, `burn_rate_display`, `data_dir` 等 snake_case/flat source 示例，但真实 schema 是 `noColor`、`SourceConfig{defaults,commands}`（见 `internal/config/config.go:25-43`），且 `cmd/tm/main.go:245-267` 只把 `--config` 接入 pricing refresh/runtime pricing/config subcommands，尚未把 config defaults 应用到 daily/session/blocks/statusline。最小修复：要么把 docs 改成“当前支持：path/show/init + pricing/webhooks legacy merge；defaults/commands/sources 暂未生效”，要么在 P1-2 后续实现 defaults/commands/sources 并加端到端测试。

## 跨 commit 关注

### 1. cmd/tm/cli/cli.go shared flag 累积是否有序

解析层的 shared flags 已覆盖 ccusage 风格的核心参数：`--json`, `--mode`, `--speed`, `--order`, `--breakdown`, `--offline`, `--timezone`, `--project`, `--no-color`, `--compact`, `--jq`, `--config`, `--session-length`, `--active`, `--no-scan`，并新增 statusline / token-limit 控制。实现上偏离 ccusage 的主要问题不是解析，而是 help/docs 分散维护导致不可发现。

建议把 `cmd/tm/cmd_help.go` 中 daily/weekly/monthly/session/blocks/statusline 的 options 与 `cli.ParseShared` 集中对齐；否则每次新增 shared flag 都会继续漏到 help 和 docs。

### 2. 测试覆盖整体盘点

逐 commit 验证结果：

| Commit | go test ./... | go vet ./... |
| --- | --- | --- |
| 9e78818 | pass | pass |
| e0ba1c4 | pass | pass |
| e13a38c | pass | pass |
| b10272d | pass | pass |
| 585b343 | pass | pass |
| 992ebd1 | pass | pass |
| 2c8b307 | pass | pass |

关键路径覆盖较强：OpenCode/Droid adapter、runtime pricing cache/offline/fallback、Codex speed、statusline overrides、blocks token-limit JSON/render、config search/legacy merge 都有单测。未覆盖的 critical-ish path 是用户可见 help/docs 同步：没有测试能发现 `tm statusline --help` 漏掉新增 flags，也没有测试 docs/site CLI reference 是否包含新增命令/flags。

### 3. 文档同步是否完整

README / README_EN / CLAUDE.md 在 e13a38c 后不再误导 `tm` 启动 TUI，但 `docs/design.md` 仍是旧 TUI 设计稿且没有历史标识。docs/site 在 585b343 + 后续 97e2734 后覆盖了大部分 source 和 guide，但 `guide/cli-reference.md` 漏 `tm config`、`--compact`、statusline thresholds、`--burn-rate-display`、`--token-limit`。`configuration/unified-config.md` 与 2c8b307 的真实 schema/生效范围不一致。

## P0/P1 行动项（如有）

- [P1] 修复 e13a38c 遗留：更新或历史化 `docs/design.md`，清掉非历史上下文的 TUI/Bubbletea/`tm tui`/`tm` starts TUI 描述。
- [P1] 修复 b10272d + 992ebd1 遗留：同步 `cmd/tm/cmd_help.go` 的 daily/weekly/monthly/session/blocks/statusline help，至少补 `--compact`、context thresholds、`--burn-rate-display`、`--token-limit`，并加 help 输出测试。
- [P1] 修复 2c8b307 文档同步：更新 docs/site CLI reference 和 unified config 文档，使其匹配当前 `internal/config.Config` schema 与一阶段真实生效范围。

## 结论

- 非全部 ACCEPT；核心代码可进入下一波，但需要派 worker 修复上述 3 个 P1 文档/help 同步问题。
- 需要 fix 的 commit hash：`e13a38c`, `b10272d`, `992ebd1`, `2c8b307`。
- Findings 分布：P0=0，P1=4，P2=3。
