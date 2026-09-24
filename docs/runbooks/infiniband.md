# InfiniBand / RoCE 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## check_ib_phy_state — IBPhyStateNotLinkUp (Critical)

**1. 是什么** — 校验所有 IB 端口物理层是否 LINK_UP(链路是否物理点亮)。

**2. 典型输出**

```
PhyState check fail: mlx5_0/p1 NOT LinkUp
（stderr:PhyState abnormal on mlx5_0/p1: 3: Disabled doesn't contain LinkUp）
```

**3. 常见根因**

- 光模块/线缆没插好、坏线、坏口(最常见)。
- 对端交换机口 shutdown 或未配置。
- HCA 固件把口置 Disabled(如固件内部故障)。

**4. 确认命令(只读)**

```bash
ibstat <device>                                            # 看 Physical state
cat /sys/class/infiniband/<dev>/ports/1/phys_state
mlxlink -d <pci> -m                                        # 信号质量,边缘链路会 flapping
```

**5. 处置**

- 节点侧可自愈/重配:Disabled/Polling 且线缆在位 → 重插光模块与线缆两端、换已知好线复测;对端交换机口问题 → 联系网络团队核对交换机侧状态。
- 该换线换卡(升级硬件团队):换线仍 down → 疑似硬件故障,走升级换卡/换口。

**6. 级别与升级** — Critical,cordon 节点尽快修;换线无效即判硬件,升级硬件/网络团队。

---

## check_ib_state — IBStateNotActive (Critical)

**1. 是什么** — 校验所有 IB 端口逻辑状态是否 ACTIVE(SM 是否已把口带起来)。

**2. 典型输出**

```
PortState check fail: mlx5_0/p1 NOT ACTIVE
（stderr:PortState abnormal on mlx5_0/p1: 1: DOWN doesn't contain ACTIVE）
```

**3. 常见根因**

- 物理层没起(先看 check_ib_phy_state,phy 不 up 时 state 必不 active)。
- OpenSM/子网管理器没跑或没发现该口(INIT 卡住)。
- 对端未连通。

**4. 确认命令(只读)**

```bash
ibstat <device>                                    # State: Active/Init/Down
sminfo                                             # 有无 SM
cat /sys/class/infiniband/<dev>/ports/1/state
```

**5. 处置**

- 节点侧可自愈/重配:phy up 但 state=Init → 检查 `systemctl status opensm`、重启 SM 或核对 SM 是否覆盖该子网。
- 该换线换卡(升级硬件团队):phy 也 down → 先按 check_ib_phy_state 处理线缆/硬件;SM 与线缆均排除后仍 down → 升级硬件/网络团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;SM/线缆均排除后升级。

---

## check_ib_port_speed — IBPortSpeedNotMax (Critical)

**1. 是什么** — 校验 IB 端口协商速率是否达到该机型 spec 基线(如 200 Gb/sec 2X NDR)。

**2. 典型输出**

```
PortSpeed check fail: mlx5_0/p1 expect [200 Gb/sec (2X NDR)], but get [40 Gb/sec (4X QDR)]
```

**3. 常见根因**

- 链路协商掉档(坏线/脏光口/线缆规格不够)。
- 对端交换机口速率配置不符。
- 端口实际 down/QDR 回落(常伴随 phy_state/state 同时 FAIL)。
- 该口本不该是算力口(spec 基线与实际角色不符,联系配置中心核对 spec)。

**4. 确认命令(只读)**

```bash
ibstat <device>                                   # Rate
cat /sys/class/infiniband/<dev>/ports/1/rate
mlxlink -d <pci>
```

**5. 处置**

- 节点侧可自愈/重配:仅速率掉档 → 换线、清洁光口、核对对端交换机口速率;spec 期望与实际角色不符 → 核对该机型 spec 基线,勿盲目换硬件。
- 该换线换卡(升级硬件团队):同时 phy/state FAIL → 按物理层链路问题处理(重插/换线),速率通常随之恢复;换线/核对 spec 后仍不达标则升级换卡。

**6. 级别与升级** — Critical,cordon 节点尽快修;换线/核对 spec 后仍不达标则升级换卡。

