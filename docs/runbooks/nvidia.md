# NVIDIA / GPU 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## GPU 状态检查

## hardware — GPULost (Fatal)

**1. 是什么** — 校验是否有 Nvidia GPU 掉卡(从总线/NVML 上消失)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=hardware component=nvidia`,ErrorName 为 `GPULost`(下为示意形状,非精确字符串)。

```
checker=hardware component=nvidia  ErrorName=GPULost  Level=Fatal
```

**3. 常见根因**

- GPU 从 PCIe 总线掉出(常伴随 Xid 79 "GPU has fallen off the bus")。
- 供电/接触不良、板卡硬件故障。
- 驱动/固件异常导致设备不再枚举。

**4. 确认命令(只读)**

```bash
nvidia-smi                                        # 数 GPU 是否齐
lspci -d 10de: -nn                                # PCI 侧 NVIDIA 设备数量
dmesg -T | grep -i xid                            # 找 Xid 79(fallen off the bus)
nvidia-smi -q | grep -iE 'GPU UUID|Bus Id'
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Coldreset the system` 对系统冷复位(断电冷启),复位后复测 GPU 是否回来。
- 该换卡(升级硬件团队):冷复位后 GPU 仍缺失、总线上确实少卡 → 升级硬件团队换卡。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon** 节点;冷复位仍不回卡即判硬件,升级硬件团队换卡。

---

## nvlink — NvlinkNotActive (Fatal)

**1. 是什么** — 校验所有 Nvidia GPU 的 NVLink 是否均处于 active 状态。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=nvlink component=nvidia`,ErrorName 为 `NvlinkNotActive`(下为示意形状,非精确字符串)。

```
checker=nvlink component=nvidia  ErrorName=NvlinkNotActive  Level=Fatal
```

**3. 常见根因**

- 某条 NVLink 未训练起来/降级(GPU 或 NVSwitch 侧链路问题)。
- nvidia-fabricmanager 未正常拉起 NVLink fabric。
- GPU 硬件故障导致部分 link inactive。

**4. 确认命令(只读)**

```bash
nvidia-smi nvlink -s                              # 各 link 状态/速率
nvidia-smi nvlink -e                              # NVLink 错误计数
systemctl status nvidia-fabricmanager             # fabric 是否 active
nvidia-smi topo -m                                # 拓扑/NVLink 连接
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reboot the system` 重启系统,让 fabric/NVLink 重新初始化;先确认 nvidia-fabricmanager active。
- 该换卡(升级硬件团队):重启后仍有 link inactive、NVLink 错误持续 → 升级硬件团队排查 GPU/NVSwitch 换卡。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon** 节点;重启无效即判硬件,升级硬件团队。

---

## gpu-recovery-action — GpuRecoveryActionRequired (Critical)

**1. 是什么** — 校验是否有 Nvidia GPU 被驱动标记为需要恢复动作(reset/reboot)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=gpu-recovery-action component=nvidia`,ErrorName 为 `GpuRecoveryActionRequired`(下为示意形状,非精确字符串)。

```
checker=gpu-recovery-action component=nvidia  ErrorName=GpuRecoveryActionRequired  Level=Critical
```

**3. 常见根因**

- 该卡先前发生 Contained/Uncontained ECC(Xid 94/95)或其他致命错误,驱动置位 recovery required。
- GPU 进入 reset-required 状态,业务再用即报 "CUDA device busy or unavailable"。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N>                              # 看 GPU Recovery Action 字段
dmesg -T | grep -i xid                            # 关联 Xid 94→95→154 链
nvidia-smi -q -i <N> -d ECC
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion cordon 节点并对被标记的 GPU 执行 reset(或 reboot);复位后复测。
- 该换卡(升级硬件团队):复位后仍复现(recurs)→ 升级硬件团队换卡(`Replace the GPU device`)。

真机经验:某卡 Xid 94(Contained)→95(Uncontained FBHUB)→154(GPU Reset Required)后,nvidia-smi 看着全空闲但 `nccltest` 报 "CUDA device busy or unavailable";用 `nvidia-smi -q -i <N>` 看 GPU Recovery Action 字段确认。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效则 cold reset/reboot,再复现则换卡升级硬件团队。

---

## remmaped-rows-pending — RemmapedRowsPending (Critical)

**1. 是什么** — 校验是否有 Nvidia GPU 存在 remapped rows pending(行重映射待生效)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=remmaped-rows-pending component=nvidia`,ErrorName 为 `RemmapedRowsPending`(下为示意形状,非精确字符串)。

