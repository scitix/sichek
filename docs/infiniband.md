# Infiniband/Ethernet Check

## Introduction
The efficiency and stability of high-speed networks are paramount. To ensure optimal performance and reliable data transmission, it is essential that all components of the network infrastructure operate seamlessly. This component has been developed to perform comprehensive detection and monitoring of high-speed networks, ensuring they function at their best.

The detection processes are categorized into three main areas: *hardware*, *software stack*, *stem configuration*.

## Dectect Event
### PHY_STATUS
- Description: Ensures that the network interface cards (NICs)/Host Channel Adapters (HCAs) are physically connected properly, with no loose cables or faulty ports that could disrupt network connectivity.
- Criticality: critical
- Suggestion: check the cable link,.

### HCA_FW_VERSION
- Description: Checks that all HCAS are running the same firmware version to prevent compatibility issues and ensure uniform performance across the network.
- Criticality: warning
- Suggestion: update the fw to the same version.

### HCA_PORT_SPEED
- Description: Verifies that all HCAs are operating at the same speed, which is crucial for maintaining consistent data transfer rates and preventing bottlenecks.
- Criticality: critical
- Suggestion: check the hca card port speed.

### HCA_NAMEING
### HCA_KERNEL_MODULE
- Description: Confirms that all necessary kernel modules required for the HCAs are fully loaded into the operating system, which is essential for their proper functionality.
- Criticality: critical
- Suggestion: modprobe the neccessy ib kernel module, include: ib_core、ib_cm、ib_umad、ib_uverbs、ib_ipoib.

### HCA_IB_STATUS
- Description: Monitors the status of InfiniBand (IB) status, ensuring that IB/RDMA in ACTIVe status .
- Criticality: critical
- Suggestion: check the opensm status/rdma status.

### HCA_OFED_FW_MATCH
- Description: Assesses the compatibility between the OpenFabrics Enterprise Distribution (OFED) software stack and the firmware of the HCAs to ensure they are correctly matched, which is critical for leveraging advanced networking features.
- Criticality: warning
- Suggestion: make sure the ofed version and fw version are matched in the spec.

### HCA_PCIe
#### HCA_PCIe_SPEED
- Description: Examines the Peripheral Component Interconnect Express (PCIe) speed of each HCA to confirm that it meets the required specifications for high-speed data transfer.
- Criticality: critical
- Suggestion: Check whether the HCA is installed in the correct PCIe slot.

#### HCA_PCIe_WIDTH
- Description: Assesses the bandwidth allocated to each PCIe slot to ensure that the HCAs have sufficient bandwidth for optimal performance.
- Criticality: critical
- Suggestion: Check whether the HCA is installed in the correct PCIe slot.

#### PCIe Tree Speed/Width 检测

`PCIETreeSpeedDownDegraded` / `PCIETreeWidthIncorrect` 不再依赖 HCA `board_id` spec
中的 `pcie_tree_speed` / `pcie_tree_width` 字段。Collector 直接从 sysfs
（`/sys/bus/pci/devices/<BDF>/{current,max}_link_{speed,width}`）枚举从 NIC 到 root
complex 的每一条 PCIe link，取整条路径上所有 link 的 `min(parent.max, child.max)`
的最小值作为路径理论上限；当某条 link 的 `current_link_*` 低于该上限时上报。

效果：

- 多 NIC 混插场景无需补 spec；新 SKU 上线零维护。
- 当 root port 物理上限低于 NIC（例如 root port 仅支持 Gen4 而 NIC 是 Gen5）时
  不再误报：路径上所有 link 都按 Gen4 跑是正常的。
- `Detail` / `Suggestion` 字段精确到出问题的具体 link，例如 `bottleneck@0000:80:01.0->0000:81:00.0`。

`pcie_tree_speed` / `pcie_tree_width` 字段在 HCA spec yaml 中保留为历史信息，
运行时不再读取。

#### HCA_PCIe_ACS
- Description: Checks the Access Control Services (ACS) settings of the PCIe to ensure proper traffic routing and prevent potential security vulnerabilities within the pcie topology.
- Criticality: critical
- Suggestion: use shell cmd "for i in $(lspci | cut -f 1 -d ' '); do setpci -v -s $i ecap_acs+6.w=0; done" disable acs.