# sichek `ovs` 组件设计

- 日期：2026-06-30
- 状态：已批准（待进入 writing-plans）
- 目标节点：Spectrum-X / BlueField DOCA-OVS 节点（样本 zy3 = hydra-gpu-214-171-47-3，8 rail × 4 plane / NDR）

## 1. 背景与目标

为 sichek 新增一个 `ovs` 组件，监控 Spectrum-X 部署中 DOCA-OVS 数据面的健康状态。要求：

1. 判定结果（pass/fail）落到 K8s annotation 与本地 snapshot（`snapshot.Issues` + `snapshot.Components["ovs"]`）。
2. 监控联动所需的数值指标上报 Prometheus（`/metrics`）。

### 数据来源（两类，职责分离）

- **`rdma_env_vv ovs`**（`/usr/bin/rdma_env_vv` 中的 bash 子命令，~1092 行）—— 专家针对 rdma_env_pre Step 5/8/10 写的 **pass/fail 判定**。配套文档 `docs/rdma_env_vv-checks.html` 逐项列出检查/基线/级别/用意。sichek 的 checker 忠实移植它。
- **`ovs-appctl` / `ovs-vsctl` / `ovs-ofctl`** 运行时计数器 —— 连续型 **指标**（dpctl lookups、PMD 周期、coverage 事件等），适合 Prometheus 时序。

### 与 rdma-doctor 的关系（关键决策）

目标节点上 `rdma-doctor-agent`（HTTPS exporter，:9188，参数含 `--ovs-naming-scheme=spectrum-x-8rail`）**已经**导出一整套 `rdma_doctor_ovs_*` 指标 + `rdma_doctor_check_status{scope="ovs"}` 检查。

**决策：sichek 独立原生采集，不抓取 rdma-doctor**，理由是**未来 rdma-doctor 将放弃采集 OVS 指标，由 sichek 接管**。为保证平滑切换，sichek 的 OVS 指标 **镜像 rdma-doctor 的 schema（指标集/标签/取值语义一一对应）**；真正的「互换」发生在 Monarch `node_*` 层 —— 由 monarch bridge 把 `sichek_ovs_*` 映射成与 rdma-doctor 当前一致的 `node_*`。

sichek 的判定**比 rdma-doctor 更严**（rdma_env_vv 基线）：

| 维度 | rdma-doctor | sichek（本设计） |
|---|---|---|
| flow 数 | `flow_count_nonzero`（仅非零） | `flows >= 18`（EXPECTED_FLOWS） |
| group | `group_count_match`（仅比数量） | group_id **精确集合** {10,20-23,30-33} |
| other_config Step-8 键 | 无 | doca-init/hw-offload/hw-offload-ct-size/max-idle/doca-eswitch-max + dpdk_initialized |
| DOCA 包是否安装 | 仅 version_info | doca-openvswitch-switch/common、collectx-clxapi、libnvhws1 |
| 端口↔流表一致性 | 无 | orphan_flow_refs（Critical）/ orphan_ports（Warning） |
| coverage 事件 | 无 | flow_offload_200ms_latency、doca_datapath_drop_upcall_error |

> 注：现有 monarch bridge（`scripts/monarch/sichek_monarch_exporter.py`，仅存在于分支 `feat/monarch-exporter-bridge`，main 上没有）目前只抓 sichek 自己的 `sichek_*`（:19091）→ `node_*`，**不抓 rdma-doctor**。把 OVS 接入 node_* 是本组件落地后的后续工作，不在本 spec 实现范围内（但 schema 已为此对齐）。

## 2. 方案

**原生 Go 组件**，仿照 `components/transceiver/`（最近的干净参照：有 spec、有 metrics、无 event filter）。collector 直接 `os/exec` 调 `ovs-vsctl`/`ovs-ofctl`/`ovs-appctl`。**不依赖节点上装有 `rdma_env_vv`** —— 逻辑由 sichek 自己实现，脚本/HTML 仅作基线文档。

### 目录布局（`components/ovs/`）

```
components/ovs/
  ovs.go                      # component：实现 common.Component，HealthCheck 编排
  collector/collector.go      # 调 ovs-* 命令，产出 OVSInfo（common.Info）
  checker/                    # 各 Checker（common.Checker）
    service_checker.go
    version_checker.go
    other_config_checker.go
    bridge_checker.go
  config/
    config.go                 # OVSUserConfig（QueryInterval/CacheSize/EnableMetrics/IgnoredCheckers）
    spec.go                   # OVSSpecConfig 加载 default_ovs_spec.yaml
    default_ovs_user_config.yaml
    default_ovs_spec.yaml
  metrics/metrics.go          # OVSMetrics：GaugeVecMetricExporter，前缀 sichek_ovs
cmd/command/component/ovs.go  # cobra 子命令
```

## 3. 门控 / 适用性

collector 启动探测：PATH 上有 `ovs-vsctl` 且 `ovs-vswitchd` 处于 active。