```
checker=remmaped-rows-pending component=nvidia  ErrorName=RemmapedRowsPending  Level=Critical
```

**3. 常见根因**

- 显存出现 ECC 错误后触发行重映射,尚需一次 GPU reset 才能生效。
- 常伴随 Xid 63(row remapping recording event)。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA5 'Remapped Rows'   # Pending: Yes
nvidia-smi -q -i <N> -d ROW_REMAPPER 2>/dev/null || true
dmesg -T | grep -i xid                                    # 关联 Xid 63
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset,使 pending 的行重映射生效后复测。
- 该换卡(升级硬件团队):reset 后 pending 反复出现或转为 failure → 升级硬件团队评估换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效或反复则升级硬件团队。

---

## remmaped-rows-failure — RemmapedRowsFailure (Critical)

**1. 是什么** — 校验是否有 Nvidia GPU 存在 remapped rows failure(行重映射失败)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=remmaped-rows-failure component=nvidia`,ErrorName 为 `RemmapedRowsFailure`(下为示意形状,非精确字符串)。

```
checker=remmaped-rows-failure component=nvidia  ErrorName=RemmapedRowsFailure  Level=Critical
```

**3. 常见根因**

- 行重映射记录失败,显存已无可用备用行或重映射机制本身故障。
- 常伴随 Xid 64(row remapper recording failure),属显存硬件劣化征兆。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA6 'Remapped Rows'   # Failure: Yes
dmesg -T | grep -i xid                                    # 关联 Xid 64
```

**5. 处置**

- 该换卡(升级硬件团队):按 Suggestion `Replace the GPU device`,行重映射失败通常意味着显存硬件劣化 → 直接升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;属显存硬件劣化,升级硬件团队换卡。

---

## ecc-sram-aggregate-uncorrectable — HighSRAMAggregateUncorrectableErrors (Critical)

**1. 是什么** — 校验是否有 Nvidia GPU 存在高 ECC SRAM aggregate(累计)不可纠正错误。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=ecc-sram-aggregate-uncorrectable component=nvidia`,ErrorName 为 `HighSRAMAggregateUncorrectableErrors`(下为示意形状,非精确字符串)。

```
checker=ecc-sram-aggregate-uncorrectable component=nvidia  ErrorName=HighSRAMAggregateUncorrectableErrors  Level=Critical
```

**3. 常见根因**

- SRAM 累计不可纠正 ECC 错误超阈值,反映 GPU 片上 SRAM 硬件劣化。
- 长期运行累计,属持久性硬件征兆(aggregate 不因重启清零)。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA20 'Aggregate'      # SRAM Uncorrectable
nvidia-smi -q -i <N> -d ECC | grep -i sram
```

**5. 处置**

- 该换卡(升级硬件团队):按 Suggestion `Replace the GPU device`,SRAM 累计不可纠正错误高 → 升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;属硬件劣化,升级硬件团队换卡。

---

## clock-events — ClockThrottleEvent (Critical)

**1. 是什么** — 校验是否有任一 Nvidia GPU 触发了 Critical 级别的 clock(时钟降频)事件。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=clock-events component=nvidia`,ErrorName 为 `ClockThrottleEvent`(下为示意形状,非精确字符串)。

```
checker=clock-events component=nvidia  ErrorName=ClockThrottleEvent  Level=Critical
```

**3. 常见根因**

- HW Slowdown / HW Thermal Slowdown 等关键降频事件触发(过热、供电异常、掉压)。
- 电源/散热硬件故障导致 GPU 强制降频保护。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d CLOCK                     # Clocks Throttle Reasons
nvidia-smi -q -i <N> -d PERFORMANCE
nvidia-smi -q -i <N> -d TEMPERATURE
```

**5. 处置**

