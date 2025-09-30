#!/bin/bash

HOSTFILE=/etc/mpi/hostfile
NCCL_TEST=/usr/local/sihpc/libexec/nccl-tests/nccl_test
INTERFACE=eth0
SLOTS_PER_NODE=8
TMP_LOG=/tmp/nccl_allgather_check.log

# Pod-to-node mapping (optional)
POD_NODE_MAPPING=""
# MPI run timeout in seconds (default: 300s)
MPIRUN_TIMEOUT=120

# Function to convert pod name to node name
get_node_name() {
  local pod_name="$1"
  if [ -n "$POD_NODE_MAPPING" ]; then
    # Extract node name from mapping: pod1:node1,pod2:node2
    echo "$POD_NODE_MAPPING" | tr ',' '\n' | grep "^${pod_name}:" | cut -d':' -f2
  else
    echo "$pod_name"
  fi
}

# Function to test a subset of nodes
check_nodes() {
  local subset=("$@")

  local host_list=()
  for h in "${subset[@]}"; do
    host_list+=("${h}:${SLOTS_PER_NODE}")
  done

  local hosts=$(IFS=','; echo "${host_list[*]}")
  echo -n "Checking: ${hosts} ... "

  timeout ${MPIRUN_TIMEOUT} /usr/local/sihpc/bin/mpirun \
    --mca routed direct \
    --mca plm_rsh_no_tree_spawn 1 \
    --allow-run-as-root \
    --host ${hosts} \
    --map-by ppr:${SLOTS_PER_NODE}:node \
    --mca oob_tcp_if_include $INTERFACE \
    --mca pml ^ucx \
    --mca btl self,tcp \
    --mca btl_tcp_if_include $INTERFACE \
    $NCCL_TEST -lallgather -b 8 -e 256M -f 2 > $TMP_LOG 2>&1

  if grep -q "Avg bus bandwidth" $TMP_LOG; then
    echo "✅ PASS"
    return 0
  else
    echo "❌ FAIL"
    return 1
  fi
}

# Recursive binary check
binary_check() {
  local group=("$@")
  if [ ${#group[@]} -eq 1 ]; then
    # Convert pod name to node name if mapping is available
    local node_name=$(get_node_name "${group[0]}")
    BAD_NODES+=("$node_name")
    return
  fi

  # 如果 group <= 2，直接测试，不再二分
  if [ ${#group[@]} -le 2 ]; then
    check_nodes "${group[@]}"
    if [ $? -ne 0 ]; then
      for pod in "${group[@]}"; do
        local node_name=$(get_node_name "$pod")
        BAD_NODES+=("$node_name")
      done
    fi
    return
  fi
  # 否则继续递归二分
  check_nodes "${group[@]}"
  if [ $? -eq 0 ]; then
    return
  fi

  local mid=$((${#group[@]} / 2))
  local left=("${group[@]:0:$mid}")
  local right=("${group[@]:$mid}")

  binary_check "${left[@]}"
  binary_check "${right[@]}"
}

# Function to display usage
usage() {
  echo "Usage: $0 [OPTIONS]"
  echo ""
  echo "Options:"
  echo "  --pod-node-mapping MAPPING    Pod-to-node mapping (format: pod1:node1,pod2:node2)"
  echo "  --mpirun-timeout SECONDS     Timeout for mpirun commands (default: 120)"
  echo "  -h, --help                    Show this help message"
  echo ""
  echo "This script uses binary search to identify slow nodes based on NCCL AllGather connectivity."
  echo "If pod-node mapping is provided, it will display node names instead of pod names."
  exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --pod-node-mapping)
      POD_NODE_MAPPING="$2"
      shift 2
      ;;
    --mpirun-timeout)
      MPIRUN_TIMEOUT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1"
      usage
      ;;
  esac
done

# Check if hostfile exists
if [ ! -f "$HOSTFILE" ]; then
  echo "Error: Hostfile $HOSTFILE not found!"
  exit 1
fi

# Read nodes from hostfile
NODES=($(awk '{print $1}' $HOSTFILE))
BAD_NODES=()

# Start binary check
echo "🔍 Starting binary search over ${#NODES[@]} nodes..."
if [ -n "$POD_NODE_MAPPING" ]; then
  echo "ℹ️  Using pod-to-node mapping for display"
fi
binary_check "${NODES[@]}"

echo
echo "=========================================="
if [ ${#BAD_NODES[@]} -eq 0 ]; then
  echo "🎉 All nodes passed NCCL AllGather check!"
else
  echo "❌ The following nodes failed:"
  for n in "${BAD_NODES[@]}"; do
    echo "   - $n"
  done
fi
