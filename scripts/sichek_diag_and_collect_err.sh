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
  nodeSelector            = scitix.ai/gpu-type: h20xnvlink141
  numWorkers              = 2
  cmd                     = sichek all -e -I podlog,gpuevents,nccltest
  imageRepository         = registry-cn-shanghai.siflow.cn/hisys/sichek
  imageTag                = v0.5.4
  defaultSpec             = hercules_spec.yaml
  timeout_to_complete_sec = 600
"

# 参数解析
JOB_NAME=${1:-"diag"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"None"}
NUM_WORKERS=${4:-2}
CMD=${5:-"sichek all -e -I podlog,gpuevents,nccltest"}
IMAGE_REPO=${6:-"registry-cn-shanghai.siflow.cn/hisys/sichek"}
IMAGE_TAG=${7:-"v0.5.4"}
DEFAULT_SPEC=${8:-"hercules_spec.yaml"}
TIMEOUT_TO_COMPLETE=${9:-600}

# 将 nodeSelector 解析为 key=value
NODE_SELECTOR_ARGS=""
if [ "$NODE_SELECTOR" != "None" ]; then
  # 对 key 中的 . 进行转义
  ESCAPED_NODE_SELECTOR=$(echo "$NODE_SELECTOR" | sed 's/\./\\\\./g')
  NODE_SELECTOR_KEY=$(cut -d= -f1 <<< "$ESCAPED_NODE_SELECTOR")
  NODE_SELECTOR_VAL=$(cut -d= -f2 <<< "$ESCAPED_NODE_SELECTOR")
  NODE_SELECTOR_ARGS="--set nodeSelector.\"${NODE_SELECTOR_KEY}\"=${NODE_SELECTOR_VAL}"
fi
ESCAPED_CMD=${CMD//,/\\,}

echo "========================================================================="
echo_info "Starting Batchjob '$JOB_NAME' in namespace '$NAMESPACE' to install sichek..."
echo_info "NodeSelector: $NODE_SELECTOR, NodeNumber: $NUM_WORKERS"
echo_info "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "========================================================================="

HELM_FAILED=0
echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --namespace $NAMESPACE \
 	--atomic \
  --timeout "${TIMEOUT_TO_COMPLETE}s" \
  --set mode=diag \
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
  echo "✅ sichek [$CMD] PASSED."
  exit 0
fi

for pod in $PODS; do
  NODE=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.spec.nodeName}')
  echo -e "\n❌ $NODE Failed (Logs from pod $pod)"
  kubectl logs -n "$NAMESPACE" "$pod" |tail -n 5 || echo "  [!] Failed to fetch logs from $pod"
done
echo "❌ sichek [$CMD] FAILED."


echo "========================================================================="
echo_info "Stop Job '$DIAGJOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="