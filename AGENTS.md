# easyeda-agent

AI-native automation layer for **EasyEDA Pro (嘉立创EDA专业版)**. A skill drives a
Go daemon, which dispatches typed schematic actions to a connector extension
running inside EasyEDA, which calls the official `eda.*` API.

```
skill ──▶ Go CLI/daemon ──WebSocket──▶ connector .eext ──▶ eda.* API
          (typed actions)   60832-60841   (in EasyEDA Pro)
```

## 官方插件库调研参考
文章：docs/ecosystem-survey.md，遇到什么不确认的情况可以来这里参考分析，并更新认知到相应文档；

## 核心概念拉通认知
[`docs/concepts.md`](docs/concepts.md) = 布局/布线域的**共享词汇表**(网 / 网感知 vs 几何 /
布局分档 T1–T4 / edge 语义 / 块数据模型 / 可信判据)。**引入或讨论新概念对象先落这里再引用**,
让后续会话、贡献者、Skill 用同一套心智模型。验收判据见 [`docs/e2e-automation-acceptance.md`](docs/e2e-automation-acceptance.md)。

## 首要准则 — 样例驱动、参数化迁移

原理图、PCB 布局和布线统一采用“找到相近样例 → 理解理由 → 修改参数 → 执行 →
观察真实结果 → 修正”的工作方式。样例必须写来源、开始状态、参数与单位、实际命令、
回读、错误修法和验证状态；没有完成态答案的题目只能先标 source-only，完成现场复现后
才能标 live-verified。参数化数据仍是可重算来源，样例不是复制固定坐标的借口。

workflow/stage、版本一致性、布局评分和 stale-read 状态只提供诊断与兼容记录，不决定
普通 action 是否允许执行。连接检查、DRC、几何和回读继续报告具体事实。需要可信最终数据的
批次必须保存、真实重载并回读；刷新失败就报告不可用，不以 force 或阶段签字代替证据。

PCB Layout 全部要求落实并保存重载后，必须向用户展示当前布局事实和预览，等待用户对该回读
版本明确确认；用户可继续描述调整，也可自行调整后回复“OK”。自行调整后先重新回读并把现场
坐标/角度固化为参数基线。确认前不进入整板布线。LDO/DCDC 等电源模块的输入/输出电容、局部
GND 回流与必要 EP/地过孔可以随布局先完成，但必须整体参数化、回读，并在模块移动时一同重算；
跨模块电源主干、普通信号和全局铜仍等待确认。该确认是用户设计取舍，不恢复 workflow/stage、
评分或版本许可门禁。

参数化 Layout 后必须用 typed 截图/导出能力生成**整板集成预览**，并连续通过两轮 Agent 自检。
第 1 轮检查空间、模块关系和视觉异常；发现问题就修改参数、重新生成预览，连续通过计数清零。
第 2 轮必须执行 `save → reload → fresh dump → fresh render`，核对持久化后的数据与整板图；这一轮
发现并修复任何问题也清零，重新从第 1 轮开始。只有连续两轮均未发现需要修复的明显问题，才可
称 Layout 完成并把该回读版本交用户确认；截图不替代对象回读、几何检查或 DRC。

Layout 观察时可以临时隐藏元件属性，但只能通过 typed API 改变**视图状态**。操作前必须读取并
保存旧可见性状态，无论截图/导出成功或失败都要恢复并回读对账，不得修改属性内容或把临时状态留在工程中。没有
可回读、可恢复的 typed 接口时标 `unsupported`，保持原视图继续检查；禁止用 GUI/属性面板兜底。

## 首要准则 — 禁止手工操作 EDA 工程

本项目的现场 EDA 操作与验证使用用户已打开的内置浏览器 Web EDA；不得启动或切换到
EasyEDA 桌面版。

Agent 不得使用 CUA、鼠标、键盘、画布、属性面板、工程树或其他 GUI 自动化来创建、修复、
补齐、保存、重载或验证原理图与 PCB，也不得把手工编辑作为 typed 工具失败后的兜底。所有
工程写入必须来自可审计的参数化数据，并通过 `easyeda` Cobra 子命令、typed action 或
`easyeda apply` 执行；任意 `debug.exec_js` 不能用于绕过缺失的设计 action。

