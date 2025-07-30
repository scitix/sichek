#!/bin/bash

HOSTFILE=/etc/mpi/hostfile
NCCL_TEST=/usr/local/sihpc/libexec/nccl-tests/nccl_test
INTERFACE=eth0
SLOTS_PER_NODE=8
TMP_LOG=/tmp/nccl_check.log

NODES=($(awk '{print $1}' $HOSTFILE))
BAD_NODES=()

# Function to test a subset of nodes
check_nodes() {
  local subset=("$@")

  local host_list=()
  for h in "${subset[@]}"; do
    host_list+=("${h}:${SLOTS_PER_NODE}")
  done

  local hosts=$(IFS=','; echo "${host_list[*]}")
  echo -n "Checking: ${hosts} ... "

  /usr/local/sihpc/bin/mpirun \
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
    BAD_NODES+=("${group[0]}")
    return
  fi

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

# Start binary check
echo "🔍 Starting binary search over ${#NODES[@]} nodes..."
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