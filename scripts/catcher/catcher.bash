#!/bin/bash
# catcher — 机型ID 与实际配置检查对比工具
#   Scan the real hardware (CPU / 内存 / 硬盘 / GPU / 网卡), build the 机型ID
#   per the naming spec, and print the human-readable config. Also decodes a
#   given 机型ID back into a config string.
#
# Companion to uqsm5090; pure bash + standard tools (lscpu, nvidia-smi,
# dmidecode, lsblk, lspci, ethtool). Run as root for memory (dmidecode):
#     sudo ./catcher.bash detect
#
# Usage:
#   ./catcher.bash detect            scan hardware -> 机型ID + config + JSON
#   ./catcher.bash id                scan hardware -> print only the 机型ID
#   ./catcher.bash describe <ID>     decode a 机型ID -> config description
#   ./catcher.bash json              scan -> machine-readable JSON only
#   ./catcher.bash -h | --help

set -u
SCRIPT_DIR="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
# shellcheck source=lib/catalog.sh
. "$SCRIPT_DIR/lib/catalog.sh"
# shellcheck source=lib/detect.sh
. "$SCRIPT_DIR/lib/detect.sh"
# shellcheck source=lib/idgen.sh
. "$SCRIPT_DIR/lib/idgen.sh"

usage() { sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; }

# ---- JSON helper (mirrors uqsm5090's jq-or-fallback style) ----------------
json_escape() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g'; }

# ---- run all detectors and populate globals ------------------------------
scan() {
  detect_cpu; detect_gpu; detect_mem; detect_disk; detect_net
  CODE_CPU="$(match_cpu  "$CPU_KEY")"
  CODE_GPU="$(match_gpu  "$GPU_KEY")"
  CODE_MEM="$(match_mem  "$MEM_KEY")"
  CODE_NIC="$(match_nic  "$NIC_KEY")"
  CODE_DISK="$(match_disk "$DISK_KEY")"
  MODEL_ID="${CODE_CPU}${CODE_GPU}${CODE_NIC}${CODE_MEM}${CODE_DISK}"
}

# ---- pretty per-component report with match status -----------------------
report_line() {  # report_line <label> <code> <key> <raw> <note>
  local label="$1" code="$2" key="$3" raw="$4" note="$5"
  if [ -n "$code" ]; then
    printf '  %-6s %-5s  <- %s\n' "$label" "$code" "$raw"
  elif [ -n "$key" ]; then
    printf '  %-6s %-5s  <- %s\n' "$label" "??" "$raw"
    printf '         \033[33m未匹配到机型库 (key=%s)。请补充命名规范。\033[0m\n' "$key"
  else
    printf '  %-6s %-5s  <- %s\n' "$label" "--" "${raw:-未检测到}"
  fi
  [ -n "$note" ] && printf '         \033[36m注: %s\033[0m\n' "$note"
}

cmd_detect() {
  scan
  echo "==================== 实际硬件检测 (detected) ===================="
  report_line "CPU"  "$CODE_CPU"  "$CPU_KEY"  "$CPU_RAW"  "$CPU_NOTE"
  report_line "GPU"  "$CODE_GPU"  "$GPU_KEY"  "$GPU_RAW"  "$GPU_NOTE"
  report_line "网卡" "$CODE_NIC"  "$NIC_KEY"  "$NIC_RAW"  "$NIC_NOTE"
  report_line "内存" "$CODE_MEM"  "$MEM_KEY"  "$MEM_RAW"  "$MEM_NOTE"
  report_line "硬盘" "$CODE_DISK" "$DISK_KEY" "$DISK_RAW" "$DISK_NOTE"
  echo "----------------------------------------------------------------"
  echo "机型ID: ${MODEL_ID:-(无法生成)}"
  local missing=""
  [ -z "$CODE_CPU" ]  && missing="$missing CPU"
  [ -z "$CODE_GPU" ]  && { [ -n "$GPU_KEY$GPU_RAW" ] && missing="$missing GPU"; }
  [ -z "$CODE_NIC" ]  && missing="$missing 网卡"
  [ -z "$CODE_MEM" ]  && missing="$missing 内存"
  [ -z "$CODE_DISK" ] && missing="$missing 硬盘"
  if [ -n "$missing" ]; then
    printf '\033[33m提示: 以下部件未能匹配机型库:%s。机型ID 不完整，请核对命名规范或补充新条目。\033[0m\n' "$missing"
  fi
  echo "----------------------------------------------------------------"
  echo "配置说明 (config，来自实测):"
  render_detected_config | sed 's/^/  /'
  echo "================================================================"
  cmd_json >/dev/null   # also drop the JSON result file
}

