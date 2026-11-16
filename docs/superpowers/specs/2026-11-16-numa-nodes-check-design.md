# NUMA 节点数检查(nvidia 组件)设计

日期:2026-11-16
分支:`feat/numa-nodes-check`(基于 main@eb1edb9)

## 1. 背景与目标

BIOS 的 Sub-NUMA Clustering(SNC,AMD 为 NPS)设置会改变 OS 看到的 NUMA 节点数,
进而影响训练任务的绑核、K8s Topology Manager 对齐和 H2D 拷贝路径。同机型同 BIOS
版本的机器因为人工设置不一致出现漂移,目前 sichek daemon **完全不检查**:

- 唯一与 NUMA 相关的 `pcie_topo` 的 `checkNuma` 只在 CLI `sichek topo` / `sichek all`
  里跑,daemon 不跑;且按"每 NUMA 几张 GPU/IB"写死,在健康节点上也误报
  (.98 把存储双口卡算进 IB → `expected 4, got 6`;`pci_switches: null` → 恒 FAIL)。

**目标**:daemon 中按 GPU 型号检查 NUMA 节点数,发现 BIOS NUMA 配置漂移,结果进入
annotation / snapshot / argus。

**期望规则(GPU 专家确认,2026-11-16)**:B300 机型 4 NUMA,其他 GPU 机型 2 NUMA。

## 2. 现场依据(2026-11-16 实测)

| 节点 | 机型 / CPU | GPU | socket | NUMA | 按规则 |
|---|---|---|---|---|---|
| 10.220.77.49 | AIVRES KR9288 / Xeon 6767P | B300 | 2 | 4 | PASS |
| thg1 | Supermicro SYS-A22GA / Xeon 6960P | B200 | 2 | 2 | PASS |
| 214.196.33.98 | 联想 WA7880a G3 / 8558P | H200 | 2 | 2 | PASS |
| **214.196.33.97** | 联想 WA7880a G3 / 8558P | H200 | 2 | **4** | **FAIL(真漂移)** |
| clnet36 | NF5688-M7 / 8480+ | H20 | 2 | 2 | PASS |
| lmg86 / lmg104 | OpenStack VM / 8457C | H20 | 2 | 2 | PASS |
| dracog24 | NF5468M6 / Gold 6348 | A100 PCIe | 2 | 2 | PASS |
| shg205 | ODM I57G90N13M63S10 / Gold 6530 | RTX 5090 | 2 | 2 | PASS |
| **shg06** | ODM I57G90N13M63S10 / Gold 6530 | RTX 5090 | 2 | **4** | **FAIL(真漂移)** |

规则对 10 台可达节点零误报,两组漂移(.97 vs .98、shg06 vs shg205)均能抓到。

## 3. 关键决策

### 3.1 放在 nvidia 组件,而不是 cpu / pcie_topo

- 规则按 GPU 型号定,nvidia 组件天然知道 GPU 型号;cpu 组件不知道。
- nvidia spec 已有 `dependence` 段承载"GPU 对宿主平台的要求"(`pcie_acs` / `iommu` /
  `nv_peermem` / `cpu_performance`),NUMA/SNC 属于同类;checker 与 `iommu.go`、
  `pcie_acs.go` 同放 `components/nvidia/checker/check_dependences/`。
- nvidia 在 `DefaultComponents` 里,daemon 默认跑;nvidia 已在 annotation schema 里,
  新 checker 无需改 `service/info.go`。

### 3.2 期望值:代码内置默认 + spec 可选覆盖

nvidia spec 的加载是**按 GPU device ID 整段二选一**,不做字段合并:集群文件
(`{cluster}_spec.yaml`)的 `nvidia:` 段已有本机 device ID 时直接用,**不会**再读
`specs/nvidia/{id}.yaml`(`EnsureSpec` → `MergeAndWriteSpec` 只补缺失 key)。
sichek_spec 中 20 个集群文件几乎都自带本集群 GPU 段,只改 per-device 文件不会生效,
需要逐个集群文件改。且现有集群文件存在型号与实际硬件不符(shmaas 实为 5090、
changliu 实为 H20,文件里都是 H200 的 `0x233510de`)。

因此:
- **默认值写在代码里**,发版即对全部集群生效,sichek_spec 无需改动。
- `dependence.numa_nodes`(int,`omitempty`)为**可选覆盖**:`> 0` 时用它,
  `0`/缺省时用内置默认。仅给将来的例外集群用。
- `omitempty` 避免 sichek 重新序列化 `default_spec.yaml` 时写出 `numa_nodes: 0`。

### 3.3 内置默认规则

```
expectedNumaNodes(gpuName):
    gpuName 含 "B300"(大小写不敏感) → 4
    其他                              → 2
```

按 GPU 名称匹配而非 device ID:B300 目前只见过 `0x318210de`("NVIDIA B300 SXM6 AC"),
名称匹配可覆盖未来同代其他 SKU。GB300 名称也含 "B300",由 3.4 的架构跳过兜住。

### 3.4 跳过条件

