# sichek 的写操作 / 副作用审计

> 设计契约：sichek 定位为**节点健康检查 agent，只做数据采集（collector）与判断（checker）**，
> 不应改变被检节点的硬件 / 内核 / 系统状态。
>
> 本文档只登记**生产路径中确实会被调用**、且会**改变系统状态**的写操作（隐式 autofix）。
> 纯内部状态写（Prometheus gauge、K8s annotation、snapshot、config/spec 下载、systemd 安装文件、日志）不在此列——
> 那些是 daemon 的既定职责。仅测试代码调用、生产链路无 caller 的写函数见文末"已审计但休眠"。

审计日期：2026-07-09。审计范围：非 `vendor/` 的全部 `.go`，`nvidia` 侧经 nvml 只读、无 `Set*` / `nvidia-smi` 写标志。

---

## 三处 active 写操作总览

| # | 动作 | 写入目标 | 触发组件 | 无守卫? |
|---|------|----------|----------|---------|
| 1 | 改写 PCIe Max Read Request 寄存器 | PCIe config space（`setpci`） | infiniband collector | 是 |
| 2 | 改写 CPU 频率调度 governor | sysfs `scaling_governor` | cpu checker | 是 |
| 3 | 在线加载内核模块 | `modprobe nvidia_peermem` | nvidia checker | 是 |

三者共同特征：**判定即修复**（把"修复"混进了采集 / 判断里），**没有开关、没有 spec 守卫、没有 dry-run**，
且都挂在 collector/checker 上，即**每个 HealthCheck / daemon tick 都会重新评估并可能再次执行**。

---

## 1. 改写 PCIe Max Read Request（MRR）寄存器

**做什么**：读到某 IB 网卡的 `MaxReadReq` ≠ 4096 时，用 `setpci` 把其 PCIe Device Control
寄存器（offset `0x68`）的高 4 位改成 `5`，使 MRR 字段（bit 14:12 = `101`）= 4096 字节，并回读校验。

**写命令**（`components/infiniband/collector/pcie_info.go:388`）：
```go
writeCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w="+writeValueStr)
// 实际形如: setpci -s <bdf> 68.w=5xxx
```

**调用链**：
```
InfiniBand HealthCheck / daemon tick
  └─ InfinibandCollector.Collect          components/infiniband/collector/infiniband_info.go:144
      └─ IBHardWareInfo.Collect           components/infiniband/collector/ib_hardware_info.go:75
          └─ GetPCIEMRR(ctx, IBDev)        ib_hardware_info.go:120-121
              └─ ModifyPCIeMaxReadRequest  components/infiniband/collector/pcie_info.go:358 (写在 :388)
```

**触发条件**（`pcie_info.go:234-243`，`GetPCIEMRR` 内）：
```go
if strings.Compare(mrr[0], "4096") != 0 {
    if err := ModifyPCIeMaxReadRequest(bdf[0], "68", 5); err != nil { ... }
}
```
即凡是 MRR≠4096 的 HCA，**无条件**触发写回。

**入参含义**（`ModifyPCIeMaxReadRequest(deviceAddr, offset, newHighNibble)`）：
- `deviceAddr` — PCI BDF（如 `80:00.0`），实参 = `GetIBDevBDF(IBDev)[0]`。
- `offset` — config space 寄存器偏移，实参 = `"68"`；`0x68` = PCIe Device Control 寄存器；拼成 `68.w` 读写 16 位。
- `newHighNibble` — 写入寄存器高 4 位（bit 15:12）的新值（0–0xF），实参 = `5`。位运算：
  `newValue = (current & 0x0FFF) | (newHighNibble << 12)`。

MRR 编码（bit 14:12）：0=128B,1=256B,2=512B,3=1024B,4=2048B,**5=4096B**。

**已知隐患**：
- **重复执行**：`GetPCIEMRR` 在 `ib_hardware_info.go:120`（`len(...)>=1` 判断）与 `:121`（赋值）被调用两次，
  故 autofix 判定与 `setpci` 写入实际上可能各跑两遍。
- **改的位比字段宽**：函数按"高 4 位（bit 15:12）"整体覆写，而 MRR 只是 bit 14:12；bit 15 是 Device Control 的
  保留 / BCR_FLR 位。当前实参 `5`（`0101`）bit15=0，无副作用；若日后传 8–F 会连带动到 bit 15。

---

## 2. 改写 CPU 频率调度 governor

**做什么**：CPU checker 发现有 CPU 不在 `performance` 模式时，直接把 `performance` 写进每个 CPU 的
sysfs governor 文件。

**写操作**（`components/cpu/checker/cpu_performance.go:147-181`，`setCPUMode`）：
```go
pattern := "/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor"
for _, file := range files {              // 遍历所有 CPU
    f, _ := os.OpenFile(file, os.O_WRONLY, 0644)
    f.WriteString(mode)                   // mode = "performance"
}
```

**调用链 & 触发条件**（`cpu_performance.go:53-62`，`CPUPerfChecker.Check`）：
```
cpu HealthCheck / daemon tick
  └─ CPUPerfChecker.Check
        if !cpuPerformanceEnable {
            setCPUMode("performance")     // 不在 performance 模式就写
        }
```
即每次 cpu 检查，只要检测到非 performance 模式即改写，然后回读确认。

