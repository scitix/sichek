# shellcheck shell=bash
# detect.sh — probe the real hardware and emit *canonical keys* matching catalog.sh.
#
# Each detect_* function sets global variables (so the caller can read both the
# canonical key used for matching AND a human "raw" string + any caveats):
#   CPU:  CPU_KEY  CPU_RAW  CPU_NOTE
#   GPU:  GPU_KEY  GPU_RAW  GPU_NOTE
#   MEM:  MEM_KEY  MEM_RAW  MEM_DDR  MEM_NOTE
#   DISK: DISK_KEY DISK_RAW DISK_NOTE  + DISK_MEDIA[label]
#   NET:  NIC_KEY  NIC_RAW  NIC_NOTE
#
# Comments explain what each command reads at the hardware level.

# Run a command with root privileges if we are not already root (non-interactive
# sudo only — on the physical host the operator runs `sudo ./catcher.bash`).
_priv() {
  if [ "$(id -u)" -eq 0 ]; then "$@"
  elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then sudo -n "$@"
  else return 127
  fi
}

# ===========================================================================
# CPU  — lscpu reads CPUID + /proc/cpuinfo: the silicon's self-reported model.
# ===========================================================================
detect_cpu() {
  CPU_KEY=""; CPU_RAW=""; CPU_NOTE=""
  if ! command -v lscpu >/dev/null 2>&1; then CPU_NOTE="lscpu not found"; return; fi
  local out vendor model sockets sku
  out="$(lscpu 2>/dev/null)"
  vendor="$(printf '%s\n' "$out" | awk -F: '/^Vendor ID/{gsub(/^[ \t]+/,"",$2);print $2}')"
  model="$(printf '%s\n'  "$out" | awk -F: '/^Model name/{gsub(/^[ \t]+/,"",$2);print $2; exit}')"
  sockets="$(printf '%s\n' "$out" | awk -F: '/^Socket\(s\)/{gsub(/[ \t]/,"",$2);print $2}')"
  [ -z "$sockets" ] && sockets=1
  CPU_RAW="$model (${sockets} socket)"

  case "$vendor" in
    *Intel*|*GenuineIntel*)
      # Strip marketing noise, then grab the SKU: 4 digits + optional letters + optional '+'
      local clean
      clean="$(printf '%s' "$model" | sed -E 's/\(R\)|\(TM\)//g; s/@.*//')"
      sku="$(printf '%s' "$clean" | grep -oE '[0-9]{4}[A-Z]*\+?' | head -n1)"
      [ -n "$sku" ] && CPU_KEY="Intel ${sku}|${sockets}"
      ;;
    *AMD*|*AuthenticAMD*)
      # AMD EPYC SKU is the token right after "EPYC": 7713 / 74F3 / 9575F ...
      sku="$(printf '%s' "$model" | awk '{for(i=1;i<=NF;i++) if($i=="EPYC"){print $(i+1); exit}}')"
      [ -z "$sku" ] && sku="$(printf '%s' "$model" | grep -oE '[0-9]{2}[0-9A-Z]{2}[A-Z]?' | head -n1)"
      [ -n "$sku" ] && CPU_KEY="AMD ${sku}|${sockets}"
      ;;
    *) CPU_NOTE="unknown CPU vendor: $vendor" ;;
  esac
  [ -z "$CPU_KEY" ] && [ -z "$CPU_NOTE" ] && CPU_NOTE="could not parse SKU from: $model"
}

# ===========================================================================
# GPU  — nvidia-smi queries the NVIDIA driver (NVML); no CUDA toolkit needed.
# We read the product name + VRAM to disambiguate variants (PCIe vs SXM, 40/80G).
# ===========================================================================
_gpu_token() {            # _gpu_token "<name>" "<mem_MiB>" -> token on stdout
  local n; n="$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"
  local mem="${2:-0}"
  local bus="SXM"; case "$n" in *PCIE*|*PCIE*) bus="PCIE";; esac
  local g80=0; [ "$mem" -ge 70000 ] 2>/dev/null && g80=1   # >=~80G
  case "$n" in
    *RTX*5090*)             echo "RTX5090" ;;
    *B200*)                 echo "B200" ;;
    *B300*)                 echo "B300" ;;
    *H200*)                 echo "H200-141G" ;;
    *H100*)                 echo "H100-SXM-80G" ;;          # only SXM-80G in catalog
    *H20*)  if [ "$mem" -ge 130000 ] 2>/dev/null; then echo "H20-141G"; else echo "H20-96G"; fi ;;
    *L40S*)                 echo "L40S" ;;
    *L40*)                  echo "L40" ;;
    *L20*)                  echo "L20" ;;
    *A800*)                 echo "A800-SXM-80G" ;;
    *A100*) if [ "$g80" -eq 1 ]; then echo "A100-${bus}-80G"; else echo "A100-${bus}-40G"; fi ;;
    *V100S*)                echo "V100S-PCIE-32G" ;;
    *V100*) if [ "$bus" = "PCIE" ]; then echo "V100-PCIE-16G"; else echo "V100-SXM2"; fi ;;
    *2080*TI*|*2080TI*)     echo "2080Ti" ;;
    *A10\ *|*\ A10|*"A10"*)  case "$n" in *A100*|*A10G*) echo "RAW:$1";; *) echo "A10";; esac ;;
    *)                      echo "RAW:$1" ;;
  esac
}