| 条件 | 行为 |
|---|---|
| `runtime.GOARCH != "amd64"`(Grace/GB200/GB300:HBM 暴露为 NUMA 节点) | Normal,Detail 写明跳过原因 |
| 采集里拿不到 GPU 名称(`DevicesInfo` 为空或 Name 为空) | Normal,Detail 写明跳过原因 |
| 读 `/sys/devices/system/node` 失败或数出 0 个节点 | 返回 error(由 `common.Check` 记日志并丢弃本轮结果),不把采集失败当成 mismatch——与 vbios checker 对"未读到"的处理一致 |
| user config `ignored_checkers` 含 `numa_nodes` | 不注册(现有机制) |

## 4. 组件与数据流

```
NvidiaCollector ─► NvidiaInfo.DevicesInfo[0].Name ─┐
                                                   ├─► NumaNodesChecker.Check
/sys/devices/system/node/node[0-9]+ (计数) ────────┤        │
spec.Dependence.NumaNodes (可选覆盖) ──────────────┘        ▼
                                              CheckerResult(Spec=期望, Curr=实际)
```

### 4.1 改动清单

| 文件 | 改动 |
|---|---|
| `components/nvidia/config/spec.go` | `Dependence` 加 `NumaNodes int \`json:"numa_nodes,omitempty" yaml:"numa_nodes,omitempty"\`` |
| `components/nvidia/config/check_items.go` | 新增 `NumaNodesCheckerName = "numa_nodes"` 及 `GPUCheckItems` 条目 |
| `components/nvidia/checker/check_dependences/numa_nodes.go`(新) | `NumaNodesChecker` + `expectedNumaNodes()` + `countNumaNodes(root)` |
| `components/nvidia/checker/check_dependences/numa_nodes_test.go`(新) | 单测 |
| `components/nvidia/checker/checker.go` | `checkerConstructors` 注册 |

不改:cpu 组件、pcie_topo、annotation schema、metrics、sichek_spec。

### 4.2 Checker 结果字段

| 字段 | 值 |
|---|---|
| Name | `numa_nodes` |
| Description | `Check NUMA node count matches the expected BIOS SNC/NPS setting for this GPU model` |
| Level | **Warning**(BIOS 配置漂移,安排修复,不需要 cordon) |
| ErrorName | `NUMANodeCountMismatch` |
| Spec | 期望值,如 `2` |
| Curr | 实际值,如 `4` |
| Detail(异常) | `NUMA nodes: expected 2 (default for NVIDIA H200), got 4` / 覆盖时 `(spec override)` |
| Suggestion | `Check BIOS Sub-NUMA Clustering (SNC) / AMD NPS setting; this GPU model expects N NUMA node(s). A reboot is required after the change.` |

### 4.3 NUMA 计数

数 `/sys/devices/system/node/` 下匹配 `^node[0-9]+$` 的目录。根路径做成包级变量
(`numaSysfsRoot`),测试用 `t.TempDir()` 构造假目录。每次 `Check` 现读(开销
是一次 `ReadDir`,随 nvidia query_interval 运行)。

## 5. 测试

**单元测试(table-driven,testify)**
- `expectedNumaNodes`:B300 / "NVIDIA B300 SXM6 AC" / 小写 b300 → 4;H200、H20、B200、
  A100、RTX 5090、空串 → 2。
- `countNumaNodes`:node0..node3 → 4;含 `has_cpu`、`possible` 等非节点文件不计;
  目录不存在 → error;空目录 → error。
- `Check`:
  - B300 + 4 节点 → Normal;H200 + 4 节点 → Abnormal / Warning,Spec=2 Curr=4;
  - spec 覆盖 `NumaNodes=4` + H200 + 4 节点 → Normal,Detail 标 override;
  - DevicesInfo 为空 → Normal 跳过;
  - data 类型错误 → error。
- GOARCH 跳过分支:把架构判断抽成包级变量便于测试。

**现场回归**(sichek-field-regression skill,隔离 SICHEK_CONFIG_DIR / snapshot / reporter)

| 节点 | 期望 |
|---|---|
| 214.196.33.97(H200,4 NUMA) | `numa_nodes` Abnormal/Warning,expected 2 got 4 |
| shg06(5090,4 NUMA) | Abnormal/Warning |
| 214.196.33.98、clnet36、lmg86、dracog24、thg1、shg205 | Normal |
| 10.220.77.49(B300,4 NUMA) | Normal |

## 6. 不在本次范围

- pcie_topo `checkNuma` 的误报修复与 sichek_spec 中旧 `numa_config` 清理。
- GPU↔计算网卡同 NUMA 配对检查、`numa_node == -1` 检查。
- `sichek_cpu_numa_nodes` 指标与 argus 机队一致性视图。
- cpu 组件 PrintInfo 只打印前 2 个 NUMA 的显示问题。
- AMD 平台(NPS)的专门规则:当前按同一规则(B300→4,其他→2)判定,现场暂无 AMD
  GPU 节点验证;如有例外用 spec 覆盖或 `ignored_checkers`。
