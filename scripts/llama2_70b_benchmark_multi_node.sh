#!/bin/bash
set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode]
Defaults:
  job-name                = llama2-13b-bench
  namespace               = default
  nodeSelector            = sichek=test
  numWorkers              = 2
  cmd                     = MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true ENABLE_CKPT=0 LOG_INTERVAL=1 EVAL_INTERVAL=200 bash /workspace/Megatron-LM/examples/llama/train_llama2_70b_bf16.sh
  imageRepository         = registry-cn-shanghai.siflow.cn/hpc/ngc_pytorch
  imageTag                = 24.06-sicl-0723
  timeout_to_complete_sec = 600
  schedulerName           = sischeduler
  macvlan                 = false
"

# 参数解析
JOB_NAME=${1:-"llama2-70b-bench"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"xlliu-test=test"}
NUM_WORKERS=${4:-2}
CMD="${5:-"MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true ENABLE_CKPT=0 LOG_INTERVAL=1 EVAL_INTERVAL=200 bash /workspace/Megatron-LM/examples/llama/train_llama2_70b_bf16.sh"}"
IMAGE_REPO="${6:-"registry-cn-shanghai.siflow.cn/hpc/ngc_pytorch"}"
IMAGE_TAG="${7:-"24.06-sicl-0723"}"
TIMEOUT_TO_COMPLETE=${8:-600}
SCHEDULER_NAME=${9:-"sischeduler"}
MACVLAN=${10:-"false"}

# 将 nodeSelector 解析为 key=value
NODE_SELECTOR=$(echo "$NODE_SELECTOR" | sed 's/\./\\\\./g')
NODE_SELECTOR_KEY=$(cut -d= -f1 <<< "$NODE_SELECTOR")
NODE_SELECTOR_VAL=$(cut -d= -f2 <<< "$NODE_SELECTOR")

echo "========================================================================="
echo_info "Starting PyTorchJob '$JOB_NAME' in namespace '$NAMESPACE'..."
echo "========================================================================="

echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --atomic \
  --namespace $NAMESPACE \
  --set mode=pytorchjob \
  --set schedulerName=$SCHEDULER_NAME \
  --set macvlan=$MACVLAN \
  --set nodeSelector.${NODE_SELECTOR_KEY}=${NODE_SELECTOR_VAL} \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set pytorchjob.name=${JOB_NAME} \
  --set pytorchjob.numWorkers=${NUM_WORKERS} \
  --set pytorchjob.cmd=\"${CMD}\"  || echo \"Helm failed, continue anyway\""

PYTORCHJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

cleanup() {
  echo "Cleaning up : $JOB_NAME"
  echo_back "helm uninstall $JOB_NAME"
  echo_back "kubectl delete pytorchjob $PYTORCHJOB_NAME -n $NAMESPACE --ignore-not-found"
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo "========================================================================="
echo_info "Waiting for pytorchjob $PYTORCHJOB_NAME to enter 'Running' state."
echo "========================================================================="
timeout=300
interval=10
elapsed=0
while (( elapsed < timeout )); do
    status=$(kubectl get pytorchjob "$PYTORCHJOB_NAME" -n $NAMESPACE | grep -v NAME | awk '{print $2}')
    if [[ "$status" == "Running" ]]; then
        echo_info "Pytorchjob $PYTORCHJOB_NAME are in Running state."
        break
    else
        echo_info "Pytorchjob $PYTORCHJOB_NAME is not in Running state yet. Retrying..."
        echo_back "sleep $interval"
        (( elapsed += interval ))
    fi
done
if (( elapsed >= timeout )); then
    echo_warn "Timeout Waiting for pytorchjob $PYTORCHJOB_NAME to reach Running state."
fi

# 获取最后一个 worker pod（按名称排序）
LAST_POD=$(kubectl get pod -n "$NAMESPACE" -l "training.kubeflow.org/replica-type=worker" |grep $PYTORCHJOB_NAME \
  | awk '{print $1}' | sort -V | tail -n 1)

if [[ -z "$LAST_POD" ]]; then
  echo_warn "❌ No worker pods found for job '$PYTORCHJOB_NAME' in namespace '$NAMESPACE'"
  exit 1
fi
echo_info "Last worker pod name: $LAST_POD"

echo "========================================================================="
echo_info "Waiting for PyTorchJob $PYTORCHJOB_NAME to complete..."
echo "========================================================================="
timeout=$TIMEOUT_TO_COMPLETE
interval=10
elapsed=0
while (( elapsed < timeout )); do
    STATUS=$(kubectl get pytorchjob "$PYTORCHJOB_NAME" -n $NAMESPACE | grep -v NAME |awk '{print $2}')
    if [[ "$STATUS" == "Succeeded" || "$STATUS" == "Failed" ]]; then
    	echo_info "PyTorchjob Status: $STATUS"
        break
    fi
    STATUS=$(kubectl get pod "$LAST_POD" -n $NAMESPACE | grep -v NAME |awk '{print $3}')
    if [[ "$STATUS" == "Running" ]]; then
    	LAST_LOG=$(kubectl logs -n $NAMESPACE $LAST_POD | tail -n 1)
    	echo_info "$LAST_LOG"
    fi
    echo_back "sleep $interval"
    (( elapsed += interval ))
done
if (( elapsed >= timeout )); then
    echo_warn "Timeout waiting for pytorchjob $PYTORCHJOB_NAME to reach complete state."
fi

echo "========================================================================="
echo_info "Fetching Pod Logs $PYTORCHJOB_NAME and Parsing TFLOPS values..."
echo "========================================================================="
# 获取 TFLOP/s/GPU 日志条目
TFLOPS=$(kubectl logs -n "$NAMESPACE" "$LAST_POD" 2>/dev/null | grep -oP 'throughput per GPU \(TFLOP/s/GPU\):\s*\K[0-9]+(\.[0-9]+)?')

if [[ -z "$TFLOPS" ]]; then
  echo_warn "❌ No 'TFLOP/s/GPU' entries found in logs for pod '$LAST_POD'"
  exit 1
fi

# 打印表头
printf "%-30s | %-9s | %-9s | %-9s | %-9s\n" "Job Name" "Avg" "Min" "Max" "StdDev"

# 使用 awk 统计 TFLOPS 值
echo "$TFLOPS" | awk -v job="$PYTORCHJOB_NAME" '
{
  sum += $1; count += 1;
  if (min == "" || $1 < min) min = $1;
  if (max == "" || $1 > max) max = $1;
  values[count] = $1;
}
END {
  if (count == 0) {
    printf "%-30s | %-9s | %-9s | %-9s | %-9s\n", job, "N/A", "N/A", "N/A", "N/A";
    exit;
  }

  avg = sum / count;
  for (i = 1; i <= count; i++) {
    stddev_sum += (values[i] - avg)^2;
  }
  stddev = sqrt(stddev_sum / count);
  printf "%-30s | %-9.2f | %-9.2f | %-9.2f | %-9.2f\n", job, avg, min, max, stddev;
}'

echo "========================================================================="
echo_info "Stop PyTorchJob '$PYTORCHJOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="