- 节点侧可自愈/重配:核对散热(风道/进风温度)与供电,排除环境因素后复测。
- 该换卡(升级硬件团队):按 Suggestion `Diagnostic the GPU for hardware issue`,确认为 GPU 侧硬件问题 → 升级硬件团队诊断/换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;确属 GPU 硬件问题则升级硬件团队。

---

## nvidia-fabricmanager — NvidiaFabricManagerNotActive (Critical)

**1. 是什么** — 校验 nvidia-fabricmanager 服务是否处于 active 状态。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=nvidia-fabricmanager component=nvidia`,ErrorName 为 `NvidiaFabricManagerNotActive`(下为示意形状,非精确字符串)。

```
checker=nvidia-fabricmanager component=nvidia  ErrorName=NvidiaFabricManagerNotActive  Level=Critical
```

**3. 常见根因**

- fabricmanager 服务未启动或崩溃(NVSwitch 机型必需)。
- 驱动与 fabricmanager 版本不匹配导致启动失败。
- 缺失 fabricmanager 会导致 NVLink fabric 不可用(连带 nvlink 检查失败)。

**4. 确认命令(只读)**

```bash
systemctl status nvidia-fabricmanager
journalctl -u nvidia-fabricmanager --no-pager | tail -50
cat /var/log/fabricmanager.log 2>/dev/null | tail -50
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `run systemctl restart nvidia-fabricmanager`(理想情况会在线自动完成);排查版本匹配。
- 该换卡(升级硬件团队):服务反复起不来且伴随 NVSwitch/NVLink 硬件错误 → 升级硬件团队。

**6. 级别与升级** — Critical:cordon 节点尽快修;属服务层,重启修复,反复失败再排查硬件/版本。

---

## nvidia_peermem — NvidiaPeerMemNotLoaded (Critical)

**1. 是什么** — 校验 nvidia_peermem 内核模块是否已加载(GPU Direct RDMA 依赖)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=nvidia_peermem component=nvidia`,ErrorName 为 `NvidiaPeerMemNotLoaded`(下为示意形状,非精确字符串)。

```
checker=nvidia_peermem component=nvidia  ErrorName=NvidiaPeerMemNotLoaded  Level=Critical
```

**3. 常见根因**

- 系统未 modprobe nvidia_peermem(默认可能未加载)。
- 重启后加载脚本未生效。
- 驱动安装不完整导致模块缺失。

**4. 确认命令(只读)**

```bash
lsmod | grep nvidia_peermem
modinfo nvidia_peermem | grep -i version
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `run modprobe nvidia_peermem` 加载模块(理想情况会在线自动完成)。

**6. 级别与升级** — Critical:cordon 节点尽快修;属软件层,节点侧 modprobe 修复,一般无需硬件团队。

---

## ibgda — IBGDANotEnabled (Critical)

**1. 是什么** — 校验 IBGDA(GPUDirect Async)相关设置是否已启用。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=ibgda component=nvidia`,ErrorName 为 `IBGDANotEnabled`(下为示意形状,非精确字符串)。

```
checker=ibgda component=nvidia  ErrorName=IBGDANotEnabled  Level=Critical
```

**3. 常见根因**

- 驱动未配置 `EnableStreamMemOPs=1;PeerMappingOverride=1` 相关 RegistryDwords。
- 修改了 modprobe 配置但未 reboot 生效。

**4. 确认命令(只读)**

```bash
cat /proc/driver/nvidia/params | grep -iE 'StreamMemOP|PeerMapping'
cat /etc/modprobe.d/nvidia.conf 2>/dev/null | grep -i RegistryDwords
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion 在 `/etc/modprobe.d/nvidia.conf` 加入 `options nvidia NVreg_RegistryDwords="EnableStreamMemOPs=1;PeerMappingOverride=1"` 并 reboot 系统。

> 说明:仅在满足 compute capability 要求(major 9-11)的 GPU 上启用该检查,不满足时会被跳过。

**6. 级别与升级** — Critical:cordon 节点尽快修;属配置项,节点侧改 modprobe + reboot 修复,无需硬件团队。

---

## pcie-acs — PCIeACSNotClosed (Critical)

