# PCIe Tree Speed/Width 检测 spec-free 化设计

- 状态：Draft（待评审）
- 范围：`components/infiniband` 中的 `PCIETreeSpeedDownDegraded` 和 `PCIETreeWidthIncorrect` 两个 checker
- 目标分支：`feat/pcie-tree-spec-free`
- 设计日期：2026-05-18

---

## 1. 背景与动机

目前 sichek 的 PCIe 链路类检查共有 4 项，全部依赖 `components/hca/config/default_spec.yaml` 中按 NIC `board_id` 维度填写的期望值：

| Checker | yaml 字段（HCA spec） | ErrorName |
| --- | --- | --- |
| `CheckPCIETreeSpeed` | `pcie_tree_speed`（无则回退 `pcie_speed`） | `PCIETreeSpeedDownDegraded` |
| `CheckPCIETreeWidth` | `pcie_tree_width`（无则回退 `pcie_width`） | `PCIETreeWidthIncorrect` |
| `CheckPCIESpeed` | `pcie_speed` | `PCIELinkSpeedDownDegraded` |
| `CheckPCIEWidth` | `pcie_width` | `PCIELinkWidthIncorrect` |

Tree Speed/Width 的判定流程（`components/infiniband/checker/pcie_tree_speed.go`、`pcie_tree_width.go`）：

1. Collector `GetPCIETreeMin` 用 `readlink /sys/bus/pci/devices/<NIC_BDF>` 解出整条上行 BDF 路径，剔除 NIC 自身 BDF，对剩余每个 BDF 读 `current_link_speed` / `current_link_width`，取最小值作为 `PCIETreeSpeedMin` / `PCIETreeWidthMin`，并记录瓶颈 BDF。
2. Checker 把该最小值与 HCA spec 里通过 `board_id` 查到的 `pcie_tree_speed` / `pcie_tree_width` 比较，不等就报 Critical。
3. 若上行只有 0/1 个节点（直连 CPU），跳过检查。

存在的问题：

- yaml 的"期望值"本质上是"该 NIC 在该机型这条 PCIe 路径上理论能跑到的最高速率"，是**对硬件能力的复述**；硬件本身已经把同样信息写在 sysfs 的 `max_link_speed` / `max_link_width` 中。
- 新机型 / 新 NIC SKU 上线要补 `board_id` 条目（OSS 下载或手工 PR），易过时、易遗漏。
- **多 NIC 混插场景下（生产+管理 NIC、不同代次 NIC 同机）**，必须保证每张卡的 board_id 在 spec 都有项，维护成本随 SKU 数量线性增长。
- 当 NIC = Gen5、root port = Gen4 时，yaml 通常按 NIC 能力填 Gen5，整条链路实际只能跑 Gen4 → **当前规则会假阳报警**。

本设计去掉 Tree Speed/Width 两个 checker 对 HCA spec 的依赖，改为完全从 sysfs 自描述地推导 expected。

## 2. 目标与非目标

### 目标

- 去除 `PCIETreeSpeedDownDegraded` 和 `PCIETreeWidthIncorrect` 这两个 checker 对 HCA spec 的运行时依赖。
- 每条上行 PCIe link 独立判定降速；多 NIC 混插下无需任何 spec 维护即可生效。
- 诊断输出从"NIC 整体最小值"细化到"哪条上行 link 降速、当前值、能力上限"，提高现场可定位性。
- 不破坏现有 Prometheus 指标 schema（label 集合不变；`spec` label 取值含义变化但维度保持）。

### 非目标

- **不动** `CheckPCIESpeed` / `CheckPCIEWidth`（NIC 自身 link 的检查），它们走 NIC 自身 `current_link_speed` vs spec 比较，下一阶段再讨论。
- **不动** `components/hca/config/default_spec.yaml` 中现有的 `pcie_tree_speed` / `pcie_tree_width` 字段；checker 运行时直接忽略它们，yaml 留作"历史信息"，不迁移不删除。
- 不引入 spec override 逃生舱（用户已确认）。本期完全 sysfs only；若实际遇到 sysfs 不可信的平台，再单独评估是否加白名单。
- 不改告警级别（仍是 `critical`），不改 metric 名称、不改 ErrorName。

