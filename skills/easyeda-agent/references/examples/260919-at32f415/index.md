# 260919 AT32F415 学习板 Demo 索引

本索引把考试资料拆成可迁移的小样例。资料包只有题目、评分表、原理图和 BOM，没有完成态 PCB；
因此不能把题目示意图或下表当成已经布通、DRC 通过的黄金答案。完整参数见
[source-manifest.json](source-manifest.json)；69 个实例见
[bom-instances.json](bom-instances.json)，题图连接转录见
[source-connectivity.json](source-connectivity.json)；全部 36 个技术点的机器可读样例字段见
[example-catalog.json](example-catalog.json)。现场初始布局参数见
[initial-placement.json](initial-placement.json)，代表性步骤的真实回读和未完成项见
[live-validation.json](live-validation.json)；关键网络如何反向修正布局见
[晶振/CAN 规划样例](critical-routing.md)，晶振第一批实际布局见
[crystal-placement-live.json](crystal-placement-live.json)，CAN 第一轮现场负例见
[can-placement-iteration-live.json](can-placement-iteration-live.json)，LDO 第二轮可执行参数见
[LDO 布局候选](ldo-layout-candidate.json)，保存重开后的局部铜见
[LDO 实际路线](ldo-route-live.json)。LED 板边、MCU 逐脚去耦和 LDO 刚体变换的候选生成见
[模块候选 Layout](layout-candidates.md)；其中 CAN/SD/USB-UART/LCD 第二批外围的保存重载结果见
[现场快照](layout-after-peripherals-live.json)，用户重开浏览器后的只读复核见
[重开证据](layout-after-browser-reopen-verification-live.json)。U3 重绑定超时后的参数化恢复见
[原理图恢复证据](schematic-u3-recovery-live.json)。

## 来源与已确认事实

| 来源 | 已确认事实 | 证据边界 |
|---|---|---|
| `物料清单.xlsx`，BOM 表第 3-29 行 | 27 行物料，数量展开为 69 个唯一位号；位号、封装、值、商品编号和功能区见 `bom-instances.json` | `offline-verified`；尚未与 EasyEDA 实例逐件回读 |
| `原理图.pdf` 第 1 页 | 一张 A4 原理图，15 个功能区；`source-connectivity.json` 转录 69 个器件、233 个端子和 13 处明确 NC | `source-only`；蜂鸣器 NC 在图面未标数字号，派生局部网名可改，真实 symbol pin、pad 与 pin→pad 映射必须现场读取 |
| `考试说明.pdf` 第 1-8 页 | 原理图、规则、机械、布局、布线、丝印和评分要求 | 已逐页提取并视觉抽查；不是完成态设计 |

当前总状态：`partial-live-verified`。原理图逐端点连接、默认 DRC、真圆角板框、显示原点、
固定件、规则/网络类、69 件初始布局、LDO 第二轮局部布局/局部铜，以及晶振 X1/C20/C21 的
次序修正已在 Web 3.2.203 保存并通过 typed reload 回读。CAN/SD/USB-UART/LCD 的 11 个外围
也已按模块候选保存重载；浏览器重开后 69 件几何、板框与 15 段 LDO 铜仍保持。CAN 的
D1/CN1 对称关系已现场回读，R12/D1 当前位置和方向由零位移刚体候选接受；两处 H/L 最短
飞线相交只保留为布线反例，不再否定 Layout。LCD 的
no-components region 只在个人库可写副本中保存，当前 U3 实例绑定尚未完成；一次重绑定超时
删除 U3 后虽已参数化恢复到 69 件和相同 13 脚网络，primitiveId/uniqueId 已变化，因此禁止
PCB `import-changes`。完整布线、丝印、泪滴和最终 PCB DRC 仍未验证。具体边界以
`live-validation.json` 为准，不能把代表性步骤外推成完成态整板。
该文件保留初始批次；后续配置入口的 typed 保存/重载实测见
[PCB 配置样例](../../pcb-config.md)，不要继续沿用早期的“新入口未现场验证”结论。

## 15 个原理图功能区

每区先核对器件身份和逐引脚连接，再迁移布局。共同来源为 `原理图.pdf` 第 1 页；BOM 身份来自
`物料清单.xlsx`。