缺少接口、宿主持续加载或对象不可读时，立即停止该现场写入，保存错误、输入和已知状态，
将能力标为 `planned` / `unsupported`，先在代码中补齐 typed 接口和自动化验证，再重新执行。
不得通过刷新浏览器、从工程树重开、拖动物件或修改属性面板来恢复任务。截图和界面观察只可
作为只读证据，不能产生工程变更，也不能替代对象回读。

Web 项目已打开不代表 connector 已连接；`easyeda health` 的 `windows` 必须精确出现目标
工程和文档后才可访问 EDA。同一窗口的 typed 调用串行执行，subagent 只并行做离线分析，或在
主 Agent 停止访问该窗口时做只读核查，避免多个 `--doc` 选择/回读相互触发文档过渡保护。

## 首要准则 — 原理图数据驱动架构

所有原理图设计、布局、检查、修复都先读并遵守随 Skill 发布的
[`数据驱动架构基准`](skills/easyeda-agent/references/schematic-data.md#数据驱动架构基准)；
职责图见 [`docs/architecture.md`](docs/architecture.md)，操作见
[`auto-layout-sop.md`](skills/easyeda-agent/references/auto-layout-sop.md)。
原始快照保留 → 源数据副本明确连接/核心外围归属/约束 → 区内和纸张两层计算 → 数据检查
→ 固定转换/Apply → 原始回读对账；失败修源数据、采集或算法再重算，不以现场逐件试摆兜底。
核心及专属外围必须整体跟随；同网/同框、碰撞为零或高分不能代替所有权与真实直连检查。
位号参与遮挡/入框，型号/参数/描述等非位号属性文字排除页面碰撞和框包络，原始属性保留。
截图仅辅助发现采集/规则遗漏，必须转为数据回归；缺测、缺生成溯源或未确认保存不能称完整通过。
旧九宫格/三层 tidy/move/历史验收记录不覆盖本基准。改文档不等于工具已实现或现场已验证。

## 首要准则 — Skill 优先

> **本项目是「边开发、边更新 Agent Skill」的联合开发模式。**
>
> - **开发和测试的主要对象是 Skill**（唯一对外入口 `skills/easyeda-agent/`）。
> - Go CLI/daemon（`cmd/easyeda` + `internal/`）和连接器插件（`extension/`）是**为 Skill 服务的基础设施**，而非最终目的。
> - 每次改动首先问：「Skill 里的工作流、知识、或 guardrail 需要同步更新吗？」——如果需要，先改 Skill，再改底层实现。
> - 修改底层 action / daemon / 插件后，必须同步更新 Skill 里对应的工具描述、示例、或注意事项。

## 首要准则 — CLI 子命令设计

详见 [`docs/cli-design.md`](docs/cli-design.md)。核心约束：所有明确的功能模块必须以 **Cobra 子命令**方式暴露（`easyeda sch`、`easyeda pcb`、`easyeda bom` …），`--help` 自描述，新功能先设计命令接口再写实现，Skill 描述与子命令签名保持同步。开发闭环：官方 API/离线 fixture 调研 → typed action → Cobra 子命令；不得用 `debug.exec_js` 临时操作工程来跳过接口开发。

## 首要准则 — 固定测试用例（端到端验收）

**每次做端到端测试，都必须把 [`esp32MiniRequire.md`](esp32MiniRequire.md) 的
**「一、客户原始需求」那一节**（4 层板 + 点灯 + 5V 供电端子 + 降压到 3V3 + CH340 USB
烧录 + BOOT/RESET 按键 + 四角 M3 固定，**故意不含 BOM/UUID/网表**）当输入，让 agent 自己
选型 → 放置 → 编组 → 布线 → `sch layout-lint` → DRC → 转 PCB（4 层叠层 / GND 内电层 /
丝印极性 / 天线 keepout）→ save 完整跑一遍**——照 `skills/easyeda-agent/references/design-flow.md`
流程脊柱（S0–S6 + P0–P10），不是只测单点，**也绝不喂加工过的答案**（喂好 BOM/网表就不叫真实场景了）。
这是 agent 从需求到成品的回归基准：layout-lint / autosave / design-flow / 连接器 任何改动后都重跑此用例。
验收：需求条条落实（0 overlap、0 fatal、网络连通、丝印/极性正、4 层电源树、已落盘）。
测试工程用 `--project ceshi`，测完清理还原。

同一份文件的**「二、怎么跑完这个 Demo」是给人看的 runbook**（环境自举、分段验收表、
会被问到的决策题目、验收命令、已知坑、收尾），**不是喂给 agent 的输入** —— 它只写
「你会被问到哪些题」，不写答案，所以不构成加工过的答案；但跑回归时仍然只交第一节。

## Notes

reply as chiense! reply as chiense! reply as chiense!

**Branch policy:** use `dev` as the default branch for ongoing local development
and integration. Commit and push verified day-to-day work directly to `dev`; do
not create per-task feature branches unless the user explicitly asks for one.
Keep local `main` clean and aligned with `origin/main`. External PRs may still be
reviewed and merged into `main`; after such merges, merge `main` back into `dev`
before continuing so the development branch contains the latest accepted work.
Promote `dev` to `main` only as a deliberate integration step after the required
checks pass. Never discard diverged local work while cleaning branches: preserve
it on `dev`, push it, then repair local tracking pointers.

**Pushing code is not a release.** Development commits, `-dev.N` manifest values,
and pushes to `dev` or `main` do not publish a version. A signed/annotated release
tag `vX.Y.Z` created by `make release VERSION=vX.Y.Z` is the sole source of truth
for a published version and its release assets. Do not create or push a release
tag, GitHub Release, ClawHub release, or skillhub.cn release unless the user has
explicitly selected/approved that release version. When the user asks to submit
fixes, update GitHub progress, or handle PRs, that authorizes committing and
pushing the related verified code without a second push confirmation, but it does
not by itself authorize a release tag. Close fixed issues or fully adopted PRs
with links to the adoption commit or approved release; keep unresolved issues open
and report remaining validation gaps accurately.

## Layout

| Path | What |
|---|---|
| `cmd/easyeda` + `internal/{app,daemon,protocol}` | Go CLI + daemon. `internal/protocol/actions.go` = the 20 typed actions. Daemon: `/health`, `/eda` (connector WS), `/action`. |
| `extension/` | TypeScript connector → esbuild → `.eext`. `src/transport.ts` (port-scan + auto-reconnect), `src/actions.ts` (eda.* handlers + `connect_pin`). |
| `skills/easyeda-agent/` | Merged public skill — short `SKILL.md` router plus `references/` for design flow, schematic, PCB, conventions, canonical data, and `scripts/` for lint/BOM/parts/calibration tools. |
| `docs/FEATURES.md` | Feature-status inventory (20 actions grouped by capability) + roadmap. |
| `docs/pcb-design-rules.md` | PCB 设计规范手册 — 线宽/间距/过孔/布局/走线/铺铜/Mark点/拼板/叠层/DRC 清单，基于 JLC 工艺能力 + IPC-2221。 |
| `skills/easyeda-agent/SKILL.md` | The user-facing skill. |

## Dev workflow

**Keep the daemon hot-reloading while you work** (rebuilds + restarts on any `.go`
change; the connector auto-reconnects because it port-scans 60832-60841 in the
background):

```bash
make dev          # air live-reload of `easyeda daemon` — leave running in a terminal
```

Requires [air](https://github.com/air-verse/air): `go install github.com/air-verse/air@latest`.
Config is `.air.toml`: on any `.go` change it runs `make dev-build` (version-stamped
build → `./bin/easyeda` **and** a best-effort copy to `$PREFIX/bin/easyeda`), then
runs the daemon from that same `./bin/easyeda`. **So the `easyeda` CLI on your PATH
is refreshed on every rebuild — daemon and CLI never drift.** (Before this, air only
rebuilt the daemon; the PATH CLI stayed frozen at the last `make install`, so a new
subcommand like `easyeda doc` was missing until you reinstalled.) If `$PREFIX/bin`
isn't writable, air prints a warning and you run `make install` once with sudo to fix
perms. The dev binary is git-describe-stamped (e.g. `v0.5.1-19-g…-dirty`); a
non-clean stamp is treated as "dev" by the `health` connector-version check, so it
never false-flags a connector as stale against a dev daemon.

Other targets:

```bash
make build        # bin/easyeda (version-stamped via git describe)
make install      # build + install to /usr/local/bin (PREFIX overridable; sudo only if needed)
make daemon       # one-shot daemon (no reload) — prefer `make dev`
make test         # go test ./...
make lint-test    # linter rule-trust harness (orientation consistency + fixtures)
make blocks-audit # 块引脚引用 vs 真实符号引脚表(离线;首审揪出 14 个块 41 处错)
make layout-calibrate # layout-score 金标准板回归(离线):参考板九维不该掉分 +
                  # 负对照九维必须还会响。改 pcb_score_*.go 的判据/阈值/权重后先跑它。
                  # fixture 与「怎么加一块真板」见 internal/app/testdata/boards/README.md
make actions      # print the typed action catalog
make eext         # bump PATCH + build importable .eext, STABLE uuid (update in place: uninstall old → import)
make eext-fresh   # fallback: bump PATCH + FRESH uuid (imports as a new entry; delete the old one) — for when the installed one won't uninstall
make connector    # build .eext at the current version/uuid (no bump — same-version dev only)

skills/easyeda-agent/scripts/lint.sh <project>          # live lint (DIFF if a baseline exists)
skills/easyeda-agent/scripts/lint.sh <project> --save   # full lint + record baseline
```

## Release workflow

```bash
# 一条命令发版：自动把 connector + CLI 统一到同一版本，交叉编译 5 平台，
# 打包 skills.tar.gz，创建 GitHub Release 并上传所有 assets。
make release VERSION=v0.5.1

# 用户一行安装
curl -fsSL https://raw.githubusercontent.com/zhoushoujianwork/easyeda-agent/main/install.sh | sh

# 装过之后的升级路径（不必再跑脚本）
easyeda update            # CLI 二进制 + skill 目录 → latest
easyeda update --check    # 只读三方版本表（cli / skill / connector）
```

**`easyeda update` 与发版的契约**：`make release` 会生成并上传 `checksums.txt`（裸文件名，
`internal/selfupdate` 按 release asset 名匹配）——`update` 有它就校验 sha256，没有（旧 release）
就降级成「跑一次下载的二进制、比对版本号」。**改动 release 资产命名 = 改动自更新的输入**，
两边要一起改：`Makefile` 的 `release` 目标 ↔ `selfupdate.AssetName`。

**版本号约定**：CLI、connector 和 skill 始终用同一版本号（`make release` 负责把 `extension.json` **和 `SKILL.md` 的 `metadata.version`** 同步到 VERSION，不需要提前跑 `make eext`）。skill 侧同步脚本是 `scripts/sync-skill-version.py`（`--check` 只校验不写，可单独跑）——**改 frontmatter 时别动 `  version:` 那行的两空格缩进格式**，脚本按它定位。注意与安装态的 `.version` 标记文件区分：那是 `easyeda update` 写在 skill 目录里的运行时标记（`internal/selfupdate`），`metadata.version` 是随包发布、离线可读的声明式元数据，发版后两者同值。`make release` 会自动打 git tag、push 并创建 GitHub Release，**并把 skill 同版本发布到 ClawHub**（best-effort，失败不阻断；重试 `make publish-skill VERSION=…`，需已 `clawhub login`）。ClawHub 版本号不可覆盖；`publish-skill` 必须用绝对路径——clawhub 的 workdir 会被全局配置劫持到 `~/clawd`，相对路径会把旧副本发上去（0.8.1 踩过）。

**skillhub.cn 走 CI 自动发布**（订正旧结论「skillhub.cn 无 CLI API（纯网页社区），不集成」——**现在有真 CLI 了**）。链路：`make release` 里的 `gh release create` 发出 `release: published` → `.github/workflows/publish-skill.yml` 触发 → 跑 `make publish-skill-hub VERSION=…`。**`release` 目标本身不用改**，也不要把 skillhub 塞进它的主流程。

- **token**：仓库 secret `SKILLHUB_TOKEN`（`skh_` 开头，建于 https://skillhub.cn/dashboard/keys ）。skillhub CLI **原生读同名环境变量**（优先级 `--token` > `SKILLHUB_TOKEN` > `~/.skillhub/credentials.json`），所以 CI 里**不跑 `skillhub login`**、凭据不落 runner 磁盘。**任何情况下都不要把 token 写进文件或 echo 出来。**
- **手动补发**：`export SKILLHUB_TOKEN=skh_xxx && make publish-skill-hub VERSION=v1.0.3`，或在 Actions 页手动跑该 workflow（`workflow_dispatch` 填版本号）。只想验打包不真发：`make publish-skill-hub VERSION=… SKILLHUB_DRY_RUN=1`（不需要 token）。
- **两套 frontmatter 规范互斥，所以必须发 staging 副本**：skillhub 硬性要求顶层 `slug` + `displayName`（缺一个直接 die），而官方 Agent Skills 规范验证器 `npx skills-ref@latest validate` **明确拒收这两个字段**。因此 repo 里的 `SKILL.md` 保持 spec 干净，`publish-skill-hub` 把 skill 拷到临时目录、只往副本注入这两个键（**幂等**：哪天 `SKILL.md` 自带了就自动跳过）。**别为了发 skillhub 去改 repo 的 `SKILL.md`，那会当场破坏 `skills-ref` 门禁。**
- **版本号**：workflow 从 release tag 取（剥 `v` 前缀），经 `skillhub publish --version` 覆盖，所以**不依赖 `SKILL.md` 的 `metadata.version`**，tag 是唯一版本源。必须是合法 SemVer（`v1.0` 会被拒）。
- **版本不可覆盖**（同 ClawHub）：同 slug 同 version 重发被服务端拒（409），补发请升版本号。发布后进 `pending_review` 审核队列，不是立刻可见。
- **装 CLI 只认官方脚本**：`curl -fsSL https://skillhub.cn/install/install.sh | bash -s -- --cli-only`（装到 `~/.local/bin/skillhub`）。
- **`skillhub` 这个 bin 名被两个项目占用，且可能同时在 PATH 上**（本机实测就是：homebrew/npm 的 `skillhub`（skills.palebluedot.live，Node）排在官方前面）。冒牌货的 `publish` 只有 `--namespace/--visibility/--registry`，没有 `--version/--host` —— 轻则炸在 `unknown flag: --version`，**重则静默发布到错误的 registry**。所以这里**不做「注释警告」，做机械检查**：`make skillhub-check`（判据 = 探 `publish --help` 是否暴露我们真正会传的 `--version/--host/--changelog/--dry-run`，绑定「用得上的能力」而非版本号字符串；不靠路径判断）。`publish-skill-hub` 内部先跑同一套解析，CI 里也是独立一步。解析顺序：`$SKILLHUB_BIN` → `~/.local/bin/skillhub` → `~/.skillhub/skills_store_cli.py` → PATH 上所有 `skillhub`，**每个候选都要过身份校验，不过就跳下一个**。要钉死用哪个：`export SKILLHUB_BIN=~/.local/bin/skillhub`。
  > 教训：这个坑我第一版**只写进了注释**，然后当场踩中（`command -v skillhub` 存在性检查选中了冒牌货）。**注释拦不住 PATH 解析——知道风险却只写成文字，遵守率上不去，必须做成机械检查。**

**Changelog 门禁**：`extension/CHANGELOG.md` 必须有对应版本的 `## [x.y.z]` 条目。`make release` 会**硬校验**（缺条目直接报错退出，发版前先补 changelog）；`make eext`（dev 循环）只**警告**不阻断。校验逻辑在 `extension/scripts/bump.mjs`（`--require-changelog`）。

## Skill scripts usage

All tools live in `skills/easyeda-agent/scripts/`.

```bash
# 原理图 lint
skills/easyeda-agent/scripts/lint.sh <project>           # 实时 lint；有 baseline 时只显示 DIFF
skills/easyeda-agent/scripts/lint.sh <project> --save    # 全量 lint + 记录 baseline

# BOM 补全 LCSC C 号（导出后运行）
skills/easyeda-agent/scripts/bom-enrich.py <bom.tsv>             # 输出到 stdout
skills/easyeda-agent/scripts/bom-enrich.py <bom.tsv> --out <out> # 写入文件

# 器件选型
skills/easyeda-agent/scripts/parts-select.py --help

# calibrate.js 仅作历史算法参考，不再粘贴到 EDA 的 debug.exec_js。
# 需要重新校准时先提供 typed 校准 action/Cobra，再由参数化命令运行与回读。
skills/easyeda-agent/scripts/calibrate.js

# lint 规则信任测试
make lint-test    # = python3 skills/easyeda-agent/scripts/tests/run.py

# 块引脚引用审计 —— 块按功能名引用引脚,此前无人对过真实符号,导致块标着
# verified 却静默错接(ch340c 的 USB 口根本没供电)。离线判定,非零退出可 gate。
skills/easyeda-agent/scripts/blocks-pin-audit.py            # 审全库(离线,用引脚表快照)
skills/easyeda-agent/scripts/blocks-pin-audit.py --probe    # 刷新快照(需连编辑器)

# 暴露面健康度体检 —— 读 ~/.easyeda-agent/audit/*.jsonl,离线,不需要连编辑器。
# 出「调用分布+失败率 / 错路回退 / 逐日多样性」三张表。判读法:长尾失败率显著
# 高于头部 = 有「用得少所以坏了没人知道」的角落;失败率 100% 的行 = 从未工作过
# 的命令(首测抓到 titleblock.modify 32 次调用 0 次成功)。收敛验收基线见
# docs/design-sch-surface-convergence.md。
skills/easyeda-agent/scripts/audit-baseline.py              # 全部历史
skills/easyeda-agent/scripts/audit-baseline.py 2026-08      # 只看某月/某天

# 成本画像 —— **每跑完一场端到端都要记一笔**(用户要求,用以改善)。
# 三个耗时指标分开:墙钟 / daemon 侧(机器真在算)/ 两者之差(agent 思考+编译)——
# 改法完全不同。动作榜**按耗时排**:首版按次数排,把「探测占 65% 调用」顶到榜首,
# 而它只花 22 秒(机器时间 1.4%);真正吃掉 86% 的是 components.list(41%)/
# connect_pin(34%)/ document.open(11%,单次 4.24s)。次数的价值在别处 —— 它是
# 「跑了多少条 CLI 命令」的代理(每条固定 2~3 发探测)。
# token 不在审计日志里(那是 agent 侧的账),用 --tokens 自报,不给就记「未记录」。
easyeda audit cost --day 2026-08-15 --since 14:12 --until 15:50 --label "…" --tokens N --record
easyeda audit cost --ledger                                 # 跨批次对比台账
```

`skills/easyeda-agent/references/standard-parts.json` — 标准器件库（libraryUuid + deviceUuid + LCSC C 号）。放置前先查这里；新选型后写回。

For a connected window, EasyEDA must be open with the project AND have **"允许外部
交互 / Allow external interaction"** enabled, or the connector's WebSocket never
reaches the daemon.

## Load-bearing gotchas

- **Re-importing the connector: EasyEDA dedups installed extensions by UUID.**
  Importing a build whose uuid is already installed **silently fails** unless you
  first **uninstall the old one** in the 已安装 tab — a version bump alone is NOT
  enough (this bit us on v0.4.2). Two paths: **`make eext`** keeps the uuid stable
  → the normal update-in-place (uninstall old → import the printed `.eext`, one
  entry). **`make eext-fresh`** mints a new uuid → imports as a *separate* entry
  with no uninstall, but you must delete the stale one (two connectors fight over
  the daemon otherwise) — it's the fallback when the installed one won't
  uninstall. Our manifest is complete. **Marketplace status: LIVE again at
  v0.21.2** — https://jlc-ext.com/item/zhoushoujian/easyeda-agent-connector
  (same slug/entry; only the `displayName` changed to "EDA Agent Connector" —
  must not contain "easyeda"; the internal `name` and uuid both stayed, per the
  admins — the earlier "扩展名错误" came from changing `name` on the same-uuid
  listing). Existing installs keep auto-updating in place. Two install
  channels remain: (1) a **sideloaded `.eext`** (the `make eext` /
  GitHub-Release path above) has **no in-place auto-update** (manual
  uninstall→import) but is **strictly version-locked to the CLI**, so it stays the
  source of truth for dev/regression; (2) a **marketplace-installed** copy the
  platform **can auto-update in place** — but the listing **lags** (there is no
  publish CLI/API for jlc-ext — each release is a manual web-portal re-submit),
  so a marketplace connector can be **older** than
  your CLI and flag `connectorVersionOk:false`. Pure CLI/daemon changes do not
  require a connector re-import; manifest or handler changes require a normal
  rebuild/update, and missing capabilities must not be bypassed with `debug.exec_js`.
  **And re-importing
  does NOT reload already-open EasyEDA windows** — an open window keeps running the
  OLD connector code and fights the freshly-imported one over the daemon socket;
  **fully quit and relaunch EasyEDA** to load new connector code.
- **EasyEDA schematic coords are y-UP** (+y renders upward). The orientation table
  in `skills/easyeda-agent/references/orientation.json` is the **stored-rotation** truth (the
  value `getState_Rotation` reads back for a correctly-oriented flag), validated
  read-only against real placed flags by `skills/easyeda-agent/scripts/calibrate.js`. **`createNetFlag` /
  `createNetPort` STORE rotation negated** on the 2026-06 build — confirmed via
  `connect_pin(direction=left)`: it passed `90`, the flag stored `270` and rendered
  pointing **right** (up/down at 0/180 are symmetric, which is why it hid for so
  long). `connect_pin` now **auto-detects this at runtime** (`detectRotationNegation`,
  a one-shot probe flag) and compensates, so its output is correct whether the build
  negates or not. Two follow-ons: (1) raw `eda.createNetFlag` handling belongs
  inside the connector; Agent workflows must use typed `connect_pin`; (2)
  `getState_Rotation()` *immediately* after create can echo
  the input — a fresh **re-pull** (`getAll`) shows the real stored value.
- **A netflag must connect via a real wire** — overlapping the pin coordinate is
  NOT a connection (DRC won't see it).
- No programmatic undo in `eda.*`; `modify` only works on components (not flags —
  delete + recreate). Pull fresh primitive IDs right before mutating.
- **Edits are in-memory until saved.** `place`/`wire`/`modify` only change the
  EasyEDA document in memory; a window reload / daemon restart / crash loses
  unsaved work (bit us: placed parts vanished after an air hot-reload). The daemon
  now runs **debounced autosave** (`daemon start --autosave-debounce`, default
  **3s**, `0` disables) — after any successful *mutating* action it fires the
  matching typed save once edits quiesce (`schematic.save` for a schematic edit,
  `pcb.save` for a PCB edit; excludes the save action itself, so no recursion).
  It's a safety net,
  not a substitute for an explicit save at a known-good checkpoint (a process death
  within the debounce window still loses the last edits). Catalog `Mutates` flag
  drives which actions arm it; see `internal/daemon/autosave.go`.
- **Placement overlap is now mechanically checkable.** `easyeda sch layout-lint`
  pulls real rendered bboxes (`schematic.components.list --include-bbox` →
  `eda.sch_Primitive.getPrimitivesBBox`) and flags overlaps (ERROR, non-zero exit
  → gate-able) + tight spacing (WARN). More accurate than the old python
  `bbox_overlap`, which used a pin-extent approximation that underreported.

Deeper notes live in the per-fact memory under
`~/.Codex/projects/-Users-mikas-github-easyeda-agent/memory/`.