**1. 是什么** — 校验 PCIe ACS(Access Control Services)是否已关闭(ACS 开启会破坏 GPU Direct RDMA 的 peer-to-peer 路径)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=pcie-acs component=nvidia`,ErrorName 为 `PCIeACSNotClosed`(下为示意形状,非精确字符串)。

```
checker=pcie-acs component=nvidia  ErrorName=PCIeACSNotClosed  Level=Critical
```

**3. 常见根因**

- BIOS 或内核未关闭 ACS(默认可能开启)。
- 节点重装/重启后 ACS 关闭脚本未生效。

**4. 确认命令(只读)**

```bash
lspci -vvv | grep -i ACSCtl                       # 看 SrcValid+ 等是否开启
for i in $(lspci | cut -f 1 -d " "); do setpci -v -s $i ECAP_ACS+6.w; done  # 读 ACS 控制寄存器
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `run for i in $(lspci | cut -f 1 -d " ");do setpci -v -s $i ecap_acs+6.w=0;done` 关闭 ACS(理想情况会在线自动完成)。

**6. 级别与升级** — Critical:cordon 节点尽快修;属节点侧配置项,通常无需升级硬件团队。

---

## iommu — IOMMUNotClosed (Critical)

**1. 是什么** — 校验 IOMMU 是否已关闭(开启会影响 GPU Direct RDMA / P2P 性能与路径)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=iommu component=nvidia`,ErrorName 为 `IOMMUNotClosed`(下为示意形状,非精确字符串)。

```
checker=iommu component=nvidia  ErrorName=IOMMUNotClosed  Level=Critical
```

**3. 常见根因**

- GRUB 内核参数未关闭 IOMMU(缺 `iommu=off`)。
- BIOS 中 VT-d/AMD-Vi 开启且内核未旁路。

**4. 确认命令(只读)**

```bash
cat /proc/cmdline | grep -iE 'iommu|intel_iommu|amd_iommu'
dmesg | grep -iE 'IOMMU|DMAR'
ls /sys/class/iommu/
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion 编辑 `/etc/default/grub`,在 `GRUB_CMDLINE_LINUX_DEFAULT` 行加 `iommu=off`,更新 grub 后 reboot 系统。

**6. 级别与升级** — Critical:cordon 节点尽快修;属节点侧内核参数配置,reboot 生效,无需硬件团队。

---

## remmaped-rows-high-uncorrectable — HighRemmapedRowsUncorrectableErrors (Warning)

**1. 是什么** — 校验是否有 Nvidia GPU 存在较高的 remapped rows uncorrectable(行重映射不可纠正)错误。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=remmaped-rows-high-uncorrectable component=nvidia`,ErrorName 为 `HighRemmapedRowsUncorrectableErrors`(下为示意形状,非精确字符串)。

```
checker=remmaped-rows-high-uncorrectable component=nvidia  ErrorName=HighRemmapedRowsUncorrectableErrors  Level=Warning
```

**3. 常见根因**

- 显存不可纠正 ECC 触发的行重映射数量偏高,显存开始劣化。
- 尚未到 failure,但趋势值得关注。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA8 'Remapped Rows'
nvidia-smi -q -i <N> -d ECC | grep -i uncorrectable
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Diagnostic the GPU for hardware issue`,先观察趋势并诊断。
- 该换卡(升级硬件团队):数量持续升高或转为 pending/failure → 升级硬件团队评估换卡。

**6. 级别与升级** — Warning:cordon 节点择期修;持续劣化则升级硬件团队。

---

## ecc-sram-volatile-uncorrectable — SRAMVolatileUncorrectableErrors (Warning)

