#!/bin/bash

set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [default-spec] [timeout_to_complete_sec]
Defaults:
  job-name                = nccl-diag-bisect
  namespace               = default
  cmd                     = sichek all -e -I podlog,gpuevents,nccltest
  imageRepository         = registry-us-east.scitix.ai/hisys/sichek
  imageTag                = latest
  defaultSpec             = hercules_spec.yaml
  timeout_to_complete_sec = 600
  cpu                     = false
  hostfile                = None (file containing hostnames, one per line)
  host                    = None (comma-separated hostnames)
"

# 参数解析
JOB_NAME=${1:-"diag"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR="None"
NUM_WORKERS=0
CMD=${3:-"sleep 10 && sichek all -e -I podlog,gpuevents,nccltest"}
IMAGE_REPO=${4:-"registry-us-east.scitix.ai/hisys/sichek"}
IMAGE_TAG=${5:-"latest"}
DEFAULT_SPEC=${6:-"hercules_spec.yaml"}
TIMEOUT_TO_COMPLETE=${7:-600}
CPU=${8:-false}
if [ "$CPU" != "true" ]; then
  GPU="true"
else
  GPU="false"
fi

HOSTFILE=${9:-"None"}
HOST=${10:-"None"}

# 使用common.sh中的函数处理hostfile和host参数
setup_host_labels "$HOSTFILE" "$HOST" "$NODE_SELECTOR"
# 将 nodeSelector 解析为 key=value
NODE_SELECTOR_ARGS="--set nodeSelector.$NODE_SELECTOR"

ESCAPED_CMD=${CMD//,/\\,}

echo "========================================================================="
echo_info "Starting Batchjob '$JOB_NAME' in namespace '$NAMESPACE' to install sichek..."
echo_info "NodeSelector: $NODE_SELECTOR, NodeNumber: $NUM_WORKERS"
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

HELM_FAILED=0
echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --set namespace=$NAMESPACE \
 	--atomic \
  --timeout "${TIMEOUT_TO_COMPLETE}s" \
  --set mode=diag \
  --set batchjob.gpu=$GPU \
  --set batchjob.name=\"${JOB_NAME}\" \
  --set batchjob.cmd=\"${ESCAPED_CMD}\" \
  --set defaultSpec="$DEFAULT_SPEC" \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set batchjob.completions=$NUM_WORKERS \
  --set batchjob.parallelism=$NUM_WORKERS $NODE_SELECTOR_ARGS  || HELM_FAILED=1 "

DIAGJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"
cleanup() {
  echo "Cleaning up : $JOB_NAME"
  echo_back "helm uninstall $JOB_NAME"
  echo_back "kubectl delete job $DIAGJOB_NAME -n $NAMESPACE --ignore-not-found"
  # 清理临时labels
  cleanup_labels
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo "========================================================================="
echo_info "Waiting for Job $DIAGJOB_NAME to complete..."
echo "========================================================================="
while true; do
  if ! kubectl get pod -n "$NAMESPACE" | grep "$DIAGJOB_NAME" | grep -qEiv "Completed|Error"; then
    break
  fi

  echo_info "Not all pods complete... waiting again."
  sleep 5
done

echo "========================================================================="
echo_info "Fetching failed pod list (excluding Completed)...."
echo "========================================================================="

PODS=$(kubectl get pods -n "$NAMESPACE" \
  -l job-name="$DIAGJOB_NAME" \
  --field-selector=status.phase!=Succeeded \
  -o custom-columns=NAME:.metadata.name --no-headers)

if [ "$HELM_FAILED" -eq 0 ] && [ -z "$PODS" ]; then
  echo "========================================================================="
  kubectl get pods -n "$NAMESPACE" -l job-name="$DIAGJOB_NAME" -o custom-columns=NAME:.metadata.name --no-headers
  echo "✅ sichek [$CMD] PASSED."
  echo "=========================================================================="
  exit 0
fi

for pod in $PODS; do
  NODE=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.spec.nodeName}')
  echo -e "\n❌ $NODE Failed (Logs from pod $pod)"
  kubectl logs -n "$NAMESPACE" "$pod" |tail -n 50 || echo "  [!] Failed to fetch logs from $pod"
done

echo "========================================================================="
echo "❌ sichek [$CMD] FAILED, FAILED number of pods: $(echo "$PODS" | wc -l)"
echo "   FAILED number: $(echo "$PODS" | wc -l)"
echo "========================================================================="

echo "========================================================================="
echo_info "Stop Job '$DIAGJOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="