#!/bin/bash
set -euo pipefail

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <=job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode]
Defaults:
  job-name                = llama2-13b-bench
  namespace               = default
  nodeSelector            = sichek=test
  numWorkers              = 2
  cmd                     = ""
  imageRepository         = registry-cn-shanghai.siflow.cn/hisys/sichek
  imageTag                = v0.5.4
  timeout_to_complete_sec = 600
  schedulerName           = sischeduler
  macvlan                 = false
"

# 参数解析
JOB_NAME=${1:-"nccl-test-2"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"sichek=test"}
NUM_WORKERS=${4:-2}
# !!! IMPORTANT: Define the exact command to run inside each pod !!!
CMD="${5:-""}"
IMAGE_REPO="${6:-"registry-cn-shanghai.siflow.cn/hisys/sichek"}"
IMAGE_TAG="${7:-"v0.5.4"}"
TIMEOUT_TO_COMPLETE=${8:-600}
SCHEDULER_NAME=${9:-"sischeduler"}
MACVLAN=${10:-"false"}

WORKER_POD_IDENTIFIER_STRING="worker"
MAX_PARALLEL_JOBS=200

NODE_SELECTOR=$(echo "$NODE_SELECTOR" | sed 's/\./\\./g')
NODE_SELECTOR_KEY=$(cut -d= -f1 <<< "$NODE_SELECTOR")
NODE_SELECTOR_VAL=$(cut -d= -f2 <<< "$NODE_SELECTOR")

# --- Initialization ---
declare -a bandwidth_values
declare -A pod_final_results # Associative array to store final results string for each pod

TMP_DIR=$(mktemp -d) # Create a temporary directory for output files
MPIJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

# Cleanup function to remove temp directory on exit
cleanup() {
  echo "Cleaning up Helm release: $JOB_NAME"
  helm uninstall $JOB_NAME -n $NAMESPACE || true
  kubectl delete mpijob $MPIJOB_NAME -n $NAMESPACE --ignore-not-found
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
echo "Image: $IMAGE_REPO:$IMAGE_TAG"
echo "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "================================================================================"
helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --atomic \
  --namespace $NAMESPACE \
  --set mode=mpijob \
  --set schedulerName=$SCHEDULER_NAME \
  --set macvlan=$MACVLAN \
  --set nodeSelector.${NODE_SELECTOR_KEY}=${NODE_SELECTOR_VAL} \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set mpijob.name=${JOB_NAME} \
  --set mpijob.numWorkers=${NUM_WORKERS}

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
# 注意：MPIJob 脚本中 job-role 是 “Worker”（首字母大写）
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
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS $MPIRUN_BASE"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS $MPIRUN_BASE -lallgather"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS $MPIRUN_BASE -lreducescatter"
  "/usr/local/sihpc/bin/mpirun $COMMON_OPTS $MPIRUN_BASE -lall2all"
)

declare -A RESULTS

# 5) 循环执行并抓取带宽
for i in "${!TEST_LABELS[@]}"; do
  label="${TEST_LABELS[$i]}"
  cmd="${TEST_CMDS[$i]}"
  TMP_LOG="$TMP_DIR/output.txt"

  echo
  echo ">>> Running NCCL test: $label"
  echo "    Command: $cmd  > $TMP_LOG 2>&1"
  echo

  if ! kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "$cmd" > $TMP_LOG 2>&1 ; then
    echo "WARNING: $cmd failed"
    tail -n 20 $TMP_LOG
  fi
  output=$(cat $TMP_LOG)
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