# nodespec:HPN/GPU 设备 spec 管理工具 — 设计文档

- 日期:2026-07-21
- 状态:设计已评审(数据模型、CLI/数据流两节均已确认)
- 工作名:`nodespec`(可改)
- 关系:独立新项目;sichek 是其下游消费方之一

## 1. 背景与问题

sichek 的 spec 体系(`components/*/config/default_*_spec.yaml`)是"给 checker 读阈值"长出来的,存在结构性局限:

- **按集群名分段 + hostname 正则匹配**:zy3 改名(`gpu-47-3` → `hydra-gpu-214-171-47-3`)导致 spec 段失配;混卡集群共用一份 cluster-wide spec 造成误报(IBLost 方向 1 因此被删)。
- **只有 checker 要读的字段**:没有"这台机器的网络是怎么配置的"这一层——功能角色(计算网/存储网/带内管理)、SR-IOV/bond/多 plane 形态、QoS 面、OVS,全都不可见。
- **监控项不可声明**:采哪些 counter、阈值多少、什么等级,散落在 Go 代码和 spec 字段里,无法按机型复用与覆盖。
- **无来源可追溯**:spec 字段是谁、从哪台机器、什么时候确认的,没有记录。

团队需要一个上游的、多组件(hpn 先行,gpu 等后续)的 spec 管理系统:**给定一台设备(SN),能回答"它的网络应该怎么配、实际怎么配、监控哪些项"**,并能把基线渲染下发给 sichek 等消费方。参照物:阿里 NUSA 的 `rqa.cfg`(阈值+等级+巡检周期声明)与 OFED↔固件兼容矩阵。

## 2. 需求决策记录

| 议题 | 决策 |
|---|---|
| 项目定位 | 独立新项目,与 sichek 是上下游关系(本工具在上游) |
| 一期覆盖层次 | 卡级基线 + 节点级网络形态/软件栈 + 监控项声明;三者**拆开建模**;fabric/拓扑级(轨道、交换机、LLDP)放二期 |
| 组件化 | 工具按多组件设计:hpn 一期,gpu 等复用同一框架 |
| 设备匹配 | 以 **SN(序列号)** 锚定设备身份;hostname 仅作注释性元数据 |
| 产品形态 | 两段式:一期 git 仓 YAML + Go CLI;二期服务化(API/DB/UI) |
| 一期验收 | 真机采集闭环 + 渲染出与 sichek 现有 spec **语义等价**的产物 + 按 SN 查询 |
| 架构方案 | 三层分离:硬件目录 + 机型 + SN 注册表,监控包独立引用(候选方案 A,已选) |

## 3. 真机逆向发现(schema 的事实依据)

设计前对 4 台形态差异最大的节点做了只读探测(2026-07-21):

| 节点 | SN | 机型 | 关键形态 |
|---|---|---|---|
| clnet36 | 21C803287 | IEIT NF5688-M7,8×H20 | 4×CX7(MT_0000000838)400G **RoCE PF,每 PF `sriov_numvfs=16`**,VF 经 `physfn` 可自动重建 PF→VF 树;计算网 MTU 9000;mgmt bond0 802.3ad MTU 1500;`rdma system = exclusive` |
| zy3 | W3S0MD0002M2 | ASUS XA NB3I-E12,8×B300 | 8×CX8(NVD0000000072)**12 口取 4 plane**(3/6/9/12 LinkUp,余者 `phys_state=Disabled`);per-plane netdev `eth_rX_pY` MTU 9266,VF `eth_rX` MTU 9216;**OVS 3.3 br-rail0..7** MTU 9216;独立存储卡 2×MT_0000000834(`s_eth0/1`);mezz×4(NVD0000000079,IB);OFED-internal-26.01;`ecn/roce_np|roce_rp` 与 debugfs `cc_params` 可只读采集 |
| shg205 | 210235A4YR5259C00019 | I57G90N13M63S10,8×RTX5090 | 仅 1×CX5(MT_0000000013)入 bond;**无 OFED(inbox 驱动)**;dmidecode manufacturer 字段为 "N/A" —— SN/DMI 质量参差的实证 |
| bjg66 | GPG663612A0067 | Giga G593-SD1,8×H200 | 8×400G IB 计算网 + **1×HDR 存储卡(MT_0000000223)混卡**;IPoIB MTU 2044;mgmt bond0 |

核心洞察:

1. **每台机器的网络都可分解为功能角色**:计算网 / 存储网 / 带内管理(4 台全有 bond0)/ mezz。这是 sichek spec 完全缺失的维度,也是 model schema 的组织主轴。
2. **形态枚举可收敛为四种**:`plain`(bjg66)、`sriov`(clnet36)、`multi-plane`(zy3)、`bond`(shg205 计算网 & 所有节点 mgmt)。
3. **capture 可自动重建结构**:`physfn` → PF/VF 关系;`ports/*/phys_state` → 活跃 plane 集合;`/proc/net/bonding/*` → bond 成员;OVS `list-br` → rail 桥。
4. **SN 可用但需兜底**:4/4 节点 SN 非空,但 DMI 其他字段可能为 "N/A",需 fingerprint 交叉校验。

