# 样例：关键网络离线规划如何反向修正布局

## 来源与状态

- `考试说明.pdf` 第 4–5、8 页：晶振靠 MCU、不在板边、短直、避免底层及顶层包地净空；
  CAN 明确要求顶层无换层，分别先经过 120Ω 电阻对应焊盘再到端子，ESD 靠端子。
- `原理图.pdf` 第 1 页：`OSC_IN/OSC_OUT`；R12 跨 CANH/CANL，D1 分别连接 H/L 并回地。
- 输入为第一轮参数化布局保存并整页刷新后的真实 pads/bbox，以及 6mil 间距、8mil 信号线宽。

状态：`partial-live-verified`。旧晶振/CAN 路线仍只做了离线反例分析；晶振的第一步
“修正 X1/C20/C21 焊盘次序”已经在 Web EDA 通过 typed 修改、保存、重载和独立只读核查，
见 [crystal-placement-live.json](crystal-placement-live.json)。CAN 又完成了一轮真实布局迭代：
D1/CN1 的 H/L 支路已改成对称关系，但 R12 候选暴露两处 H/L 飞线相交，作为负例保留在
[can-placement-iteration-live.json](can-placement-iteration-live.json)。晶振铜和 CAN 铜仍未现场写入。
后续原文复核发现旧 CAN 计划未证明 R12 主路径顺序；离线 passed 或布局命令成功都不表示完整题目拓扑已验证。
结果证明“几何上能布通”仍可能是差布局；本样例的完成动作是把绕行原因反馈给布局参数，而不是
为了让工程看起来更完整而落下 77 段不理想走线。

## 开始状态与参数

目标 PCB 以 UUID 定位，不依赖 Board 显示名。开始时无走线，使用本轮读取的 pad 中心和矩形，
旧计划把其他网络 pad、器件和已规划轨迹统一按 6mil 膨胀；这是简化模型，不是题目对所有
间距的规定。正式规划须按对象类型读取实际规则并计入线宽，器件占地与铜层障碍也要区分。

| 参数 | 晶振 | CAN |
|---|---|---|
| 网络 | OSC_IN、OSC_OUT | CANH、CANL |
| 层/线宽/过孔 | TOP / 8mil / 0 | TOP / 8mil / 0 |
| 必达拓扑 | U6.2 ↔ X1.1/C21；U6.3 ↔ X1.3/C20 | H/L 各自 U5 → R12 对应焊盘 → CN1；R12 跨 H/L，D1 靠 CN1 |
| 转角 | 0/45/90/135° | 0/45/90/135° |
| 计划哈希 | `2bdf39440d2d1a47dfb2e0de87dc5a5ec09f10e2f82a449a887478e0f6e6d92f` | 同一批 |

晶振 TOP/0via 是本例的推荐策略，原题没有对晶振单列绝对零过孔禁令。转角表中的数字是
线段方向，不是允许相邻线段形成直角；实际转折仍遵守本题无直角/锐角要求。

完整离线计划为本次开发记录，不随 Skill 打包；样例只保留能迁移的参数、结论和修法，避免把
58KB 的一次性绝对坐标塞进入口。迁移时必须从当前板重新读取 pads/bbox 并重算。

## 计划、观察与决定

离线规划先做 pad-normal escape，再在 45°格点上避开膨胀障碍，最后检查真实 pad 端点、板内、
异网轨迹间距、层和过孔数。检查覆盖 14 个必达端点，共 77 段、0 过孔、0 离线违规。

| 组 | 离线结果 | 观察 | 下一步 |
|---|---|---|---|
| 晶振 | 28 段；OSC_IN 882.7mil、OSC_OUT 269.4mil | U6.2/.3 与 X1.1/.3 左右次序反转，OSC_IN 被迫绕晶振区一大圈 | 旋转/重排 X1、C20、C21，保持两网引脚次序后重新求短解 |
| CAN | 49 段；保持 H/L 网表跨接，但未证明主路径先过 R12 | D1 未对齐端子信号 pad，CANL 绕行 1422.4mil；旧计划还遗漏 R12 必经顺序 | H/L 各显式经过 R12 对应焊盘再到 CN1；D1 对齐端子且保留短地支路，重新规划 |

