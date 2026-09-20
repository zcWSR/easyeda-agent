# 架构：数据驱动的设计、计算与执行

现行原理图架构延续 1.4 Connectivity IR。唯一规范正文是随 Skill 发布的
[数据驱动架构基准](../skills/easyeda-agent/references/schematic-data.md#数据驱动架构基准)，
字段见同一手册，操作顺序见 [计算与 Apply SOP](../skills/easyeda-agent/references/auto-layout-sop.md)。
本页解释职责边界，不维护第二份布局规则。术语由 [concepts.md](concepts.md) 定义。

## 数据流与职责

```text
需求/手册证据 + 官方原始快照（保留）
              ↓
Skill/Agent：在目标源数据中表达电气事实、核心/外围归属、约束
              ↓
Go 纯计算：SCH 区内/纸张计划，或 PCB 模块 layout-plan 候选 → 数据校验
              ↓
固定渲染 / compose --layout-page → 可检查的 Apply 队列
              ↓
Go CLI/daemon ── typed actions / WebSocket ── Connector ── 官方 eda.* API
              ↓
官方原始回读 → 与目标对账、列出检查覆盖和差异 → 显式保存/重载回读
              └─ 发现差异：修源数据/采集器/算法，重算受影响阶段
```

### Skill / Agent

负责需求与手册依据、选择入口、准备源数据、解释结构化问题和授权范围。
核心与专属外围必须在数据里明确绑定，不能只留在对话或看图判断中；截图是辅助审阅，
不是布局权威或漏检后的人工补签。不会为了排版改变电气事实或真实器件参数。

### Go CLI / 纯计算内核

CLI 以 Cobra 子命令暴露功能；布局内核消费数据、返回完整结果或明确失败，不操作编辑器。
原理图区内负责核心/外围、连接与避碰，纸张层只选择和平移完整合法区域；PCB 模块规划器
读取实测 anchor/bbox/pads，用显式 pad 所有权生成少量完整坐标候选，不访问编辑器、不替
Agent 合成总分。候选经选择后转成 typed Apply，写后再由官方回读对账。
检查器基于原始几何/连接证据报告对象与规则，缺测不是零问题，评分也不替代具体事实。
Compose 固定转换已确认几何，Apply 执行和回读，不在写入阶段偷偷重设计。

### daemon / Connector

daemon 已实现长连接、端口管理、多窗口路由、输入校验、审计与防抖 autosave；
Connector 在宿主内将 typed actions 转成官方 `eda.*` 调用，并序列化真实状态和错误。
运行时 primitiveId 是操作句柄，不替代 canonical 稳定身份。采集不足应修适配器，
不能填默认坐标/空数组制造成功。autosave 不替代检查点的 `saved:true` 证据。

workflow/stage、版本对账、布局评分和 staleRisk 是兼容诊断面，不是 action 的执行许可。
需要权威最终读的批次负责 save → real reload → readback；刷新失败时结果保持 incomplete，
不能把 reload 前的缓存或旧阶段记录当作完成证据。

## 单一事实与能力边界

- 连接事实与可重算几何分离，见 [连接数据模型](schematic-connectivity-model.md)。
- 核心/外围唯一所有权、真实直连及完整区域迁移，不由 bbox 无重叠自动推出。
- 位号参与遮挡/入框；其他器件属性文字排除页面碰撞与框包络，原属性数据仍保留。
- 生成输入、参数/代码版本、哈希、队列、回读和验证证据必须能串成同一轮记录。
- `incomplete`、缺测、预算耗尽、部分 Apply 与未确认保存不能报告完整通过。

这些是执行/验收契约，不表示所有安装版本已经机械覆盖全部要求。
当前边界见 [数据基准](../skills/easyeda-agent/references/schematic-data.md#数据驱动架构基准)
及 [检查覆盖](../skills/easyeda-agent/references/schematic.md#检查覆盖边界原理图验收)。
现场验证、离线测试和正式发布分别举证，文档更新不等于运行时升级。

## 存量工具与历史设计

`group/zone/sheet tidy/move` 是存量维护能力，不是新设计的主架构；小修也须同步源数据。
[旧层级设计](schematic-layout-hierarchy.md)、ADR-0003/0004 保留历史背景和移动安全经验，
不能覆盖现行数据主链，也不提供事务回滚保证。原理图任务不自动延伸为 PCB。

旧 `stage confirm-*`、`layout-lint --gate`、`--force/--force-unsafe` 与版本 skip 参数为脚本兼容
保留；新样例不依赖它们授权执行。安装对账使用显式 `easyeda update --check --exit-code`。
