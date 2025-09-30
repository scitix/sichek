#!/bin/bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode] [hostfile] [host] [diag-mode]
Defaults:
  job-name                = nccl-diag-bisect
  namespace               = default
  cmd                     = bash /var/sichek/scripts/check_bad_nodes.sh
  imageRepository         = registry-us-east.scitix.ai/hisys/sichek
  imageTag                = latest
  timeout_to_complete_sec = 600
  schedulerName           = si-scheduler
  roceSharedMode          = vf
  hostfile                = None (file containing hostnames, one per line)
  host                    = None (comma-separated hostnames)
  diagMode                = conn (conn: connectivity diag, perf: performance diag)
"

# 参数解析
JOB_NAME=${1:-"nccl-diag-bisect"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR="None"
NUM_WORKERS=0
CMD=${3:-"bash /var/sichek/scripts/nccltest-diag-bisect.sh"}
IMAGE_REPO=${4:-"registry-us-east.scitix.ai/hisys/sichek"}
IMAGE_TAG=${5:-"latest"}
TIMEOUT_TO_COMPLETE=${6:-120}
SCHEDULER_NAME=${7:-"si-scheduler"}
ROCE_SHARED_MODE=${8:-"none"}
HOSTFILE=${9:-"None"}
HOST=${10:-"None"}
DIAG_MODE=${11:-"conn"}

# 使用common.sh中的函数处理hostfile和host参数
setup_host_labels "$HOSTFILE" "$HOST" "$NODE_SELECTOR"

echo "========================================================================="
echo_info "Starting MPIJob '$JOB_NAME' in namespace '$NAMESPACE'..."
echo_info "NodeSelector: $NODE_SELECTOR"
if [ ${#HOSTNAMES[@]} -gt 0 ]; then
  echo_info "Target hostnames: ${HOSTNAMES[*]}"
  NUM_WORKERS=${#HOSTNAMES[@]}
  echo_info "NUM_WORKERS auto-derived from hostnames: $NUM_WORKERS"
else
  echo_warn "No hostnames provided, exiting..."
  exit 1
fi
echo_info "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "========================================================================="

echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
 	--atomic \
  --timeout "${TIMEOUT_TO_COMPLETE}s" \
  --set namespace=$NAMESPACE \
  --set mode=mpijob \
  --set schedulerName=$SCHEDULER_NAME \
  --set roceSharedMode=$ROCE_SHARED_MODE \
  --set nodeSelector.${NODE_SELECTOR} \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set mpijob.name=${JOB_NAME} \
  --set mpijob.numWorkers=${NUM_WORKERS} || echo \"Helm failed, continue anyway\""

MPIJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

cleanup() {
  echo "Cleaning up : $JOB_NAME"
  echo_back "helm uninstall $JOB_NAME"
  echo_back "kubectl delete mpijob $MPIJOB_NAME -n $NAMESPACE --ignore-not-found"
  cleanup_labels  # 清理临时labels
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo "========================================================================="
echo_info "Waiting for MPIJob $MPIJOB_NAME to enter 'Running' state."
echo "========================================================================="
sleep 5
while true; do
  echo_back "kubectl wait --for=condition=Ready pod -l training.kubeflow.org/job-name=$MPIJOB_NAME -n $NAMESPACE --timeout=${TIMEOUT_TO_COMPLETE}s"
  if ! kubectl get pod -n "$NAMESPACE" | grep "$MPIJOB_NAME" | grep -q Terminating; then
    break
  fi

  echo_info "Some pods are still Terminating... waiting again."
  sleep 5
done

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

# 获取pod到node的映射关系
echo "========================================================================="
echo_info "🔍 Getting pod-to-node mapping..."
echo "========================================================================="

# 获取所有worker pods及其对应的节点
POD_NODE_MAPPING=""
WORKER_PODS=$(kubectl get pods -n "$NAMESPACE" -l training.kubeflow.org/job-name="$MPIJOB_NAME" \
  -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep -v 'launcher')
[ -n "$WORKER_PODS" ] || { echo "Error: cannot find worker Pod"; exit 1; }
for pod in $WORKER_PODS; do
  node_name=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.spec.nodeName}')
  if [ -n "$node_name" ]; then
    if [ -n "$POD_NODE_MAPPING" ]; then
      POD_NODE_MAPPING="${POD_NODE_MAPPING},${pod}:${node_name}"
    else
      POD_NODE_MAPPING="${pod}:${node_name}"
    fi
    echo_info "Pod: $pod -> Node: $node_name"
  fi
done

echo "========================================================================="
echo_info "🚀 Starting NCCL diagnostics using binary search strategy..."
echo "========================================================================="

# 根据diag-mode选择对应的脚本
if [ "$DIAG_MODE" == "perf" ]; then
  CMD="bash /var/sichek/scripts/nccltest-allreduce-perf-bisect.sh"
fi

# 如果有pod-node映射，将其作为参数传递给bisect脚本
if [ -n "$POD_NODE_MAPPING" ]; then
  CMD="${CMD} --pod-node-mapping '${POD_NODE_MAPPING}'"
fi
# 添加mpirun timeout参数
CMD="${CMD} --mpirun-timeout ${TIMEOUT_TO_COMPLETE}"

echo_info "Executing command: $CMD"
kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "$CMD"

echo "========================================================================="
echo_info "Stop MPIJob '$MPIJOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="