- 不满足 → `OVSInfo{Available:false, SkipReason:...}`，`HealthCheck` 返回单个 `StatusNormal` 的 info checker（`OVSNotPresent`），**不产生 issue、不报错**。
- `sichek_ovs_present` gauge 记录 0/1。

因此组件可加入 `DefaultComponents`，每节点都跑，但非 Spectrum-X 节点自动 no-op。

## 4. Collector → `OVSInfo`（落 `snapshot.Components["ovs"]`）

`OVSInfo` 实现 `common.Info`（含 `JSON()`），字段：

- `Available bool` / `SkipReason string`
- `Services map[string]string` —— openvswitch-switch / ovs-vswitchd / ovsdb-server 的 `systemctl is-active` 结果
- `Packages map[string]string` —— DOCA 包 → 版本（空=未装）
- `OVSVersion string` / `DPDKVersion string` / `DPDKInitialized bool`
- `OtherConfig map[string]string` —— Step-8 键当前值
- `Bridges []BridgeInfo`，每桥：`Name, DatapathType, FailMode, Ports int, Flows int, GroupIDs []int, OrphanFlowRefs []int, OrphanPorts []int, PortDetails []PortInfo{Name, OFPort, AdminState, LinkState, Error, RxBytes, TxBytes, RxErrPkts}`
- `Datapath DatapathInfo{Name string, DPFlows int, LookupsHit/Missed/Lost uint64, PMDs []PMDInfo{Core, NUMA, BusyRatio float64, IdleCycles, ProcessingCycles, RxPackets uint64}}`
- `Coverage map[string]uint64` —— 选定 coverage 事件 total

采集命令对照：

| 字段 | 命令 |
|---|---|
| Services | `systemctl is-active <svc>` |
| Packages | `dpkg-query -W -f='${db:Status-Abbrev} ${Version}'`（第 2 位=i 视为已装） |
| OVS/DPDK 版本、dpdk_initialized、other_config | `ovs-vsctl get Open_vSwitch . <field/other_config:k>` |
| 桥/端口/datapath_type/fail_mode | `ovs-vsctl br-exists / list-ports / get bridge / get interface` |
| flows | `ovs-ofctl dump-flows <br> \| grep -c cookie=` |
| group_id 集合 | `ovs-ofctl dump-groups <br>` |
| orphan refs/ports | `ovs-ofctl show <br>` 的 ofport 集合 vs `dump-flows` 引用的 in_port/output 集合 |
| dp_flows / lookups | `ovs-appctl dpctl/show`、`dpctl/dump-flows` |
| PMD | `ovs-appctl dpif-netdev/pmd-perf-show` |
| coverage | `ovs-appctl coverage/show` |

## 5. Checkers（pass/fail → `snapshot.Issues` + annotation）

忠实于专家脚本。级别映射（已确认）：**FAIL→Critical；版本空 + orphan_ports→Warning；无 Fatal**。

| Checker | 判定依据 | 级别 |
|---|---|---|
| `OVSServiceChecker` | 三守护进程 active；`ovs-vswitchd` 是 gate（挂了则其余 checker 标记为未知/跳过） | Critical |
| `OVSVersionChecker` | DOCA 包已安装；OVS/DPDK runtime 版本非空（**不做最低版本比较**） | 包缺失→Critical；版本空→Warning |
| `OVSOtherConfigChecker` | Step-8 键全部匹配 spec + `dpdk_initialized=true` | Critical |
| `OVSBridgeChecker` | 每桥 br-rail{0..N-1}：存在、`datapath=netdev`、`ports==spec.ports_per_bridge`、`flows>=spec.min_flows`、group_id ⊇ `spec.expected_group_ids`、无 orphan_flow_refs | Critical；`orphan_ports`→Warning |

**Topology 与 Datapath/PMD 不做判定**（与脚本一致）：仅作 info 进 snapshot、作 metrics 上报 Prometheus，不产生 issue。

每个 Checker 产出 `common.CheckerResult{Name, Description, Status, Level, ErrorName, Curr, Detail, Suggestion}`，并通过 `common.Check()` 并行汇总成 `Result`。

## 6. Spec —— `default_ovs_spec.yaml`

值逐字拷自 `rdma_env_vv` 脚本顶部常量：

```yaml
ovs:
  bridge_prefix: "br-rail"
  num_rails: 8
  ports_per_bridge: 5
  min_flows: 18
  expected_group_ids: [10, 20, 21, 22, 23, 30, 31, 32, 33]
  datapath_type: "netdev"
  other_config:
    doca-init: "true"
    hw-offload: "true"
    hw-offload-ct-size: "0"
    max-idle: "300000"
    doca-eswitch-max: "4"
  required_packages:
    - doca-openvswitch-switch
    - doca-openvswitch-common
    - collectx-clxapi
    - libnvhws1
  coverage_events:
    - flow_offload_200ms_latency
    - doca_datapath_drop_upcall_error
```

> 这些是脚本当前硬编码值，已与 `docs/rdma_env_vv-checks.html` 对齐；专家如需调整改 yaml 即可，不动代码。

`default_ovs_user_config.yaml`：