---

## check_ib_lost — IBLost (Critical)

**1. 是什么** — 校验 IB 设备是否掉卡(HCA 从 PCI/RDMA 栈上消失)。

**2. 典型输出**

```
IBLost: HCAPCINum != IBCapablePCINum(7 != 8)
```

**3. 常见根因**

- HCA 固件崩溃/内部故障后从总线掉出(如 mlx5_core probe 失败)。
- PCIe 链路故障、板卡/插槽接触不良。
- 启动竞态导致 mlx5 异步注册未完成即采集(重启后偶发误报,通常下一 tick 自愈)。

**4. 确认命令(只读)**

```bash
lspci -d 15b3: -nn                                 # 数 Mellanox 设备是否齐
ibstat -l                                          # 列出当前 RDMA 设备
ls /sys/class/infiniband/                          # 与预期 HCA 数比对
dmesg | grep -i mlx5_core                          # 找 probe 失败/固件报错
```

**5. 处置**

- 节点侧可自愈/重配:重启后偶发误报可复测确认;确属固件挂死 → host 侧尝试 `mlxfwreset`(KVM guest 内不可用)。
- 该换线换卡(升级硬件团队):FLR/软复位清不掉、总线上确实少卡 → host 侧冷复位或换卡,升级硬件团队。

真机正例:lmg104 节点 mlx5_1(PCIe 65:01.0)固件崩溃后掉卡。

**6. 级别与升级** — Critical,cordon 节点尽快修;软复位无效即判硬件,升级硬件团队。

---

## check_net_operstate — IBNetOperStateNotUP (Critical)

**1. 是什么** — 校验网络接口 operstate 是否为 UP(RoCE/以太模式下网卡链路状态)。

**2. 典型输出**

```
NetOperstate check fail: <hca> expected state = up, current state = down
```

**3. 常见根因**

- 网卡驱动未加载或接口未 up。
- 线缆/光模块故障、对端交换机口 down。
- 接口被配置为 down 或未纳入 bond。

**4. 确认命令(只读)**

```bash
cat /sys/class/net/<iface>/operstate
ip link show <iface>
ethtool <iface>                                    # Link detected / Speed
```

**5. 处置**

- 节点侧可自愈/重配:接口 down 且线缆在位 → 核对驱动加载与接口配置、`ip link set <iface> up`。
- 该换线换卡(升级硬件团队):换线仍 down → 疑似线缆/网卡硬件故障,升级硬件/网络团队。

> 说明:该 checker 当前在 `checker.go` 中未注册(构造函数被注释),节点上默认不会实际运行;此条为语义参考。

**6. 级别与升级** — Critical,cordon 节点尽快修;换线无效即判硬件,升级硬件/网络团队。

---

## check_ib_num — IBDeviceCountMismatch (Critical)

**1. 是什么** — 校验检测到的 IB 设备数量是否与 PCI 扫描一致(是否有 NIC 缺失)。

**2. 典型输出**

该 checker 的失败态无固定单行模板;正常态 Detail 为 `All expected IB NICs are detected`,异常态表意为「检测到的 IB NIC 数与 PCI 扫描不符」。

**3. 常见根因**

- IB NIC 未被识别/掉卡(与 check_ib_lost 同源)。
- PCIe 状态异常、插槽接触不良。
- 驱动未加载导致设备未注册。

**4. 确认命令(只读)**

```bash
lspci -d 15b3: -nn                                 # PCI 侧 Mellanox 数量
ibstat -l                                          # RDMA 侧设备数量
ls /sys/class/infiniband/
```

**5. 处置**

- 节点侧可自愈/重配:核对驱动加载、复测确认数量。
- 该换线换卡(升级硬件团队):PCI 侧确实少卡 → 检查 PCIe 状态/换卡,升级硬件团队。

> 说明:该 checker 当前在 `checker.go` 中未注册(构造函数被注释),节点上默认不会实际运行;掉卡场景实际由 check_ib_lost 覆盖。

**6. 级别与升级** — Critical,cordon 节点尽快修;PCIe/换卡问题升级硬件团队。

---

## check_pcie_tree_speed — PCIETreeSpeedDownDegraded (Critical)

