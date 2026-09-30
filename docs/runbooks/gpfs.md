# GPFS 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## gpfs-installed — GPFSNotInstalled (Critical)

**1. 是什么** — 校验节点上是否已安装 GPFS 软件。

**2. 典型输出**

```
checker=gpfs-installed component=gpfs
ErrorName: GPFSNotInstalled
Detail: <xstor-health basic-check 给出的未安装说明>
```

**3. 常见根因**

- 节点未安装 GPFS(Spectrum Scale)软件包。
- 新上线/重装系统节点尚未纳入存储客户端安装流程。
- 安装包版本与内核不匹配导致 GPFS 组件缺失。

**4. 确认命令(只读)**

```bash
rpm -qa | grep -i gpfs                             # 看 gpfs.base/gpfs.gplbin 等包是否存在
mmgetstate -a                                      # 若命令不存在,说明未安装
```

**5. 处置**

- 节点侧可修:按存储侧标准流程安装 GPFS 软件包及匹配当前内核的 gplbin。
- 升级存储团队:安装包缺失或与集群版本不一致、需要存储侧提供安装介质与配置时,升级 GPFS/存储团队处理。

**6. 级别与升级** — Critical,cordon 节点尽快修;GPFS 安装类问题通常升级给存储/GPFS 团队处理。

---

## node-in-cluster — GPFSNotInCluster (Critical)

**1. 是什么** — 校验节点是否已加入 GPFS 集群。

**2. 典型输出**

```
checker=node-in-cluster component=gpfs
ErrorName: GPFSNotInCluster
Detail: <xstor-health basic-check 给出的未入集群说明>
```

**3. 常见根因**

- 节点从未被加入 GPFS 集群(新节点漏配)。
- 节点被从集群中移除(如硬件更换、重装系统后未重新加入)。
- 集群配置分发失败,节点侧 GPFS 配置与集群不一致。

**4. 确认命令(只读)**

```bash
mmlscluster                                        # 看节点是否在集群成员列表中
mmgetstate -a                                      # 看节点状态是否可见
```

**5. 处置**

- 节点侧可修:若已有集群配置但未生效,可核对本机 GPFS 配置与网络连通性后重试加入。
- 升级存储团队:需要将节点加入 GPFS 集群(`mmaddnode` 等集群侧变更)属存储/GPFS 团队操作范畴,应升级处理。

**6. 级别与升级** — Critical,cordon 节点尽快修;加入/移除集群属存储侧操作,升级给存储/GPFS 团队。

---

## gpfs-started — GPFSNotStarted (Critical)

**1. 是什么** — 校验节点上 GPFS 软件(mmfsd 守护进程)是否已启动。

**2. 典型输出**

```
checker=gpfs-started component=gpfs
ErrorName: GPFSNotStarted
Detail: <xstor-health basic-check 给出的未启动说明>
```

**3. 常见根因**

- GPFS 服务未随节点启动/重启后未自动拉起。
- 上次异常关闭(如 quorum 丢失、被 expel)后未重新 `mmstartup`。
- 依赖的网络(RDMA/以太)未就绪导致启动失败。

**4. 确认命令(只读)**

```bash
mmgetstate -a                                      # 看本机状态是否为 active
mmhealth node show                                 # 看 GPFS 服务健康摘要
systemctl status gpfs 2>/dev/null                  # 部分部署以 systemd 管理
```

**5. 处置**

- 节点侧可修:确认依赖网络(RDMA/以太)正常后,执行 `mmstartup`(需按现场变更流程)重启 GPFS 服务。
- 升级存储团队:反复启动失败或伴随集群侧告警(如被 expel)时,升级存储/GPFS 团队排查集群侧原因。

**6. 级别与升级** — Critical,cordon 节点尽快修;反复无法启动升级给存储/GPFS 团队。

---

## gpfs-mounted — GPFSNotMounted (Critical)

**1. 是什么** — 校验 GPFS 文件系统是否已在节点上挂载。

**2. 典型输出**

```
checker=gpfs-mounted component=gpfs
ErrorName: GPFSNotMounted
Detail: <xstor-health basic-check 给出的未挂载说明>
```

**3. 常见根因**

- GPFS 服务未启动或未就绪,文件系统尚未挂载(先看 gpfs-started)。
- 文件系统被手动/自动强制卸载(如 quorum 丢失触发 forced unmount)。
- 挂载点配置(`/etc/fstab` 或 `mmremotefs`/`mmchfs` 的 automount 设置)缺失或错误。

**4. 确认命令(只读)**

```bash
mmlsmount all                                      # 只读查看已挂载节点列表
df -t gpfs                                         # 或 mount | grep gpfs
mmgetstate -a                                      # 确认服务本身是否 active
```

