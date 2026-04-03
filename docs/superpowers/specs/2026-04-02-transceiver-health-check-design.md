# 光模块健康检查组件设计

> 日期: 2026-04-02

---

## 目标

新增独立 `transceiver` component，统一检查服务器上所有光模块（IB / RoCE / 管理网）的健康状态，支持按网络类型分级告警。

## 非目标

- 不做光模块固件升级
- 不做跨节点光链路质量对比
- 不做历史趋势预测

---

## 一、架构

遵循 sichek 标准三层模式：

```
Collector（采集 DOM 数据）
  ├── ethtool -m {netdev}       → 以太网/RoCE/管理网
  └── mlxlink -d {ibdev} -m     → InfiniBand
      ↓
Checkers（8 个检查项，并行执行）
      ↓
Component Result → K8s annotation / Prometheus metrics / snapshot
```

### 数据流

1. Collector 枚举所有带光模块的网络接口
2. 根据接口类型选择 `ethtool` 或 `mlxlink` 采集 DOM 数据
3. 根据 spec 配置将接口分类为 `business`（业务网）或 `management`（管理网）
4. 各 Checker 根据网络分类使用不同阈值进行检查
5. 汇总结果，最高严重级别作为 component 级别

---

## 二、检查项（8 个 Checker）

| # | Checker 名称 | 检查内容 | 级别 | 管理网 |
|---|-------------|---------|------|--------|
| 1 | `check_tx_power` | 发送光功率是否在阈值内（每 lane） | critical | warning |
| 2 | `check_rx_power` | 接收光功率是否在阈值内（每 lane） | critical | warning |
| 3 | `check_temperature` | 光模块温度是否超限 | warning/critical | warning |
| 4 | `check_voltage` | 供电电压是否在范围内 | warning | 跳过 |
| 5 | `check_bias_current` | 偏置电流是否异常（每 lane） | warning | 跳过 |
| 6 | `check_vendor` | 厂商/型号是否在认证清单 | warning | 跳过 |
| 7 | `check_link_errors` | CRC/FCS 错误计数是否异常增长 | critical | 跳过 |
| 8 | `check_presence` | 预期槽位光模块是否在位 | fatal | warning |

**管理网"跳过"**：对应 checker 对管理网接口直接返回 StatusNormal，不做检查。

---

## 三、数据采集

### 3.1 接口枚举

1. 扫描 `/sys/class/net/` 获取所有网络接口
2. 对每个接口判断是否有光模块：
   - 以太网：`ethtool -m {dev}` 是否返回有效数据
   - IB：检查 `/sys/class/infiniband/{dev}` 是否存在
3. 根据 spec 中的 `interfaces` 模式匹配分类为 business 或 management

### 3.2 以太网/RoCE 光模块（ethtool）

| 数据 | 命令 | 解析内容 |
|------|------|---------|
| DOM 全量数据 | `ethtool -m {netdev}` | Tx/Rx Power、Temperature、Voltage、Bias Current（每 lane） |
| 模块内置阈值 | `ethtool -m {netdev}` | High/Low Alarm/Warning threshold 字段 |
| 厂商/型号/SN | `ethtool -m {netdev}` | Vendor name、Part number、Serial number |
| 模块类型 | `ethtool -m {netdev}` | Identifier 字段（SFP/QSFP28/QSFP-DD/OSFP） |
| 链路错误计数 | `ethtool -S {netdev}` | rx_crc_errors、rx_fcs_errors 等 |

### 3.3 InfiniBand 光模块（mlxlink）

| 数据 | 命令 | 解析内容 |
|------|------|---------|
| DOM 全量数据 | `mlxlink -d {dev} -m` | Temperature、Voltage、Bias Current、Tx/Rx Power（每 lane） |
| 厂商/型号/SN | `mlxlink -d {dev} -m` | Vendor Name、Part Number、Serial Number |
| 模块类型 | `mlxlink -d {dev} -m` | Cable Type / Identifier |
| 模块在位 | `mlxlink -d {dev}` | 是否返回 "No cable/module" |
| 链路错误 | sysfs `/sys/class/infiniband/{dev}/ports/1/counters/` | symbol_error、VL15_dropped 等 |

### 3.4 Collector 输出结构