# Authoritative config render from the live scan: real DDR, real SSD/HDD media,
# real Ethernet/IB split. Falls back to a RAID best-guess line for masked disks.
render_detected_config() {
  [ -n "$CODE_CPU" ] && printf 'CPU: %s\n' "${CPU_DESC[$CODE_CPU]}"
  if [ -n "$CODE_MEM" ]; then printf '内存: %s%s\n' "${MEM_DDR:+$MEM_DDR }" "${MEM_DESC[$CODE_MEM]}"
  elif [ -n "$MEM_RAW" ]; then printf '内存: %s%s\n' "${MEM_DDR:+$MEM_DDR }" "$MEM_RAW"; fi
  # disk: matched -> describe; else RAID best-guess; else raw
  if [ -n "$CODE_DISK" ]; then
    local dkey=""; for k in "${!DISK_ID[@]}"; do [ "${DISK_ID[$k]}" = "$CODE_DISK" ] && dkey="$k"; done
    declare -A LM=(); for l in "${!DISK_MEDIA[@]}"; do LM[$l]="${DISK_MEDIA[$l]}"; done
    render_disk_desc "$dkey" "$CODE_DISK" LM
  else
    local guess; guess="$(guess_disk_raid "$DISK_KEY")"
    if [ -n "$guess" ]; then
      printf '硬盘: %s  \033[33m(实测=%s；按 RAID1 推断为机型库 %s)\033[0m\n' \
        "$DISK_RAW" "$DISK_KEY" "${guess%%|*}"
    elif [ -n "$DISK_RAW" ]; then printf '硬盘: %s\n' "$DISK_RAW"; fi
  fi
  [ -n "$CODE_GPU" ] && printf 'GPU: %s\n' "${GPU_DESC[$CODE_GPU]}"
  [ -z "$CODE_GPU" ] && [ -n "$GPU_RAW" ] && printf 'GPU: %s\n' "$GPU_RAW"
  # 直接读取我们精心配置的字典描述，如果没匹配上则输出原始抓取结果
  if [ -n "$CODE_NIC" ] && [ -n "${NIC_DESC[$CODE_NIC]:-}" ]; then
    printf '网卡: %s\n' "${NIC_DESC[$CODE_NIC]}"
  else
    printf '网卡: %s\n' "$NIC_RAW"
  fi
}

cmd_id() { scan; echo "$MODEL_ID"; }

cmd_describe() {
  local id="${1:-}"
  [ -z "$id" ] && { echo "用法: ./catcher.bash describe <机型ID>"; exit 1; }
  id="$(printf '%s' "$id" | tr '[:lower:]' '[:upper:]')"
  echo "机型ID: $id"
  echo "----------------------------------------------------------------"
  local out; out="$(describe_id "$id")"
  if [ -z "$out" ]; then echo "未能解析该机型ID（无可识别的部件代码）。"; exit 1; fi
  printf '%s\n' "$out"
}

cmd_json() {
  scan
  local out="$SCRIPT_DIR/result/catcher_result.json"
  mkdir -p "$SCRIPT_DIR/result"
  local desc; desc="$(render_detected_config | sed 's/\x1b\[[0-9;]*m//g')"   # authoritative live render, ANSI-stripped
  if command -v jq >/dev/null 2>&1; then
    jq -n \
      --arg id "$MODEL_ID" \
      --arg cpu_code "$CODE_CPU"   --arg cpu_raw "$CPU_RAW" --arg cpu_key "$CPU_KEY" \
      --arg gpu_code "$CODE_GPU"   --arg gpu_raw "$GPU_RAW" --arg gpu_key "$GPU_KEY" \
      --arg nic_code "$CODE_NIC"   --arg nic_raw "$NIC_RAW" --arg nic_key "$NIC_KEY" \
      --arg mem_code "$CODE_MEM"   --arg mem_raw "$MEM_RAW" --arg mem_key "$MEM_KEY" \
      --arg disk_code "$CODE_DISK" --arg disk_raw "$DISK_RAW" --arg disk_key "$DISK_KEY" \
      --arg desc "$desc" \
      '{model_id:$id,
        components:{
          cpu:{code:$cpu_code,detected:$cpu_raw,key:$cpu_key},
          gpu:{code:$gpu_code,detected:$gpu_raw,key:$gpu_key},
          nic:{code:$nic_code,detected:$nic_raw,key:$nic_key},
          mem:{code:$mem_code,detected:$mem_raw,key:$mem_key},
          disk:{code:$disk_code,detected:$disk_raw,key:$disk_key}},
        config:$desc}' > "$out"
  else
    { printf '{"model_id":"%s","components":{' "$(json_escape "$MODEL_ID")"
      printf '"cpu":{"code":"%s","detected":"%s","key":"%s"},'  "$(json_escape "$CODE_CPU")"  "$(json_escape "$CPU_RAW")"  "$(json_escape "$CPU_KEY")"
      printf '"gpu":{"code":"%s","detected":"%s","key":"%s"},'  "$(json_escape "$CODE_GPU")"  "$(json_escape "$GPU_RAW")"  "$(json_escape "$GPU_KEY")"
      printf '"nic":{"code":"%s","detected":"%s","key":"%s"},'  "$(json_escape "$CODE_NIC")"  "$(json_escape "$NIC_RAW")"  "$(json_escape "$NIC_KEY")"
      printf '"mem":{"code":"%s","detected":"%s","key":"%s"},'  "$(json_escape "$CODE_MEM")"  "$(json_escape "$MEM_RAW")"  "$(json_escape "$MEM_KEY")"
      printf '"disk":{"code":"%s","detected":"%s","key":"%s"}'  "$(json_escape "$CODE_DISK")" "$(json_escape "$DISK_RAW")" "$(json_escape "$DISK_KEY")"
      printf '},"config":"%s"}\n' "$(json_escape "$(printf '%s' "$desc" | tr '\n' '#')")"
    } > "$out"
  fi
  echo "JSON written: $out"
}

# ---- entry ----------------------------------------------------------------
case "${1:-detect}" in
  detect)   cmd_detect ;;
  id)       cmd_id ;;
  describe) shift; cmd_describe "${1:-}" ;;
  json)     cmd_json ;;
  -h|--help|help) usage ;;
  *) echo "未知命令: $1"; usage; exit 1 ;;
esac
