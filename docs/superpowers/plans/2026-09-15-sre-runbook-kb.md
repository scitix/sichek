# sichek SRE 排障知识库(Runbook KB)实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建一套以 checker 名为主键、SRE 拿着 `sichek` 报错能 Ctrl-F 自助排查的 Markdown 知识库,首批填满 infiniband 模块。

**Architecture:** 每个组件一个 Markdown 落 `docs/runbooks/`;`README.md` 做总导航+索引表+级别语义;每个 checker 一个六段条目,字段从 `components/infiniband/config/check_items.go`(权威映射)与各 checker `.go` 的 Detail 模板抽取,不臆造。

**Tech Stack:** 纯 Markdown,无构建。验证靠 `grep`。

> **相对 spec 的一处细化:** spec 原定检索键含数字 checker ID(如 4013)。实现中发现多个 checker 构造器复制粘贴 `id: consts.CheckerIDInfinibandFW`,运行时 ID 不可靠;且 `sichek i` 面向 SRE 的输出只显示 `checker=check_ib_port_speed`,不显示数字 ID。故**放弃数字 ID 作为检索键**,改用 `checker 名 + ErrorName + Level` 三键(全部来自 `check_items.go`,权威且与 SRE 输出一致)。

---

## 权威数据源(实现时逐字转录,勿改写)

以下 17 条来自 `components/infiniband/config/check_items.go` 的 `InfinibandCheckItems` map(已读取全文,字段权威)。**「典型输出」列**的模板来自对应 checker `.go` 的 `result.Detail = ...` / `Errorf(...)`。

| checker 名 | ErrorName | Level | Description(是什么) | Suggestion(官方建议) | 典型输出模板 |
|---|---|---|---|---|---|
| check_ib_ofed | OFEDVersionMismatch | warning | Check if the installed OFED version matches the specification | Upgrade or reinstall OFED to match specification | `OFED version mismatch, expected:<spec>  current:<curr>` |
| check_ib_num | IBDeviceCountMismatch | critical | Check if the number of IB devices matches PCI scan | Check PCIe status or IB NIC connectivity | (见 ib_devs.go,`(missing)`/count 不符) |
| check_ib_fw | IBFirmwareVersionMismatch | warning | Check if firmware version matches the specification | Update firmware to match version in specification | `fw check fail: hca:<dev> psid:<board_id> curr:<fw>, spec:<spec>` |
| check_ib_state | IBStateNotActive | critical | Check if all IB ports are in ACTIVE state | Check OpenSM and IB connection | `PortState check fail: <hca> NOT ACTIVE`(stderr:`PortState abnormal on <hca>: 1: DOWN doesn't contain ACTIVE`) |
| check_ib_phy_state | IBPhyStateNotLinkUp | critical | Check if all IB physical states are LINK_UP | Verify IB cable and link status | `PhyState check fail: <hca> NOT LinkUp`(stderr:`PhyState abnormal on <hca>: 3: Disabled doesn't contain LinkUp`) |
| check_net_operstate | IBNetOperStateNotUP | critical | Check if network operstate is UP | Check network interface and driver | `NetOperstate check fail: <hca> expected state = <s>, current state = <c>` |
| check_ib_port_speed | IBPortSpeedNotMax | critical | Check if IB port speed is set to maximum | Ensure IB speed settings are correct in firmware | `PortSpeed check fail: <hca> expect <spec>, but get <curr>` |
| check_pcie_acs | PCIEACSNotDisabled | critical | Check if PCIe ACS is disabled | Disable ACS in BIOS or kernel settings | (ACS 未关) |
| check_pcie_mrr | PCIEMRRIncorrect | info | Check if PCIe Max Read Request (MRR) is set correctly (4096) | Set MRR to 4096 via system config | `PCIEMRR check fail: <hca> expect <spec>, but get <curr>` |
| check_pcie_tree_speed | PCIETreeSpeedDownDegraded | critical | Check full PCIe tree speed to root complex | Check upstream PCIe device speed and configuration | (逐跳链路 speed 降级行,见 pcie_tree_speed.go suggestionLines) |
| check_pcie_tree_width | PCIETreeWidthIncorrect | critical | Check full PCIe tree width to root complex | Check PCIe switch and topology configuration | (逐跳链路 width 降级行) |
| check_ib_kmod | IBKernelModulesNotAllInstalled | critical | Check if all required IB kernel modules are installed | Install or reload missing kernel modules | `need to install kmod:<list>` |
| check_ib_devs | IBDeviceNameMismatch | warning | Check if IB device names match expectation | Verify udev or naming rules | `<expected> (missing)` / `<expected> -> <actual> (expected <exp>)` |
| check_roce | RoCENotEnabled | warning | Check if RoCE vf is enabled | Enable RoCE in the device configuration | `RoCE checks failed: <detail>`(网关不可达:`gateway '<gw>' is unreachable...`) |
| check_ib_lost | IBLost | critical | Check if IB device is lost | Check IB device status | `IBLost: <details>` |
| check_ib_rail_count | IBRailCountOdd | critical | Check if the number of compute-rail HCAs is plausible (even, or a single rail) | An odd rail count usually means an HCA vanished from the RDMA stack; compare against the node's expected topology and check dmesg for mlx5_core probe failures | (奇数轨告警,见 ib_rail_count.go) |
| check_ib_mezz_name | IBMezzNameMismatch | critical | Check that each mezz card (board_id NVD0000000079) RDMA device is named mezz_<k> | The mezz card was not renamed to mezz_<k>; ensure rdma-env-pre interface-naming ran on this node | `<dev> port 1 ==> <netdev> (<link>)  [expected mezz_<k>]` |

