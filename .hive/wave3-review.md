# Wave 3 Acceptance Review

> Reviewer: 公孙弘  ·  时间: 2026-05-25  ·  范围: 5 commit (7215719 → 432baab)

## 总评

这 5 个 commit 把 Wave 2 之后的关键能力继续往前推：项目别名、tool error analysis、help regression tests、unified config 二阶段、首页同步都已经有实际代码或文档落点。核心 smoke 结果不是全绿：`7215719`, `5ec2cb3`, `892de23`, `432baab` 的主功能都能跑，但 `5ef334f` 在自己的 HEAD 上 `go test ./...` 命中过一次 `internal/projectalias.TestResolveFirstProjectWins` 失败，暴露的是 `7215719` 引入的 map iteration 非确定性。

本轮不是全部 ACCEPT。阻塞风险集中在用户可见入口：新增 flags 没进 help、`--config` 作为“shared flag”不能放在命令前、README quickstart 里有一条必然 exit 1 的命令。按 Hive 约束，本次未启动子代理；这是主线程 degraded review，关键证据已直接核查。

## 验证矩阵

| Commit | go test ./... | go vet ./... | 备注 |
| --- | --- | --- | --- |
| `7215719` | pass | pass | clean worktree |
| `5ec2cb3` | pass | pass | clean worktree |
| `5ef334f` | fail | pass | `TestResolveFirstProjectWins` 偶发/非确定性失败 |
| `892de23` | pass | pass | clean worktree |
| `432baab` | pass | pass | clean worktree |

用户可见命令验证在 clean `HEAD=432baab` worktree 构建 `/tmp/tm-review2` 后执行。关键结果：

- `tm daily --instances --help` 可运行，但 help 未列 `--instances` / `--project-aliases`。
- `tm analyze --tool-errors --help` 可运行，但 help 未列 `--tool-errors`。
- `tm analyze --tool-errors --range all` 能输出 Top Failing Tools；真实库中 `tool_calls.status='fail'` 共 810 行，`result_summary` 非空为 0，所以 Error Pattern Groups 为 `none` 不是 normalize bug。
- `tm --config /tmp/test-config.json daily --help` 失败：`unknown command: --config`。
- `tm config show --config /tmp/test-config.json` 可输出 `defaults.breakdown=true`。
- `tm pricing refresh --offline` exit 1，输出 `pricing refresh skipped: offline mode`。

## 逐 commit Findings

### 7215719 李诫 P1-6 项目别名 / --instances / --project-aliases

- ⚠️ NEEDS-FIX
- 变更范围：`cmd/tm/cli/*`, `internal/projectalias/*`, `internal/render/*`, `internal/storage/db.go`，范围匹配项目别名和 project column 渲染。
- Commit message：首行 32 字符，无品牌；合规。
- 测试：`go test ./...` pass；`go vet ./...` pass。
- 行为：`tm daily --instances --help` 不展示 `--instances`，`tm daily --project-aliases '{"test":["/x"]}' --help` 不展示 `--project-aliases`。

Findings:

- [P1] 新增 project alias flags 不可从命令 help 发现。`cmd/tm/cli/cli.go` 已解析 `--instances` 和 `--project-aliases`，但 `cmd/tm/cmd_help.go:133-142` 的 daily help 只列 `--speed`, `--compact`, `--offline`。触发条件：用户按 README 看到 `tm daily --instances --project-aliases ...` 后用 `--help` 查参数，help 不承认这两个高价值 flags。最小修复：在 daily/weekly/monthly/session/blocks help 加 `--instances`、`--project-aliases JSON|PATH`，并补 help regression test。

- [P1] `projectalias.Aliases` 的“first project wins”契约不成立，且测试可 flaky。`internal/projectalias/projectalias.go:40-47` 对 Go map 直接 range；`internal/projectalias/projectalias_test.go:48-55` 期望 `"first"` 赢过 `"second"`。Go map 无顺序保证，本轮 `5ef334f` clean worktree 的 `go test ./...` 实际失败：`Resolve="second" want first`。最小修复：不要承诺 first wins；改为检测同一路径重复 alias 并返回配置错误，或把 JSON schema 改成有序数组；至少先把测试改成确定性契约。