| 区 | 器件 | 必须单独核对的点 |
|---|---|---|
| TYPE-C 输入 | USB1、R1、R2 | A6/B6 与 A7/B7 的重复 D+/D- 脚；CC1/CC2 各 5.1kΩ；VBUS、GND、外壳 |
| RGB-LED | U1、LED1、R3、C1、C2 | LED1 的 `3.7V~5.3V` 显示属性；C1→U1.5、C2→LED1.1 的去耦归属 |
| LDO-3V3 | U2、C3-C6 | VIN 输入大/小电容；VOUT/TAB 同属 +3V3；输出大/小电容；顶层 GND 回流 |
| LCD | U3、Q1、R4-R6、C7 | U3.1/.2/.9 NC；背光三极管驱动；封装外形内的元件禁放区 |
| 功能按键 | SW1-SW3、R7-R9、C8 | SW1 与 SW2/SW3 的上下拉方向不同；NRST、PA0/WKUP、PA1 丝印 |
| USB 转串口 | U4、C9、C10 | CH340N V3 与 VCC 的供电/去耦；RTS NC；TXD/RXD 对 PA9/PA10 |
| 光敏电阻 | R10、R11、C11 | 分压与滤波；靠板边并远离 RGB LED |
| CAN | U5、D1、R12、C12、C13、CN1 | VREF NC；H/L 各经 R12 对应焊盘再到端子，120Ω 跨接 H/L；ESD 靠端子；针脚丝印 |
| 主控 | U6、C14-C16 | C14→U6.1、C15→U6.5、C16→U6.17；EP 接地；本例按 footprint anchor 执行 |
| BOOT | R13、R14 | BOOT0、PB2 各自下拉，不把相邻网络合并 |
| 无源蜂鸣器 | D2、Q2、R15-R17、C17、BUZZER1 | 驱动电阻、续流二极管、下拉和去耦整体跟随 |
| MICRO-SD | CARD1、C18、C19、R18-R23 | DAT/CMD 上拉、VDD 去耦、外壳地及固定方向 |
| 晶振 | X1、C20、C21 | OSC_IN/OSC_OUT；四脚晶振接地脚；靠 MCU 且不贴板边 |
| SWD | H1、C22 | 3V3/DIO/CLK/GND 的真实针脚顺序与接口丝印 |
| M3 螺丝孔 | SCREW1-SCREW4 | 四角精确坐标、顶层实例和锁定状态 |

## 技术点到样例

### 原理图与器件

| ID | 来源 | 技术点 | 样例/观测 |
|---|---|---|---|
| SCH-01 | 说明 p1；BOM | 按位号和商品编号展开 69 个实例，核对符号与封装 | [bom-instances.json](bom-instances.json) 是离线基线；`sch read/connectivity` 后逐件比较 |
| SCH-02 | 说明 p1 | 原要求为工程名 `客户编号-AT32F415 学习板`，图签“绘制”填真实姓名；Demo 用脱敏占位值替代 | `sch titleblock-get` 确认字段 key → `sch titleblock --data ...` → 回读；保留原要求与 Demo 替代的映射 |
| SCH-03 | 说明 p1；原理图 p1 | LED1 显示 `3.7V~5.3V` 工作电压 | 回读自定义属性与官方导图；待现场验证 |
| SCH-04 | 说明 p1 | 15 区布局、连线和功能文本以 PDF 为来源 | 先做连接数据再计算几何，不按旧 Z 排版重排题图 |
| SCH-05 | 说明 p1 | 网络标识、标签、NC 与连接关系保持一致 | [source-connectivity.json](source-connectivity.json) 与现场 `connectivity`、`check`、NC 明细分别比较 |
| SCH-06 | 说明 p1-2 | 原理图使用默认规则并查看 DRC 警告/错误 | 记录官方 `sch drc` 聚合与 `sch check` 明细，二者不互相替代 |
| SCH-07 | 原理图 p1 | USB-C 重复数据脚与两个 CC 下拉 | 逐引脚黄金表；不能只检查同名网 |
| SCH-08 | 原理图 p1 | CH340N 的 V3/VCC、RTS NC | 逐脚连接与两只 100nF 去耦 |
| SCH-09 | 原理图 p1 | AMS1117 的 pin 2 与 TAB/pin 4 同属输出 | [LDO 样例](ldo-placement.md) |
| SCH-10 | 原理图 p1 | 晶振接地脚、LCD/CAN 的 NC、SD 上拉、SWD 针脚顺序 | 每项单独回读；缺一个都不能用“DRC 数字正常”替代 |

### 设计规则与机械

