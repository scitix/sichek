# Sichek 体检项覆盖度评估

本文档对 Sichek 项目的健康检查覆盖范围进行全面评估，识别已有检查项和待补充的方向。

## 已覆盖的检查项

### 1. GPU（nvidia 组件，~21 项）

覆盖非常全面，包括：

- **依赖检查**：PCIe ACS、IOMMU、FabricManager、nvidia_peermem
- **GPU 特性**：应用时钟、NVLink、持久模式、性能状态、硬件丢失、驱动/CUDA 版本、温度、PCIe 降速、时钟节流、IBGDa、P2P 拓扑
- **ECC 内存**：SRAM volatile/aggregate uncorrectable 错误、高 correctable 错误率、remapped rows（失败/挂起/高 uncorrectable）
- **XID 事件**：31（页错误）、48（DBE ECC）、63（行重映射挂起）、64（行重映射失败）、74（NVLink 错误）、79（GPU 丢失）、92（高单位 ECC 错误率）、94（受控 ECC 错误）、95（未受控 ECC 错误）

### 2. InfiniBand（infiniband 组件，17 项）

覆盖全面，包括：

- **链路与固件**：固件版本、OFED 版本、内核模块、驱动
- **设备状态**：端口状态（ACTIVE）、物理状态（LINK_UP）、端口速率、设备名称、设备丢失、网络 operstate
- **PCIe 拓扑**：ACS 禁用、Max Read Request、链路速度、链路宽度、全路径速度/宽度

### 3. 光模块（transceiver 组件，8 项）

覆盖全面：

- 发送/接收光功率、模块温度、供电电压、激光偏置电流、厂商验证、链路错误计数、插槽在位检查
- 区分业务网络（Critical）和管理网络（Warning）的告警级别

### 4. 以太网（ethernet 组件，5 项）

按 OSI 分层覆盖：

- L1（物理链路）：Bond 接口、物理链路检测、链路速率、CRC/载波错误
- L2（Bond）：Bond 配置、最低 slave 数、链路故障
- L3（LACP）：LACP 协议状态
- L4（ARP）：ARP 功能验证
- L5（路由）：路由配置

### 5. GPFS（gpfs 组件，6 项）

覆盖全面：

- GPFS 安装、节点加入集群、守护进程运行、文件系统挂载、节点健康、RDMA 网络

### 6. CPU（cpu 组件，1 项）

- 仅检查 CPU 性能模式（是否为 performance 模式）

### 7. 内存（memory 组件）

- 基于事件检测，无传统 checker

### 8. PCIe 拓扑（pcie 组件，2 项）

- NUMA 设备关系验证、PCIe Switch 设备关系验证

### 9. GPU 事件/挂起检测（gpuevents 组件，2 项）

- GPU Hang 检测、SM 时钟卡低频检测

### 10. 日志监控（dmesg/syslog/podlog，事件型）

- 内核日志（dmesg）、系统日志（syslog）、Pod 日志（podlog）
- 基于事件规则的正则匹配

### 11. HCA（hca 组件）

- 基于板卡 ID 的性能基线对比：单向带宽（Gbps）、平均延迟（微秒）

## 待补充的检查项

### 优先级 P0：高影响、常见隐性故障源

#### 时钟同步（NTP/PTP）

分布式训练依赖节点间的时序协调，时钟偏差会导致：

- **NCCL 超时误判**：NCCL 集合通信有 timeout 机制，时钟漂移导致超时计算不准确，可能误判正常慢节点为超时，也可能延迟发现真正的 hang
- **日志排障困难**：几百张卡的训练任务出故障时，依赖各节点日志时间戳做关联分析。时钟不同步则无法确定事件先后顺序
- **调度和心跳异常**：K8s 的 lease 机制、kubelet 心跳依赖时间。时钟偏差严重时节点可能被误判为 NotReady
- **RDMA 层面**：某些 RDMA 操作有时间戳校验，PTP 不同步可能引发微妙的性能下降

时钟漂移是渐进的，不会立刻报错——等到影响训练时往往已经漂了很久。

建议检查项：
- PTP 服务（ptp4l/phc2sys）是否在运行
- 时钟偏移量（offset）是否超过阈值（通常 >1ms 应告警）
- NTP fallback 是否正常（chrony/ntpd 状态）
- 最近一次成功同步的时间距今多久

#### CPU MCE（Machine Check Exception）错误

MCE 是 x86 CPU 的硬件错误报告机制。对 GPU 集群尤为重要：