## 3. 总体方案

```
sysfs /sys/bus/pci/devices/<NIC_BDF>
  readlink → [BDF_root, BDF_switch_up, BDF_switch_dn, BDF_nic]
       │
       ▼
   for each adjacent pair (parent, child) in path:
     read parent.{current,max}_link_speed/width
     read child.{current,max}_link_speed/width
       │
       ▼
   []PCIETreeLink {
       ParentBDF, ChildBDF,
       CurSpeed, CurWidth,
       ParentMaxSpeed, ChildMaxSpeed,
       ParentMaxWidth, ChildMaxWidth,
   }
       │  (附在 IBHardWareInfo 上，per-NIC)
       ▼
   IBPCIETreeSpeedChecker / IBPCIETreeWidthChecker
       for each link in PCIETreeLinks:
           cap_speed = min(ParentMaxSpeed, ChildMaxSpeed)
           if link.CurSpeed < cap_speed: 记录降速链路（多条都记）
       同理 Width
```

关键变化：

- **expected = 每条 link 的 `min(parent.max, self.max)`**，不再来自 yaml。这能正确处理"root port 只支持 Gen4 而 NIC 是 Gen5"的合法场景：那条 link 的 cap 就是 Gen4，current=Gen4 时不报警。
- collector 一次扫描产出 `[]PCIETreeLink`，**Speed/Width 两个 checker 共用同一份数据**，不会重复读 sysfs。
- HCA spec yaml 不动；checker 不再读 `pcie_tree_speed` / `pcie_tree_width` 字段。

## 4. 模块改动清单

```
components/infiniband/
├── collector/
│   ├── pcie_info.go            (改：GetPCIETreeMin → GetPCIETreeLinks)
│   └── ib_hardware_info.go     (改：IBHardWareInfo 新增 PCIETreeLinks)
├── checker/
│   ├── pcie_tree_speed.go      (改写：基于 PCIETreeLinks)
│   └── pcie_tree_width.go      (改写：基于 PCIETreeLinks)
```

### 4.1 `IBHardWareInfo` 数据结构（`collector/ib_hardware_info.go`）

新增字段：

```go
type PCIETreeLink struct {
    ParentBDF       string `json:"parent_bdf" yaml:"parent_bdf"`
    ChildBDF        string `json:"child_bdf" yaml:"child_bdf"`
    CurSpeed        string `json:"cur_speed" yaml:"cur_speed"`
    CurWidth        string `json:"cur_width" yaml:"cur_width"`
    ParentMaxSpeed  string `json:"parent_max_speed" yaml:"parent_max_speed"`
    ChildMaxSpeed   string `json:"child_max_speed" yaml:"child_max_speed"`
    ParentMaxWidth  string `json:"parent_max_width" yaml:"parent_max_width"`
    ChildMaxWidth   string `json:"child_max_width" yaml:"child_max_width"`
}

type IBHardWareInfo struct {
    // ...existing fields...
    PCIETreeSpeedMin    string         `json:"pcie_tree_speed" yaml:"pcie_tree_speed"`
    PCIETreeSpeedMinBDF string         `json:"pcie_tree_speed_bdf" yaml:"pcie_tree_speed_bdf"`
    PCIETreeWidthMin    string         `json:"pcie_tree_width" yaml:"pcie_tree_width"`
    PCIETreeWidthMinBDF string         `json:"pcie_tree_width_bdf" yaml:"pcie_tree_width_bdf"`
    PCIETreeLinks       []PCIETreeLink `json:"pcie_tree_links" yaml:"pcie_tree_links"`  // NEW
}
```