- [P2] weekly bucket 在普通路径和 `--instances/--project-aliases` 路径仍有跨年差异风险。storage pushdown 用 `strftime('%Y-W%W')`（`internal/storage/aggregate.go:179-180`），entry-level project path 用 Go `ISOWeek()`（`cmd/tm/cli/aggregate.go:174-180`）。现有 `TestAggregateUsageWeek` 只覆盖 2026-05 同年普通周，没有覆盖 1 月第一周或 12/31 ISO week-year。影响范围通常是每年年初至第一个周一前的 1-6 天，低频但会让 `tm weekly` 与 `tm weekly --instances` 在跨年边界不一致。最小修复：补跨年测试，并决定统一 ISO week 还是明确记录 SQLite `%W` 限制。

### 5ec2cb3 颜真卿 超越 #1 tool error pattern 分析

- ⚠️ NEEDS-FIX
- 变更范围：`cmd/tm/analyze.go`, `internal/storage/tool_errors.go`, `internal/render/tool_errors.go` 及测试，范围匹配 tool error analysis。
- Commit message：首行 23 字符，无品牌；合规。
- 测试：`go test ./...` pass；`go vet ./...` pass。
- 行为：`tm analyze --tool-errors --range all` 能输出失败工具排行；真实数据中 WebFetch 失败率 28.9%，但 pattern groups 为 `none`。

Findings:

- [P1] `tm analyze --tool-errors --help` 隐藏了新能力。实现解析在 `cmd/tm/analyze.go:72-88`，但 help 的 analyze options 仍只有 `--range` 和 `--json`（`cmd/tm/cmd_help.go:257-266`）。触发条件：用户不知道 `--tool-errors` 存在，只能从 README/docs 反查。最小修复：在 analyze help 加 `--tool-errors`, `--compact`, `--since`, `--until`，并补 `TestHelp_AnalyzeMentionsToolErrors`。

- [P2] tool error pattern 分析对现有真实库的诊断价值不足。真实 legacy DB `/Users/admin/.agmon/data/agmon.db` 查询结果是 `fail_total=810`、`nonempty_summary=0`，所以 `Error Pattern Groups` 为 `none` 不是 normalize 逻辑问题，而是历史/当前采集数据没有可归一化的 `result_summary`。`internal/storage/tool_errors.go:111-148` 完全依赖失败行的 `result_summary`。最小修复：验证 Claude `PostToolUseFailure` 是否稳定携带 `tool_result`，对空 summary 显示“no result summaries captured”而不是只写 `none`，并考虑用 params/status/session context 做 fallback pattern。

### 5ef334f 公孙弘 Wave 2 P1 fix

- ⚠️ NEEDS-FIX
- 变更范围：`cmd/tm/cmd_help.go`, `cmd/tm/cmd_help_test.go`, `docs/design.md`, `docs/site/configuration/unified-config.md`, `docs/site/guide/cli-reference.md`，范围符合 Wave 2 P1 文档/help 修复。
- Commit message：首行 35 字符，无品牌；合规。
- 测试：`go test ./...` 在 clean `5ef334f` worktree 失败于 `internal/projectalias.TestResolveFirstProjectWins`；`go vet ./...` pass。根因来自 `7215719` 的 map 顺序契约，不是本 commit 改动文件。
- 自审：6 个 help regression tests 在 clean `5ef334f` worktree 单独通过；在临时 worktree 删除 daily help 的 `--compact` 后，`TestHelp_DailyMentionsCompact` 失败，说明测试能抓住目标回归。

Findings:

- [P1] 该 commit 的 acceptance test 不是稳定绿。虽然本 commit 没动 `internal/projectalias`，但按“每个 commit 在自己 HEAD 上跑”的验收规则，`5ef334f` 当前不能直接 ACCEPT。最小修复同 `7215719` 的 duplicate alias 非确定性修复；修完后重跑 `go test ./...`。

- [P2] `docs/site/configuration/unified-config.md:52-68` 的 Current Effective Scope 在 `892de23` 后过时：文档仍写 `statusline` partial、`defaults`/`commands` not fully effective，但 892de23 已把 defaults/commands 应用到 `cli.ParseShared`，并把 unified statusline 配置接进 `RunStatusline`。最小修复：更新该章节为 `defaults ✅`, `commands.<name> ✅ with bool false limitation`, `statusline ✅`, `sources ⏳`。