这一步没有把 `passed=true` 解释为“应该执行”。离线 passed 只证明给定障碍和检查器下没有已知
几何违规；短直、回流、EMC 和人工布局质量仍需看长度、拓扑与局部关系。

## 晶振第一批现场修正：只改次序，不落铜

第一批把“布局导致绕线”拆成一个独立步骤。U6 保持题定 anchor、0°和锁定；X1 只从 180°
改为 0°，C20/C21 复用彼此原有占位并把信号 pad 朝向 X1、GND pad 朝外。两只电容交换时
先把 C20 放到一个经实时 bbox 确认的临时空位，避免中间态重叠；临时点和 primitiveId
只属于本次现场，迁移时必须重新计算。实际命令与回读见
[crystal-placement-live.json](crystal-placement-live.json)。执行形态为：

```bash
easyeda pcb modify --id <C20_ID> --center --x <TEMP_FREE_X> --y <TEMP_FREE_Y> \
  --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb modify --id <C21_ID> --center --x <OLD_C20_CENTER_X> --y <OLD_C20_CENTER_Y> \
  --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb modify --id <C21_ID> --patch '{"rotation":180}' --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb modify --id <C20_ID> --center --x <OLD_C21_CENTER_X> --y <OLD_C21_CENTER_Y> \
  --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb modify --id <C20_ID> --patch '{"rotation":0}' --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb modify --id <X1_ID> --patch '{"rotation":0}' --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb save --doc <PCB_DOC_UUID> --project ceshi
easyeda doc reload <PCB_DOC_UUID> --project ceshi --json
```

保存重开后的事实：

- U6 未移动且仍锁定；X1/C20/C21 都在 TOP，69 件仍为 0 overlap、0 outside、0 tight-spacing。
- U6 侧从左到右为 OSC_IN/OSC_OUT；X1 侧也改为 OSC_IN/OSC_OUT。两条 U6↔X1 直连从
  相交变为不相交，整板 ratsnest 从 27058.07mil 降到 27045.82mil，crossing 从 58 降到 57。
- U6.2→X1.1 的直距从 99.81mil 增至 148.23mil，U6.3→X1.3 从 151.73mil 降至
  91.07mil；四个信号关系的直距合计只减少 12.25mil。因此本批的主要价值是消除交叉和
  纠正负载电容方向，不能声称两网都缩短。
- OSC_IN/OSC_OUT 仍各为 0 track、0 arc、0 via；官方 DRC 仍含这六个信号端点的
  Connection Error。本批状态只覆盖布局，不能外推为晶振布线通过。

独立 subagent 只读取原题、source-connectivity 和保存重开后的原始回读，确认上述结果。
下一批才按 U6.2→X1.1→C21.1、U6.3→X1.3→C20.1 规划 8mil TOP 短线。当前连接器 1.5.1
缺 pad source shape 与 `arcsAvailable`，`pcb net-path` 会 fail-closed；新版 typed
连接器未生效前不写晶振铜，也不以 AABB 或同网名降级判 PASS。

## CAN 第一批现场迭代：保留局部改善，拒绝整组候选

第一轮保持 U5 和题定 `CN1 y=42mm / 180°` 不动，把 D1 旋转到 270°并令其 H/L 与 CN1
信号脚中心对齐；R12 候选移动到中心 `(2630,1510)mil`、0°。所有动作经 typed CLI 写入，
保存、`doc reload` 后同一 primitive 和 pads 保持。完整命令、原始回读哈希和两轮独立核查见
[can-placement-iteration-live.json](can-placement-iteration-live.json)。

这轮得到一个可保留的局部关系和一个必须拒绝的结论：

- D1.1→CN1.2 与 D1.2→CN1.1 都约 `168.94mil`，比修改前约 `217/295mil` 对称；D1 与
  CN1 的 bbox 净距约 `10.06mil`，只说明器件未重叠，不能把这条缝当走线通道。