> **兼容策略**：保留旧 4 个字段（`PCIETreeSpeedMin / PCIETreeWidthMin / PCIETreeSpeedMinBDF / PCIETreeWidthMinBDF`），值由 `PCIETreeLinks` 派生填充（`PCIETreeSpeedMin` = `PCIETreeLinks` 中 `CurSpeed` 最小的那条，对应的 `ChildBDF` 即 `PCIETreeSpeedMinBDF`；Width 同理）。这保证：JSON snapshot 消费方、Prometheus 现有 label 维度不会破坏。

### 4.2 Collector：`GetPCIETreeLinks`（替换 `GetPCIETreeMin`）

签名：

```go
// GetPCIETreeLinks enumerates every PCIe link on the upstream path of an IB
// device. Each adjacent (parent, child) BDF pair in the readlink path becomes
// one PCIETreeLink, with current_link_{speed,width} and max_link_{speed,width}
// read from both endpoints. Direct-to-CPU devices (<2 BDFs in path) return nil.
func GetPCIETreeLinks(IBDev string) []PCIETreeLink
```

实现要点：

1. `readlink /sys/bus/pci/devices/<NIC_BDF>` 提取 BDF 序列 `[bdf_0, bdf_1, ..., bdf_n]`（`bdf_n` 即 NIC 自身）。
2. 若 `len(bdfs) < 2`，返回 `nil`（保持今天"直连 CPU 跳过"的语义）。
3. 对每对相邻 `(bdf_{i-1}, bdf_i)`：
   - 读 `current_link_speed` / `current_link_width`：优先取 child（`bdf_i`）的；与 parent 不一致时打 debug log，仍以 child 为准（链路两端共享同一 link，理论上一致）。
   - 读 parent.`max_link_speed` / child.`max_link_speed`；同 Width。
4. 任一文件读不到 → **该 link 跳过**，warn log；不阻塞其它 link。
5. `PCIPath` 抽取为可注入参数 / 包级变量，方便 t.TempDir() 单测。

新增辅助 `readSysfsLine(filepath) (string, error)`（trim 换行、空文件返回错误），如果 repo 中已有类似工具复用之。

### 4.3 `IBHardWareInfo.Get` 派生填充旧字段

在 `ib_hardware_info.go` 调用 `GetPCIETreeLinks` 后，做一次 reduce：

```go
hw.PCIETreeLinks = GetPCIETreeLinks(IBDev)
hw.PCIETreeSpeedMin, hw.PCIETreeSpeedMinBDF = minLinkCurSpeed(hw.PCIETreeLinks)
hw.PCIETreeWidthMin, hw.PCIETreeWidthMinBDF = minLinkCurWidth(hw.PCIETreeLinks)
```

`minLinkCurSpeed` / `minLinkCurWidth` 是新加的小工具函数：扫一遍 `PCIETreeLinks`，按数值最小的 `CurSpeed` / `CurWidth` 返回值 + 对应 `ChildBDF`；空列表返回 `("","")`。

### 4.4 Checker：`pcie_tree_speed.go`

完全去掉对 `c.spec.HCAs[hwInfo.BoardID].Hardware.PCIETreeSpeedMin` / `PCIESpeed` 的读取。重写主循环：

```go
for _, hwInfo := range uniqueByDev(infinibandInfo.IBHardWareInfo) {
    if len(hwInfo.PCIETreeLinks) == 0 {
        // direct-to-CPU or sysfs unavailable; treat as normal
        continue
    }
    for _, link := range hwInfo.PCIETreeLinks {
        cap := minNumericSpeed(link.ParentMaxSpeed, link.ChildMaxSpeed)
        if cap == "" {
            // both endpoints' max unreadable; skip silently (warn already in collector)
            continue
        }
        if pcieSpeedLessThan(link.CurSpeed, cap) {
            failedDevices = append(failedDevices, fmt.Sprintf(
                "%s(%s, bottleneck@%s->%s)",
                hwInfo.IBDev, hwInfo.PCIEBDF, link.ParentBDF, link.ChildBDF))
            failedCurr = append(failedCurr, link.CurSpeed)
            failedCap  = append(failedCap, cap)
        }
    }
}
```