**1. 是什么** — 校验是否有 Nvidia GPU 存在 ECC SRAM volatile(易失,本次运行内)不可纠正错误。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=ecc-sram-volatile-uncorrectable component=nvidia`,ErrorName 为 `SRAMVolatileUncorrectableErrors`(下为示意形状,非精确字符串)。

```
checker=ecc-sram-volatile-uncorrectable component=nvidia  ErrorName=SRAMVolatileUncorrectableErrors  Level=Warning
```

**3. 常见根因**

- 本次运行内 SRAM 出现不可纠正 ECC(volatile 计数,重启会清零)。
- 偶发可能为软错误,持续则指向硬件劣化。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA20 'Volatile'       # SRAM Uncorrectable
nvidia-smi -q -i <N> -d ECC | grep -i sram
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset(可连带清 volatile 计数),复位后观察。
- 该换卡(升级硬件团队):reset 后仍反复出现或叠加 aggregate 升高 → 升级硬件团队评估换卡。

**6. 级别与升级** — Warning:cordon 节点择期修;reset 无效且反复则升级硬件团队。

---

## ecc-sram-high-correctable — HighSRAMCorrectableErrors (Warning)

**1. 是什么** — 校验是否有 Nvidia GPU 存在较高的 ECC SRAM correctable(可纠正)错误。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=ecc-sram-high-correctable component=nvidia`,ErrorName 为 `HighSRAMCorrectableErrors`(下为示意形状,非精确字符串)。

```
checker=ecc-sram-high-correctable component=nvidia  ErrorName=HighSRAMCorrectableErrors  Level=Warning
```

**3. 常见根因**

- SRAM 可纠正 ECC 计数偏高,虽被纠正但反映片上 SRAM 质量下降。
- 可纠正错误率上升常是不可纠正错误的先兆。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d ECC | grep -iA20 'Correctable'
nvidia-smi -q -i <N> -d ECC | grep -i sram
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Diagnostic the GPU for hardware issue`,先诊断并观察可纠正错误率趋势。
- 该换卡(升级硬件团队):可纠正率持续升高或开始出现不可纠正错误 → 升级硬件团队评估换卡。

**6. 级别与升级** — Warning:cordon 节点择期修;劣化趋势明显则升级硬件团队。

---

## pcie — PCIeLinkDegraded (Warning)

**1. 是什么** — 校验是否有 GPU 的 PCIe 链路降级(性能降级的指示器)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=pcie component=nvidia`,ErrorName 为 `PCIeLinkDegraded`(下为示意形状,非精确字符串)。

```
checker=pcie component=nvidia  ErrorName=PCIeLinkDegraded  Level=Warning
```

**3. 常见根因**

- GPU PCIe 链路协商速率/宽度低于额定(降速/降宽)。
- 板卡/插槽接触不良、PCIe 信号质量差。
- 主机拓扑/BIOS 分叉配置导致链路受限。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> | grep -iA6 'GPU Link Info'          # 当前/最大 Gen 与 Width
lspci -vvv -s <bdf> | grep -iE 'LnkSta|LnkCap'
lspci -tv
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reboot the system` 重启让链路重训;核对是否为拓扑固有低速。
- 该换卡(升级硬件团队):重启后仍降级、确属硬件降级 → 重插/换卡,升级硬件团队。

**6. 级别与升级** — Warning:cordon 节点择期修;确属硬件降级则升级硬件团队。

---

## temperature — HighTemperature (Warning)

**1. 是什么** — 校验 GPU 温度是否超过指定阈值(如 75°C,性能降级的指示器)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=temperature component=nvidia`,ErrorName 为 `HighTemperature`(下为示意形状,非精确字符串)。

```
checker=temperature component=nvidia  ErrorName=HighTemperature  Level=Warning
```

**3. 常见根因**

- 散热不足(风道堵塞、进风温度高、风扇故障)。
- GPU 满载 + 环境温度偏高。
- 导热/散热硬件劣化。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d TEMPERATURE               # GPU/Memory 当前温度与阈值
nvidia-smi --query-gpu=temperature.gpu --format=csv
nvidia-smi -q -i <N> -d CLOCK                      # 是否已因过热降频
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Observing the performance of application`,先观察应用性能;核对散热/风道/进风温度。
- 该换卡(升级硬件团队):排除环境因素后仍高温 → 升级硬件团队检查散热硬件。

> 说明:该 checker 当前在 `checker.go` 中未注册,默认不会实际运行。

**6. 级别与升级** — Warning:cordon 节点择期修;环境排除后仍高温则升级硬件团队。

---

## pstate — GPUStateNotMaxPerformance (Warning)

**1. 是什么** — 校验 Nvidia GPU 的 performance state 是否处于 P0(最大性能)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=pstate component=nvidia`,ErrorName 为 `GPUStateNotMaxPerformance`(下为示意形状,非精确字符串)。

