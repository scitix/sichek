#!/bin/bash
declare -A physical_cards

# 遍历所有的以太网和 IB 网卡
for dev in /sys/class/net/* /sys/class/infiniband/*; do
    # 必须有物理 PCI 映射
    [ -e "$dev/device" ] || continue
    
    # 过滤掉不需要的虚拟接口
    case "$(basename "$dev")" in
        lo|veth*|docker*|br-*|bond*|roce_*|nv_*|eth_r*) continue ;;
    esac

    # 获取完整的 PCI 地址 (例如: 0000:0d:00.1)
    pci_full=$(basename "$(readlink -f "$dev/device")")
    
    # 截掉小数点及后面的 Function 号，得到物理卡 ID (例如: 0000:0d:00)
    pci_card="${pci_full%.*}"
    
    # 记录这块物理卡上有哪些端口
    physical_cards["$pci_card"]="${physical_cards[$pci_card]} $(basename "$dev")"
done

# 打印合并结果
echo "===== 物理网卡聚合结果 ====="
for card in "${!physical_cards[@]}"; do
    echo "物理卡 PCI $card 包含了以下网口: ${physical_cards[$card]}"
done