**排除项:** `check_ib_driver` 自 2025-09-30 是死代码(map 条目被顶掉、含 `log.Fatalf`),**不建条目**;`dev_port` 非独立 checker。

**级别语义(逐字引用 `docs/errors-categorization.md` 第 4–6 行):**
- **Fatal**:Stop the task immediately and resubmit it.(任务级,不 cordon)
- **Critical**:Cordon the node and fix hardware or software issues as soon as possible.
- **Warning**:Cordon the node and schedule hardware or software fixes at a convenient time.

---

## 文件结构

- 创建 `docs/runbooks/README.md` — 总导航:如何读 sichek 输出、级别语义、组件↔文件对照表、IB 索引表。
- 创建 `docs/runbooks/infiniband.md` — 17 条 IB checker 六段条目(全填)。
- 创建骨架(标题+索引占位+「待填」标注):`docs/runbooks/nvidia.md`、`pcie.md`、`nccl.md`、`ethernet.md`、`cpu.md`、`gpfs.md`、`hang.md`、`transceiver.md`。
- 已存在、不动:`docs/runbooks/bf3-storage-nic-lldp-sop.md`。

---

## Task 1: 建 README.md 骨架(导航 + 级别语义 + 组件对照)

**Files:**
- Create: `docs/runbooks/README.md`

- [ ] **Step 1: 写 README.md**

写入以下完整内容:

```markdown
# sichek 排障知识库(SRE Runbook)

本目录面向 SRE:拿着 `sichek i` / `sichek <component>` 的报错,按 **checker 名** 对号入座,查「是什么 / 为什么 / 怎么确认 / 怎么修 / 何时升级」。

## 怎么用

1. 看 `sichek` 输出里的 `checker=<名字>` 字段,或 `Errors Events` 里的报错句子。
   例:`checker=check_ib_port_speed`。
2. 在下方索引表 Ctrl-F 搜该 checker 名 / ErrorName,点链接跳到对应模块文件的条目。
3. 每个条目分六段,照「处置」段操作;拿不准或需换硬件时按「级别与升级」段处理。

## 级别语义(权威:`../errors-categorization.md`)

| 级别 | 动作 | 是否 cordon 节点 |
|---|---|---|
| **Fatal** | 立即停止任务并重新提交(任务级) | **否** |
| **Critical** | Cordon 节点,尽快修复硬件/软件 | **是** |
| **Warning** | Cordon 节点,择期修复 | **是** |

> 注意:Critical 与 Warning **都会 cordon 节点**,区别只在修复紧迫度。Fatal 是杀任务重投,不 cordon。

## 组件 ↔ 文件对照

| 组件(`component=`) | 文件 | 状态 |
|---|---|---|
| infiniband | [infiniband.md](infiniband.md) | 已填满 |
| nvidia / gpu | [nvidia.md](nvidia.md) | 待填 |
| pcie / pcie_topo_check | [pcie.md](pcie.md) | 待填 |
| nccl / nccltest | [nccl.md](nccl.md) | 待填 |
| ethernet | [ethernet.md](ethernet.md) | 待填 |
| cpu | [cpu.md](cpu.md) | 待填 |
| gpfs | [gpfs.md](gpfs.md) | 待填 |
| hang | [hang.md](hang.md) | 待填 |
| transceiver | [transceiver.md](transceiver.md) | 待填 |

## InfiniBand checker 索引

| checker 名 | ErrorName | 级别 | 条目 |
|---|---|---|---|
| _(Task 3 填入 17 行)_ | | | |
```