```
checker=pstate component=nvidia  ErrorName=GPUStateNotMaxPerformance  Level=Warning
```

**3. 常见根因**

- GPU 停在较低 P-state(未跑到 P0),影响性能。
- 持久化模式/电源管理状态异常。
- 偶发卡在非 P0(可通过 reset 恢复)。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d PERFORMANCE               # Performance State: P0..
nvidia-smi --query-gpu=pstate --format=csv
nvidia-smi -q -i <N> -d CLOCK
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset GPU` 对该卡 reset,复位后确认回到 P0。
- 该换卡(升级硬件团队):reset 后仍无法进入 P0 且伴随其他硬件征兆 → 升级硬件团队。

**6. 级别与升级** — Warning:cordon 节点择期修;reset 修复,反复异常再排查硬件。

---

## app-clocks — AppClocksNotMax (Warning)

**1. 是什么** — 校验所有 Nvidia GPU 的 application clocks 是否已设为最大。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=app-clocks component=nvidia`,ErrorName 为 `AppClocksNotMax`(下为示意形状,非精确字符串)。

```
checker=app-clocks component=nvidia  ErrorName=AppClocksNotMax  Level=Warning
```

**3. 常见根因**

- application clocks 未锁定到最大值(默认或被改低)。
- 重启后设置 application clocks 的脚本未生效。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> -d CLOCK                      # Applications Clocks vs Max
nvidia-smi --query-gpu=clocks.applications.gr,clocks.max.gr --format=csv
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `run nvidia-smi -rac` 把 application clocks 设为最大(理想情况会在线自动完成)。

**6. 级别与升级** — Warning:cordon 节点择期修;属节点侧配置,`nvidia-smi -rac` 修复,无需硬件团队。

---

## persistenced — GPUPersistencedModeNotEnabled (Warning)

**1. 是什么** — 校验 Nvidia GPU 的 persistence 模式是否已启用并正常工作。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=persistenced component=nvidia`,ErrorName 为 `GPUPersistencedModeNotEnabled`(下为示意形状,非精确字符串)。

```
checker=persistenced component=nvidia  ErrorName=GPUPersistencedModeNotEnabled  Level=Warning
```

**3. 常见根因**

- nvidia-persistenced 未启动(persistence 模式关闭)。
- 重启后 persistenced 未随系统拉起。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N> | grep -i 'Persistence Mode'
systemctl status nvidia-persistenced
nvidia-smi --query-gpu=persistence_mode --format=csv
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `run nvidia-persistenced` 自动启用 persistence 模式(理想情况会在线自动完成)。

**6. 级别与升级** — Warning:cordon 节点择期修;属节点侧配置,启动 persistenced 修复,无需硬件团队。

---

## software — SoftwareVersionIncorrect (Warning)

**1. 是什么** — 校验各相关软件版本是否与预期一致。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=software component=nvidia`,ErrorName 为 `SoftwareVersionIncorrect`(下为示意形状,非精确字符串)。

```
checker=software component=nvidia  ErrorName=SoftwareVersionIncorrect  Level=Warning
```

**3. 常见根因**

- 驱动 / CUDA / 相关组件版本与集群基线不一致。
- 重装系统后装了不同版本软件。

**4. 确认命令(只读)**

```bash
nvidia-smi                                         # Driver Version / CUDA Version
cat /proc/driver/nvidia/version
nvidia-smi --query-gpu=driver_version --format=csv
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Update the software to the expected version`,把相关软件升级/回退到 spec 期望版本。

**6. 级别与升级** — Warning:cordon 节点择期修;属软件层,节点侧升级修复,无需硬件团队。

---

## p2p_topo — P2PNotSupported (Warning)

