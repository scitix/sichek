# PCIe 拓扑排查

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

本页对应 `pcie_topo_check` 伪组件(CLI 命令 `sichek topo`),是一次性的 PCIe **拓扑关系**校验:把本机实际的 GPU/IB 在 NUMA 节点、PCIe Switch 上的挂载关系,与按 GPU 型号索引的 spec 基线逐项比对,不判定链路速率/宽度。它没有独立的 daemon `checker/` 目录,只能作为一次性 CLI 子命令或随 `sichek all` 的 `pcie_topo` 子检查触发,不会常驻轮询。

> 若要排查**单张 NIC 的 PCIe 链路速率/宽度/MRR/ACS** 降级问题(如 `check_pcie_tree_speed`/`check_pcie_tree_width`/`check_pcie_mrr`/`check_pcie_acs`),请见 [infiniband.md](infiniband.md),这些 checker 挂在 infiniband 组件下,不在本页范围内。

---

## PciTopoNumaCheckerName — NumaDeviceRelationError (Critical)

**1. 是什么** — 校验各 NUMA 节点下实际挂载的 GPU/IB 设备数量,是否与该 GPU 型号对应的 spec 基线(`numa_config`)一致;不一致意味着设备挂在了错误的 NUMA 节点或存在掉卡,会导致跨 NUMA 访问、带宽/延迟劣化。

**2. 典型输出**

```
（示意,checker=PciTopoNumaCheckerName ErrorName=NumaDeviceRelationError)
NUMA node 0 GPU count mismatch: expected 4, got 3
NUMA node 1 missing in actual data
unexpected NUMA node 2 found
```

**3. 常见根因**

- 某 NUMA 节点下 GPU/IB 掉卡(与 check_ib_lost 同源,设备数对不上)。
- spec 基线按 GPU deviceID 索引、不区分网卡配置:同型号 GPU 但 IB 配置不同的机器共用一条基线时会误报。
- BIOS/插槽调整后实际 NUMA 拓扑变化,而 spec 基线未同步更新。

**4. 确认命令(只读)**

```bash
sichek topo -s <spec.yaml> -v                          # dump 完整拓扑树:BDF/NUMA/domain
nvidia-smi topo -m                                     # GPU 间与 NUMA 亲和性
lspci -tv                                              # PCIe 树拓扑
for f in /sys/bus/pci/devices/*/numa_node; do echo "$f: $(cat $f)"; done
```

**5. 处置**

- 节点侧可自愈/重配:核对 spec 基线是否对应本机真实 GPU 型号与 IB 配置;确系基线过期或型号误判 → 联系配置中心核对/更新 spec,不要盲目判故障。
- 该换线换卡(升级硬件团队):结合 PCI 扫描/`check_ib_lost` 确认某 NUMA 下确有 GPU/IB 掉卡 → 按掉卡流程处理(复位/换卡),升级硬件团队。

**6. 级别与升级** — Critical,cordon 节点尽快修;确认是真掉卡才升级硬件团队,配置/基线类误报联系配置中心处理。

---

## PciTopoSwitchCheckerName — SwitchDeviceRelationError (Critical)

**1. 是什么** — 校验 GPU 与 IB 网卡是否挂载在预期的 PCIe Switch 分组下(按"最低公共 switch"聚合出的 `gpu_X&&ib_Y` 形态分布是否与 spec 一致);不一致即插槽错位,GPU 与其配对 NIC 不在同一 switch 下,会损害 GPUDirect RDMA 的 P2P 路径亲和性。

**2. 典型输出**

```
（示意,checker=PciTopoSwitchCheckerName ErrorName=SwitchDeviceRelationError)
switch configuration mismatch.
Expected: map[gpu_2&&ib_1:4 gpu_1&&ib_1:4]  Actual: map[gpu_2&&ib_1:3 gpu_1&&ib_1:5 gpu_1&&ib_0:1]
```

**3. 常见根因**

- GPU 或 IB 卡插错槽位,导致其归属的最低公共 PCIe switch 发生变化(常见于上架/维修后插回错误插槽)。
- 扩容/换卡后未按标准拓扑走线,原有的 GPU-IB 配对关系被打乱。
- spec 基线与实际机型不符(型号误判或基线过期)。

**4. 确认命令(只读)**

```bash
sichek topo -s <spec.yaml> -v                          # dump 拓扑树,按 PCIe Switch 分组打印 GPU/IB
lspci -tv                                              # 确认每张卡挂在哪个 PCIe bridge 下
nvidia-smi topo -m                                     # GPU 间 PIX/PXB/NODE 等级,判断是否同 switch
```

**5. 处置**

- 节点侧可自愈/重配:核对 spec 基线是否匹配本机机型;确系基线过期或型号误判 → 联系配置中心更新基线,不要盲目动硬件。
- 该换线换卡(升级硬件团队):结合 `-v` 拓扑树定位具体 BDF,确认是插槽错位 → 按标准拓扑图重新插接 GPU/IB;无法现场调整或涉及扩容换卡场景则升级硬件团队处理。

**6. 级别与升级** — Critical,cordon 节点尽快修;插槽错位可现场重插修复,涉及扩容/换卡场景需硬件团队配合。
