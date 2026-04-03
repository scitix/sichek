# Sichek 新增组件开发指南

> 基于 transceiver 组件开发过程总结，包含踩过的坑和最佳实践。

---

## 一、新增组件完整 Checklist

### 1. 注册与配置（必须先做）

| 步骤 | 文件 | 说明 |
|------|------|------|
| 注册组件 ID 和名称 | `consts/consts.go` | 新增 `ComponentID` 和 `ComponentName` |
| 加入默认组件列表 | `consts/consts.go` → `DefaultComponents` | **容易遗漏** — 不加则 `sichek all` 和 daemon 不会检查该组件 |
| 创建用户配置结构 | `components/<name>/config/config.go` | 实现 `ComponentUserConfig` 接口（GetQueryInterval/SetQueryInterval） |
| 创建 spec 结构 | `components/<name>/config/spec.go` | 参照 nvidia 模式：EnsureSpec → FilterSpec → LoadSpec |
| 创建默认 spec YAML | `components/<name>/config/default_spec.yaml` | 开发环境回退用 |
| 追加用户配置 | `config/default_user_config.yaml` | 追加 query_interval, cache_size 等 |
| 追加 spec 配置 | `config/default_spec.yaml` | **容易遗漏** — 生产环境读这个文件 |

### 2. 数据采集层（Collector）

| 步骤 | 文件 | 说明 |
|------|------|------|
| 定义 Info 结构体 | `collector/<name>_info.go` | 实现 `common.Info` 接口（JSON 方法） |
| 实现 Collector | `collector/<name>_info.go` | 实现 `Collect(ctx) (*Info, error)` |
| 命令/sysfs 解析器 | `collector/*.go` | 每种数据源一个文件 |

### 3. 检查层（Checkers）

| 步骤 | 文件 | 说明 |
|------|------|------|
| 定义 check items 模板 | `config/check_items.go` | 集中管理 ErrorName/Level/Description/Suggestion |
| 注册所有 checkers | `checker/checker.go` | NewCheckers() + ignored 过滤 |
| 实现各 checker | `checker/check_*.go` | 实现 `common.Checker` 接口 |

### 4. 组件主体

| 步骤 | 文件 | 说明 |
|------|------|------|
| Component 主体 | `<name>.go` | 实现 `common.Component` 全部方法，使用 `CommonService` |
| Metrics 导出 | `metrics/metrics.go` | Prometheus 指标 |

### 5. 命令注册

| 步骤 | 文件 | 说明 |
|------|------|------|
| CLI 子命令 | `cmd/command/component/<name>.go` | cobra Command |
| 注册到根命令 | `cmd/command/command.go` | `rootCmd.AddCommand()` |
| 工厂函数 | `cmd/command/component/all.go` | `NewComponent` switch 加 case + import |

---

## 二、踩过的坑

### 坑 1: 忘记加入 DefaultComponents

**现象**: `sichek all` 和 daemon 模式不检查新组件，只有 `sichek <name>` 单独命令能跑。

**原因**: `consts.go` 里注册了 ComponentName 但没加入 `DefaultComponents` 切片。

**修复**: 在 `DefaultComponents` 切片末尾追加 `ComponentNameXxx`。

### 坑 2: 忘记更新 config/default_spec.yaml

**现象**: 生产环境 FilterSpec 报错 `match failed. Check if hardwareID exists`。

**原因**: 只在 `components/<name>/config/default_spec.yaml` 写了 spec，没同步到顶层 `config/default_spec.yaml`。生产环境读的是顶层文件。

**修复**: 两个 spec YAML 保持同步。注意 spec 格式要包含 key 层（如 `transceiver: { default: { ... } }`）。

### 坑 3: ethtool 输出解析 — switch case 顺序

**现象**: `Laser tx power high alarm threshold` 被 `strings.HasPrefix(key, "Laser tx power")` 错误匹配，阈值被当作功率值追加到数组。

**原因**: Go switch case 按顺序匹配，HasPrefix 的 case 写在精确匹配 alarm threshold 之前。

**教训**: **精确匹配的 case 必须放在 HasPrefix/Contains 的 case 之前**。写完解析器一定要用真实输出写单元测试验证。

### 坑 4: CheckerResult.Device 字段为空

**现象**: Prometheus 指标中 `device=""`，无法区分是哪个设备异常。

**原因**: 所有 checker 都没给 `result.Device` 赋值。

**教训**: 新建 checker 时就要设置 Device 字段，不要等 metrics 接入后才发现。

### 坑 5: mlx5 以太网口走错采集路径

**现象**: 400G mlx5 网卡的 `ethtool -m` 输出是 hex dump 而非可读格式，解析结果全是零。

**原因**: mlx5 驱动的以太网口（非 IB 模式）不在 `/sys/class/infiniband/` 下，代码只对 IB 接口用 mlxlink。

**修复**: 检测网卡驱动（读 `/sys/class/net/<dev>/device/driver` 链接），mlx5_core 驱动的以太网口也走 `mlxlink -d <PCIe BDF> -m`。