**1. 是什么** — 校验 GPU Peer-to-Peer(P2P)读能力是否可用。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=p2p_topo component=nvidia`,ErrorName 为 `P2PNotSupported`(下为示意形状,非精确字符串)。

```
checker=p2p_topo component=nvidia  ErrorName=P2PNotSupported  Level=Warning
```

**3. 常见根因**

- NVLink 连接异常或未建立,P2P 路径不通。
- PCIe 拓扑上 ACS 开启破坏了 P2P 路径。
- 拓扑/桥接配置导致某对 GPU 不支持 P2P。

**4. 确认命令(只读)**

```bash
nvidia-smi topo -m                                 # 看 GPU 间连接矩阵(NV#/PIX/SYS)
nvidia-smi nvlink -s                               # NVLink 状态
lspci -vvv | grep -i ACSCtl                         # ACS 是否破坏 P2P
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Check NVLink connections or PCIe topology settings (ACS)`,核对 NVLink 连接与 PCIe ACS 设置(ACS 关闭见 pcie-acs 条目)。
- 该换卡(升级硬件团队):NVLink 物理连接确属硬件故障 → 升级硬件团队。

**6. 级别与升级** — Warning:cordon 节点择期修;配置问题节点侧修复,NVLink 硬件故障则升级硬件团队。

---

## Xid 事件(dmesg / 驱动日志)

Xid 是 NVIDIA 驱动打到 dmesg 的错误码;sichek 的 dmesg/hang 检查会捕获关键 Xid。典型形状形如 `NVRM: Xid (PCI:0000:xx:00): <NN>, ...`,下列条目按严重度排列。

---

## xid-79 — xid79-GPULost (Fatal)

**1. 是什么** — GPU 从总线掉出(GPU has fallen off the bus)。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 79, GPU has fallen off the bus.
```

**3. 常见根因**

- GPU 从 PCIe 总线掉出(供电/接触/板卡硬件故障)。
- 严重硬件故障导致设备不再响应(与 hardware/GPULost 同源)。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 79, fallen off the bus
lspci -d 10de: -nn                                 # 数 NVIDIA 设备是否齐
nvidia-smi                                         # 看该卡是否消失
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Coldreset the system` 对系统冷复位(断电冷启),复位后复测。
- 该换卡(升级硬件团队):冷复位后 GPU 仍缺失 → 升级硬件团队换卡。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon** 节点;冷复位仍不回卡即判硬件,升级硬件团队换卡。

---

## xid-31 — xid31-GPUMemoryPageFault (Critical)

**1. 是什么** — GPU 显存 page fault(内存页错误)。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 31, Ch ..., MMU Fault ...
```

**3. 常见根因**

- 业务代码非法显存访问(越界/已释放指针等),多为软件问题。
- 少数情况由显存硬件问题触发。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 31 及涉及的 channel/地址
nvidia-smi -q -i <N> -d ECC                        # 排除显存 ECC 因素
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device`;并提醒检查业务代码是否存在非法内存访问操作(Xid 31 多为业务侧问题)。
- 该换卡(升级硬件团队):排除业务代码后仍反复且伴随显存 ECC 征兆 → 升级硬件团队。

**6. 级别与升级** — Critical:cordon 节点尽快修;先排查业务代码,硬件征兆明确再升级硬件团队。

---

## xid-48 — xid48-GPUMemoryDBE (Critical)

**1. 是什么** — DBE(Double Bit Error)双比特 ECC 错误。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 48, Double Bit ECC Error ...
```

**3. 常见根因**

- 显存出现不可纠正的双比特 ECC 错误,属显存硬件劣化。
- 常连带触发行重映射(Xid 63/64)。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 48
nvidia-smi -q -i <N> -d ECC | grep -iA10 'Uncorrectable'
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset,复位后观察是否复现。
- 该换卡(升级硬件团队):reset 后仍反复出现 DBE → 升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效且反复则升级硬件团队换卡。

---

## xid-63 — xid63-ECCRowremapperPending (Critical)

**1. 是什么** — ECC 页退役 / 行重映射记录事件(pending)。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 63, Row Remapper: ...
```

**3. 常见根因**

- 显存 ECC 错误触发行重映射记录,需一次 reset 生效(对应 remmaped-rows-pending)。
- 显存出现可修复的坏行。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 63
nvidia-smi -q -i <N> -d ECC | grep -iA6 'Remapped Rows'   # Pending: Yes
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset 使行重映射生效,复位后复测。
- 该换卡(升级硬件团队):reset 后反复出现或转为 failure(Xid 64)→ 升级硬件团队。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效或反复则升级硬件团队。

---

## xid-64 — xid64-ECCRowremapperFailure (Critical)

**1. 是什么** — ECC 页退役 / 行重映射记录失败。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 64, Row Remapper: ... failure ...
```