---

## 3. 在线加载内核模块 nvidia_peermem

**做什么**：nvidia checker 发现 `nvidia_peermem` 未被 `ib_uverbs` / `ib_core` 持有时，直接 `modprobe` 加载它。

**写操作 & 触发条件**（`components/nvidia/checker/check_dependences/nvidia_peermem.go:45-76`，`NvPeerMemChecker.Check`）：
```go
if !usingPeermem {
    _, err := utils.ExecCommand(ctx, "modprobe", "nvidia_peermem")   // :62 在线加载
    ...
}
```

**调用链**：
```
nvidia HealthCheck / daemon tick
  └─ NvPeerMemChecker.Check
        └─ modprobe nvidia_peermem   （未加载时）
```
成功后 checker 报 `LoadedOnline`。

> 注意区分：nvidia 的 `config/check_items.go` 里 `Suggestion` 文案含
> `setpci ... ecap_acs+6.w=0`、`modprobe nvidia_peermem` 等——那些是**给用户看的建议字符串**，
> 不是 sichek 自己执行的命令。

---

## 4. 下载并以 root 身份执行 sysinfo 采集脚本

**做什么**：`sysinfo` 组件按配置 `sysinfo.sources` 列表，对每个 source 通过 HTTP(S) 下载一个 shell 脚本到临时文件，
用 `bash <tmpfile>` 以当前进程权限（daemon / systemd / DaemonSet 场景下即 root）执行，解析其 stdout 的 `key=value`
行作为采集结果，执行完即删除临时文件。

**写/执行操作**（`components/sysinfo/collector/collector.go:49-86`，`Collect`）：
```go
body, err := download(ctx, url, timeout)           // :52  HTTP(S) GET，不做任何校验和/签名验证
...
tmp, err := os.CreateTemp("", "sichek-"+sanitize(name)+"-*.sh")
tmp.Write(body)
...
cmd := exec.CommandContext(cctx, "bash", tmpPath)   // :71  以当前进程权限执行下载到本地的内容
```

**调用链**：
```
sysinfo HealthCheck（CLI 一次性）/ daemon runSource 定时 tick
  └─ collectAndStore                     components/sysinfo/sysinfo.go:124
      └─ collector.Collect                components/sysinfo/collector/collector.go:49
          └─ download（HTTP GET，无校验）   :52
          └─ bash <tmpfile>（exec）        :71
```

**触发条件**（`components/sysinfo/sysinfo.go:109-121`，`runSource` :177-205`）：每个 `enable` 的 source 在其自身
`interval`（或引擎级 `query_interval`）到期时、daemon 启动首轮、以及 CLI 一次性调用 (`HealthCheck` / `CollectOne`)
都会无条件下载并执行，无用户确认、无 dry-run 开关。

**信任边界与风险**：
- 下载地址来自 config `sysinfo.sources[].url`（或 `base_url + path` 拼接），缺省经 `resolveBaseURL` 回落到与 spec
  下载相同的 OSS host（HTTPS）——即与 `SICHEK_SPEC_URL` 同一信任边界。
- **不做任何校验和 / 签名验证**——只要该 URL 返回 HTTP 200，其响应体即被当作 shell 脚本原样执行，等价于对该 host
  的**远程代码执行（RCE）信任**。
- 因此 `sysinfo.sources` 列表本质上是一份"以 root 权限执行的 URL allowlist"：**新增或修改这个列表里的条目是一项
  特权操作**，应当按代码变更（而非普通配置调整）的严格度去评审——任何能够向该 host 写入内容、或篡改这份列表的人，
  都能在所有运行 sysinfo 的节点上以 root 身份任意执行代码。
- 与上面 3 处"判定即修复"的 autofix 不同，这里执行的是**外部下发、内容不受 sichek 自身控制的任意脚本**，风险类别
  不同（供应链 / RCE，而非本地状态误写），因此单列一节，不计入前面"三处 active 写操作"总表。
- `Collect` 现在会在 `download` 前用 `validateURL` 校验解析出的 URL scheme：非 `https` 一律拒绝（`fail`，不 panic、不下载），
  仅对 loopback host（`127.0.0.1` / `::1` / `localhost`）放行 `http`，以兼容测试与本地镜像，从而关闭明文中间人篡改脚本内容的 MITM 路径。

---

## 已审计但休眠（不在生产链路，故不计入上表）

- **PCIe ACS 开关**：`pkg/utils/pcie_inspect.go` 的 `DisableACS` / `DisableAllACS` / `BatchDisableACS` /
  `EnableACS`（`setpci ... ecap_acs+6.w=0` 关闭 / `=f` 开启 ACS）。
  经调用图确认：仅被 `*_test.go` 调用，`DisableAllACS` 零 caller，**生产路径当前无人触发**。
  代码仍在、关 ACS 影响 P2P/隔离，若日后接入需重新评估。

---

## 建议（未实施，待决策）

1. 给上述 3 处加统一 autofix 开关（默认关，保持 read-only），把"判断"与"修复"解耦。
2. 修 `GetPCIEMRR` 被调用两次导致 `setpci` 可能写两遍的问题（改为取一次、复用返回值）。
3. `ModifyPCIeMaxReadRequest` 收窄为只改 MRR 字段（bit 14:12），避免误动 bit 15。
