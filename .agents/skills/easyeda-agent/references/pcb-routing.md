# EasyEDA PCB — 布线 / 铺铜 / 禁布区 / 填充区域

> 从 [`pcb.md`](pcb.md) 拆出(RFC #178):这几节只在**动铜**时才需要,不该压在每次 PCB 调用的上下文里。
入口、坐标系、正确性检查与写后回读语义仍在 `pcb.md` —— **先读它**,再按需读本文件。

---

### 关键网与叠层

`pcb route-critical --spec <S0.json> --dry-run` 先检查真实铜层数与关键网方案。
执行时沿用同一份 spec；`stackup.layers` 与活板不一致，或读不到可靠铜层证据，
命令会因缺少可靠叠层输入而拒绝非法计划。先用 `pcb layers` 检查；即时读取若带 `staleRisk`，
可用于诊断，最终叠层证据在 save/reload 后重读。
不把未启用的内层、图层总数或默认两层当成真实叠层。

- 两层板的电源步走 `pcb power-pour`；四层及以上走 `pcb power-planes`。
  `route-critical --allow-stackup-change` 不会把正常两层板改走内电层。
- 独立 `power-planes` 要求至少四层；已有六层等叠层保持原层数。
  只有用户已明确授权将已确认的两层板升级为四层，才使用
  `pcb power-planes --allow-stackup-change`。未知层数不能被此开关绕过。
- `power-planes --dry-run` 运行相同叠层预检；允许升级时，结果中的 `stackup`
  明确列出当前层数、目标层数、证据来源和是否需要修改，不写入板子。
  需要改变已确定的设计叠层时，应先单独确认需求并更新 S0，不能为方便布线擅改。

### Routing (copper tracks + vias)

Real routing primitives — **additive creates**, like the schematic
`wire.create`. Bind to a net **by name** (pull from `pcb.nets.list`); layer ids from
`pcb.layers.list`. EasyEDA's `create()` is **lenient** — it can return no primitive on a
bad layer/coords without throwing, so each action verifies a primitive came back and
fails honestly otherwise. **PCB autosave is on** (debounced) — still **save explicitly**
at checkpoints. There is **no one-call autorouter** on this build
(`pcb_Document.autoRouting` is undefined — see `docs/ecosystem-survey.md` §6/§7); route
segment-by-segment, or use the file-exchange autoroute flow. 布线方式见
[`design-flow.md`](./design-flow.md) P7：稀疏短线可逐段或 `route-short`，稠密板使用题目允许的
EasyEDA 原生自动布线或已配置的外部路由器，完成后都按网回读并运行 DRC。

- `pcb.line.create` — a copper **track** (导线): line segment on a copper layer
  (`TOP=1`, `BOTTOM=2`; **inner-copper ids are higher** — `id 3` is silkscreen, not
  copper, so read real ids from `pcb.layers.list`) between `(startX,startY)` and
  `(endX,endY)` (mil, y-up), `lineWidth` (default 6 mil), optional `net`. Verify with
  `pcb.drc.check`.
- `pcb.via.create` — a **via** (过孔) at `(x,y)` with `holeDiameter` (drill, default 12
  mil) + `diameter` (outer pad, default 24 mil), optional `net`.
- `pcb.line.list` / `pcb.via.list` — read what's routed (filter by net/layer) before
  rip-up or reroute.
- `easyeda pcb net-path --from C3.1 --through C4.1 --to U2.3 --layer 1 [--net +5V] [--json]`
  — **只读的逐焊盘铜路径举证**。它读取焊盘原始 `shape` / `rotation` / `specialPad`、
  线段/圆弧、层、线宽和过孔，在 Go
  侧构图；`--through` 是一条不重复使用铜图元的路径中按给定顺序出现的**必经焊盘**，
  不是物理过孔。只因各焊盘分别可达、但必须沿分支回头的网络不会冒充顺序通过。输出每一段的
  primitive 路径、层序列、线宽集合/最小值和唯一过孔数，因此可以分别证明
  `C3.1 → C4.1`、`C4.1 → U2.3`，也能证明晶振路径是否确实 TOP-only。
  `--layer 1` 在受限图中求路，直接排除其它层和物理过孔；不是先任取一条跨层路径再检查
  结果。因此 TOP-only 报告的 `requestedLayer=1`、`layers=[1]`、`viaCount=0`。
  同网名本身不算连通，异层同坐标也不算连通，必须有实际接触的铜或过孔。旋转 RECT（含
  圆角）和 OVAL 按实际几何判定；ELLIPSE、NGON 与 pad-to-pad 接触使用完全包含在真实铜内的
  保守几何，允许漏掉边缘落点但不制造假连通。`POLYGON`、`specialPad`、缺失/畸形 shape
  不使用 bbox 或中心点猜测：必经焊盘直接返回 unknown/error；其它同网未知焊盘若可能改变
  FAIL 也返回 unknown/error。结果的 `excludedCopper` 固定列出
  pours / filled regions / PLANE layers：第一版**不把铺铜或内电层推断成已验证路径**，所以
  FAIL 只表示“没有被 listed routed copper 证明”，整板开路结论仍由 `pcb drc` 给出。
  圆弧作为连续铜参与路径，但其它图元只在圆弧回读端点处与它建立接触；`arcsAvailable`
  缺失/为假时不把空数组当成“没有圆弧”。圆弧中段分叉也会明确列为限制。对 ordered proof，
  工具按实际线宽检查同层铜：平行近邻、正长度叠线、内部交叉、
  track↔arc、arc↔arc 或 via↔via 的非规范重叠都返回 unknown/error；一个普通 endpoint 或
  T junction 仍可使用。圆弧以有误差上界的中心线离散参与检查，无法在预算内可靠规范化时
  同样返回 unknown，避免借不同 primitive ID 在同一物理铜上回走。直接 via-on-pad 重叠不算连接（平台实测需有 track/arc stub），避免制造假
  连通。该命令适合每组关键网布完后的局部核查；保存重开后的最终批次仍应重跑。
- `pcb check` 的 `dangling-end` 也复用 pad shape/rotation：同网 track 端部或其铜宽实际接触
  pad 铜才算 anchor。旧 connector 只有 `width/height` 时使用保守 ellipse/cardinal 或内切圆，
  不用完整 AABB；JSON/human report 的 `limitations` 会说明 legacy/unknown pad 数量。这样
  AMS1117 的大 TAB/窄脚端部落铜不会被固定 30mil 半径误报，旋转焊盘的 AABB 空角也不会假通过。
- `pcb.route.rip_up` — **reliable rip-up**: delete tracks+arcs+vias, `--net` to scope
  (string or list) or omit for ALL. **Copper layers only** — never deletes the board
  outline, silkscreen/assembly/mechanical artwork, or **locked** primitives. The
  iteration primitive: `rip_up → re-route`. (Reports `{requested, ok}` per type, since
  `delete()` is a batch boolean.)
- `easyeda pcb clear` (`pcb.page.clear`) — **一键整版复位**,`sch clear` 的 PCB 对称版。
  一次删掉所有**板级内容** primitive:器件 + 布线(轨/弧/过孔)+ 铺铜/填充(pour/fill)+
  keep-out/规则区域 + 自由丝印(**丝印层 3/4** 的字符串 + 线/弧图形,不碰铜层/文档层的自由文字或
  机械/装配线弧)。`pcb delete`(`pcb.component.delete`)**只删器件**,
  布线/铺铜/区域/丝印会静默残留(`components.list` 看着空了、铜其实还在)——要真正清板重来
  用这个。**默认保留锁定图元 + 板框(layer 11)**(板框是布局前提,和 `sch clear` 保留图框对称)。
  收窄:`--only components,routing,copper,regions,silk`(逗号子集,省略 = 全部);`--no-preserve-outline`
  连板框一起删;`--include-locked` 连锁定图元一起删(危险)。**无 undo**,确认门控。
  **默认自带 verify 复合流程(#121)**:清 → save → `doc reload` → 二遍清 → 最终 dry-run 计数——
  部分图元只在 save/reload 时被引擎物化,单次 handler 调用内任何枚举(含 #112 的循环)都看不到
  (R2 实测 reload 后冒出 3 条轨);返回 `{pass1, pass2, remainingAfterVerify, verified}`,
  `remainingAfterVerify` 非零 = 锁定/保留件或更深的引擎问题,绝不假报干净。`--no-verify` 回到
  单遍(快,但你要自己 reload 后 `--dry-run` 复查)。
  ⚠️ **破坏性**:生产流程必须**先 `--dry-run` 报告删除计数、等用户确认**,再执行。
  生成→检测→清板→重试闭环用这个。
- `easyeda pcb via-delete --ids …` / `pcb track-delete --ids …` (`pcb.route.delete`) —
  **surgical delete by primitiveId**: one bad via no longer costs re-routing the whole
  net (rip-up is net-scoped). Ids come from `pcb via-list` / `pcb track-list` / `pcb drc
  --json` `objs`; **pull them fresh — ids churn after edits**. `--ids` takes **CSV
  (`id1,id2`) or a JSON array (`'["id1","id2"]'`) — both work**; all delete-by-id
  commands (`pcb delete` / `pour-delete` / `region delete` / `fill delete` /
  `track-delete` / `via-delete`) now accept both formats (issue #109), so `pcb drc
  --json` `objs` arrays paste straight in. Each subcommand guards its
  kind (pasting track ids into `via-delete` errors out); locked primitives are skipped,
  stale ids reported as `notFound`. The result's `removed[]` echoes each primitive's full
  before-state (net/layer/geometry) so the audit log can recreate it. **Embedded-primitive
  pre-check + readback (#120, live-verified)**: a footprint-embedded via's id is its
  parent component's primitiveId + a suffix (`ba45…f3` + `e184`); deleting one lies
  TWICE — the SDK returns true AND an immediate getAll shows it gone, but the next
  save/reload re-materializes it from the footprint. The handler refuses these UPFRONT
  (`notDeletable[]` with the parent component + `ok:false`; use `pcb via-bond` to net
  them, or delete the whole component) and additionally readback-verifies the rest
  (`removed`/`count` only count what actually vanished; unattributable survivors land
  in `notDeleted`). ⚠️ **After surgical
  edits (delete/via-hop/fill changes), a burst of same-net (usually GND) Connection
  Errors in DRC is pour-mediated connectivity gone stale, not real breaks — run
  `pcb pour-rebuild` first, then re-judge** (verified live: 11→1 baseline).
- `easyeda pcb via-bond [--component U1] [--dry-run]` — **bond netless footprint-embedded
  vias (EPAD thermal vias) to the net of the pad they sit in** (#118). Scans every net:""
  via whose center sits inside a net-carrying pad's copper rect and assigns that pad's
  net via raw `eda.pcb_PrimitiveVia.modify` (debug-exec backed — works on every deployed
  connector, no re-import). Idempotent, readback-verified (`{planned, assigned, verified}`).
  ⚠️ **Platform limit (live-verified)**: the assignment does NOT survive a doc reload —
  embedded vias re-materialize netless every time; re-run after any reload, before
  DRC / power-planes. `pcb check`'s **netless-via-in-pad** WARN fires whenever a re-bond
  is due, with this command as the fix.
- `easyeda pcb via-hop --net N --from-x … --from-y … --to-x … --to-y …`
  (`pcb.route.via_hop`) — **composite layer hop**: entry stub → via → hop-layer track →
  via → exit stub. **track↔via registers as connected on its own** — no bond fill needed
  (see the truth table below). Vias sit `--stub` (default 20mil) inside the endpoints so
  they stay **off pads** (via-on-pad ≠ connected). `--layer` (default 1=TOP) /
  `--hop-layer` (default 2=BOTTOM), `--width`. `--bond-fill` (default **off**) adds
  optional extra copper over the vias for thermal/current — not for connectivity. Rolls
  back everything it created on mid-sequence failure. Verify with `pcb drc`.
- `pcb.clear_routing` — native `clearRouting` (`@alpha`, may be undefined on this build,
  and does NOT protect unlocked outline) — prefer `pcb.route.rip_up`.

#### 连通性键合真值表 (what actually registers as CONNECTED)

⚠️ **Corrected 2026-07-07 (跟进 pro-api-sdk#31).** The earlier claim — "track↔via does
not register on 4-layer / ex-PLANE boards, a bond fill is the only reliable bridge" —
was **our misdiagnosis** and has been retracted (official confirmed live; we reproduced
the correction on real hardware). What actually happened: DRC Connection Errors are
driven by netlist **ratlines**; a `track(L1)→via→track(L2)→via→track(L1)` bridge between
two same-net pads **satisfies the ratline and clears the error** in every plane state
(clean 4-layer / Inner=PLANE / flipped SIGNAL↔PLANE — all tested). The original
"+5V/U0TXD floating" symptom was **stale pour-mediated GND connectivity**, cured by
`pcb pour-rebuild` (same phenomenon as the ⚠️ note under `via-delete` above) — the fills
that "fixed" it were a red herring; the re-pour/recompute did the work.

| junction | registers? |
|---|---|
| track endpoint on a via (center or inside via copper) | ✅ (needs a fresh ratline recompute) |
| via on a track's body (mid-segment) | ✅ |
| pad ↔ track endpoint at pad center | ✅ |
| net-bound FILL overlapping via + track | ✅ (works, but **not** required) |
| pour (same net) flowing over via | ✅ (but pour reflow has its own traps — see pour section) |
| via ON a pad | ⚠️ offset + stub anyway (a via centered on a pad is redundant, not a bond failure) |

**Via-bridge SOP**: just route the hop with `pcb via-hop` — no bond fill needed. If DRC
shows same-net (usually GND) Connection Errors after routing surgery, that's **stale
pour connectivity**: run `pcb pour-rebuild`, let ratlines recompute, then re-judge — do
**not** paper over it with fills.

### Length constraints — differential pairs / equal-length groups (#176)

**布线前声明,布线后量。** 差分对与等长组是**约束对象**,不是走线:建了它们,EasyEDA 的 DRC 才把
两条网当一对查,`easyeda pcb report` 的 `skew`(|lenP−lenN|)/`spread`(max−min)才有东西可量。
不建 → 那两个数组恒空,报告里的测量能力空转。

```bash
# 差分对(USB / 以太网 / HDMI 这类)
easyeda pcb diff-pair create --name USB0 --positive USB_DP --negative USB_DM
easyeda pcb diff-pair list                       # 约束清单(≠ pcb report 的测量值)
easyeda pcb diff-pair rename --name USB0 --to USB
easyeda pcb diff-pair delete --name USB

# 等长网络组(并行总线 / 地址线),至少 2 条网
easyeda pcb eq-group create --name DDR_ADDR --nets A0,A1,A2
easyeda pcb eq-group add    --name DDR_ADDR --nets A3,A4     # 已是成员的自动跳过
easyeda pcb eq-group delete --name DDR_ADDR
```

**要点**(真机验过):网名**前置校验**,指向板上没有的网 = 零写入拒绝并点名(网名大小写敏感、来自
原理图,用 `easyeda pcb nets` 取准);回执的 `verified` 是连接器**重读比对**出来的,不是平台返回值;
同名同内容重建 = `alreadyExists`(可重放),同名不同内容 = 拒绝并给下一步;差分对**只能改名**,
要换绑定得删了重建。改完可即时读取诊断；最终证据使用 `pcb save → doc reload → list/report`。

**在流程里的位置**:P7 布线之前建好 → `route-critical` / typed track action → `easyeda pcb report` 回读
skew/spread 验收。

### Copper pour (铺铜)

A pour is a net-bound copper region (usually GND/power plane). **The agent passes raw
points** — the connector builds the `IPCB_Polygon` (`pcb_MathPolygon.createPolygon`)
and re-pours; passing raw points to the bare `eda.*` create fails ("无法创建覆铜边框图元").

- `pcb.pour.create` — pour from a closed polygon `points` (`[[x,y],…]`, mil, y-up) on a
  copper layer, bound to a `net` (**required — a netless pour is dead copper; `pcb pour`
  now refuses an empty `--net`, issue #34**). `fill = solid` (default) `| grid | grid45`.
  Size it to the board outline; verify `poured:true` + `pcb.drc.check`.
- `pcb.pour.list` / `pcb.pour.delete` — inspect / remove pours.
- `pcb pour-clean --netless` (daemon-side) — remove pours bound to **no net** (net:"" dead
  copper that `pour-fit --replace` can't clear — it only matches same-net pours). `--dry-run`
  lists them first. Detected by `pcb check` (netless-pour rule).
- `pcb.pour.rebuild` — re-pour all (or by net) after moving components/routing so the
  copper reflows around new obstacles.
- `pcb pour-fit` (daemon-side) — **auto-size a pour to the board**: reads the outline
  and insets its bbox by `--inset` (mil, default 20) so copper keeps edge clearance
  (fixes Board-Outline-to-Copper), then pours `--net`/`--layer`. `--replace` (default)
  clears the net's existing pours first so they don't stack. v1 pours a RECTANGLE within
  the bbox; for an odd outline draw a custom polygon with `pcb pour`. `--dry-run` previews.
- `pcb via-stitch` (daemon-side) — fill a `--rect "x0,y0,x1,y1"` with a `--pitch`-spaced
  grid of `--net` vias: **thermal vias** under a power-IC center pad (tie it to the GND
  plane) or **GND stitching** between top & bottom pours. Run `pcb pour-rebuild` after so
  the planes reflow onto the new vias. `--margin` insets from the rect edges. `--dry-run`.
- `pcb via-fence` (daemon-side) — place vias only on the perimeter of a derived rectangle.
  Use it for a crystal/RF guard boundary where the center must remain free of copper and vias;
  do not substitute `via-stitch`, whose full grid would populate the protected interior.
  `--pitch` is the maximum edge spacing; corners are included once, short final gaps are
  redistributed along each edge, and `--margin` expands outward from the protected rectangle.
  Hole/diameter default to the live rule. Dry-run first, then read back the GND vias and run
  `pcb pour-rebuild` + official DRC.

### Keep-out / rule regions (禁止区域)

A region (`eda.pcb_PrimitiveRegion`) is a polygon carrying **rule types** that keep
things OUT of an area — antenna clearance, board-edge inset, mechanical exclusion.
It is **NOT net-bound copper** (that's a pour) — `create` takes no net. EasyEDA's own
DRC + copper pour respect it (a pour avoids a `no-pours` region). Same raw-points
convention as pour (connector builds the polygon).

- `pcb region create` (`pcb.region.create`) — specify the area **three ways** (pick one):
  `--points '[[x,y],…]'` (explicit polygon), `--rect x0,y0,x1,y1` (rectangular
  shorthand), or **`--ref <designator>`** (the placed component's bbox — e.g. the
  antenna module). `--margin <mil>` expands the `--rect`/`--ref` box outward (antenna
  clearance). `--rule` (repeatable, name or enum number): `no-components(2)` /
  `no-wires(5)` / `no-fills(6)` / `no-pours(7)` / `no-inner-electrical(8)` /
  `follow-rule(9)`. **Default** (no `--rule`) is a hard keep-out
  `[no-components, no-wires, no-pours]` — the antenna / board-edge case. `--locked`
  pins it. Verify with `pcb region list` + `pcb drc`.
  E.g. antenna keep-out under U1: `pcb region create --ref U1 --margin 40 --rule no-pours`.
- `pcb region list` / `pcb region delete` — inspect / remove (note `pcb delete`
  removes components, NOT regions — use `region delete`). `--ids` takes CSV or a
  JSON array.

> **Read-back limit (verified #18):** `--name` on a region is fire-and-forget —
> `getState_RegionName` never reads it back, so `region list` shows `null` and the
> injected DSN keepout is named `region_keepout_N`. Likewise `pcb fill`'s `fillMode`
> always reads back `solid`. Geometry / layer / net / **ruleType** persist fine —
> just don't gate logic on reading a region's name or a fill's mode. Platform SDK
> quirk (same family as the netflag rotation echo trap), not fixable from here.

> **ESP32-S3-WROOM-1 ships with NO antenna keep-out** — you must create it (test-case
> P1). **`getDsnFile` drops regions**, but `pcb export-dsn` now **re-injects** them as
> Specctra `(keepout (polygon …))` by default (reports `keepouts=N`; `--raw` to skip),
> so external Freerouting no longer routes under the antenna. Transform is a verified
> pure translation (1:1 mil, no flip).

### Net-bound filled region (填充区域 / 异形大块铜)

`eda.pcb_PrimitiveFill` — a **STATIC filled polygon bound to a net** (a 3V3/RF-ground
patch, thermal copper, an odd-shaped plane). Three net-copper primitives, don't confuse:
**fill** (static, no reflow), **pour** (`覆铜`, reflows around obstacles), **region**
(keep-out, no net). Same raw-points convention.

- `pcb fill create` (`pcb.fill.create`) — area via `--points` | `--rect x0,y0,x1,y1` |
  `--at x,y --size w,h` | `--ref <designator>` (+ `--margin`), on a `--layer`, bound to
  `--net`. `--fill-mode solid` (default) `| mesh | inner`. `--locked`. Verify with
  `pcb fill list`. ⚠️ **`--rect` 的四个数是两个对角点 `x0,y0,x1,y1`,不是 `x,y,宽,高`**
  (issue #109 实踩:按 x,y,w,h 传参生成盖住 USB-C 区的巨型 fill,原生 DRC 爆 ~50 条)——
  想按「角点 + 宽高」表达就用 **`--at x,y --size w,h`**(与 `--rect` 互斥,`--size` 从
  `--at` 向 +x/+y 延伸)。**防呆护栏**:fill bbox 面积 > 板框 bbox 的 **25%**(板框可读时;
  读不到板框则 > 4,000,000 mil² ≈ 50×50mm)直接拒绝,报错教两角点语义;确属故意的超大 fill
  加 `--force-large` 放行。
- `pcb fill list` / `pcb fill delete` — inspect / remove (filter list by `--layer`/`--net`);
  `delete --ids` takes CSV or a JSON array.

**Board cutout / slot (挖槽) — `pcb slot`.** A fill on the **MULTI layer (12)** IS a
board cutout (per the eda API: *"填充所属层为 MULTI 时代表挖槽区域"*; manufacturing
emits it as a `BoardCutout`). `pcb slot --rect … | --ref ANT1 --margin 20` mills a
hole — antenna isolation / mechanical opening. No net. It's a `pcb_PrimitiveFill` on
layer 12, so list/delete via `pcb fill list --layer 12` / `pcb fill delete`.

**M3 安装孔 — `pcb mount-holes`** (issue #102). Places corner mounting holes
**automatically and collision-checked** — never hand-place M3 holes at guessed
coordinates (#102: a blind hole landed on C1). Reads the real board outline
(errors without one — run `pcb outline-fit` first), computes each corner center
at `--inset` (default 197mil ≈ 5mm) from both edges, and mills a near-circular
MULTI-layer cutout (`--dia` default 126mil = M3 Ø3.2mm) — the same primitive as
`pcb slot`, so `pcb place-constrained` avoids it as a **Tier-1 obstacle** and
`pcb check` keeps copper off the milled edge. Each corner is checked against
every component's rendered bbox with the fastener keep-out radius
`max(hole R+40mil, M3 washer R118mil)` (conventions §2.3): a conflicting corner
is **warned + skipped**, never force-placed (`--clearance` overrides the radius
for a smaller fastener head you knowingly accept); a corner that already has a
cutout reports `exists` (idempotent rerun). `--corners tl,tr,bl,br` picks a
subset; `--dry-run` prints the per-corner plan. Save after placing; delete via
`pcb fill list --layer 12` + `pcb fill delete`.

  easyeda pcb mount-holes --dry-run          # plan only
  easyeda pcb mount-holes                    # 4 corners, M3 defaults
  easyeda pcb mount-holes --corners tl,tr --inset 250
> **Snapshot can't confirm it visually** — `pcb snapshot` (`getCurrentRenderedAreaImage`)
> does NOT auto-redraw after API edits and does not render filled copper/cutouts, so a
> fresh snapshot shows a **stale frame**. Verify slots/fills/pours by **data** (`pcb fill
> list`, DRC, manufacture export), not screenshot — the snapshot is for component layout only.
>
> **Stale-frame detection (issue #31).** `pcb snapshot` carries the anti-stale machinery (the schematic-side snapshot was removed — sch renders via `sch export-image`):
> the result exposes a frame `sha256`, and `--previous-sha256 <sha>` lets the connector
> detect a byte-identical (stale) frame, force a redraw (ratline recompute + zoom-to-all)
> and retry once, reporting `stale:true` if it still cannot refresh. **Reliable recording
> workflow** for user-facing videos/tutorials where the visual artifact is required:
> 1. `easyeda view region --left … --right … --top … --bottom …`（或 `easyeda view fit`）框住目标视口。
> 2. `easyeda pcb snapshot --fit=false --previous-sha256 <上一次的 sha256>`。
> 3. 若结果 `stale:true`，说明画布未刷新 — 告警/失败，不要用该帧。
> 4. 用 `pcb list` / `pcb drc` / `pcb check` / `pcb layout-lint` 做**权威**正确性校验（截图只作视觉终检）。
>
> **底面视觉 QA（issue #40）** — 不再需要人工点 UI 切层。`easyeda pcb view-side --side bottom`
> 会选底铜为当前层并聚焦底面铜+丝印层，随后 `easyeda pcb snapshot`（thread `--previous-sha256`
> 防陈帧）即反映底面（底丝印/底铜/背面装配标记）。更细的显隐用 `easyeda pcb layer-visibility
> --preset bottom-only|top-only|copper-only|silk-only` 或 `--show/--hide`。切当前编辑层用
> `easyeda pcb layer-set --layer bottom|Inner1|<id>`。**注意**：EasyEDA 无原生画布翻面/镜像视图
> API，`view-side` 是「层聚焦」近似（切当前层 + 只显示该面层），不是物理翻板；丝印极性仍以
> `pcb check` 的 silkscreen-flipped 规则做数据级判定为准——该规则判**位号与元件不同层**、
> **`reverse`（任一层）**、以及**顶层 `mirror`**。底层 `mirror` 平台语义未确证（仓库 fixture
> 板多为 `mirror=false`，而 `pcb silk-align` 写 `mirror=true`），两种极性都会误报，故不判。

> **Routing boundary (load-bearing — see `docs/ecosystem-survey.md` §7):** EasyEDA's
> interactive 布线 menu (single/multi/differential **routing**, stretch, optimize,
> length-tuning/serpentine, fanout, remove-loops) has **NO `eda.*` API** — the agent
> cannot do smart/avoiding/push-and-shove routing. Programmatic routing is limited to:
> create tracks/vias/pours by coordinate (above), rip-up, the `@alpha` `autoRouting`
> (undefined on 3.2.148), or read-primitives → external engine → write (the official
> kirouting pattern). So route segment-by-segment, pour planes, and leave smart routing
> to the human/UI. **Shipped: copper pour + rip-up (R1/R2).** **net-class WIDTHS
> are shipped daemon-side** (R3-width): `pcb net-classes` prints the role→spec-width
> ladder, `route-short` sizes each net by role (signal / power-branch / power-trunk /
> high-current — `pcb_netclass.go`), and `pcb check` **width-under-spec** reports
> under-sized power tracks. Still pending: writing those roles into EasyEDA's NATIVE
> net-class rules (`createNetClass`/`overwriteNetRules`, @beta — so the native DRC
> enforces per-class width) + diff-pair/equal-length **definitions** (read side is
> in `pcb.report`).