- 新增 `minNumericSpeed(a, b string) string`：解析两侧数值，返回较小者的原始字符串；任意一侧解析失败返回 `""`（让 checker 跳过该 link）。
- 新增 `pcieSpeedLessThan(a, b string) bool`：浮点解析后 `a < b`；解析失败时返回 `false`（保守，不报警）。复用 `extractNumericSpeed`。

`CheckerResult` 填充：

```go
result.Status     = "abnormal"   // 仅当 failedDevices 非空
result.Level      = "critical"   // 与现有一致
result.Device     = strings.Join(failedDevices, ",")
result.Curr       = strings.Join(failedCurr, ",")
result.Spec       = strings.Join(failedCap, ",")   // ← 填实际 cap 值（"16.0 GT/s" 等），不是 yaml
result.Detail     = "<per-link 详情，格式见 §5.3 降速输出示例>"
result.Suggestion = "<per-link 修复建议，格式见 §5.3 降速输出示例>"
result.ErrorName  = "PCIETreeSpeedDownDegraded"    // 不变
```

正常分支与今天保持一致：`result.Curr` 填该 NIC 整体最小 current speed（来自派生字段 `PCIETreeSpeedMin`），`result.Spec` 填 `min(所有 link cap)`（描述链路理论上限）。**告警 series 在没有降速时不会出现，这点由 metrics 层的 `ResetMetric` 保证（见 `metrics/prometheus.go:62-109`），跟今天行为一致。**

### 4.5 Checker：`pcie_tree_width.go`

与 §4.4 完全对称：字段名从 `Speed` 换成 `Width`（读 `link.CurWidth` / `ParentMaxWidth` / `ChildMaxWidth`），数值比较**复用同一个 `pcieSpeedLessThan` 辅助函数**——`extractNumericSpeed` + `strconv.ParseFloat` 对 width 字符串（`"16"` / `"8"`）同样成立。`ErrorName` 维持 `PCIETreeWidthIncorrect`。

## 5. 算法细节与边界情况

### 5.1 数值比较

复用现有 `extractNumericSpeed`：把 `"32.0 GT/s PCIe"` / `"32"` / `"32.00"` 等统一抽出首段数字字符串，再 `strconv.ParseFloat`。

```go
func pcieSpeedLessThan(a, b string) bool {
    af, errA := strconv.ParseFloat(extractNumericSpeed(a), 64)
    bf, errB := strconv.ParseFloat(extractNumericSpeed(b), 64)
    if errA != nil || errB != nil {
        return false   // unknown → not less; checker stays normal
    }
    return af < bf-1e-9   // 用一个小 epsilon 避免浮点误判
}
```

Width 解析路径直接走整数解析（`current_link_width` 是纯数字 "16" / "8"）。

### 5.2 边界情况一览

| 场景 | 行为 |
| --- | --- |
| `readlink` 路径 < 2 个 BDF（NIC 直连 CPU） | `PCIETreeLinks=nil`，checker 跳过该 NIC，不报警 |
| 某条 link 的 `current_link_speed` 文件读不到 | 该 link 不进 `PCIETreeLinks`，collector warn log，不报警 |
| 某条 link 的 `max_link_speed` 任一侧读不到 | 该 link 在 collector 仍可收集 current，但 checker 阶段 `cap=""` 直接跳过；不报警 |
| 某 BDF 的 `max_link_speed` 解析失败（"Unknown" / 非数字） | 同上 |
| NIC = Gen5、root port = Gen4 → 实际跑 Gen4 | root↔switch 这条 link 的 `cap = min(Gen5, Gen4) = Gen4`，current = Gen4 → 不报警（修复今天的假阳） |
| 多条 link 同时降速 | 全部进 `failedDevices`，`Device` 多条 `bottleneck@parent->child` 并列；`Curr` / `Spec` 同步多条拼接 |
| 同一节点多张 NIC（混插 CX-7 + CX-6） | 每张 NIC 独立扫树独立判定，无干扰 |
| Bond IB / 虚拟函数 / `mezz` 卡 | 已在 `GetIBPFBoardIDs` 阶段过滤，本设计无变化 |
| 读到 `max_link_speed = 0` / 负值 | collector 视为异常，warn log，该 link 不进结果 |
| parent 与 child 报告的 `current_link_speed` 不一致 | 取 child 的，打 debug log（不抛错）；保留诊断空间 |