- **训练任务代价极高**：大模型训练可能跑几天到几周，MCE 隐患未检测到则中途宕机，浪费的算力成本巨大
- **可预测性**：Corrected Error (CE) 的增长趋势可预测，持续上升说明 CPU 或主板在退化，应在变成 Uncorrected Error (UCE) 宕机之前主动迁移负载
- **与已有模式一致**：项目对 GPU ECC 错误做了细致检查（correctable/uncorrectable/remapped rows），CPU MCE 本质相同但完全未覆盖

建议检查项：
- `/var/log/mcelog` 或 `rasdaemon` 中的 MCE 事件
- CE 计数是否超过阈值或增长过快
- UCE 是否发生过（一旦发生应标记为 Critical）

#### 内存 ECC 错误（EDAC）

内存错误是数据中心最常见的硬件故障类型（Google 研究：约三分之一服务器每年至少经历一次 correctable 内存错误）：

- **数据静默损坏**：ECC 耗尽纠错能力后，内存错误可能导致训练数据或模型权重被静默篡改——比训练中断更糟糕
- **CE 是 UCE 的前兆**：correctable 错误频繁出现意味着 DIMM 老化，提前发现可安排维护窗口更换
- **当前空白**：项目对 GPU 显存 ECC 检查做得很细，但对系统主内存（DDR）的 EDAC 信息完全未采集

建议检查项：
- EDAC（`/sys/devices/system/edac/mc*/`）CE/UCE 计数
- 内存容量是否与 spec 一致（DIMM 故障可能导致容量减少）
- `mcelog` / `rasdaemon` 中的内存相关事件

### 优先级 P1：运维关键项

| 检查项 | 说明 | 理由 |
|--------|------|------|
| CPU 温度 | 监控 CPU 温度是否超过阈值 | 过热可能导致降频影响训练性能 |
| CPU 频率 | 检查实际频率是否达标（Turbo Boost 状态） | 频率异常直接影响数据预处理性能 |
| NUMA 均衡 | 检查 NUMA 节点间内存/进程分布 | 不均衡会导致性能下降 |
| 磁盘 SMART 状态 | 本地磁盘健康检查 | 磁盘故障影响 checkpoint 写入 |
| 磁盘空间/inode | 文件系统空间和 inode 使用率 | 空间不足导致训练中断 |
| NVMe 温度和寿命 | NVMe SSD 温度和剩余寿命 | SSD 到达寿命末期性能急剧下降 |
| 关键服务状态 | kubelet、containerd 等核心服务 | 服务异常导致 Pod 无法调度或运行 |
| 内核参数合规 | sysctl 参数是否符合最佳实践 | 不当配置影响网络和内存性能 |
| ulimit / fd 限制 | 文件描述符等资源限制 | 限制过低可能导致训练进程异常 |

### 优先级 P2：增强项

| 检查项 | 说明 | 理由 |
|--------|------|------|
| 网络丢包/重传率 | TCP 重传和丢包统计 | 网络质量影响分布式训练效率 |
| 节点间延迟 | 节点间 ping 延迟基准 | 延迟异常可能指示网络设备问题 |
| DNS 解析 | DNS 解析功能和延迟 | DNS 故障影响服务发现 |
| MTU 一致性 | 检查 MTU 配置是否一致 | MTU 不一致导致通信碎片化 |
| 内核版本一致性 | 集群内核版本是否统一 | 版本不一致可能引入兼容性问题 |
| GPU 间带宽基准 | NCCL allreduce 带宽测试 | 端到端性能验证 |
| MIG 模式检查 | GPU MIG 模式状态 | MIG 状态不一致影响任务分配 |
| GPU 功耗异常 | 功耗是否在正常范围 | 功耗异常可能指示硬件问题 |
| 内存带宽基准 | 内存带宽是否达标 | 带宽下降影响数据加载性能 |

## 总体评价

**当前核心覆盖度约 75-80%。**

**优势：**
- GPU 和 InfiniBand 作为 GPU 集群最关键的领域，检查项非常全面
- 光模块监控（transceiver）在同类工具中属于差异化优势
- 事件驱动的日志监控（dmesg/syslog/podlog）提供了灵活的异常捕获能力
- 错误分级体系（Fatal/Critical/Warning）设计合理

**短板：**
1. **CPU/内存检查过于薄弱** —— CPU 仅 1 个检查项，内存几乎没有传统 checker
2. **缺少本地存储检查** —— 没有磁盘健康相关组件
3. **缺少系统级合规检查** —— 时钟同步、内核参数、ulimit 等运维关键项未覆盖
4. **缺少端到端性能基准** —— 如 NCCL allreduce 带宽测试

**建议优先补充：时钟同步（PTP/NTP）、CPU MCE 错误检测、内存 ECC/容量检查**，这三类是大规模训练中最常见的隐性故障源。三者共同特点是故障隐蔽、后果严重、检测简单，且项目已有类似模式（GPU ECC），实现成本低。
