#!/bin/bash

HOSTFILE=/etc/mpi/hostfile
NCCL_TEST=/usr/local/sihpc/libexec/nccl-tests/nccl_test
INTERFACE=eth0
SLOTS_PER_NODE=8
TMP_LOG=/tmp/nccl_allreduce_check.log

# Performance threshold settings
MIN_BANDWIDTH_GBPS=""  # Minimum expected bandwidth in GB/s (optional)
# MPI run timeout in seconds (default: 300s)
MPIRUN_TIMEOUT=120

# Pod-to-node mapping (optional)
POD_NODE_MAPPING=""

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

# Function to test a subset of nodes with allreduce
check_nodes_performance() {
  local subset=("$@")

  local host_list=()
  for h in "${subset[@]}"; do
    host_list+=("${h}:${SLOTS_PER_NODE}")
  done

  local hosts=$(IFS=','; echo "${host_list[*]}")
  echo -n "Testing performance: ${hosts} ... "

  # Run allreduce test - NCCL test tool has built-in bandwidth expectations
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
    $NCCL_TEST -lallreduce > $TMP_LOG 2>&1

  local exit_code=$?
  
  # Parse bandwidth from the log (NCCL outputs in GB/s)
  local bandwidth=$(grep "Avg bus bandwidth" $TMP_LOG | sed -E 's/.*: *([0-9]+\.[0-9]+).*/\1/' | head -1)
  
  # Check if we got valid bandwidth
  if [ -z "$bandwidth" ]; then
    bandwidth="N/A"
  fi
  
  # Determine if test passed based on exit code and optional bandwidth threshold
  local test_passed=0
  
  # First check: NCCL test must return 0 (built-in expectations met)
  if [ $exit_code -eq 0 ]; then
    # Second check: if minimum bandwidth is specified, check bandwidth threshold
    if [ -n "$MIN_BANDWIDTH_GBPS" ] && [ "$bandwidth" != "N/A" ]; then
      # Use bc for floating point comparison
      if command -v bc >/dev/null 2>&1; then
        if (( $(echo "$bandwidth >= $MIN_BANDWIDTH_GBPS" | bc -l) )); then
          test_passed=1
        fi
      else
        # Fallback to awk for floating point comparison
        if awk "BEGIN {exit !($bandwidth >= $MIN_BANDWIDTH_GBPS)}"; then
          test_passed=1
        fi
      fi
    else
      # No minimum bandwidth specified, just check exit code
      test_passed=1
    fi
  fi
  
  # Display results
  if [ $test_passed -eq 1 ]; then
    if [ -n "$MIN_BANDWIDTH_GBPS" ]; then
      echo "✅ PASS (bandwidth: ${bandwidth} GB/s >= ${MIN_BANDWIDTH_GBPS} GB/s)"
    else
      echo "✅ PASS (bandwidth: ${bandwidth} GB/s)"
    fi
    return 0
  else
    if [ $exit_code -ne 0 ]; then
      echo "❌ SLOW (bandwidth: ${bandwidth} GB/s, exit code: $exit_code)"
    else
      echo "❌ SLOW (bandwidth: ${bandwidth} GB/s < ${MIN_BANDWIDTH_GBPS} GB/s)"
    fi
    return 1
  fi
}