### 5.3 输出示例

正常：
```
status: normal
curr:   16.0 GT/s       (派生自 PCIETreeSpeedMin)
spec:   16.0 GT/s       (链路理论 cap)
```

降速（mlx5_3 root↔switch 这条 link 跑到 Gen3，而两端 max 都是 Gen5）：
```
status:     abnormal
level:      critical
error_name: PCIETreeSpeedDownDegraded
device:     mlx5_3(0000:c1:00.0, bottleneck@0000:80:01.0->0000:81:00.0)
curr:       8.0 GT/s
spec:       32.0 GT/s
detail:     mlx5_3 upstream link 0000:80:01.0->0000:81:00.0 current 8.0 GT/s < cap 32.0 GT/s
suggestion: Check upstream PCIe bridge/switch link 0000:80:01.0->0000:81:00.0 for mlx5_3,
            current 8.0 GT/s is below link capability 32.0 GT/s (min of both endpoints' max).
```

多条降速：
```
device: mlx5_3(...,bottleneck@A->B), mlx5_3(...,bottleneck@C->D)
curr:   8.0 GT/s,16.0 GT/s
spec:   32.0 GT/s,32.0 GT/s
detail: <两行>
```

## 6. 兼容性 / 风险

### 6.1 现有 Prometheus 标签的 churn

`metrics/prometheus.go` 把 `CheckerResult` 的字段当 label。`spec` label 这次从 yaml 字面量（按 NIC 型号大致只有几种取值）变成 sysfs 派生的 cap 值（"16.0 GT/s" / "32.0 GT/s" 等），维度数没变，但**取值集合可能更分散**，且老 series 会因 label 值变化"消失"、出现新 series。

**应对**：
- PR 描述里 highlight 这一行为变化，提示监控/告警面板侧 review 报警规则中是否用到 `spec` label 的具体值。
- 若有过强依赖，可以考虑把 `spec` label 改为统一字符串 `"link-cap"` 等占位值；但失去诊断价值，**默认不采用**。

### 6.2 sysfs `max_link_speed` 不准的平台

少数老平台 / 虚拟化场景 root port 的 `max_link_speed` 可能为空 / "Unknown"。本设计选择 **静默跳过**（不报警，warn log），是为了避免假阳。代价是这些场景下"真降速"也无法识别 → 假阴。

**应对**：上线后跑一份现网 sample（`for d in /sys/bus/pci/devices/*/; do echo $d; cat $d/{current,max}_link_speed; done` 在各机型上采样），核对覆盖率；如发现某 vendor:device 普遍不报 `max_link_speed`，再考虑加例外路径。

### 6.3 sysfs 调用次数放大

每条 link 4 个 sysfs 文件 vs 今天每个上行 BDF 1 个文件。8 NIC * 平均 3 跳 * 4 文件 = 96 次读，比今天约 8 * 3 * 1 = 24 次多 4 倍。10 s 周期下 sysfs 小文件读完全可忽略。

### 6.4 JSON snapshot schema 兼容

`IBHardWareInfo` 新增 `PCIETreeLinks` 字段（数组，包含 8 个 string 字段的对象）。若 sichek-collector 后端有强 schema 校验，需先同步；否则解析方一般忽略未知字段。**老 4 个字段保留**意味着已有消费方不会断。

### 6.5 yaml 字段未来如何处理

本期不动 `pcie_tree_speed` / `pcie_tree_width`。后续若 Link Speed/Width 也走 spec-free，再统一从 spec 中清理；记 follow-up issue 即可。

## 7. 测试计划

### 7.1 collector 单测（`collector/pcie_info_test.go`，新增）