detect_gpu() {
  GPU_KEY=""; GPU_RAW=""; GPU_NOTE=""
  if ! command -v nvidia-smi >/dev/null 2>&1; then
    # No NVIDIA driver -> maybe a CPU-only or non-NVIDIA box.
    if lspci 2>/dev/null | grep -qi nvidia; then GPU_NOTE="NVIDIA PCI device present but nvidia-smi missing"; fi
    return
  fi
  local q; q="$(nvidia-smi --query-gpu=name,memory.total --format=csv,noheader,nounits 2>/dev/null)"
  if [ -z "$q" ]; then
    # nvidia-smi binary present but no usable driver/device (e.g. driver not loaded).
    GPU_NOTE="nvidia-smi present but no NVIDIA driver/device available"
    lspci 2>/dev/null | grep -qi nvidia || GPU_NOTE="$GPU_NOTE; no NVIDIA PCI device found"
    return
  fi
  local count=0 token first_token="" mixed=0 name mem line
  while IFS=, read -r name mem; do
    name="$(printf '%s' "$name" | sed -E 's/^[ \t]+|[ \t]+$//g')"
    mem="$(printf '%s' "$mem"  | tr -dc '0-9')"
    # Skip driver-error banner lines: a real row is "<product name>, <MiB>".
    [ -z "$name" ] && continue
    case "$name" in *[Nn]vidia-smi*|*failed*|*Failed*|*driver*) continue;; esac
    [[ "$mem" =~ ^[0-9]+$ ]] || continue
    token="$(_gpu_token "$name" "$mem")"
    count=$((count+1))
    if [ -z "$first_token" ]; then first_token="$token"; GPU_RAW="$name"
    elif [ "$token" != "$first_token" ]; then mixed=1; fi
  done <<< "$q"
  if [ "$count" -eq 0 ]; then
    GPU_NOTE="nvidia-smi returned no usable GPU rows (driver error?)"
    return
  fi
  GPU_RAW="${GPU_RAW} *${count}"
  [ "$mixed" -eq 1 ] && GPU_NOTE="mixed GPU models detected; using first"
  case "$first_token" in
    RAW:*) GPU_NOTE="GPU model not in catalog: ${first_token#RAW:} *${count}"; GPU_KEY="" ;;
    *)     GPU_KEY="${first_token}|${count}" ;;
  esac
}

# ===========================================================================
# MEM  — dmidecode reads the firmware SMBIOS/DMI tables (per-DIMM layout).
# /proc/meminfo only gives the total, which can't tell 16x64G from 32x32G.
# ===========================================================================
detect_mem() {
  MEM_KEY=""; MEM_RAW=""; MEM_DDR=""; MEM_NOTE=""
  if ! command -v dmidecode >/dev/null 2>&1; then MEM_NOTE="dmidecode not found (need root tool)"; return; fi
  local dmi
  dmi="$(_priv dmidecode -t memory 2>/dev/null)"
  if [ -z "$dmi" ]; then
    local tot; tot="$(awk '/MemTotal/{printf "%.0f", $2/1024/1024}' /proc/meminfo 2>/dev/null)"
    MEM_NOTE="dmidecode needs root (run: sudo ./catcher.bash). Total≈${tot}GB only."
    MEM_RAW="${tot}GB total"
    return
  fi
  MEM_DDR="$(printf '%s\n' "$dmi" | awk -F: '/^[ \t]*Type:/{gsub(/[ \t]/,"",$2); if($2 ~ /^DDR/){print $2; exit}}')"
  # Collect each populated module's size in GB (skip "No Module Installed").
  local sizes; sizes="$(printf '%s\n' "$dmi" | awk '
    /^[ \t]*Size:/ {
      if ($0 ~ /No Module|Not Installed|Unknown/) next
      v=$2; u=$3
      if (u ~ /GB/) printf "%d\n", v
      else if (u ~ /MB/) printf "%d\n", v/1024
    }')"
  [ -z "$sizes" ] && { MEM_NOTE="no populated DIMMs parsed"; return; }
  local count uniqsize nsizes
  count="$(printf '%s\n' "$sizes" | grep -c .)"
  # dominant size (mode); note if mixed
  uniqsize="$(printf '%s\n' "$sizes" | sort -n | uniq -c | sort -rn | awk 'NR==1{print $2}')"
  nsizes="$(printf '%s\n' "$sizes" | sort -u | grep -c .)"
  [ "$nsizes" -gt 1 ] && MEM_NOTE="mixed DIMM sizes detected; using dominant ${uniqsize}G"
  MEM_KEY="${uniqsize}|${count}"
  MEM_RAW="${MEM_DDR:+$MEM_DDR }${uniqsize}G *${count}"
}

