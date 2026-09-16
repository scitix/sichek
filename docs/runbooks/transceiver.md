# 光模块 (Transceiver) 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

该组件用 mlxlink/ethtool 采光模块 DDM 与物理层信号,按业务网/管理网套用不同严格度阈值。

---

## check_presence — TransceiverMissing (Fatal)

**1. 是什么** — 校验所有预期的光模块槽位是否都插了模块(在位检查)。

**2. 典型输出**

```
checker=check_presence component=transceiver
TransceiverMissing: <netdev>/<slot> 光模块缺失(expected populated)
```

**3. 常见根因**

- 光模块被拔出或未插到位(接触不良)。
- 模块彻底损坏、EEPROM/DDM 读不出被判为不在位。
- 该槽位本不该有模块(拓扑/预期与实际不符)。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 能否读出光模块 DDM,读不出=不在位
lspci -d 15b3: -nn                                 # 确认 HCA 在位
```

**5. 处置**

- 节点侧可试(重插/清洁):模块缺失且槽位应有模块 → 重插光模块、确认卡扣到位后复测。
- 换模块/换纤(升级硬件团队):重插仍读不出 → 疑似模块损坏,更换光模块,升级硬件团队。

**6. 级别与升级** — Business=Fatal(杀任务重投,**不 cordon** 节点);重插无效即换模块,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_tx_power — TxPowerOutOfRange (Critical)

**1. 是什么** — 校验光模块每 lane 的 Tx 发光功率是否在模块告警阈值内(带 margin)。

**2. 典型输出**

```
checker=check_tx_power component=transceiver
TxPowerOutOfRange: <netdev> laneN Tx power <val> dBm out of range
```

**3. 常见根因**

- 激光器老化/衰退,发光功率偏低。
- 光口/连接器脏污或光纤弯折,耦合损耗大。
- 模块本身故障导致某 lane 发光异常。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 DDM: 每 lane Tx power / 阈值
mlxlink -d <pci> -m -c                             # 采光功率 / BER / signal integrity
```

**5. 处置**

- 节点侧可试(重插/清洁):清洁光纤连接器与光口、重插模块两端后复测。
- 换模块/换纤(升级硬件团队):清洁/重插无效 → 更换光模块或光纤,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;清洁/重插无效即换硬件,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_rx_power — RxPowerOutOfRange (Critical)

**1. 是什么** — 校验光模块每 lane 的 Rx 收光功率是否在模块告警阈值内(带 margin)。

**2. 典型输出**

```
checker=check_rx_power component=transceiver
RxPowerOutOfRange: <netdev> laneN Rx power <val> dBm out of range
```

**3. 常见根因**

- 对端发光弱或对端模块故障(收光偏低)。
- 光纤链路损耗大:连接器脏污、弯折、断纤。
- 本端模块接收侧故障。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 DDM: 每 lane Rx power / 阈值
mlxlink -d <pci> -m -c                             # 采光功率 / BER / signal integrity
```

**5. 处置**

- 节点侧可试(重插/清洁):清洁光纤连接器与光口、重插模块两端后复测。
- 换模块/换纤(升级硬件团队):清洁/重插无效 → 检查对端模块、更换光纤或光模块,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;需连带排查对端,清洁/重插无效即换硬件,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_temperature — TransceiverOverheat (Critical)

**1. 是什么** — 校验光模块温度是否在 warning/critical 阈值内。

**2. 典型输出**

```
checker=check_temperature component=transceiver
TransceiverOverheat: <netdev> temperature <val> C over threshold
```

**3. 常见根因**

- 机箱风道/散热不良,进风温度过高。
- 模块散热片积灰或贴合不良。
- 模块自身故障导致异常发热。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 DDM: Module temperature / 阈值
```

**5. 处置**

- 节点侧可试(重插/清洁):检查风道与散热、降低环境温度、清理积灰后复测。
- 换模块/换纤(升级硬件团队):散热正常仍过热 → 更换过热模块,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;散热排除后仍过热即换模块,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_voltage — VoltageOutOfRange (Critical)

**1. 是什么** — 校验光模块供电电压是否在模块内置告警阈值内。

**2. 典型输出**

```
checker=check_voltage component=transceiver
VoltageOutOfRange: <netdev> supply voltage <val> V out of range
```

**3. 常见根因**