要求把 `PCIPath` 从硬编码常量改为可注入（包级变量 + setter，或 `GetPCIETreeLinks` 接收 root 参数）。这是测试基础设施的小重构。

测试用例：

1. **全链路全速**：3 个 BDF（root→switch→NIC），两条 link 两端 max 均 = 32，current=32 → `len(PCIETreeLinks)=2`，每条 cap=32、cur=32。
2. **switch↔NIC 降速**：current=16，两端 max=32 → 该 link cur=16、cap=32。
3. **root port 只支持 Gen4**：parent.max=16、child.max=32、current=16 → 该 link cur=16、cap=16（关键场景：不应被 checker 报警）。
4. **switch 的 `max_link_speed` 文件缺失** → 该 link 在 collector 输出中 `ParentMaxSpeed` 为 ""（视具体实现也可能整个 link 跳过），checker 阶段安全降级。
5. **NIC 直连 CPU**（路径只有 NIC + 1 个 root port，少于 2 个 upstream） → 返回 nil。
6. **多条 link 同时降速** → 都进入 `PCIETreeLinks`。
7. **parent 与 child 的 current 报不一致** → 取 child；不抛错。

### 7.2 checker 单测（`checker/pcie_tree_speed_test.go`、`pcie_tree_width_test.go`）

直接构造 `IBHardWareInfo.PCIETreeLinks` 输入，**不涉及 sysfs**：

1. `PCIETreeLinks` 为空 → `status=normal`。
2. 1 条 link 降速 → `status=abnormal`，`Device`/`Curr`/`Spec`/`Detail` 全字段 assert。
3. 多条 link 降速 → `Device` 多段、`Detail` 多行。
4. 多 NIC：A 全速、B 一条 link 降速 → 只 B 在 `failedDevices`。
5. `cap` 任一侧无法解析 → 该 link 静默跳过，不报警。
6. `current` 与 `cap` 等值（边界）→ 不报警（用 `<` 不用 `<=`）。

### 7.3 真机回归

按 `sichek-field-regression` skill 流程：在至少 1 台多 NIC 节点上：

1. 跑 `./build/bin/sichek infiniband`，对比 stdout 中 `PCIETreeSpeedDownDegraded` / `PCIETreeWidthIncorrect` 输出。
2. `lspci -vv | grep -E "LnkSta:|LnkCap:"` 抓取每个 PCIe 设备的当前/能力速率，与 sichek 的 Detail 对照，验证 cap 值取自正确的 endpoint。
3. （如可能）人为制造一条降速 link（更换 cable、PCIe slot 切到旁边 slot）并验证报警与 Detail 准确。

## 8. 落地步骤（不约束顺序，给 writing-plans 排）

1. collector 重构：`GetPCIETreeMin` → `GetPCIETreeLinks`，`PCIPath` 可注入，写表驱动单测。
2. `IBHardWareInfo` 新增 `PCIETreeLinks`；老 4 个字段从 `PCIETreeLinks` 派生填充。
3. `pcie_tree_speed.go` / `pcie_tree_width.go` 改写并写单测。
4. `result.Spec` 填实际 cap 值；`Detail` / `Suggestion` 文案落到上面 §5.3 示例。
5. dev 机真机跑一遍 sichek，对照 `lspci -vv`。
6. 更新 `docs/infiniband.md` 中 PCIe Tree 检查的描述。
7. PR 描述中明确点出 Prometheus `spec` label 取值变化，提示监控侧 review。

## 9. 不在本设计范围、但记为 follow-up

- `CheckPCIESpeed` / `CheckPCIEWidth`（NIC 自身 link 检查）下一阶段同样走 spec-free（直接用 NIC 自身的 `current_link_speed` vs `max_link_speed`）。
- `default_spec.yaml` 中 `pcie_tree_speed` / `pcie_tree_width` 字段的最终清理（先观察一个版本，确认无消费方再删）。
- 若现网采样发现确有"sysfs `max_link_speed` 不准"的平台，再单独评估白名单 / fallback 路径。