- [ ] **Step 2: 验证文件建成、级别语义正确**

Run: `grep -c 'cordon' docs/runbooks/README.md && grep -q 'Fatal' docs/runbooks/README.md && echo OK`
Expected: 输出 ≥2 与 `OK`(Fatal/Critical/Warning 三行齐)。

- [ ] **Step 3: Commit**

```bash
git add -f docs/runbooks/README.md
git commit -m "docs(runbooks): scaffold SRE KB README with level semantics and index"
```

---

## Task 2: 写 infiniband.md 全部 17 条条目

**Files:**
- Create: `docs/runbooks/infiniband.md`
- 参考(只读):`components/infiniband/config/check_items.go`、`docs/infiniband.md`、上文权威数据表。

- [ ] **Step 1: 写文件头 + 前 3 条(本会话报错三连,作模板样例)**

写入以下内容(文件开头):

```markdown
# InfiniBand / RoCE 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## check_ib_phy_state — IBPhyStateNotLinkUp (Critical)

**1. 是什么** — 校验所有 IB 端口物理层是否 LINK_UP(链路是否物理点亮)。

**2. 典型输出**
```
PhyState check fail: mlx5_0/p1 NOT LinkUp
（stderr:PhyState abnormal on mlx5_0/p1: 3: Disabled doesn't contain LinkUp）
```

**3. 常见根因**
- 光模块/线缆没插好、坏线、坏口(最常见)。
- 对端交换机口 shutdown 或未配置。
- HCA 固件把口置 Disabled(如固件内部故障)。

**4. 确认命令(只读)**
```
ibstat <device>                 # 看 Physical state
cat /sys/class/infiniband/<dev>/ports/1/phys_state
mlxlink -d <pci> -m             # 看链路/信号质量(边缘链路会 flapping)
```

**5. 处置**
- Disabled/Polling 且线缆在位:重插光模块与线缆两端;换一根已知好线复测。
- 换线仍 down:该口/该卡疑似硬件故障 → 走「6. 升级」换卡/换口。
- 对端交换机口问题:联系网络团队核对交换机侧 admin/oper 状态。

**6. 级别与升级** — Critical:**cordon 节点**,尽快修。换线无效即判硬件,升级给硬件/网络团队换卡或换口。

---

## check_ib_state — IBStateNotActive (Critical)

**1. 是什么** — 校验所有 IB 端口逻辑状态是否 ACTIVE(SM 是否已把口带起来)。

**2. 典型输出**
```
PortState check fail: mlx5_0/p1 NOT ACTIVE
（stderr:PortState abnormal on mlx5_0/p1: 1: DOWN doesn't contain ACTIVE）
```

**3. 常见根因**
- 物理层没起(先看 `check_ib_phy_state`,phy 不 up 时 state 必不 active)。
- OpenSM/子网管理器没跑或没发现该口(INIT 卡住)。
- 对端未连通。

**4. 确认命令(只读)**
```
ibstat <device>                 # State: Active/Init/Down
sminfo                          # 有无 SM
cat /sys/class/infiniband/<dev>/ports/1/state
```

**5. 处置**
- phy 也 down:先按 `check_ib_phy_state` 处理线缆/硬件。
- phy up 但 state=Init:检查 OpenSM 状态(`systemctl status opensm`),重启 SM 或核对 SM 是否覆盖该子网。
- 排除后仍 down:升级硬件/网络团队。

**6. 级别与升级** — Critical:**cordon 节点**,尽快修。SM/线缆均排除后升级。

---

## check_ib_port_speed — IBPortSpeedNotMax (Critical)

**1. 是什么** — 校验 IB 端口协商速率是否达到该机型 spec 基线(如 200 Gb/sec 2X NDR)。

**2. 典型输出**
```
PortSpeed check fail: mlx5_0/p1 expect [200 Gb/sec (2X NDR)], but get [40 Gb/sec (4X QDR)]
```

**3. 常见根因**
- 链路协商掉档(坏线/脏光口/线缆规格不够)。
- 对端交换机口速率配置不符。
- 端口实际处于 down/QDR 回落态(常伴随 phy_state / state 同时 FAIL,如本例 eth0 同时三报)。
- 该口本不该是算力口(spec 基线与实际角色不符,联系配置中心核对 spec)。

**4. 确认命令(只读)**
```
ibstat <device>                                   # Rate
cat /sys/class/infiniband/<dev>/ports/1/rate
mlxlink -d <pci>                                  # 协商速率与线缆信息
```

**5. 处置**
- 同时 phy/state FAIL:按物理层链路问题处理(重插/换线),速率通常随之恢复。
- 仅速率掉档:换线、清洁光口;核对对端交换机口速率。
- spec 期望与实际角色不符(如管理口被当算力口判):核对该机型 spec 基线,勿盲目换硬件。

**6. 级别与升级** — Critical:**cordon 节点**,尽快修。换线/核对 spec 后仍不达标则升级换卡。
```

