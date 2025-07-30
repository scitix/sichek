#!/bin/bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode]
Defaults:
  job-name                = nccl-diag-bisect
  namespace               = default
  nodeSelector            = sichek=test
  numWorkers              = 2
  cmd                     = bash /var/sichek/scripts/check_bad_nodes.sh
  imageRepository         = registry-cn-shanghai.siflow.cn/hisys/sichek
  imageTag                = v0.5.4
  timeout_to_complete_sec = 600
  schedulerName           = sischeduler
  macvlan                 = false
"

# 参数解析
JOB_NAME=${1:-"nccl-diag-bisect"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"sichek=test"}
NUM_WORKERS=${4:-2}
CMD=${5:-"bash /var/sichek/scripts/check_bad_nodes.sh"}
IMAGE_REPO=${6:-"registry-cn-shanghai.siflow.cn/hisys/sichek"}
IMAGE_TAG=${7:-"v0.5.4"}
TIMEOUT_TO_COMPLETE=${8:-600}
SCHEDULER_NAME=${9:-"sischeduler"}
MACVLAN=${10:-"false"}

# 将 nodeSelector 解析为 key=value
NODE_SELECTOR=$(echo "$NODE_SELECTOR" | sed 's/\./\\\\./g')
NODE_SELECTOR_KEY=$(cut -d= -f1 <<< "$NODE_SELECTOR")
NODE_SELECTOR_VAL=$(cut -d= -f2 <<< "$NODE_SELECTOR")

echo "========================================================================="
echo_info "Starting MPIJob '$JOB_NAME' in namespace '$NAMESPACE'..."
echo_info "NodeSelector: $NODE_SELECTOR"
echo_info "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "========================================================================="

echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
 	--atomic \
  --timeout "${TIMEOUT_TO_COMPLETE}s" \
  --namespace $NAMESPACE \
  --set mode=mpijob \
  --set schedulerName=$SCHEDULER_NAME \
  --set macvlan=$MACVLAN \
  --set nodeSelector.${NODE_SELECTOR_KEY}=${NODE_SELECTOR_VAL} \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set mpijob.name=${JOB_NAME} \
  --set mpijob.numWorkers=${NUM_WORKERS} || echo \"Helm failed, continue anyway\""

MPIJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

cleanup() {
  echo "Cleaning up : $JOB_NAME"
  echo_back "helm uninstall $JOB_NAME"
  echo_back "kubectl delete mpijob $MPIJOB_NAME -n $NAMESPACE --ignore-not-found"
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

echo "========================================================================="
echo_info "🚀 Starting NCCL diagnostics using binary search strategy..."
echo "========================================================================="
kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "$CMD"


echo "========================================================================="
echo_info "Stop MPIJob '$MPIJOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="