```yaml
ovs:
  query_interval: 60s
  cache_size: 5
  enable_metrics: true
  ignored_checkers: []
```

## 7. Prometheus 指标（`metrics/`，前缀 `sichek_ovs`，受 `EnableMetrics` 控制）

`sichek_ovs_*` 镜像 rdma-doctor 的 OVS schema，便于 node_* 层互换。所有指标自动带 `node` label（沿用 `GaugeVecMetricExporter`）。

| sichek 指标 | 标签 | 对应 rdma-doctor |
|---|---|---|
| `sichek_ovs_present` | — | （新增，门控） |
| `sichek_ovs_service_up` | `ovs_service` | `rdma_doctor_ovs_service_up` |
| `sichek_ovs_version_info` | `version` | `rdma_doctor_ovs_version_info` |
| `sichek_ovs_bridge_flow_count` | `bridge` | 同名 |
| `sichek_ovs_bridge_group_count` | `bridge` | 同名 |
| `sichek_ovs_bridge_port_count` | `bridge` | 同名 |
| `sichek_ovs_bridge_issue_count` | `bridge` | 同名 |
| `sichek_ovs_bridge_datapath_type_info` | `bridge,type` | 同名 |
| `sichek_ovs_bridge_fail_mode_info` | `bridge,mode` | 同名 |
| `sichek_ovs_datapath_flows` | `datapath` | 同名 |
| `sichek_ovs_datapath_lookup` | `datapath,result=hit\|missed\|lost` | 同名 |
| `sichek_ovs_pmd_busy_ratio` | `core,numa` | 同名 |
| `sichek_ovs_pmd_idle_cycles` | `core,numa` | 同名 |
| `sichek_ovs_pmd_processing_cycles` | `core,numa` | 同名 |
| `sichek_ovs_pmd_rx_packets` | `core,numa` | 同名 |
| `sichek_ovs_port_ofport` | `bridge,port` | 同名 |
| `sichek_ovs_port_bytes` | `bridge,port,direction` | 同名 |
| `sichek_ovs_port_packets` | `bridge,port,direction,kind` | 同名 |
| `sichek_ovs_check_status` | `scope="ovs",check,target,iface,reason` | `rdma_doctor_check_status{scope="ovs"}` |

sichek 独有的补充指标（rdma-doctor 无）：

- `sichek_ovs_other_config_ok{key}` 0/1
- `sichek_ovs_dpdk_initialized` 0/1
- `sichek_ovs_coverage_total{event}`

> 与 IB metrics 一样，需处理消失的 series（桥/端口/PMD 在两次 scrape 间消失时 `DeleteLabelValues`），避免 stale 时序。

## 8. 接线（仓库约定，易漏点）

- `consts/consts.go`：新增 `ComponentNameOVS = "ovs"`，并追加进 `DefaultComponents`；按需加 checker 名常量。
- `cmd/command/component/all.go` 的 `NewComponent` switch：加 `case consts.ComponentNameOVS: return ovs.NewComponent(cfgFile, specFile, ignoredCheckers)`。
- 新建 `cmd/command/component/ovs.go` cobra 子命令，并在 `cmd/command/command.go` 注册。
- **`service/info.go`**：给 `nodeAnnotation` 加 `OVS` 字段，**并且**在 `getAnnotationsByItem` 与 `setAnnotationsByItem` 两个 switch 都加 `case consts.ComponentNameOVS`。否则 OVS 的 issue 会被静默丢出 annotation/snapshot（本仓库已知坑）。
- snapshot 自动经 daemon 的 `Update(componentName, info)` 落 `Components["ovs"]`，无需额外改 snapshot.go。

## 9. 测试

- **Checker 单测**：表驱动，用从 zy3 抓取的 `ovs-vsctl/ofctl/appctl` 输出做 fixture（健康用例 + 构造故障：服务挂、other_config 改值、桥缺失、ports/flows 不足、group_id 缺失、orphan_flow_refs/ports）。放 `collector/testdata/`。
- **Collector 解析单测**：各命令输出 → `OVSInfo` 解析。
- **Metrics 单测**：schema/label 与 rdma-doctor 对齐校验。
- **真机回归**：在 zy3（参照 sichek-field-regression skill）跑 CLI（`sichek ovs`）+ daemon + 检查 snapshot.json + 抓 `/metrics`；注意不要传 `NODE_NAME=zy`（见 zy3 记忆）。

## 10. 非目标 / YAGNI

- 不做 OVS runtime 最低版本号比较（脚本也不做）。
- 不做 Datapath/PMD 的 pass/fail 判定。
- 不在本 spec 内实现 monarch bridge 的 OVS→node_* 映射（schema 已对齐，留作后续）。
- 不抓取 rdma-doctor 9188（独立采集）。

## 11. 待专家最终核对

`default_ovs_spec.yaml` 中的基线常量（端口数、流表数、group_id 集合、other_config 期望、DOCA 包清单、coverage 事件）—— 与 `docs/rdma_env_vv-checks.html` 的「确认/修订」列一致，量产前请专家逐项确认。
