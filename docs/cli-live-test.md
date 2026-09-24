# 基础 CLI 真实链路测试

## 目标与使用方式

本套用例验证用户实际调用的 `easyeda` 命令能经 daemon、WebSocket 连接器到达
Web EasyEDA Pro 官方 API，并能把结果回读、保存和重载。它补充现有的 Go/连接器离线测试
与[从客户需求到成品的固定验收](e2e-automation-acceptance.md)，不替代后者。

先在专用测试工程运行[只读预检脚本](../scripts/cli-live-smoke.py)：

```bash
python3 scripts/cli-live-smoke.py --expected-version vX.Y.Z-dev.N \
  --project <测试工程UUID> --doc <原理图页UUID> --type schematic \
  --out <本地证据目录>
```

预检通过后，由 Codex 执行员按[详细用例与前置条件](cli-live-test-detail.md)串行操作。
另一名无历史上下文的 Codex 复核员只读核对冻结的输入、命令日志与保存重载后的新鲜回读。
每例记录 `pass`、`fail`、`blocked`、`not-run`；只读预检通过不表示写入或整板验收通过。

## 本期覆盖

| 阶段 | 用户能验证什么 |
|---|---|
| 环境与发现 | 版本、窗口、工程/页面身份正确，命令和 typed action 可查 |
| 工程与原理图 | 新建/查找工程与页面，器件、引脚、网络、布局、检查、导出可写可读 |
| PCB | 原理图转板、层叠/规则、布局、铜/过孔/禁布、DRC、导出可写可读 |
| 安全与持久化 | 错页/过期状态写前拒绝，失败后可判定现场状态，保存重载后结果仍在 |
| 证据 | 每个现场写入有输入、命令、原始回包、对象差分和独立复核结论 |

## 当前可见的测试状态

[2026-09-23 起的 Codex subagent 原理图验收记录](reviews/2026-09-23-v1.6.0-schematic-acceptance.md)
可查。它记录了无历史上下文执行与独立只读复核；原理图整体 Apply、局部修改及 PCB
端到端尚未通过。本清单是新增的基础 CLI 链路验收入口，只有现场逐例执行并留下证据后
才能写入通过结论。