## 4. 数据模型

Git 仓库,全 YAML,附 JSON Schema 校验:

```
specs/
  catalog/hca/<board_id>.yaml     # 硬件目录:这种卡是什么(与装在哪无关)
  models/<机型名>.yaml             # 机型:这类节点网络怎么配(核心对象)
  monitors/<监控包名>.yaml         # 监控包:采什么指标、阈值、等级、周期
  registry/<集群>.yaml             # SN → 机型 绑定 + per-SN 例外
facts/                            # capture 产物(现状快照,按 SN 归档;不是 spec)
```

### 4.1 catalog/hca/<board_id>.yaml — 硬件目录

继承 sichek hca spec 的字段,补三块:

```yaml
identity:
  board_id: MT_0000000838
  opn: MCX75310AAS-NEAT          # 订货号
  hca_type: MT4129
  vpd: "..."
  net_port: 1
baseline:
  fw_ver: ">=28.43.2566"
  port_speed: "400 Gb/sec (4X NDR)"
  pcie: {width: 16, speed: "32.0 GT/s", tree_width: 16, tree_speed: 32, mrr: 4096, acs: disable}
capabilities:
  link_layers: [InfiniBand, Ethernet]   # VPI 卡两种都能跑
perf:
  one_way_bw_gbps: 360
  avg_latency_us: 10.0
provenance:
  - {source: "captured from SN 21C803287", date: 2026-07-21}
  - {source: "vendor doc <url>", date: ...}
```

### 4.2 models/<机型名>.yaml — 机型(按功能角色组织)

```yaml
# models/g593-sd1-h200-ib.yaml   (bjg66 一类)
fingerprint:                      # 用于 propose 匹配与 SN 兜底校验
  product: "G593-SD1*"
  gpu: {model: "NVIDIA H200", count: 8}
fabrics:
  compute:
    link_layer: InfiniBand
    hca: MT_0000000838            # 引用 catalog
    count: 8
    form: plain                   # plain | sriov | multi-plane | bond
    netdev: "ib[0-7]"
    mtu: 2044
    rdma_mode: shared             # shared | exclusive
    monitors: [ib-link, ib-counters]
  storage:
    link_layer: InfiniBand
    hca: MT_0000000223
    count: 1
    netdev: "ib8"
    monitors: [ib-link]
  mgmt:
    form: bond
    bond: {mode: 802.3ad, xmit_hash: layer3+4}
    mtu: 1500
sw_stack:
  ofed: ">=MLNX_OFED_LINUX-24.10"
  kernel_modules: [rdma_ucm, rdma_cm, ib_ipoib, mlx5_core, mlx5_ib, ib_uverbs, ib_umad, ib_cm, ib_core, mlxfw]
  kernel: ">=5.15"
```

`form` 的参数化示例(来自实证):

- `sriov`:`{numvfs: 16, vf_netdev: "eth[0-3].\\d+"}`(clnet36)
- `multi-plane`:`{ports: [3,6,9,12], plane_netdev: "eth_r[0-7]_p[0-3]", ovs_bridges: "br-rail[0-7]", plane_mtu: 9266, vf_mtu: 9216}`(zy3)
- QoS 段挂在 fabric 下(`qos: {dcqcn: ..., dscp: ..., trust: ..., ecn: ...}`);**一期只采集与记录,不下发**。

### 4.3 monitors/<监控包名>.yaml — 监控包

```yaml
# monitors/roce-lossless.yaml    (种子阈值来自 NUSA rqa.cfg + sichek 现有检查项)
source: ethtool                   # sysfs-counter | ethtool | mlxlink | cmd
interval: 10s
items:
  - {metric: rx_crc_errors_phy, warn: ">0/min", error: ">100/min", level: Critical}
  - {metric: link_down_events,  error: ">0",   level: Critical}
  - {metric: out_of_sequence,   warn: ">0/min", level: Warning}
  - {metric: local_ack_timeout, warn: ">0/min", level: Warning}
```

`level` 对齐 sichek 的 Fatal / Critical / Warning 三级语义(docs/errors-categorization.md)。机型引用监控包,可在 fabric 下覆盖单项阈值。

### 4.4 registry/<集群>.yaml — SN 注册表

