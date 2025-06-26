#!/bin/bash

print_ib_info() {
    for ibdev in /sys/class/infiniband/*; do
        IBDEV=$(basename "$ibdev")
        HCA_PATH="/sys/class/infiniband/$IBDEV"
        PORT="1"  # 默认使用 port 1

        # 获取 net device（可能有多个，这里取第一个）
        NETDEV=$(ls "$HCA_PATH"/device/net 2>/dev/null | head -n1)
        NET_PATH="/sys/class/net/$NETDEV"

        # PCIe 信息
        PCIE_PATH=$(readlink -f "$HCA_PATH"/device)
        PCIEBDF=$(basename "$PCIE_PATH")
        SLOT=$(basename $(dirname "$PCIE_PATH"))
        PCIESPEED=$(cat "$PCIE_PATH"/current_link_speed 2>/dev/null)
        PCIEWIDTH=$(cat "$PCIE_PATH"/current_link_width 2>/dev/null)
        PCIEMRR=$(cat "$PCIE_PATH"/max_read_request_size 2>/dev/null)
        NUMANODE=$(cat "$PCIE_PATH"/numa_node 2>/dev/null)
        CPULISTS=$(cat "$PCIE_PATH"/local_cpulist 2>/dev/null)

        # InfiniBand 属性
        HCATYPE=$(cat "$HCA_PATH"/hw_rev 2>/dev/null)
        SYSTEMGUID=$(cat "$HCA_PATH"/sys_image_guid 2>/dev/null)
        NODEGUID=$(cat "$HCA_PATH"/node_guid 2>/dev/null)
        PHYSTATE=$(cat "$HCA_PATH"/ports/$PORT/phys_state 2>/dev/null)
        PORTSTATE=$(cat "$HCA_PATH"/ports/$PORT/state 2>/dev/null)
        LINKLAYER=$(cat "$HCA_PATH"/ports/$PORT/link_layer 2>/dev/null)
        PORTSPEED=$(cat "$HCA_PATH"/ports/$PORT/rate 2>/dev/null)
        FWVER=$(cat "$HCA_PATH"/fw_ver 2>/dev/null)
        VPD=$(cat "$HCA_PATH"/vpd 2>/dev/null)
        BOARDID=$(cat "$HCA_PATH"/board_id 2>/dev/null)
        DEVICEID=$(cat "$PCIE_PATH"/device 2>/dev/null)

        # 网卡状态
        NETOPERSTATE=$(cat "$NET_PATH"/operstate 2>/dev/null)

        # OFED 版本（根据路径，可能需要用户调整 OFED 安装位置）
        OFEDVER=$(ofed_info -s 2>/dev/null | awk '{print $2}')

        # 输出 JSON（格式化输出）
        cat <<EOF
{
  "IBdev": "$IBDEV",
  "net_dev": "$NETDEV",
  "hca_type": "$HCATYPE",
  "system_guid": "$SYSTEMGUID",
  "node_guid": "$NODEGUID",
  "phy_state": "$PHYSTATE",
  "port_state": "$PORTSTATE",
  "link_layer": "$LINKLAYER",
  "net_operstate": "$NETOPERSTATE",
  "port_speed": "$PORTSPEED",
  "board_id": "$BOARDID",
  "device_id": "$DEVICEID",
  "pcie_bdf": "$PCIEBDF",
  "pcie_speed": "$PCIESPEED",
  "pcie_width": "$PCIEWIDTH",
  "pcie_mrr": "$PCIEMRR",
  "slot": "$SLOT",
  "numa_node": "$NUMANODE",
  "cpu_lists": "$CPULISTS",
  "fw_ver": "$FWVER",
  "vpd": "$VPD",
  "ofed_ver": "$OFEDVER"
}
EOF
    done
}

print_ib_info

