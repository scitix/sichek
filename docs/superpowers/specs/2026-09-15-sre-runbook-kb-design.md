# sichek SRE 排障知识库(Runbook KB)设计

日期:2026-09-15
状态:已批准,待实现

## 背景与问题

SRE 使用 `sichek i` / `sichek <component>` 做节点体检,拿到形如下面的输出后
反复来问「这是什么问题?怎么解决?」:

```
ERRO[0001] Check Abnormal: PortSpeed check fail: mlx5_0/p1 expect [200 Gb/sec (2X NDR)], but get [40 Gb/sec (4X QDR)]  checker=check_ib_port_speed component=infiniband
 - infiniband: FAIL
```

现有文档分散且不面向「拿着报错自助排查」:`errors-categorization.md`(级别语义)、
`infiniband.md`(检测项介绍)、`infiniband-checks.html`、`docs/runbooks/` 下只有一篇
`bf3-storage-nic-lldp-sop.md`。缺一个以 checker 为主键、SRE 能 Ctrl-F 对号入座的排障库。

## 目标

- SRE 从 `sichek` 输出里的 `checker=<id>` / `Errors Events` / `ErrorName` 任一字段,
  能在 KB 里 Ctrl-F 命中对应条目,读到「是什么 / 为什么 / 怎么确认 / 怎么修 / 何时升级」。
- 内容与 checker 代码一一对应,级别语义单一真相源,不臆造。

## 非目标(YAGNI)

- 不做搜索型 Web 应用 / 不接检索后端。
- 不改 checker 代码、不改 `Suggestion` 文案(KB 是文档层,只读引用)。
- 不做自动生成流水线(首版人工从代码抽取;后续如需 md→html 再议)。

## 形态与目录

每个组件一个 Markdown,落在现有 `docs/runbooks/` 下,与 `bf3-storage-nic-lldp-sop.md` 同级:

```
docs/runbooks/
  README.md          总导航:索引表 + 通用速查(级别语义、怎么读 sichek 输出)
  infiniband.md      首批全填(约 20 条 IB/RoCE/PCIe checker)
  nvidia.md          骨架 + 索引占位(Xid/GPU,次热)
  pcie.md            骨架(pcie_topo 相关)
  nccl.md            骨架(nccltest)
  ethernet.md cpu.md gpfs.md hang.md transceiver.md   骨架占位,按模块热度后续补
```

理由:纯文本可 Ctrl-F/grep、进仓可 review 可 diff、与 checker 代码同仓不易脱钩、
与现有 runbooks 一致。选 Markdown 而非单个 HTML Artifact,即为规避「与代码脱钩、难 diff」。

## 检索键(主键 = checker ID / ErrorName)

- `README.md` 顶部一张三级索引表,每行:
  **checker ID(如 4013)| checker 名(check_ib_port_speed)| ErrorName(IBPortSpeedNotMax)| 级别 | 模块#锚点**。
  SRE 从输出里拿到的任一列都能 Ctrl-F 命中,点锚点跳到条目。
- 每模块内条目标题即锚点键,格式统一:
  `## check_ib_port_speed — IBPortSpeedNotMax (ID 4013 · Critical)`

## 每条目模板(标准六段)

```
## check_ib_port_speed — IBPortSpeedNotMax (ID 4013 · Critical)

**1. 是什么** — 一句话:校验 HCA 端口速率是否达到 spec 基线。
**2. 典型输出** — 从代码抽的原始报错样例(供 Ctrl-F 命中),代码块原样:
    PortSpeed check fail: mlx5_0/p1 expect [200 Gb/sec (2X NDR)], but get [40 Gb/sec (4X QDR)]
**3. 常见根因** — 按发生频率列 2–4 条(掉速/掉链路/没插满/对端配置不符…)。
**4. 确认命令** — 只读核实命令(ibstat / cat /sys/class/infiniband/*/ports/*/rate 等)。
**5. 处置** — 分根因给步骤;明确区分「节点侧可自愈/重配」vs「该换线换卡(升级硬件团队)」。
**6. 级别与升级** — 引用级别语义(见下)+ 何时升级、给谁。
```

## 级别语义(单一真相源)

直接引用 `docs/errors-categorization.md` 第 4–6 行原文,不再自行转述:

- **Fatal**:立即停止任务并重新提交(任务级动作,**不 cordon 节点**)。
- **Critical**:**cordon 节点** + 尽快修复硬件/软件。
- **Warning**:**cordon 节点** + 择期修复。

即 Critical 与 Warning **都 cordon**,区别只在修复紧迫度;Fatal 是杀任务重投、不 cordon。
`README.md` 通用速查段落原文引用此三条,各模块条目第 6 段引用/链接回该段。

## 内容来源(不编造)

每条六段的字段来源:

- **1 是什么 / 6 级别**:从 checker 代码抽 —— ID 常量(`consts/consts.go`)、
  `Description()`、`result.Level`、ErrorName(代码或 spec)。
- **2 典型输出**:从 checker 里的 `fmt.Sprintf(... "check fail" ...)` 报错模板抽真实样例。
- **3/4/5 根因/确认/处置**:结合代码逻辑 + `errors-categorization.md` 的 Suggestion +
  `infiniband.md` 既有描述 + 现场经验人工补全。

首版用 `sichek i` 的 PortSpeed / PhyState / IBState 三连作为 IB 模块首批样例验证模板。

## 首批交付范围

1. `docs/runbooks/README.md`:索引表 + 通用速查(级别语义、怎么读 sichek 输出、组件↔文件对照)。
2. `docs/runbooks/infiniband.md`:填满全部 IB/RoCE/PCIe checker 条目。
3. 其余模块建骨架:文件标题 + 该模块 checker 索引占位 + 「待填」标注,按热度排后续。

## 验收标准

- 拿本会话开头那段 `sichek i` 输出,SRE 能用 `check_ib_port_speed` /
  `check_ib_phy_state` / `check_ib_state` 三个 key 各自 Ctrl-F 命中 `infiniband.md` 条目。
- 每条 IB 条目六段齐全,级别与 `errors-categorization.md` 一致。
- README 索引表覆盖 IB 全部 checker,每行可点锚点跳转。
- 所有新文件带 Markdown、进仓、无 TODO 占位残留(骨架模块除外,骨架显式标「待填」)。

## 后续(不在首版)

- 按模块热度补齐 nvidia / pcie / nccl / ethernet / cpu / gpfs / hang / transceiver。
- 如需发给不看仓库的 SRE,再从 md 生成导航 HTML。