**3. 常见根因**

- 行重映射记录失败(无可用备用行或重映射机制故障),属显存硬件劣化(对应 remmaped-rows-failure)。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 64
nvidia-smi -q -i <N> -d ECC | grep -iA6 'Remapped Rows'   # Failure: Yes
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 先 reset 尝试;若失败态持续则不可自愈。
- 该换卡(升级硬件团队):行重映射失败通常意味着显存硬件劣化 → 升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效即判显存硬件劣化,升级硬件团队换卡。

---

## xid-74 — xid74-NVLinkError (Critical)

**1. 是什么** — NVLink 错误。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 74, NVLink: ...
```

**3. 常见根因**

- NVLink 链路错误(GPU 或 NVSwitch 侧链路/信号问题)。
- fabricmanager/NVLink fabric 异常(连带 nvlink 检查失败)。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 74
nvidia-smi nvlink -e                               # NVLink 错误计数
nvidia-smi nvlink -s
systemctl status nvidia-fabricmanager
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset;必要时重启使 NVLink fabric 重新初始化。
- 该换卡(升级硬件团队):reset/重启后 NVLink 错误持续 → 升级硬件团队排查 GPU/NVSwitch 换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效且错误持续则升级硬件团队。

---

## xid-92 — xid92-HighSingleBitECCErrorRate (Critical)

**1. 是什么** — 高单比特 ECC 错误率。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 92, High single-bit ECC error rate ...
```

**3. 常见根因**

- 单比特(可纠正)ECC 错误率过高,虽被纠正但反映显存质量下降。
- 单比特率高常是不可纠正错误的先兆。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 92
nvidia-smi -q -i <N> -d ECC | grep -iA10 'Correctable'
```

**5. 处置**

- 该换卡(升级硬件团队):按 Suggestion `Replace the GPU device`,高单比特 ECC 错误率意味着显存劣化 → 升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;属显存劣化,升级硬件团队换卡。

---

## xid-94 — xid94-ContainedECCError (Critical)

**1. 是什么** — Contained ECC error(已被隔离/收容的 ECC 错误)。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 94, Contained: ...
```

**3. 常见根因**

- 显存出现不可纠正 ECC 但被驱动隔离(contained),影响单个应用。
- 常是 Uncontained(Xid 95)的前置信号。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 94(常后接 95、154)
nvidia-smi -q -i <N> | grep -i 'Recovery Action'   # 是否置位 recovery
nvidia-smi -q -i <N> -d ECC
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion `Reset the GPU device` 对该卡 reset,复位后观察是否升级为 Uncontained。
- 该换卡(升级硬件团队):reset 后反复或升级为 Xid 95(Uncontained)→ 升级硬件团队换卡。

**6. 级别与升级** — Critical:cordon 节点尽快修;reset 无效或升级为 Uncontained 则升级硬件团队换卡。

---

## xid-95 — xid95-UncontainedECCError (Critical)

**1. 是什么** — Uncontained ECC error(未能收容的 ECC 错误)。

**2. 典型输出**

```
NVRM: Xid (PCI:0000:xx:00): 95, Uncontained: ...
```

**3. 常见根因**

- 显存不可纠正 ECC 未能被隔离,影响面扩大(如 FBHUB),常连带 Xid 94 前置与 154(GPU Reset Required)。
- 属显存硬件严重劣化。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找 94→95→154 链
nvidia-smi -q -i <N> | grep -i 'Recovery Action'   # 确认 reset-required
nvidia-smi -q -i <N> -d ECC
```

**5. 处置**

- 该换卡(升级硬件团队):按 Suggestion `Replace the GPU device`,Uncontained ECC 意味着显存严重劣化 → 升级硬件团队换卡。

真机经验:某卡 Xid 94(Contained)→95(Uncontained FBHUB)→154(GPU Reset Required)后,nvidia-smi 看着全空闲但 `nccltest` 报 "CUDA device busy or unavailable";用 `nvidia-smi -q -i <N>` 看 GPU Recovery Action 字段确认。

**6. 级别与升级** — Critical:cordon 节点尽快修;属显存严重劣化,升级硬件团队换卡。