# ===========================================================================
# DISK — 优先使用 storcli/storcli2 穿透硬件 RAID 卡获取真实物理盘；
#        若无 RAID 工具，则降级使用 lsblk 读取 /sys/block。
# Sizes are raw BYTES (decimal) snapped to marketed buckets (960G, 3.84T ...).
# ===========================================================================
_snap_disk() {            # _snap_disk <bytes> -> "label|gb" or "" if no bucket within 8%
  local bytes="$1" gb best="" bestd="" b d
  gb=$(( bytes / 1000000000 ))     # decimal GB
  for b in "${DISK_BUCKETS[@]}"; do
    d=$(( gb>b ? gb-b : b-gb ))
    if [ -z "$bestd" ] || [ "$d" -lt "$bestd" ]; then bestd="$d"; best="$b"; fi
  done
  [ -z "$best" ] && { echo ""; return; }
  # tolerance: within 8% of the bucket
  if [ "$(( bestd * 100 ))" -le "$(( best * 8 ))" ]; then echo "${DISK_LABEL[$best]}|$best"; else echo ""; fi
}

detect_disk() {
  DISK_KEY=""; DISK_RAW=""; DISK_NOTE=""
  declare -gA DISK_MEDIA=()
  declare -A cnt=() gbof=()
  local raw_parts=() raid=0 label gb snap

  # -------------------------------------------------------------------------
  # 策略 A：尝试使用 storcli / storcli2 穿透 RAID 卡获取绝对真实的物理盘
  # -------------------------------------------------------------------------
  local stor_cmd=""
  if command -v storcli2 >/dev/null 2>&1; then stor_cmd="storcli2"; fi
  if [ -z "$stor_cmd" ] && command -v storcli >/dev/null 2>&1; then stor_cmd="storcli"; fi

  # 如果找到了存储命令行工具，并且当前是 root 用户（或者有免密 sudo）
  if [ -n "$stor_cmd" ] && _priv "$stor_cmd" -v >/dev/null 2>&1; then
    raid=1
    DISK_NOTE="Detected hardware RAID via $stor_cmd — physical disks successfully parsed."

    # 执行 /cALL/eALL/sALL show 命令（查询所有控制器、所有扩展柜、所有槽位的磁盘）
    # 解析输出，寻找类似 "953.3 GB" "3.49 TB" 且包含固态/机械标记的物理流
    local stor_out; stor_out="$(_priv "$stor_cmd" /cALL/eALL/sALL show 2>/dev/null)"

    # 逐行分析物理盘信息
    while read -r size unit med; do
      [ -z "$size" ] && continue

      # 将容量统一换算为 bytes
      local bytes=0
      if [[ "$unit" == *"TB"* || "$unit" == *"TB"* ]]; then
        # 考虑到浮点数，用 awk 放大避免 bash 无法处理浮点
        bytes=$(awk -v s="$size" 'BEGIN {printf "%.0f", s*1000*1000*1000*1000}')
      else
        bytes=$(awk -v s="$size" 'BEGIN {printf "%.0f", s*1000*1000*1000}')
      fi

      snap="$(_snap_disk "$bytes")"
      if [ -z "$snap" ]; then
        DISK_NOTE="${DISK_NOTE}; physical size ${size}${unit} not in catalog buckets"
        continue
      fi

      label="${snap%|*}"; gb="${snap#*|}"
      cnt[$label]=$(( ${cnt[$label]:-0} + 1 ))
      gbof[$label]="$gb"

      # 判定介质是 SSD 还是 HDD (storcli 通常输出 SSD, HDD, Med 等)
      if [[ "$(printf '%s' "$med" | tr '[:lower:]' '[:upper:]')" == *"HDD"* ]]; then
        [ "${DISK_MEDIA[$label]:-SSD}" = "SSD" ] && DISK_MEDIA[$label]="HDD"
      else
        DISK_MEDIA[$label]="${DISK_MEDIA[$label]:-SSD}"
      fi
    done < <(printf '%s\n' "$stor_out" | awk '
      # 匹配物理盘表项行。storcli 的典型物理盘行包含类似 "UGood", "Onln" 等状态及大小
      /^[0-9]+:[0-9]+/ {
        # 寻找包含大小和单位的列 (例如 953.25 GB 或 3.636 TB)
        for(i=1; i<=NF; i++) {
          if($i ~ /GB|TB/) {
            size=$(i-1); unit=$i;
            # 寻找介质类型列
            med="SSD"; for(j=1; j<=NF; j++) { if($j ~ /HDD|Hdd/) med="HDD" }
            print size, unit, med;
            break;
          }
        }
      }')
  fi

  # -------------------------------------------------------------------------
  # 策略 B：若未获取到物理盘（没有 RAID 工具），降级使用原有的 lsblk 逻辑
  # -------------------------------------------------------------------------
  if [ ${#cnt[@]} -eq 0 ]; then
    if ! command -v lsblk >/dev/null 2>&1; then DISK_NOTE="lsblk not found"; return; fi
    local name bytes type rota model
    while read -r name bytes type rota model; do
      case "$name" in loop*|zram*|sr*|fd*|dm-*|rbd*) continue;; esac
      [ "$type" = "disk" ] || continue
      case "$model" in *Logical*Volume*|*LOGICAL*VOLUME*|*Virtual*Disk*|*MR9*|*PERC*|*AVAGO*) raid=1;; esac
      snap="$(_snap_disk "$bytes")"
      if [ -z "$snap" ]; then
        raw_parts+=("$(( bytes/1000000000 ))G(unmatched)")
        DISK_NOTE="${DISK_NOTE}; size $(( bytes/1000000000 ))GB not in catalog buckets"
        continue
      fi
      label="${snap%|*}"; gb="${snap#*|}"
      cnt[$label]=$(( ${cnt[$label]:-0} + 1 ))
      gbof[$label]="$gb"
      if [ "$rota" = "1" ]; then
        [ "${DISK_MEDIA[$label]:-SSD}" = "SSD" ] && DISK_MEDIA[$label]="HDD"
      else
        DISK_MEDIA[$label]="${DISK_MEDIA[$label]:-SSD}"
      fi
    done < <(lsblk -dnb -o NAME,SIZE,TYPE,ROTA,MODEL 2>/dev/null)

    [ "$raid" -eq 1 ] && DISK_NOTE="RAID present — physical disk count may be masked. Used lsblk fallback. ${DISK_NOTE}"
  fi

  # -------------------------------------------------------------------------
  # 统一组装规范的 KEY 和描述文本
  # -------------------------------------------------------------------------
  local key="" k
  while read -r k; do
    [ -z "$k" ] && continue
    label="${k#*|}"
    key="${key:+$key+}${label}*${cnt[$label]}"
    raw_parts+=("${DISK_MEDIA[$label]} ${label}*${cnt[$label]}")
  done < <(for label in "${!cnt[@]}"; do echo "${gbof[$label]}|$label"; done | sort -n | awk -F'|' '{print $1"|"$2}')

  DISK_KEY="$key"
  DISK_RAW="$(IFS='; '; echo "${raw_parts[*]}")"
  DISK_NOTE="${DISK_NOTE#; }"
}

# ===========================================================================
# NET  — 物理卡聚合增强版 (PCI BDF Grouping)
# 按物理网卡的 PCI 根地址 (如 0000:0d:00) 进行聚合，统计真实物理卡数量。
# 最终生成的 Key 格式形如 "100Gx2*2+200Gx4*8" (代表8张单卡4口的200G物理卡等)
# ===========================================================================
detect_net() {
  NIC_KEY=""; NIC_RAW=""; NIC_NOTE=""
  declare -A card_max_speed=()
  declare -A card_total_ports=()
  local down=0 i dev spd g rate gen port_name pci_full pci_card

  # --- 1. 遍历以太网端口 ---
  for i in /sys/class/net/*; do
    dev="$(basename "$i")"
    
    # 排除纯虚拟逻辑网卡 (保留 bond 等待底层剥离)
    case "$dev" in
      lo|veth*|docker*|br-*|cni*|cali*|flannel*|tun*|tap*|bond*) continue ;;
    esac

    # 必须有物理 PCI 映射，且排除 VF 自身设备节点
    [ -e "$i/device" ] || continue
    [ -e "$i/device/physfn" ] && continue

    # 排除 Mellanox 子网卡 (Sub-Function) 和 VF Rep (Virtual Function Representative)
    port_name="$(cat "$i/phys_port_name" 2>/dev/null)"
    case "$port_name" in *vf*|*sf*) continue ;; esac

    # 跳过原生 IB 端口 (留在 Infiniband 阶段处理，防止双重计数)
    [ "$(cat "$i/type" 2>/dev/null)" = "32" ] && continue

    # --- 核心：提取物理网卡的 PCI 根地址 (抹去 Function) ---
    pci_full=$(basename "$(readlink -f "$i/device")")
    pci_card="${pci_full%.*}"

    spd="$(cat "$i/speed" 2>/dev/null)"
    if ! [[ "$spd" =~ ^[0-9]+$ ]] || [ "$spd" -le 0 ]; then
      if command -v ethtool >/dev/null 2>&1; then
        spd="$(ethtool "$dev" 2>/dev/null | awk '/Speed:/{if(match($0,/[0-9]+/)){print substr($0,RSTART,RLENGTH);exit}}')"
      fi
    fi

    # 只有当前端口 Link UP 且有真实速率时，才算入这张物理卡的有效端口中
    if [[ "$spd" =~ ^[0-9]+$ ]] && [ "$spd" -gt 0 ]; then
      card_total_ports[$pci_card]=$(( ${card_total_ports[$pci_card]:-0} + 1 ))
      g=$(( spd / 1000 ))                            # 换算为 Gbps
      if [ "$g" -gt "${card_max_speed[$pci_card]:-0}" ]; then
        card_max_speed[$pci_card]=$g
      fi
    else
      down=$(( down + 1 ))
    fi
  done

  # --- 2. 遍历 InfiniBand 端口 ---
  if [ -d /sys/class/infiniband ]; then
    for d in /sys/class/infiniband/*; do
      [ -d "$d/ports" ] || continue
      [ -e "$d/device/physfn" ] && continue

      pci_full=$(basename "$(readlink -f "$d/device")")
      pci_card="${pci_full%.*}"

      for p in "$d"/ports/*; do
        [ -e "$p/rate" ] || continue
        # 如果是 RoCE 模式，已在上方以太网中统计，此处坚决跳过
        local ll; ll="$(cat "$p/link_layer" 2>/dev/null)"
        if [ "$ll" = "Ethernet" ]; then continue; fi

        rate="$(cat "$p/rate" 2>/dev/null)"
        g="$(printf '%s' "$rate" | grep -oE '^[0-9]+')"
        [ -z "$g" ] && continue

        # 统计有效原生 IB 端口
        card_total_ports[$pci_card]=$(( ${card_total_ports[$pci_card]:-0} + 1 ))
        if [ "$g" -gt "${card_max_speed[$pci_card]:-0}" ]; then
          card_max_speed[$pci_card]=$g
        fi
      done
    done
  fi

  # --- 3. 聚合物理签名 (例如一张卡提取为 "200Gx4") ---
  declare -A sig_count=()
  for pci_card in "${!card_total_ports[@]}"; do
    local ports=${card_total_ports[$pci_card]:-0}
    local speed=${card_max_speed[$pci_card]:-0}
    [ "$speed" -eq 0 ] && continue  # 如果整张卡全部 down，则跳过提取
    
    local sig="${speed}Gx${ports}"
    sig_count[$sig]=$(( ${sig_count[$sig]:-0} + 1 ))
  done

  # --- 4. 生成极其规范的可读指纹 ---
  local key="" raw=""
  # 使用 sort -V 确保按照 100G, 200G 以及口数的标准顺序输出
  while read -r sig; do
    [ -z "$sig" ] && continue
    local count=${sig_count[$sig]}
    
    key="${key:+$key+}${sig}*${count}"
    
    local s="${sig%x*}"
    local p="${sig#*x}"
    raw="${raw:+$raw + }${s}bps物理卡(${p}口)*${count}"
  done < <(for sig in "${!sig_count[@]}"; do echo "$sig"; done | sort -V)

  NIC_KEY="$key"
  NIC_RAW="$raw"
  [ -z "$NIC_RAW" ] && NIC_RAW="(no physical NIC link detected)"
  [ "$down" -gt 0 ] && NIC_NOTE="${down} physical port(s) link-down or filtered"
}