### 坑 6: 枚举了大量虚拟接口

**现象**: 输出中出现 `bonding_masters`、`eth0.10`（VLAN 子接口）等几十个无关条目。

**原因**: `/sys/class/net/` 下有大量虚拟接口，初版只过滤了 lo/veth/docker。

**修复**: 额外过滤 VLAN 子接口（含 `.` 的接口名）、`bonding_masters`、`virbr*`。

### 坑 7: 接口重复（IB + ETH 双重枚举）

**现象**: 同一个物理口出现两次 — 一次从 `/sys/class/net/` 枚举，一次从 `/sys/class/infiniband/` 枚举。

**修复**: IB 接口优先采集（mlxlink 数据更全），用 `seen` map 去重，ETH 侧跳过已采集的接口。

### 坑 8: 无 DOM 的光模块误报

**现象**: AOC 线缆没有 DOM 数据，但 voltage/power checker 用零值阈值比较，触发告警。

**修复**: 当 `LowAlarm == 0 && HighAlarm == 0` 时跳过检查。

### 坑 9: 未激活 lane 误报

**现象**: 2x200G breakout 模式下，lane 1-2 的 -40 dBm（无光）被当作功率异常报错。

**修复**: 功率 <= -30 dBm 的 lane 视为未激活，跳过检查。Bias current 同理，结合对应 lane 的 Tx power 判断。

### 坑 10: 网络分类靠接口名不可靠

**现象**: `eth0` 在某些环境是管理网，在另一些环境是 400G 业务口。

**修复**: 改为**速率优先分类** — `max_speed_mbps: 100000`，<=100G 自动归为管理网。接口名模式匹配作为回退。

### 坑 11: 告警级别硬编码在 checker 里

**现象**: 调整管理网/业务网的告警级别需要改 8 个文件。

**修复**: 集中定义 `BusinessCheckItems` 和 `ManagementCheckItems`（check_items.go），checker 通过 `GetCheckItem(name, networkType)` 获取级别。改级别只需改一个文件。

---

## 三、推荐开发流程

```
1. 设计
   ├── 明确检查项（哪些 checker）
   ├── 明确数据源（哪些命令/sysfs）
   └── 在真实设备上跑一遍采集命令，记录实际输出格式

2. 骨架搭建（先编译通过）
   ├── consts 注册 + DefaultComponents
   ├── config + spec（含 check_items.go）
   ├── collector 结构体 + 空 Collect()
   ├── checker 注册 + 空 Check()
   ├── component 主体
   ├── CLI 命令注册
   └── 两处 YAML 配置（config/ + components/）

3. 解析器实现（用真实输出驱动）
   ├── 拿到真实命令输出
   ├── 写解析器
   ├── 立即写单元测试（用真实输出作为测试数据）
   └── 注意 switch case 顺序、字段名精确匹配

4. Checker 实现
   ├── 从 check_items.go 获取 level/errorName（不要硬编码）
   ├── 设置 result.Device 字段
   ├── 处理边界：无阈值跳过、无 DOM 跳过、未激活 lane 跳过
   └── 写 checker 单元测试（构造 mock Info）

5. 多环境验证
   ├── 不同厂商光模块（ethtool 输出格式不同）
   ├── 不同驱动（mlx5 走 mlxlink，其他走 ethtool）
   ├── 不同网络配置（breakout、IB、RoCE、管理网）
   └── 无光模块的接口（应被跳过）

6. 收尾
   ├── Prometheus metrics 验证（device 字段有值）
   ├── Snapshot 验证（daemon 模式下 snapshot.json 包含数据）
   ├── PrintInfo 输出格式检查（无效值显示 "-"）
   └── 单元测试覆盖
```

---

## 四、文件结构模板

```
components/<name>/
├── <name>.go                    # Component 主体（实现 common.Component）
├── config/
│   ├── config.go                # UserConfig + checker 名称常量
│   ├── spec.go                  # Spec 结构体 + EnsureSpec/LoadSpec/FilterSpec
│   ├── check_items.go           # Business/Management CheckerResult 模板
│   └── default_spec.yaml        # 开发环境默认 spec
├── collector/
│   ├── <name>_info.go           # Info 结构体 + Collect() 入口
│   ├── <source1>.go             # 数据源解析器
│   ├── <source2>.go             # 数据源解析器
│   └── utils.go                 # 通用工具
├── checker/
│   ├── checker.go               # NewCheckers() 注册
│   └── check_<item>.go          # 各检查项
└── metrics/
    └── metrics.go               # Prometheus 指标导出
```

注册点（容易遗漏）：
- `consts/consts.go` — ComponentID + ComponentName + **DefaultComponents**
- `cmd/command/command.go` — rootCmd.AddCommand
- `cmd/command/component/all.go` — NewComponent switch + import
- `config/default_user_config.yaml` — 用户配置
- `config/default_spec.yaml` — **生产 spec（必须同步）**