- [ ] **Step 2: 追加剩余 14 条**

按同一六段模板,用「权威数据表」逐条转录以下 checker(顺序建议:先 critical 硬件类,后 warning 软件类):

`check_ib_lost`、`check_ib_rail_count`、`check_ib_num`、`check_ib_kmod`、`check_net_operstate`、`check_pcie_tree_speed`、`check_pcie_tree_width`、`check_pcie_mrr`(注意 Level=**info**,不 cordon,只提示)、`check_pcie_acs`、`check_ib_mezz_name`、`check_ib_ofed`、`check_ib_fw`、`check_ib_devs`、`check_roce`。

每条要求:
- 标题格式 `## <checker名> — <ErrorName> (<Level 首字母大写>)`。
- 「1. 是什么」用表里 Description。
- 「2. 典型输出」用表里模板;模板标注不全的(num/tree_speed/tree_width/rail_count/acs),先 `grep -nE 'Detail *=|Errorf|Sprintf' components/infiniband/checker/<file>.go` 取准确字符串再填。
- 「5. 处置」以表里 Suggestion 为骨架,结合根因展开;区分「节点侧可修」vs「换硬件」。
- 「6. 级别与升级」按该条 Level 引用 README 级别语义;**info 级**(pcie_mrr)注明「不 cordon,提示性」。
- 已知真机正例可一句话带过(如 lmg104 mlx5_1 固件崩溃对应 check_ib_lost;changliu-g88-2 对应 check_pcie_tree_speed),不强求。

- [ ] **Step 3: 验证 17 条齐全、每条六段齐全**

Run:
```bash
grep -c '^## check_' docs/runbooks/infiniband.md
grep -oE '^\*\*[1-6]\.' docs/runbooks/infiniband.md | sort | uniq -c
```
Expected: 第一行 `17`;第二行每个 `**1.`~`**6.` 各出现 17 次。

- [ ] **Step 4: 验证本会话三连可 Ctrl-F 命中**

Run: `for k in check_ib_phy_state check_ib_state check_ib_port_speed; do grep -q "## $k " docs/runbooks/infiniband.md && echo "$k OK" || echo "$k MISSING"; done`
Expected: 三行均 `OK`。

- [ ] **Step 5: Commit**

```bash
git add -f docs/runbooks/infiniband.md
git commit -m "docs(runbooks): add InfiniBand/RoCE troubleshooting entries (17 checkers)"
```

---

## Task 3: 回填 README 的 IB 索引表

**Files:**
- Modify: `docs/runbooks/README.md`(替换 `_(Task 3 填入 17 行)_` 占位行)

- [ ] **Step 1: 用 17 行替换占位**

把索引表占位行换成 17 行,每行:`| <checker名> | <ErrorName> | <级别> | [跳转](infiniband.md#<锚点>) |`。
锚点由标题小写化生成:`## check_ib_phy_state — IBPhyStateNotLinkUp (Critical)` → 锚点 `check_ib_phy_state--ibphystatenotlinkup-critical`(空格转 `-`、去括号、`—` 前后各留一 `-` 成 `--`)。
生成锚点用命令核对:
```bash
grep '^## ' docs/runbooks/infiniband.md
```
逐条按 GitHub 规则(小写、非字母数字去除、空格转连字符)手工写锚点。