**5. 处置**

- 节点侧可修:GPFS 服务已 active 但未挂载 → 核对 automount 配置,必要时 `mmmount all`(需按变更流程执行)。
- 升级存储团队:文件系统被集群侧强制卸载(如 quorum/网络问题导致)需存储团队介入排查后再挂载。

**6. 级别与升级** — Critical,cordon 节点尽快修;强制卸载类根因升级给存储/GPFS 团队。

---

## gpfs-health — GPFSNodeNotHealthy (Critical)

**1. 是什么** — 校验 GPFS 节点整体健康状态是否正常(汇总 mmhealth 视角的节点健康)。

**2. 典型输出**

```
checker=gpfs-health component=gpfs
ErrorName: GPFSNodeNotHealthy
Detail: <xstor-health basic-check 给出的不健康说明,通常指向具体子系统>
```

**3. 常见根因**

- 某个 GPFS 子组件(如 FILESYSTEM、NETWORK、GPFS 守护进程)处于 DEGRADED/FAILED。
- 磁盘/NSD(Network Shared Disk)故障或不可达。
- 节点资源(内存/文件描述符)紧张导致 GPFS 内部检测异常。

**4. 确认命令(只读)**

```bash
mmhealth node show                                 # 看具体哪个子系统不健康
mmhealth node show -v                              # 详细事件列表
mmhealth node eventlog                             # 历史健康事件(只读查询)
```

**5. 处置**

- 节点侧可修:若为本机资源问题(内存/句柄紧张)或本机网络问题,按 `mmhealth node show` 指出的子系统对症处理。
- 升级存储团队:涉及 NSD/磁盘/集群侧组件异常时,升级存储/GPFS 团队协同排查 GPFS 日志。

**6. 级别与升级** — Critical,cordon 节点尽快修;NSD/磁盘等集群侧异常升级给存储/GPFS 团队。

---

## gpfs-rdma-network — GPFSRDMAError (Critical)

**1. 是什么** — 校验 GPFS 是否正常使用 RDMA 网络进行节点间通信。

**2. 典型输出**

```
checker=gpfs-rdma-network component=gpfs
ErrorName: GPFSRDMAError
Detail: <xstor-health basic-check 给出的 RDMA 异常说明>
```

**3. 常见根因**

- RDMA 网卡/端口未 up 或掉卡(与 infiniband 组件的 check_ib_lost/check_ib_state 同源)。
- GPFS `verbsPorts` 配置的 HCA 与实际存储轨 HCA 不符。
- RDMA 相关内核模块未加载,GPFS 回落到 TCP 通信。

**4. 确认命令(只读)**

```bash
mmfsadm dump verbs                                 # 看 GPFS 侧 verbs/RDMA 连接状态
mmdiag --network                                   # 看 GPFS 网络诊断信息
ibstat -l                                          # 核对 HCA 是否在位、是否 Active
```

**5. 处置**

- 节点侧可修:确认 `verbsPorts` 配置的 HCA 与实际存储轨一致,核对 RDMA 内核模块加载与网卡状态。
- 升级存储团队:排除本机网卡/配置问题后仍异常,升级存储/GPFS 团队结合 GPFS 日志进一步定位。

**6. 级别与升级** — Critical,cordon 节点尽快修;排除节点侧网络配置后升级给存储/GPFS 团队。

---

## 附:基于日志事件的 GPFS 告警

除上述 6 个 checker 外,sichek 还通过 dmesg/日志事件规则捕获以下 GPFS 相关问题(来自 `docs/errors-categorization.md`,并非通过上面的 6 个 checker 判定),分别在触发相应日志模式时上报:

- **QuorumConnectionDown**(Critical)— quorum 节点连接中断。
- **FilesystemUnmount**(Critical)— 文件系统在本节点被意外卸载。
- **ExpelledFromCluster**(Critical)— 本节点被 GPFS 集群踢出(expel)。
- **TimeClockError**(Warning)— 系统时间可能回跳,影响 GPFS 时钟一致性。
- **OSLockup**(Warning)— OS 锁死/停顿,可能导致 GPFS 心跳失败进而卸载。
- **RDMAStatusError**(Warning)— RDMA 网络状态异常。
- **BadTcpState**(Warning)— GPFS 节点间 TCP 连接状态异常。
- **Unauthorized**(Warning)— 节点对远程文件系统未获授权。

这些告警由事件规则(正则匹配 dmesg/GPFS 日志)触发,而非上述 6 个主动检测 checker;详细描述与建议见 `docs/errors-categorization.md` 与 `docs/gpfs.md`。