**1. 是什么** — 校验从 HCA 到 root complex 的整条 PCIe 链路速率是否有降级瓶颈。

**2. 典型输出**

```
mlx5_3 upstream link 0000:16:02.0->0000:17:00.0 current 16GT/s < cap 32GT/s
（Device 字段:mlx5_3(0000:19:00.0 bottleneck@0000:16:02.0->0000:17:00.0)）
```

**3. 常见根因**

- 上游 PCIe switch/根端口链路协商降速(cur < min(两端 max))。
- 板卡/插槽接触不良、PCIe 信号质量差。
- 主机拓扑本身某段为低速(需与真降级区分)。

**4. 确认命令(只读)**

```bash
lspci -vvv -s <bdf> | grep -iE 'LnkSta|LnkCap'     # 逐级看 Speed
lspci -tv                                          # 看 PCIe 树拓扑
```

**5. 处置**

- 节点侧可自愈/重配:核对该段两端 max 能力,确认是真降级还是拓扑固有低速。
- 该换线换卡(升级硬件团队):确属链路协商降级 → 重插/换卡、检查上游 switch,升级硬件团队。

真机正例:changliu-g88-2(H200)mlx5_3 上游 PEX890xx Gen5 switch 链路 cur=16/max=32。

**6. 级别与升级** — Critical,cordon 节点尽快修;确属硬件降级则升级硬件团队。

---

## check_pcie_tree_width — PCIETreeWidthIncorrect (Critical)

**1. 是什么** — 校验从 HCA 到 root complex 的整条 PCIe 链路宽度(lane 数)是否有降级瓶颈。

**2. 典型输出**

```
mlx5_3 upstream link 0000:16:02.0->0000:17:00.0 current width x8 < cap x16
（Device 字段:mlx5_3(0000:19:00.0 bottleneck@0000:16:02.0->0000:17:00.0)）
```

**3. 常见根因**

- 上游 PCIe switch/根端口链路 lane 数协商不足(cur < min(两端 max))。
- 板卡/插槽接触不良、部分 lane 未训练起来。
- 主机拓扑/BIOS 分叉配置导致宽度受限。

**4. 确认命令(只读)**

```bash
lspci -vvv -s <bdf> | grep -iE 'LnkSta|LnkCap'     # 看 Width x1/x4/x8/x16
lspci -tv
```

**5. 处置**

- 节点侧可自愈/重配:核对该段两端 max 与 BIOS 分叉设置。
- 该换线换卡(升级硬件团队):确属 lane 降级 → 重插/换卡、检查上游 switch 与主板槽位,升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;确属硬件降级则升级硬件团队。

---

## check_pcie_acs — PCIEACSNotDisabled (Critical)

**1. 是什么** — 校验 IB 路径上的 PCIe ACS(Access Control Services)是否已关闭(ACS 开启会破坏 GPU Direct RDMA 的 peer-to-peer 路径)。

**2. 典型输出**

该 checker 的失败态无固定单行模板;正常态 Detail 为 `PCIe ACS is disabled on all IB paths`,异常态表意为「某些 IB 路径上 ACS 仍启用」。

**3. 常见根因**

- BIOS 或内核未关闭 ACS(默认可能开启)。
- 节点重装/重启后 ACS 关闭脚本未生效。

**4. 确认命令(只读)**

```bash
lspci -vvv -s <bdf> | grep -i ACSCtl               # 看 SrcValid+ 等是否开启
setpci -s <bdf> ECAP_ACS+6.w                       # 读 ACS 控制寄存器(只读)
```

**5. 处置**

- 节点侧可自愈/重配:在 BIOS 或内核参数中关闭 ACS(见 sichek 写操作审计,ACS 关闭属节点侧可重配项)。

> 说明:该 checker 当前在 `checker.go` 中未注册(构造函数被注释),节点上默认不会实际运行;此条为语义参考。

**6. 级别与升级** — Critical,cordon 节点尽快修;属节点侧配置项,通常无需升级硬件团队。

---

## check_ib_kmod — IBKernelModulesNotAllInstalled (Critical)

**1. 是什么** — 校验所有必需的 IB 内核模块是否都已安装/加载。