示例 3 行:
```markdown
| check_ib_phy_state | IBPhyStateNotLinkUp | Critical | [跳转](infiniband.md#check_ib_phy_state--ibphystatenotlinkup-critical) |
| check_ib_state | IBStateNotActive | Critical | [跳转](infiniband.md#check_ib_state--ibstatenotactive-critical) |
| check_ib_port_speed | IBPortSpeedNotMax | Critical | [跳转](infiniband.md#check_ib_port_speed--ibportspeednotmax-critical) |
```

- [ ] **Step 2: 验证索引 17 行、无占位残留**

Run:
```bash
grep -c 'infiniband.md#' docs/runbooks/README.md
grep -c 'Task 3 填入' docs/runbooks/README.md
```
Expected: 第一行 `17`;第二行 `0`。

- [ ] **Step 3: Commit**

```bash
git add -f docs/runbooks/README.md
git commit -m "docs(runbooks): populate InfiniBand checker index in README"
```

---

## Task 4: 建其余模块骨架

**Files:**
- Create: `docs/runbooks/{nvidia,pcie,nccl,ethernet,cpu,gpfs,hang,transceiver}.md`

- [ ] **Step 1: 每个文件写统一骨架**

每个文件写入(以 nvidia 为例,其余同构、改标题与组件名):

```markdown
# NVIDIA / GPU 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

**状态:待填。** 本模块条目尚未编写,先按下述来源自助排查:
- 级别与官方建议:`../errors-categorization.md`
- 检测项说明:`../nvidia.md`

<!-- 后续按 infiniband.md 的六段模板补充每个 checker 条目 -->
```

各文件对应参考文档:pcie→`../pcie-topo-check.html`、nccl→`../nccl.md`、ethernet→`../ethernet.md`、cpu→`../system.md`、gpfs→`../gpfs.md`、hang→`../hang.md`、transceiver→`../transceiver-checks.html`。参考文档不存在的就只留 `../errors-categorization.md`。

- [ ] **Step 2: 验证 8 个骨架都建成且标注待填**

Run: `for f in nvidia pcie nccl ethernet cpu gpfs hang transceiver; do grep -q '待填' docs/runbooks/$f.md && echo "$f OK" || echo "$f MISSING"; done`
Expected: 8 行均 `OK`。

- [ ] **Step 3: Commit**

```bash
git add -f docs/runbooks/nvidia.md docs/runbooks/pcie.md docs/runbooks/nccl.md docs/runbooks/ethernet.md docs/runbooks/cpu.md docs/runbooks/gpfs.md docs/runbooks/hang.md docs/runbooks/transceiver.md
git commit -m "docs(runbooks): scaffold remaining component runbook stubs"
```

---

## Task 5: 终检(验收标准)

- [ ] **Step 1: 验收 — 本会话 sichek 输出的三个 key 各能命中 infiniband.md 条目**

Run: `for k in check_ib_port_speed check_ib_phy_state check_ib_state; do grep -q "## $k " docs/runbooks/infiniband.md && echo "$k HIT"; done`
Expected: 三行 `HIT`。

- [ ] **Step 2: 验收 — 无 TODO/占位残留(骨架的「待填」除外)**

Run: `grep -rnE 'TBD|TODO|填入|XXX' docs/runbooks/*.md | grep -v '待填'`
Expected: 无输出。

- [ ] **Step 3: 验收 — README 索引级别与 check_items.go 一致**

Run:
```bash
# 抽 README 里各 checker 的级别,人工比对 check_items.go 的 Level
grep 'infiniband.md#' docs/runbooks/README.md
```
Expected: 每行级别与 `components/infiniband/config/check_items.go` 对应条目 Level 一致(critical/warning/info)。

- [ ] **Step 4: 汇报完成**

列出新增文件、条目数、验收命令输出。无剩余提交则结束。

---

## Self-Review 记录

- **Spec 覆盖:** 形态(每模块 md,Task 1/2/4)、主键(checker 名+ErrorName+Level,Task 3 索引)、六段模板(Task 2)、级别语义单一源(Task 1 引 errors-categorization.md)、内容来源不臆造(权威数据表)、首批范围(README+IB 满填+其余骨架,Task 1–4)、验收标准(Task 5)——均有对应任务。
- **偏差:** 数字 ID 弃用,已在计划头说明理由,并同步了 spec 的检索键字段。
- **占位扫描:** 数据表已含 17 条真实字段;模板不全的 5 条明确给了 grep 取值命令(非「自行发挥」)。
- **一致性:** checker 名、ErrorName、Level 全程取自 `check_items.go`,跨任务一致。