- R12.1=CANH 在左、R12.2=CANL 在右，仍是跨接 120Ω；但纯 MST 飞线出现两处 H/L
  相交。最初“预计不会先天交叉”的离线候选被真实回读否定。
- 将 R12 的 y 对齐到 D1 信号 pad 的 `1488.8mil` 也不是修法：它会让 R12.2 落在
  CANH 水平段、D1.1 落在 CANL 水平段，把点交叉变成异网共线穿越。
- U5.6 右侧还有 U5.5 NC。按 8mil 线和 6mil 净距膨胀后，该 pad 的禁入矩形为
  `x=2402.6..2447.4, y=1408.9..1502.3mil`；只分别给两网找最短路会漏掉相互穿越。

因此当前 R12 坐标只作为负例，不升级为正向布局答案，也不继续凭单一 crossing 数盲移。
下一步必须做 H/L 双线联合寻路：每网带 R12 必经节点，D1 作为靠端子的短支路，统一检查
pad/track/NC/板框/既有铜、8mil 线宽、6mil 净距和 45°转折。只有计划给出可执行段并通过
独立几何检查后，才再移动 R12；当前连接器证据不足时仍不落铜。

## 执行形态与回读

重排后重新生成段。每段映射到现有 typed CLI；下面只展示数据形态，不复制本轮绝对坐标：

```bash
easyeda pcb track --x1 <PAD_ESCAPE_X> --y1 <PAD_ESCAPE_Y> \
  --x2 <NEXT_X> --y2 <NEXT_Y> --layer 1 --width 8 --net OSC_IN \
  --doc <PCB_DOC_UUID> --project ceshi
```

按网络小批执行，每批后读取该网 tracks/vias；超时或部分写入时先回读，只 rip-up 当前失败网并
从新状态重算，不重放整组。晶振先于 CAN，二者通过后保存并做持久化回读：

```bash
easyeda pcb track-list --net OSC_IN --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb track-list --net OSC_OUT --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb via-list --doc <PCB_DOC_UUID> --project ceshi
easyeda pcb check --strict --doc <PCB_DOC_UUID> --project ceshi --json
easyeda pcb drc --doc <PCB_DOC_UUID> --project ceshi --json
easyeda pcb save --doc <PCB_DOC_UUID> --project ceshi
```

若 `doc reload` 在 Web 版持续显示加载动画，停止重试和现场写入，保存故障与当前对象证据，
将持久化验证标为 `incomplete`。先修复 typed reload/open，再重复 tracks/vias/check/DRC 读取；
禁止通过浏览器或工程树手工恢复。

## 暴露的工具缺口

- 当前 `route-short` 的 MST 只理解“同网端点”，不理解 H/L 各自 `U5 → R12 对应焊盘 → CN1`
  的先后关系。R12 仍跨接两网，不能串入单根信号线；D1 是靠近端子的保护支路。
  后续接口需要显式 topology anchors 与 forbidden shortcuts，不能按最近距离偷连。
- 关键网需要每网 `topOnly`、`noVia`、pad-normal escape 等约束；这些应是计划输入与回读断言，
  不能靠 Agent 看完题目后记住。
- `pcb.line.create` 没有多段事务，超时可能已经写入前几段。工具必须返回部分执行证据；样例按
  net-scope 回读与删除，避免整板重放。
- DRC 规则配置、网络类成员及成员到 Track 规则的关联是三类事实，要分别回读。

## 验证边界

- 已验证：旧简化模型的 pads/bbox、6mil 障碍、14 个端点和 TOP/8mil/0via 几何检查；
  独立规划给出绕行反例。晶振三件的次序修正已现场保存重开并独立核查。旧 CAN 检查未覆盖
  题定 R12 必经顺序，不能升级为题意通过。
- 未验证：晶振/CAN 的现场 track 创建、铜的保存持久化、顶层地回流、包地净空和最终短路线。
- 修法的验收不是“段数变少”本身：还要重新核对端点、拓扑、长度、净距、via 数和保存后对象。
