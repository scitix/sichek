#!/bin/bash

set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <=job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode] [hostfile] [host]
Defaults:
  job-name                = llama2-13b-bench
  namespace               = default
  cmd                     = ""
  imageRepository         = registry-us-east.scitix.ai/hisys/sichek
  imageTag                = latest
  timeout_to_complete_sec = 600
  schedulerName           = si-scheduler
  roceSharedMode          = vf
  hostfile                = None (file containing hostnames, one per line)
  host                    = None (comma-separated hostnames)
"

# 参数解析
JOB_NAME=${1:-"nccl-test-2"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR="None"
NUM_WORKERS=0
# !!! IMPORTANT: Define the exact command to run inside each pod !!!
CMD="${3:-""}"
IMAGE_REPO="${4:-"registry-us-east.scitix.ai/hisys/sichek"}"
IMAGE_TAG="${5:-"latest"}"
TIMEOUT_TO_COMPLETE=${6:-600}
SCHEDULER_NAME=${7:-"si-scheduler"}
ROCE_SHARED_MODE=${8:-"none"}
HOSTFILE=${9:-"None"}
HOST=${10:-"None"}

WORKER_POD_IDENTIFIER_STRING="worker"
MAX_PARALLEL_JOBS=200

# 使用common.sh中的函数处理hostfile和host参数
setup_host_labels "$HOSTFILE" "$HOST" "$NODE_SELECTOR"

NODE_SELECTOR_ARGS="--set nodeSelector.$NODE_SELECTOR"

# --- Initialization ---
declare -a bandwidth_values
declare -A pod_final_results # Associative array to store final results string for each pod

TMP_DIR=$(mktemp -d) # Create a temporary directory for output files
if [ ${#HOSTNAMES[@]} -gt 0 ]; then
  echo_info "Target hostnames: ${HOSTNAMES[*]}"
  NUM_WORKERS=${#HOSTNAMES[@]}
  echo_info "NUM_WORKERS auto-derived from hostnames: $NUM_WORKERS"
else
  echo_warn "No hostnames provided, exiting..."
  exit 1
fi
MPIJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

# Cleanup function to remove temp directory on exit
cleanup() {
  echo "Cleaning up Helm release: $JOB_NAME"
  helm uninstall $JOB_NAME -n $NAMESPACE || true
  kubectl delete mpijob $MPIJOB_NAME -n $NAMESPACE --ignore-not-found
  cleanup_labels  # 清理临时labels
  [[ -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo "================================================================================"
echo "Launching MPIJob '$JOB_NAME' with $NUM_WORKERS workers in namespace '$NAMESPACE'"
echo "NodeSelector: $NODE_SELECTOR"
if [ ${#HOSTNAMES[@]} -gt 0 ]; then
  echo_info "Target hostnames: ${HOSTNAMES[*]}"
fi
echo "Image: $IMAGE_REPO:$IMAGE_TAG"
echo "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "================================================================================"
echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --atomic \
  --set namespace=$NAMESPACE \
  --set mode=mpijob \
  --set schedulerName=$SCHEDULER_NAME \
  --set roceSharedMode=$ROCE_SHARED_MODE \
  $NODE_SELECTOR_ARGS \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set mpijob.name=${JOB_NAME} \
  --set mpijob.numWorkers=${NUM_WORKERS}"

echo "================================================================================"
echo "Waiting for all worker pods to be ready..."
echo "================================================================================"
sleep 5
while true; do
  kubectl wait --for=condition=Ready pod -l training.kubeflow.org/job-name=$MPIJOB_NAME -n $NAMESPACE --timeout=${TIMEOUT_TO_COMPLETE}s
  if ! kubectl get pod -n "$NAMESPACE" | grep "$MPIJOB_NAME" | grep -q Terminating; then
    break
  fi

  echo "Some pods are still Terminating... waiting again."
  sleep 5
done

# 2) 定位 launcher Pod（先从 status，再 name grep）
LAUNCHER_POD=$(
  kubectl get mpijob "$MPIJOB_NAME" -n "$NAMESPACE" \
    -o jsonpath='{.status.launcherStatus.podName}' 2>/dev/null || true
)
if [ -z "$LAUNCHER_POD" ]; then
  LAUNCHER_POD=$(
    kubectl get pods -n "$NAMESPACE" -o name \
      | grep "${MPIJOB_NAME}-launcher" \
      | head -n1 | sed 's|pods/||'
  ) || true
fi
[ -n "$LAUNCHER_POD" ] || { echo "Error: cannot find launcher Pod"; exit 1; }
echo "Found launcher pod: $LAUNCHER_POD"

# 3) 收集 Worker Pod 列表及其节点
echo
echo "Test machines (Worker Pods and their nodes):"
# 注意：MPIJob 脚本中 job-role 是 "Worker"（首字母大写）
WORKER_INFO=$(
  kubectl get pods \
  -l training.kubeflow.org/replica-type=worker,training.kubeflow.org/job-name="$MPIJOB_NAME" \
  -o 'jsonpath={range .items[*]}{.metadata.name}{" on "}{.spec.nodeName}{"\n"}{end}'
)
if [ -z "$WORKER_INFO" ]; then
  echo "  (no worker pods found)"
else
  echo "$WORKER_INFO" | sed 's/^/  - /'
fi

# 4) 定义 NCCL 基准测试命令
# MPIRUN_BASE="/usr/local/sihpc/libexec/nccl-tests/nccl_test -n 100 -w 10 -c 1 -b 8G -e 8G"
# MPIRUN_BASE="/usr/local/sihpc/libexec/nccl-tests/nccl_test -n 20 -w 5 -c 1"
MPIRUN_BASE="/usr/local/sihpc/libexec/nccl-tests/nccl_test"
COMMON_OPTS="--allow-run-as-root --map-by ppr:8:node \
  --mca oob_tcp_if_include eth0 --mca pml ^ucx \
  --mca btl self,tcp --mca btl_tcp_if_include eth0 \
  --mca routed direct --mca plm_rsh_no_tree_spawn 1"

declare -a TEST_LABELS=("all_reduce" "all_gather" "reduce_scatter" "all2all")
declare -a TEST_CMDS=(
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS -x UCX_TLS=tcp $MPIRUN_BASE"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS -x UCX_TLS=tcp $MPIRUN_BASE -lallgather"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS -x UCX_TLS=tcp $MPIRUN_BASE -lreducescatter"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS -x UCX_TLS=tcp $MPIRUN_BASE -lalltoall"
)

declare -A RESULTS

# 5) 循环执行并抓取带宽
for i in "${!TEST_LABELS[@]}"; do
  label="${TEST_LABELS[$i]}"
  cmd="${TEST_CMDS[$i]}"
  TMP_LOG="$TMP_DIR/output_${label}.txt"

  echo
  echo ">>> Running NCCL test: $label"
  echo "    Command: timeout $TIMEOUT_TO_COMPLETE $cmd  > $TMP_LOG 2>&1"
  echo

  # 使用 timeout 命令包装 mpirun 执行
  if ! kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "timeout $TIMEOUT_TO_COMPLETE $cmd" > $TMP_LOG 2>&1 ; then
    echo "WARNING: $cmd failed or timed out after $TIMEOUT_TO_COMPLETE seconds"
    tail -n 20 $TMP_LOG
  fi
  output=$(cat $TMP_LOG)
  
  # 检查是否因为超时而失败
  if echo "$output" | grep -q "timeout: command terminated"; then
    echo "ERROR: Command timed out after $TIMEOUT_TO_COMPLETE seconds"
    RESULTS["$label"]="TIMEOUT"
    exit 1
  fi
  
  # 打印原始输出
  echo "$output" | grep "Avg bus bandwidth"

  # 提取 Avg bus bandwidth
  bw=$(echo "$output" \
    | grep -E "Avg bus bandwidth" \
    | sed -E 's/.*: *([0-9]+\.[0-9]+).*/\1/')

  if [ -z "$bw" ]; then
    echo "Warning: 未能解析 '$label' 带宽，记作 0"
    bw=0
  fi
  RESULTS["$label"]="$bw"
done

# 6) 打印汇总
echo
echo "========== NCCL Benchmark Summary for $MPIJOB_NAME =========="
printf "\n%-20s %10s\n" "Test" "GB/s"
echo "-------------------- ----------"
for label in "${TEST_LABELS[@]}"; do
  printf "%-20s %10s\n" "$label" "${RESULTS[$label]}"
done
echo "========================================================="

echo "========================================================================="
echo "Stop Job '$JOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="