# Recursive binary check for slow nodes
binary_check() {
  local group=("$@")

  # If single, directly record as slow
  if [ ${#group[@]} -eq 1 ]; then
    local node_name=$(get_node_name "${group[0]}")
    SLOW_NODES+=("$node_name")
    return
  fi

  # If group <= 4, test directly, no more binary search
  if [ ${#group[@]} -le 4 ]; then
    check_nodes_performance "${group[@]}"
    if [ $? -ne 0 ]; then
      for pod in "${group[@]}"; do
        local node_name=$(get_node_name "$pod")
        SLOW_NODES+=("$node_name")
      done
    fi
    return
  fi

  # Otherwise continue recursive binary search
  check_nodes_performance "${group[@]}"
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
  echo "  -b, --min-bandwidth GBPS      Minimum expected bandwidth in GB/s (optional)"
  echo "  --pod-node-mapping MAPPING    Pod-to-node mapping (format: pod1:node1,pod2:node2)"
  echo "  --mpirun-timeout SECONDS     Timeout for mpirun commands (default: 120)"
  echo "  -h, --help                    Show this help message"
  echo ""
  echo "This script uses binary search to identify slow nodes based on NCCL AllReduce performance."
  echo "The NCCL test tool has built-in bandwidth expectations and returns non-zero exit code"
  echo "if the performance requirements are not met."
  echo ""
  echo "The script will:"
  echo "  1. Test all nodes together first"
  echo "  2. If performance is insufficient, use binary search to isolate slow nodes"
  echo "  3. Report which specific nodes are causing performance issues"
  echo "  4. Display bandwidth values for both PASS and SLOW results"
  echo ""
  echo "Pass criteria:"
  echo "  - NCCL test exit code must be 0 (built-in expectations met)"
  echo "  - If --min-bandwidth is specified, actual bandwidth must >= minimum"
  echo ""
  echo "Examples:"
  echo "  $0                                    # Use only NCCL built-in expectations"
  echo "  $0 --min-bandwidth 1.0               # Require >= 1.0 GB/s bandwidth"
  echo "  $0 -b 1.5                            # Require >= 1.5 GB/s bandwidth"
  echo "  $0 --pod-node-mapping 'pod1:node1,pod2:node2'  # Use pod-to-node mapping"
  exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -b|--min-bandwidth)
      MIN_BANDWIDTH_GBPS="$2"
      shift 2
      ;;
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

# Check if NCCL test binary exists
if [ ! -f "$NCCL_TEST" ]; then
  echo "Error: NCCL test binary $NCCL_TEST not found!"
  exit 1
fi

# Read nodes from hostfile
NODES=($(awk '{print $1}' $HOSTFILE))
SLOW_NODES=()

echo "🚀 NCCL AllReduce Slow Node Detection"
echo "======================================"
echo "Hostfile: $HOSTFILE"
echo "Nodes to test: ${#NODES[@]}"
echo "NCCL test tool: $NCCL_TEST"
if [ -n "$MIN_BANDWIDTH_GBPS" ]; then
  echo "Minimum bandwidth threshold: ${MIN_BANDWIDTH_GBPS} GB/s"
else
  echo "Minimum bandwidth threshold: Using NCCL built-in expectations only"
fi
if [ -n "$POD_NODE_MAPPING" ]; then
  echo "Pod-to-node mapping: Enabled"
fi
echo ""
echo "ℹ️  The NCCL test tool has built-in bandwidth expectations."
echo "   It will return non-zero exit code if performance is insufficient."
echo "   Bandwidth values will be displayed in GB/s for all test results."
echo ""

# Start binary check
echo "🔍 Starting binary search over ${#NODES[@]} nodes..."
binary_check "${NODES[@]}"

echo
echo "=========================================="
if [ ${#SLOW_NODES[@]} -eq 0 ]; then
  echo "🎉 All nodes passed NCCL AllReduce performance check!"
  if [ -n "$MIN_BANDWIDTH_GBPS" ]; then
    echo "   All nodes meet both NCCL built-in expectations and minimum bandwidth threshold."
  else
    echo "   All nodes meet the NCCL built-in bandwidth requirements."
  fi
else
  echo "⚠️  The following nodes are slow or failed:"
  for n in "${SLOW_NODES[@]}"; do
    echo "   - $n"
  done
  echo ""
  echo "💡 Consider investigating these nodes for:"
  echo "   - Network connectivity issues"
  echo "   - Hardware performance problems"
  echo "   - Resource contention"
  echo "   - Configuration issues"
fi

echo ""
echo "📊 Performance check completed."
if [ -n "$MIN_BANDWIDTH_GBPS" ]; then
  echo "   Criteria: NCCL built-in expectations + minimum bandwidth >= ${MIN_BANDWIDTH_GBPS} GB/s"
else
  echo "   Criteria: NCCL built-in expectations only"
fi