```go
type TransceiverInfo struct {
    Modules []ModuleInfo `json:"modules"`
}

type ModuleInfo struct {
    // 标识
    Interface   string `json:"interface"`     // 网络接口名 (eth0 / mlx5_0 等)
    IBDev       string `json:"ib_dev"`        // IB 设备名（IB 光模块时有值）
    NetworkType string `json:"network_type"`  // "business" | "management"
    CollectTool string `json:"collect_tool"`  // "ethtool" | "mlxlink"

    // 模块信息
    Present     bool   `json:"present"`
    ModuleType  string `json:"module_type"`   // SFP / QSFP28 / QSFP-DD / OSFP
    Vendor      string `json:"vendor"`
    PartNumber  string `json:"part_number"`
    SerialNumber string `json:"serial_number"`

    // DOM 数据
    Temperature  float64   `json:"temperature_c"`
    Voltage      float64   `json:"voltage_v"`
    TxPower      []float64 `json:"tx_power_dbm"`      // 每 lane
    RxPower      []float64 `json:"rx_power_dbm"`      // 每 lane
    BiasCurrent  []float64 `json:"bias_current_ma"`   // 每 lane

    // 模块内置阈值
    TxPowerHighAlarm  float64 `json:"tx_power_high_alarm_dbm"`
    TxPowerLowAlarm   float64 `json:"tx_power_low_alarm_dbm"`
    RxPowerHighAlarm  float64 `json:"rx_power_high_alarm_dbm"`
    RxPowerLowAlarm   float64 `json:"rx_power_low_alarm_dbm"`
    TempHighAlarm     float64 `json:"temp_high_alarm_c"`
    TempLowAlarm      float64 `json:"temp_low_alarm_c"`
    VoltageHighAlarm  float64 `json:"voltage_high_alarm_v"`
    VoltageLowAlarm   float64 `json:"voltage_low_alarm_v"`

    // 错误计数
    LinkErrors  map[string]uint64 `json:"link_errors"`
}
```

---

## 四、网络分级配置

### 4.1 用户配置（default_user_config.yaml 追加）

```yaml
transceiver:
  query_interval: 30s
  cache_size: 5
  enable_metrics: true
  ignored_checkers: []
```

### 4.2 Spec 配置（default_spec.yaml）

```yaml
transceiver:
  networks:
    business:
      interface_patterns:
        - "mlx5_*"
        - "ib*"
        - "enp*s0f*"
        - "bond*"
      thresholds:
        tx_power_margin_db: 1.0
        rx_power_margin_db: 1.0
        temperature_warning_c: 65
        temperature_critical_c: 75
      check_vendor: true
      check_link_errors: true
      approved_vendors:
        - "Mellanox"
        - "NVIDIA"
        - "Innolight"
        - "Hisense"
    management:
      interface_patterns:
        - "eno*"
        - "eth0"
        - "mgmt*"
      thresholds:
        tx_power_margin_db: 3.0
        rx_power_margin_db: 3.0
        temperature_warning_c: 75
        temperature_critical_c: 85
      check_vendor: false
      check_link_errors: false
      approved_vendors: []
```

**阈值策略**：优先使用模块内置的 DOM 告警阈值（从 EEPROM 读取），spec 中的 `margin` 作为在内置阈值基础上的额外裕量。如果模块无内置阈值，使用 spec 中的绝对值。

---

## 五、Checker 详细逻辑

### check_tx_power / check_rx_power

- 遍历每个 lane 的功率值
- 比较：`value < low_alarm + margin` 或 `value > high_alarm - margin` → abnormal
- 业务网 level=critical，管理网 level=warning
- detail 输出具体哪个接口哪个 lane 异常及当前值

### check_temperature

- 比较模块温度 vs 阈值
- `> temperature_critical_c` → level=critical
- `> temperature_warning_c` → level=warning

### check_voltage / check_bias_current

- 比较 vs 模块内置 High/Low Alarm 阈值
- 管理网跳过
- level=warning

### check_vendor

- 检查 Vendor 是否在 `approved_vendors` 列表中（大小写不敏感）
- 管理网跳过
- level=warning

### check_link_errors

- 对比两次采集间的错误计数增量
- 增量 > 0 → abnormal
- 管理网跳过
- level=critical

### check_presence

- 预期有光模块但 `Present=false` → abnormal
- 业务网 level=fatal（可能导致训练中断），管理网 level=warning

---

## 六、文件结构

```
components/transceiver/
├── transceiver.go              # Component 主体
├── collector/
│   ├── transceiver_info.go     # TransceiverInfo 结构体 + Collect()
│   ├── ethtool_dom.go          # ethtool -m / -S 解析
│   ├── mlxlink_dom.go          # mlxlink -d -m 解析
│   └── utils.go                # 接口枚举、网络分类、通用解析
├── checker/
│   ├── checker.go              # NewCheckers() 注册
│   ├── check_tx_power.go
│   ├── check_rx_power.go
│   ├── check_temperature.go
│   ├── check_voltage.go
│   ├── check_bias_current.go
│   ├── check_vendor.go
│   ├── check_link_errors.go
│   └── check_presence.go
├── config/
│   ├── config.go               # TransceiverUserConfig
│   ├── check_items.go          # Checker 名称常量
│   ├── spec.go                 # TransceiverSpec
│   └── default_spec.yaml
└── metrics/
    └── metrics.go
```

### 注册点

- `consts/consts.go`：新增 `ComponentNameTransceiver = "transceiver"`
- `cmd/command/command.go`：新增 `NewTransceiverCmd()`
- `cmd/command/component/`：新增 transceiver 子命令
- `config/default_user_config.yaml`：追加 transceiver 段
