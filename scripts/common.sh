#!/bin/bash

color_green="\033[1;32m"
color_yellow="\033[1;33m"
color_purple="\033[1;35m"
color_reset="\033[0m"

echo_back() {
    local _cmdLog=${1}
    printf "[${color_purple}EXEC${color_reset}] ${_cmdLog}\n"
    eval ${_cmdLog}
}

echo_info() {
    local _cmdLog=${1}
    printf "[${color_green}INFO${color_reset}] ${_cmdLog}\n"
}

echo_warn() {
    local _cmdLog=${1}
    printf "[${color_yellow}WARN${color_reset}] ${_cmdLog}\n"
}

# 临时label相关变量
TEMP_LABEL_KEY="sichek-temp-test"
TEMP_LABEL_VALUE="$(date +%s)-$$"  # 使用时间戳和进程ID确保唯一性
declare -a LABELED_NODES=()  # 存储被标记的节点
declare -a HOSTNAMES=()      # 全局数组，存储解析的hostnames
# 函数：解析hostname列表
parse_hostnames() {
  local hostfile="$1"
  local host="$2"
  local hostnames=()

  # 优先使用host参数，如果host参数有效则忽略hostfile
  if [ "$host" != "None" ] && [ -n "$host" ]; then
    echo_info "Parsing hostnames from parameter: $host"
    IFS=',' read -ra HOST_ARRAY <<< "$host"
    for hostname in "${HOST_ARRAY[@]}"; do
      # 去掉前后空格
      hostname=$(echo "$hostname" | xargs)
      if [[ -n "$hostname" ]]; then
        hostnames+=("$hostname")
      fi
    done
  elif [ "$hostfile" != "None" ] && [ -n "$hostfile" ] && [ -f "$hostfile" ]; then
    echo_info "Reading hostnames from file: $hostfile"
    while IFS= read -r line; do
      # 跳过空行和注释行
      if [[ -n "$line" && ! "$line" =~ ^[[:space:]]*# ]]; then
        # 提取hostname（去掉可能的IP地址和端口号）
        hostname=$(echo "$line" | awk '{print $1}' | cut -d: -f1)
        if [[ -n "$hostname" ]]; then
          hostnames+=("$hostname")
        fi
      fi
    done < "$hostfile"
  fi

  # 将结果存储到全局数组
  HOSTNAMES=("${hostnames[@]}")
}

# 函数：为节点设置临时label
label_nodes() {
  local hostnames=("$@")

  if [ ${#hostnames[@]} -eq 0 ]; then
    return 0
  fi

  echo_info "Setting temporary labels on nodes..."
  for hostname in "${hostnames[@]}"; do
    #echo_info "  Labeling node: $hostname"
      if kubectl label node "$hostname" "$TEMP_LABEL_KEY=$TEMP_LABEL_VALUE" --overwrite > /dev/null 2>&1; then
      LABELED_NODES+=("$hostname")
      #echo_info "    ✓ Successfully labeled $hostname"
    else
      echo_warn "    ✗ Failed to label $hostname"
    fi
  done

  # 更新NODE_SELECTOR以使用临时label
  NODE_SELECTOR="$TEMP_LABEL_KEY=$TEMP_LABEL_VALUE"
  echo_info "Updated nodeSelector to: $NODE_SELECTOR"
}

# 函数：清理临时labels
cleanup_labels() {
  if [ ${#LABELED_NODES[@]} -gt 0 ]; then
    echo_info "Cleaning up temporary labels..."
    for hostname in "${LABELED_NODES[@]}"; do
      #echo_info "  Removing label from node: $hostname"
      kubectl label node "$hostname" "$TEMP_LABEL_KEY-" > /dev/null 2>&1 || echo_warn "    Failed to remove label from $hostname"
    done
  fi
}

# 函数：处理hostfile和host参数，设置临时labels
setup_host_labels() {
  local hostfile="$1"
  local host="$2"
  local node_selector="$3"

  # 解析hostname列表到全局数组
  parse_hostnames "$hostfile" "$host"

  # 如果提供了hostfile或host参数，设置临时labels
  if [ ${#HOSTNAMES[@]} -gt 0 ]; then
    echo_info "Found ${#HOSTNAMES[@]} hostname(s) to test: ${HOSTNAMES[*]}"
    label_nodes "${HOSTNAMES[@]}"
  else
    echo_info "No specific hostnames provided, exiting..."
    exit 1
  fi
}