```yaml
cluster: lh                       # 注释性;不参与匹配
nodes:
  - sn: GPG663612A0067
    model: g593-sd1-h200-ib
    meta: {hostname: lh-g23-141, alias: bjg66}    # 仅注释
    exceptions: []
  - sn: EXAMPLE0000001            # 示意条目:演示例外结构(真实 SN 由 capture 采得)
    model: g593-sd1-h200-ib
    exceptions:
      - {field: "fabrics.storage.pcie_path", note: "6-BDF 非均匀路径,两跳 Gen2 retimer", date: 2026-05-18, reason: "已知硬件形态,非故障"}
```

例外必须带 `reason` 与 `date`。**一台设备的完整视图** = registry(SN) → model → 展开 catalog 引用 + 展开 monitors 引用 + 叠加 per-SN 例外。

## 5. CLI 与数据流

Go + cobra 单二进制,**静态编译(`CGO_ENABLED=0`)** —— 消除 zgg76 式 glibc 地板问题,任何目标节点直接可跑。

| 动词 | 作用 |
|---|---|
| `capture` | 在节点只读采集全部事实 → `facts/<SN>.yaml`:dmidecode 身份、GPU 清单、IB 设备树(含 PF→VF/plane/bond 自动重建)、OFED/内核模块/rdma mode、MTU/QoS(ecn、cc_params)/OVS。本机跑或被 scp 过去跑,零依赖 |
| `propose` | facts → 机型草稿:按 fingerprint 匹配已有 model,匹配上输出偏差,匹配不上生成新 model 骨架供人工审定 |
| `diff <SN>` | 现状 facts vs 期望展开(registry→model→catalog)→ 偏差报告;日常审计动词 |
| `render --target sichek` | model+catalog+monitors → 生成 sichek `default_spec.yaml` / hca spec;二期加 `--target prometheus-rules` |
| `query <SN>` | 人读视图:这台机器网络怎么配、监控哪些项、已知例外 |
| `validate` | JSON Schema 校验 + 引用完整性(model 引用的 board_id / 监控包必须存在;registry 引用的 model 必须存在) |

**闭环数据流**:

```
capture(真机) → propose(草稿) → 人工审定 PR 入库(git 审批 = 审定流程)
    ↘ diff(周期审计)          ↘ render(下发 sichek) / query(服务运维)
```

## 6. 与 sichek 对接(一期不动 sichek 代码)

- render 产物与现网 `default_*_spec.yaml` 做**结构化语义等价对比**(YAML 解析后逐字段,非文本 diff),以回归矩阵覆盖的集群为金标准 —— 这是一期验收项。
- sichek 继续从 OSS 拉 spec;变化只是 spec 的生产源头换成本工具。二期再评估 sichek 直接消费 nodespec 展开视图。

## 7. 一期验收标准

对回归矩阵真机(含 SR-IOV / BF3 / CX8 多 plane / bond / RoCE / 无 IB 全形态):

1. `capture` 能产出正确的 facts(与人工核对一致);
2. 人工审定后形成机型库(预计 ≤10 个 model 覆盖全部矩阵节点);
3. `render --target sichek` 产物与现网 spec 语义等价;
4. `query <SN>` 能回答"网络怎么配、监控哪些项";
5. `validate` 与 `diff` 在 CI 可跑。

## 8. 测试策略

- 本次逆向的 4 台真机 facts 做 golden fixtures;
- capture 的 sysfs 解析:`t.TempDir()` 构造假 sysfs 树,表驱动(`name/input/want/wantErr` + `t.Run`);
- render 等价性:以回归矩阵集群现网 spec 为 golden;
- 全部纯 Go 单测,不依赖真机;真机冒烟归入现场回归流程。

## 9. 风险与对策

| 风险 | 对策 |
|---|---|
| SN 质量参差(shg205 manufacturer="N/A";SN 可能重复/为空) | fingerprint 交叉校验兜底;SN 为空或冲突时 capture 报错、registry 拒绝入库 |
| capture 误触发写操作(sichek 已有 5 类隐式写的前科) | 严守只读契约:禁用 mlxreg/mlxconfig 写路径、不 modprobe、不改 sysfs;评审清单逐条核 |
| 机型爆炸(例外淹没基线) | 例外必须带 reason/date;propose 发现例外占比过高时提示应拆新机型 |
| render 语义漂移(sichek spec 格式演进) | 等价性对比进 CI;sichek spec 格式变更时同步更新 render 目标 |

## 10. 路线图

- **一期(本设计)**:git YAML + CLI,hpn 组件,采集闭环 + 等价渲染 + 查询。
- **二期**:服务化(API/DB/UI);`--target prometheus-rules`;gpu 组件(`catalog/gpu/` + model 的 gpu 段);fabric 级(LLDP 轨道尾号规律、交换机判别已有积累);QoS 基线核查(采集→比对,仍不下发)。
- **非目标(明确不做)**:配置下发/改写设备(NUSA 式 QoS 重申、CC 寄存器编程)——与只读契约冲突,若未来要做必须是独立的、有护栏的配置栈。