规则参数现在可直接用 [PCB 配置命令](../../pcb-config.md) 设置，无需手改整份 JSON。
2026-09-20 后续已完成新入口的写入、typed 保存/重载、幂等重放和恢复；具体范围见该样例。
6mil 只改导线到导线，24/12mil 是过孔最小值；规则设置不自动修改已有铜。

| ID | 来源 | 技术点 | 样例/观测 |
|---|---|---|---|
| PCB-01 | 说明 p2、p5 | 两层板 | `pcb layers` 回读真实铜层数 |
| PCB-02 | 说明 p2、p7 | 导线间距 6mil；信号默认/最小 8mil | 已按 Web 3.2.203 的 `{name,config}` 读取形态写裸 `config`，整页刷新后回读为 6/8/8mil |
| PCB-03 | 说明 p2、p7 | `PWR` 默认 20mil、最小 8mil；`PWR_Class` 绑定全部电源网 | 已实建 `PWR_Class=[+5V,+3V3,GND]`，父子 netRules 均绑定 `PWR`，刷新后保持 |
| PCB-04 | 说明 p2、p7 | 过孔最小外径/孔径 24/12mil | 已写入完整规则副本，整页刷新后回读 24/12mil |
| PCB-05 | 说明 p2 | 90×50mm、线宽 0.254mm、R3 真圆角、左下显示原点、锁定 | [固定机械样例](fixed-mechanics.md)；保存及整页刷新回读已 `live-verified` |
| PCB-06 | 说明 p2 | 四孔、U6、CARD1 的固定题面坐标、角度和锁定 | 现场确认本批输入为 footprint anchor；六件刷新后坐标、角度和锁定保持 |
| PCB-07 | 说明 p3 | CN1 只固定 y=42mm、180°，x 是自由参数 | 现场候选 x=69mm；题定 y/角度保持，rendered bbox 顶边约超 1.17mil 的冲突单独保留 |
| PCB-08 | 说明 p3 | LCD 封装轮廓内禁止其他元件 | 当前 U3 已实证绑定 C2890616、OLED-SMD_ST7735S 和既有 3D model；个人库副本 region 只是未绑定接口试验。应在现有 source footprint 原位增加区域并证明三项关联不变，禁止为此重绑模型；U3 identity 修复前仍禁止 PCB `import-changes` |

### 布局关系

| ID | 来源 | 技术点 | 样例/观测 |
|---|---|---|---|
| LAY-01 | 说明 p3-4 | 按键板边等距；USB、SWD、端子面向可插拔方向 | 读真实 bbox、开口方向和板框距离，不只看中心点 |
| LAY-02 | 说明 p4 | RGB 靠 TYPE-C；光敏远离 RGB | [模块候选 Layout](layout-candidates.md)：按真实板框中心线和 pad 所有权生成 3 个板边候选，选择理由与铜限制分开记录 |
| LAY-03 | 说明 p4、p7 | LDO 输入/输出电容靠对应引脚，大电容在前、小电容在后 | [LDO 样例](ldo-placement.md)；第二轮布局与15段TOP/20mil局部铜已 `live-verified`，整板主干不在本条范围内 |
| LAY-04 | 说明 p4、p7 | MCU 等电源脚逐脚去耦，电源先经过电容再入芯片 | [模块候选 Layout](layout-candidates.md)：C14/C15/C16 分别绑定 U6.1/.5/.17；同一所有权表达已迁移到 CAN、SD、CH340N 与 LCD，布局只证明所属 pad 距离，实际铜路径另验 |
| LAY-05 | 说明 p4 | 晶振靠 MCU、不在板边；蜂鸣器/背光驱动整体放置 | [晶振现场布局](crystal-placement-live.json) 已验证 X1/C20/C21 次序修正；LCD 背光链已按候选保存重载，蜂鸣器和两组实际铜仍待验证 |
| LAY-06 | 说明 p4 | 全部器件顶层、无重叠、外形不出板 | `pcb list --include-bbox`、`layout-lint` 和 LCD 禁放区分别观察 |

### 布线与收尾