### 892de23 李诫 P1-2 二阶段 statusline 集成 + defaults/commands

- ⚠️ NEEDS-FIX
- 变更范围：`internal/config`, `cmd/tm/cli`, `cmd/tm/main.go`，范围匹配 unified config 二阶段。
- Commit message：首行 34 字符，无品牌；合规。
- 测试：`go test ./...` pass；`go vet ./...` pass。
- 行为：`tm config show --config /tmp/test-config.json` 能输出 `defaults.breakdown=true`；`tm --config /tmp/test-config.json daily --help` 失败。

Findings:

- [P1] `--config` 被文档和 CLI reference 表达为 shared flag，但放在命令前会被顶层 `main` 拒绝。`cmd/tm/main.go:82-91` 直接 `switch os.Args[1]`，只有第一个 token 是 `daily|weekly|...|config` 才进入 `runCLIDispatch`；`cmd/tm/cli/cli.go:149-155` 的 `scanConfigFlag` 因此永远拿不到 `tm --config file daily` 这种常见写法。触发条件：用户按 ccusage/global-flag 习惯把 config 放在命令前，得到 `unknown command: --config`。最小修复：在 `main` 中把已知 shared flags 前置路由到 `runCLIDispatch(os.Args[1:])`，或先用 `cli.Route` 识别 CLI-first 命令再 fallback legacy switch；同时补 e2e。

- [P2] bool 默认值仍无法表达“命令级 false 覆盖全局 true”。`internal/config/config.go:26-40` 用普通 bool；`mergeDefaults` 只在 `src.JSON/src.Offline/src.Breakdown/src.NoColor/src.Compact` 为 true 时覆盖（`internal/config/config.go:221-244`）。触发条件：用户配置 `defaults.breakdown=true`，再想让 `commands.daily.breakdown=false`，JSON false 会和未设置不可区分。最小修复：把 bool 默认字段改为 `*bool` 或引入 presence-aware merge；这是 P2 follow-up，不阻塞当前能力上线。

- [P2] `sources` schema 仍未生效。`Config.Sources` 存在（`internal/config/config.go:20-21`），但 `applyConfigDefaults` 只读取全局 defaults 和 `commands[command]`（`cmd/tm/cli/cli.go:128-145`），没有 source-specific defaults/commands。最小修复：文档明确 `sources ⏳`，后续实现 adapter command 的 source override。

### 432baab 公孙弘 README/docs 首页同步

- ⚠️ NEEDS-FIX
- 变更范围：`README.md`, `README_EN.md`, `docs/site/index.md`, `docs/site/guide/cli-reference.md`, `docs/site/sources/index.md`，范围符合首页同步任务。
- Commit message：首行 36 字符，无品牌；合规。
- 测试：`go test ./...` pass；`go vet ./...` pass。
- 自审 grep：`rg -n 'agmon|TUI|Bubbletea' README.md README_EN.md` 只剩 v0.x historical TUI 和 legacy `~/.agmon/` 兼容说明，未恢复误导性 TUI 描述。

Findings:

- [P1] README quickstart 放入了必然非零退出的命令。`README.md:121-129` 和 `README_EN.md:116-125` 把 `tm pricing refresh --offline` 放在连续 quickstart 示例里；clean `432baab` binary 验证该命令 exit 1，输出 `pricing refresh skipped: offline mode`。触发条件：用户复制 quickstart 到带 `set -e` 的 shell/session，流程中断。最小修复：quickstart 用 `tm pricing refresh`，把 `--offline` 移到 “offline fallback check” 示例；或改变 `pricing refresh --offline` 语义为 exit 0 的 dry-run/diagnostic。

- [P2] 首页没有同步 `892de23` 已 in-main 的二阶段 config 生效范围。`docs/site/index.md:78` 只写 `tm config show/path/init` 暴露入口，README 也只写 show/init + legacy merge；没有告诉用户 defaults/commands/statusline 现在已接入 shared flags。最小修复：在 README Features / docs site What's New 增补 `defaults`、`commands.<name>`、statusline config 已生效，并标注 bool false limitation / sources 未生效。