- 供电轨(3.3V)异常或不稳。
- 模块金手指接触不良,供电压降。
- 模块内部电源故障。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 DDM: Module voltage / 阈值
```

**5. 处置**

- 节点侧可试(重插/清洁):检查供电轨、重插模块使金手指接触良好后复测。
- 换模块/换纤(升级硬件团队):电压仍异常 → 更换光模块,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;供电/重插排除后仍异常即换模块,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_bias_current — BiasCurrentAbnormal (Critical)

**1. 是什么** — 校验光模块每 lane 的激光器偏置电流(bias current)是否异常。

**2. 典型输出**

```
checker=check_bias_current component=transceiver
BiasCurrentAbnormal: <netdev> laneN bias current <val> mA abnormal
```

**3. 常见根因**

- 激光器老化,偏置电流被拉高以维持功率(临近寿命末期)。
- 激光器驱动电路故障,偏置电流异常。
- 某 lane 激光器失效。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 DDM: 每 lane Tx bias current
mlxlink -d <pci> -m -c                             # 采光功率 / BER 佐证激光器状态
```

**5. 处置**

- 节点侧可试(重插/清洁):重插模块排除接触因素后复测(bias 异常多为模块本身)。
- 换模块/换纤(升级硬件团队):激光器可能正在失效 → 更换光模块,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;bias 异常多指向模块寿命,尽快换模块,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_link_errors — LinkErrorsIncreased (Critical)

**1. 是什么** — 校验两次相邻健康检查之间光模块 link error 计数器的增量(是否在持续增长)。

**2. 典型输出**

```
checker=check_link_errors component=transceiver
LinkErrorsIncreased: <netdev> link error counter delta +<n>
```

**3. 常见根因**

- 光纤完整性差:连接器脏污、弯折、微裂,误码持续累积。
- 光功率处于临界(收/发光偏弱),链路带病工作。
- 模块或线缆故障导致物理层误码。

**4. 确认命令(只读)**

```bash
ethtool -S <netdev>                                # 看 link error / 物理层错误计数器
ethtool -m <netdev>                                # 结合光功率判断是否临界
mlxlink -d <pci> -m -c                             # 采 BER / signal integrity
```

**5. 处置**

- 节点侧可试(重插/清洁):清洁连接器、检查光纤走线与弯折半径、重插后观察增量是否停止。
- 换模块/换纤(升级硬件团队):清洁/重插后误码仍增长 → 更换光模块或光纤,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;清洁/重插无效即换模块或线缆,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_signal_integrity — BadSignalIntegrity (Critical)

**1. 是什么** — 校验 mlxlink Recommendation 报告的物理层信号完整性问题(不依赖计数器)。

**2. 典型输出**

```
checker=check_signal_integrity component=transceiver
BadSignalIntegrity: <netdev>/<pci> mlxlink Recommendation reports bad signal integrity
```

**3. 常见根因**

- 物理层信号完整性差,FEC 正在带病纠错工作。
- 光模块/光纤劣化、连接器脏污导致信号裕量不足。
- 边缘链路信号在临界,间歇性 flapping。

**4. 确认命令(只读)**

```bash
mlxlink -d <pci> -m -c                             # 看 Recommendation 行 / BER / signal integrity
ethtool -m <netdev>                                # 结合 DDM 光功率佐证
```

**5. 处置**

- 节点侧可试(重插/清洁):低峰期隔离节点,清洁光口与连接器、重插模块两端后复测。
- 换模块/换纤(升级硬件团队):清洁/重插无效 → 尽快更换光模块与光纤,升级硬件团队。

真机正例:bjg66(lh-g23-141)/mlx5_8 稳定复现 Bad signal integrity;它是每次查询的瞬时判定,边缘链路会 flapping(mlxlink Recommendation 行为)。

**6. 级别与升级** — Critical,cordon 节点尽快修;FEC 带病工作,低峰期隔离并清洗/更换模块与光纤,升级硬件团队。管理网上该项为 Warning(择期)。

---

## check_vendor — VendorNotApproved (Warning)

**1. 是什么** — 校验光模块厂商是否在准入(approved)厂商列表内。

**2. 典型输出**

```
checker=check_vendor component=transceiver
VendorNotApproved: <netdev> vendor <name> not in approved list
```

**3. 常见根因**

- 装了非准入厂商/型号的光模块(采购或换件时混入)。
- 模块 EEPROM 厂商字段与准入列表不符。

**4. 确认命令(只读)**

```bash
ethtool -m <netdev>                                # 读 Vendor name / Vendor PN / SN
```

**5. 处置**

- 节点侧可试(重插/清洁):核对模块厂商字段,确认是否为记录错误或混件。
- 换模块/换纤(升级硬件团队):确属非准入模块 → 更换为准入厂商模块,升级硬件团队。

**6. 级别与升级** — Warning,cordon 节点择期修;非准入模块择期更换为准入型号,升级硬件团队。管理网上该项为 Warning(择期)。