**2. 典型输出**

```
need to install kmod:<list>
```

**3. 常见根因**

- OFED/内核模块未装或版本不匹配。
- 内核升级后模块未重建(DKMS 失败)。
- 模块被 blacklist 或未 modprobe。

**4. 确认命令(只读)**

```bash
lsmod | grep -iE 'mlx5|ib_|rdma'
modinfo mlx5_core
ofed_info -s                                       # OFED 版本
```

**5. 处置**

- 节点侧可自愈/重配:`modprobe <mod>` 加载缺失模块;OFED 与内核不匹配 → 重装/重建对应 OFED 内核模块。

**6. 级别与升级** — Critical,cordon 节点尽快修;属软件层,节点侧修复,一般无需硬件团队。

---

## check_ib_rail_count — IBRailCountOdd (Critical)

**1. 是什么** — 校验算力轨 HCA 数量是否合理(偶数,或恰好单轨);奇数通常意味着有 HCA 掉出 RDMA 栈。

**2. 典型输出**

```
Odd compute-rail HCA count: 7 device(s) at 200 Gb/sec (mlx5_0,mlx5_1,mlx5_2,mlx5_3,mlx5_4,mlx5_6,mlx5_7). Rail-symmetric nodes expose an even number of compute HCAs (or exactly one), so one appears to have vanished from the RDMA stack
```

**3. 常见根因**

- 某算力口 HCA 掉出 RDMA 栈(mlx5_core probe 失败/掉卡),导致奇数。
- 该口协商到非最高速率档,被排除出算力轨计数。
- 节点拓扑本身与预期不符(需核对该机型期望轨数)。

**4. 确认命令(只读)**

```bash
ibstat -l                                          # 列当前 RDMA 设备
for d in /sys/class/infiniband/*/ports/1/rate; do echo "$d: $(cat $d)"; done
dmesg | grep -i mlx5_core                          # 找 probe 失败
```

**5. 处置**

- 节点侧可自愈/重配:核对节点期望拓扑,复测确认是否偶发。
- 该换线换卡(升级硬件团队):确认某 HCA 掉出 RDMA 栈 → 按 check_ib_lost 流程处理(软复位/换卡),升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;确属掉卡则升级硬件团队。

---

## check_ib_mezz_name — IBMezzNameMismatch (Critical)

**1. 是什么** — 校验每张 mezz 卡(board_id NVD0000000079)的 RDMA 设备是否已被命名为 `mezz_<k>`。

**2. 典型输出**

```
mlx5_2 port 1 ==> eth2 (Ethernet)  [expected mezz_0]
```

**3. 常见根因**

- rdma-env-pre 的接口命名步骤未在该节点执行,mezz 卡未被改名。
- 命名脚本失败或顺序错乱。

**4. 确认命令(只读)**

```bash
ibstat -l                                          # 看设备名是否为 mezz_<k>
ibdev2netdev                                       # RDMA 设备 ↔ netdev 映射
cat /sys/class/infiniband/<dev>/board_id           # 确认 NVD0000000079
```

**5. 处置**

- 节点侧可自愈/重配:确认并重跑 rdma-env-pre 的 interface-naming 步骤,使 mezz 卡改名为 `mezz_<k>`。

> 注意:改名故障可能连带让常规 checker 因设备名不符而全不跑,需优先修复命名。

**6. 级别与升级** — Critical,cordon 节点尽快修;属节点侧命名配置,通常无需硬件团队。

---

## check_ib_ofed — OFEDVersionMismatch (Warning)

**1. 是什么** — 校验已安装的 OFED 版本是否与 spec 一致。

**2. 典型输出**

```
OFED version mismatch, expected:<spec>  current:<curr>
```

**3. 常见根因**

- 节点 OFED 版本与该集群基线不一致(未升级/回退)。
- 重装系统后装了不同版本 OFED。

**4. 确认命令(只读)**

```bash
ofed_info -s
modinfo mlx5_core | grep -i version
```

**5. 处置**

- 节点侧可自愈/重配:按 spec 升级或重装 OFED 到目标版本。