## 跨 commit 关注

### 1. shared flags 与 help/docs 继续漂移

`cmd/tm/cli/cli.go` 的 shared flags 已有 `--instances`, `--project-aliases`, `--config`, statusline thresholds, `--token-limit` 等，但 `cmd/tm/cmd_help.go` 仍靠手写维护。5ef334f 增加的 6 个 help tests 能防住当时发现的 flags，但没有覆盖 7215719 的 project alias flags，也没有覆盖 5ec2cb3 的 analyze flags。建议把 shared flag help 集中生成，或至少为每个新 shared flag 加 command-level regression test。

### 2. unified config 当前真实范围

- ✅ 已生效：search path、legacy pricing merge、legacy webhooks merge、pricing runtime sync、webhooks model、`defaults`、`commands.<name>`、statusline unified config。
- ⚠️ 部分限制：bool 字段无法用 command-level false 覆盖 defaults true；`--config` 只能放在命令后，例如 `tm daily --config file`，不能放在命令前。
- ⏳ 未生效：`sources` section 对 adapter/source-specific defaults 的应用。

### 3. 测试覆盖整体盘点

新增功能的单元测试数量足够，但验收暴露了三个盲区：

- help regression tests 没覆盖所有新增 flags。
- project alias duplicate path 测试自身依赖 Go map 顺序，导致 clean commit 测试可失败。
- config e2e 没覆盖 `tm --config file daily` 这种全局 flag 位置。

### 4. ANSI 颜色 follow-up

`992ebd1` 的 `TestRenderBlocksTokenLimitColorsStatuses` ANSI 颜色问题本轮按 Orchestrator 指示只记录为外部已派 follow-up，不计入 P0/P1。

## P0/P1 行动项

- [P1] 修复 project alias help：daily/weekly/monthly/session/blocks help 增补 `--instances`、`--project-aliases JSON|PATH`，并加 regression test。涉及 `7215719` / `5ef334f`。
- [P1] 修复 projectalias duplicate path 非确定性：不要用 Go map 承诺 first wins；检测重复 path 或定义确定排序，并修复 flaky test。涉及 `7215719`，同时会让 `5ef334f` clean HEAD 测试恢复稳定。
- [P1] 修复 analyze help：`tm analyze --help` 增补 `--tool-errors`、`--compact`、`--since`、`--until`，并加 regression test。涉及 `5ec2cb3`。
- [P1] 修复 global `--config` 位置：支持 `tm --config file daily`，或文档/help 明确只支持命令后置；推荐支持前置并加 e2e。涉及 `892de23`。
- [P1] 修复 README quickstart：不要把 exit 1 的 `tm pricing refresh --offline` 放在连续 quickstart 示例。涉及 `432baab`。

## P2 Follow-up

- [P2] 增加 weekly cross-year 测试并统一 SQLite `%W` 与 Go ISOWeek 的边界行为。
- [P2] tool error report 对空 `result_summary` 给出明确诊断，并验证新采集是否能填充失败摘要。
- [P2] unified config bool 字段改为 presence-aware，以支持 command-level false 覆盖 defaults true。
- [P2] 更新 `docs/site/configuration/unified-config.md` 的 Current Effective Scope，反映 `892de23` 后的真实状态。
- [P2] README / docs site 首页补充 defaults/commands/statusline config 已生效，同时标注 `sources` 未生效。

## 结论

- 非全部 ACCEPT。
- Findings 分布：P0=0，P1=5，P2=5。
- P1 行动项数量：5。
- 需要 fix 的 commit hash：`7215719`, `5ec2cb3`, `5ef334f`, `892de23`, `432baab`。
- 自审发现：
  - `5ef334f`：help tests 本身有效，但该 commit clean HEAD 受 projectalias 非确定性影响，`go test ./...` 不能稳定通过；unified-config 文档在 892de23 后过时。
  - `432baab`：README quickstart 包含 exit 1 的 `tm pricing refresh --offline`；首页漏写 892de23 的 defaults/commands/statusline 生效范围。