| ID | 来源 | 技术点 | 样例/观测 |
|---|---|---|---|
| RTE-01 | 说明 p5 | 焊盘末端出线、线宽不大于焊盘、窄焊盘缩颈、无直角/锐角 | 读轨迹端点、宽度与角度；DRC 不覆盖全部观感规则 |
| RTE-02 | 说明 p5、p8 | 电源主干按电流加粗，过孔按载流能力，流向清楚 | PWR 规则 + 实际每段/过孔回读 |
| RTE-03 | 说明 p4–5、p8 | 晶振靠 MCU、不在板边；短直、避免底层、顶层包地净空 | [关键网络规划](critical-routing.md)：X1/C20/C21 次序已现场修正；铜仍待新版 connector 下验证。TOP/0via 是推荐策略，非题目明文零过孔禁令 |
| RTE-04 | 说明 p5、p8 | USB_D+/D- 顶层、无过孔、类差分，不额外要求等长 | 不自行增加等长约束；回读两网层与 via 数 |
| RTE-05 | 说明 p5、p8 | CANH/CANL 顶层、无过孔；各先经过 R12 对应焊盘再到端子，120Ω 仍跨接；ESD 靠端子 | [CAN 现场迭代](can-placement-iteration-live.json) 保留最短飞线相交的历史反例；[模块候选 Layout](layout-candidates.md) 已接受 R12/D1 当前位置与方向，实际有序铜路留到布线阶段证明 |
| RTE-06 | 说明 p5、p8 | PA9/10、PA11/12、PA13/14 顶层且不换层 | 分网回读 layer 与 via count |
| RTE-07 | 说明 p5 | U6 EP 添加散热过孔；其他焊盘不允许 via-in-pad | EP 与普通焊盘使用不同判据 |
| RTE-08 | 说明 p6、p8 | 普通信号同网过孔不超过 2；GND 扇孔与缝合孔 | 按网计数并检查地回流，不以总 via 数判断 |
| FIN-01 | 说明 p6、p8 | Arial、字符高度 ≥45mil、顶层、朝向不超过两个方向、接口提示丝印 | `silk-add/set --font-family Arial`，`silk list` 回读；接口离线测试通过、现场待验证 |
| FIN-02 | 说明 p6、p8 | 顶底层 GND 铺铜且保持完整 | `pour-list`、`pour-rebuild`、连通与 DRC |
| FIN-03 | 说明 p6、p8 | 添加真实泪滴后重建铺铜 | 当前为 `unsupported`；走线圆角化不是泪滴，禁止手工补做 |
| FIN-04 | 说明 p6、p8 | 100% 布通、无断头、PCB DRC 无错误，保存并重开核查 | 连通、`pcb check`、官方 DRC 分开记录；最后 `pcb save` + `doc reload` |

## Demo 实施顺序

1. 以 `bom-instances.json` 的 69 个唯一位号和 `source-connectivity.json` 的题图连接为源基线，
   回读真实 symbol pin、footprint pad 与 pin→pad 映射；保留差异，不猜测补齐。
2. 用当前 `project create --help` 核对后建立工程容器；若当前版本未暴露该能力，先补齐并验证 typed 接口。
3. 按 PDF 的功能区和连线表达构建原理图，回读属性、连接、NC、`check` 与官方 DRC。
4. 转入正确绑定的 PCB，确认 69 件和焊盘网；设置两层及真实规则/网络类，让间距与线宽参与后续布局。
5. 建 90×50mm 板框和左下显示原点，放置并锁定孔、U6、CARD1；CN1 保留 x 自由。
6. 建 LCD 元件禁放区，安排屏幕与板边器件；用 `pcb layout-plan` 按模块生成完整候选，
   依据板框中心线、开口/关系、pad 距离和空隙选择，不能在现场逐件试摆。
7. 围绕明确所属引脚生成 MCU 去耦、晶振、LDO、CAN、蜂鸣器和 SD 模块候选，同时预留
   电源主干和顶层回流；同网多个供电脚必须在输入中逐脚绑定。
8. 先 EP/地回流与晶振、受限顶层网络；局部去耦短线随模块处理，再连接电源主干及普通信号。组间次序按通道冲突调整，每组写后回读。
9. 用 typed 接口完成丝印、双面 GND、缝合孔；真实泪滴能力未实现时保持未完成。泪滴后重铺铜，复查连接、几何、DRC，保存重开。

固定考试板遵循“板框/固定件 → 机械空间 → 关键路径 → 外围”。另建的可调尺寸教学变体遵循
“功能模块/接口 → 关键路径 → 估算板框 → 合法化”，并明确标为自建变体，不能回写考试基准。
两种情况都在精细布局前确认制造规则。去耦 150mil、晶振守护区 200mil、装配间隙 12mil
都不是本题的数值要求；若作为搜索初值或工程建议使用，须记录来源和可调整性。