**6. 级别与升级** — Warning,cordon 节点择期修;属软件层,节点侧修复,无需硬件团队。

---

## check_ib_fw — IBFirmwareVersionMismatch (Warning)

**1. 是什么** — 校验 HCA 固件版本是否与 spec 一致。

**2. 典型输出**

```
fw check fail: hca:mlx5_0 psid:MT_0000000838 curr:<fw>, spec:<spec>
```

**3. 常见根因**

- 固件版本落后于集群基线(未刷新)。
- 混卡/换卡后固件版本不齐。

**4. 确认命令(只读)**

```bash
ibstat <device> | grep -i firmware
flint -d <pci> q                                   # 查固件版本与 PSID
cat /sys/class/infiniband/<dev>/fw_ver
```

**5. 处置**

- 节点侧可自愈/重配:按 spec 用 `flint`/`mlxfwmanager` 刷新固件到目标版本(维护窗口内)。

**6. 级别与升级** — Warning,cordon 节点择期修;刷固件属节点侧维护,一般无需硬件团队。

---

## check_ib_devs — IBDeviceNameMismatch (Warning)

**1. 是什么** — 校验 IB 设备名(mlx5 ↔ ib 映射)是否与预期一致。

**2. 典型输出**

```
mlx5_0 (missing)
mlx5_1 -> ib3 (expected ib1)
（整体 Detail:Mismatched IB devices: <detailStr>. Current map: <cur>, Expected map: <exp>）
```

**3. 常见根因**

- udev/命名规则未生效或与预期不符。
- 设备枚举顺序变化导致 ib 名错位。
- 某设备缺失(missing,与掉卡相关)。

**4. 确认命令(只读)**

```bash
ibdev2netdev
ibstat -l
cat /etc/udev/rules.d/*  | grep -i mlx             # 核对命名规则
```

**5. 处置**

- 节点侧可自愈/重配:核对 udev/命名规则,修正映射;missing 场景先按 check_ib_lost 排查掉卡。

**6. 级别与升级** — Warning,cordon 节点择期修;命名问题节点侧修复,missing 若确属掉卡则升级硬件团队。

---

## check_roce — RoCENotEnabled (Warning)

**1. 是什么** — 校验 RoCE VF 是否已启用(RoCE 网卡配置是否就绪)。

**2. 典型输出**

```
RoCE checks failed: <detail>
（网关不可达时:gateway '<gw>' is unreachable. All checks failed. Last error: <err>）
```

**3. 常见根因**

- RoCE VF 未在设备配置中启用(VF 数不符)。
- 网关不可达(路由/交换机侧问题)。
- PFC/无损网络参数未配好。

**4. 确认命令(只读)**

```bash
cat /sys/class/infiniband/<dev>/ports/1/link_layer   # 应为 Ethernet
ibstat <device>
ip route get <gateway>                               # 网关可达性
ethtool -S <iface> | grep -i pfc                     # RoCE 真信号在 PFC
```

**5. 处置**

- 节点侧可自愈/重配:按设备配置启用 RoCE VF、核对无损/PFC 参数;网关不可达 → 联系网络团队核对路由与交换机侧配置。

**6. 级别与升级** — Warning,cordon 节点择期修;VF/参数节点侧修复,网关/路由问题联系网络团队。

---

## check_pcie_mrr — PCIEMRRIncorrect (Info)

**1. 是什么** — 校验 PCIe Max Read Request(MRR)是否设置正确(4096)。

**2. 典型输出**

```
PCIEMRR check fail: mlx5_0 expect [4096], but get [512]
```

**3. 常见根因**

- 系统未把 MRR 设为 4096(默认可能为 512)。
- 重启后 MRR 设置脚本未生效。

**4. 确认命令(只读)**

```bash
lspci -vvv -s <bdf> | grep -i MaxReadReq           # 看 MaxReadReq 字段
```

**5. 处置**

- 节点侧可自愈/重配:通过系统配置把 MRR 设为 4096(见 sichek 写操作审计,MRR 属节点侧可重配项)。

**6. 级别与升级** — info 级(提示性,**不 cordon** 节点);属性能优化项,节点侧择机调整,无需升级硬件团队。
