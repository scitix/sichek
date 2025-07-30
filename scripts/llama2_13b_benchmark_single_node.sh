#!/bin/bash

set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 <=job-name> [namespace] [nodeSelector] [num-workers] [cmd] [image-repo] [image-tag] [timeout_to_complete_sec] [rdma_mode]
Defaults:
  job-name                = llama2-13b-bench
  namespace               = default
  nodeSelector            = sichek=test
  numWorkers              = 2
  cmd                     = TP=2 PP=1 GBS=256 MBS=1 MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true ENABLE_CKPT=0 LOG_INTERVAL=1 EVAL_INTERVAL=200 bash /workspace/Megatron-LM/examples/llama/train_llama2_13b_bf16.sh
  imageRepository         = registry-cn-shanghai.siflow.cn/hpc/ngc_pytorch
  imageTag                = 24.06-sicl-0723
  timeout_to_complete_sec = 600
  schedulerName           = sischeduler
  macvlan                 = false
"

# 参数解析
JOB_NAME=${1:-"llama2-13b-bench"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"xlliu-test=test"}
NUM_WORKERS=${4:-2}
# !!! IMPORTANT: Define the exact command to run inside each pod !!!
CMD="${5:-"TP=2 PP=1 GBS=256 MBS=1 MAX_STEPS=4 EVAL_ITERS=1 MOCK_DATA=true ENABLE_CKPT=0 LOG_INTERVAL=1 EVAL_INTERVAL=200 bash /workspace/Megatron-LM/examples/llama/train_llama2_13b_bf16.sh"}"
IMAGE_REPO="${6:-"registry-cn-shanghai.siflow.cn/hpc/ngc_pytorch"}"
IMAGE_TAG="${7:-"24.06-sicl-0723"}"
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

# Cleanup function to remove temp directory on exit
cleanup() {
  echo_info "Cleaning up Helm release: $JOB_NAME"
  echo_back "helm uninstall $JOB_NAME -n $NAMESPACE || true"
  echo_back "kubectl delete mpijob $MPIJOB_NAME -n $NAMESPACE --ignore-not-found"
  [[ -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo "================================================================================"
echo_info "Launching MPIJob '$JOB_NAME' with $NUM_WORKERS workers in namespace '$NAMESPACE'"
echo_info "Tests will be run in parallel (up to $MAX_PARALLEL_JOBS concurrently)."
echo_info "Command to be executed in each pod: $CMD"
echo_info "Temporary output files will be stored in: $TMP_DIR"
echo_info "NodeSelector: $NODE_SELECTOR"
echo_info "Image: $IMAGE_REPO:$IMAGE_TAG"
echo_info "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "================================================================================"
echo_back "helm upgrade --install $JOB_NAME $SICHEK_HELM_DIR \
  --atomic \
  --namespace $NAMESPACE \
  --set mode=mpijob \
  --set schedulerName=$SCHEDULER_NAME \
  --set macvlan=$MACVLAN \
  --set nodeSelector.${NODE_SELECTOR_KEY}=${NODE_SELECTOR_VAL} \
  --set image.repository=${IMAGE_REPO} \
  --set image.tag=${IMAGE_TAG} \
  --set mpijob.name=${JOB_NAME} \
  --set mpijob.numWorkers=${NUM_WORKERS} \
  --set mpijob.cmd=\"${CMD}\" || echo \"Helm failed, continue anyway\""

MPIJOB_NAME="sichek-${JOB_NAME}-${NUM_WORKERS}"

echo "================================================================================"
echo_info "Waiting for all worker pods to be ready..."
echo "================================================================================"
sleep 5
while true; do
  echo_back "kubectl wait --for=condition=Ready pod -l training.kubeflow.org/job-name=$MPIJOB_NAME -n $NAMESPACE --timeout=${TIMEOUT_TO_COMPLETE}s"
  if ! kubectl get pod -n "$NAMESPACE" | grep "$MPIJOB_NAME" | grep -q Terminating; then
    break
  fi

  echo_info "Some pods are still Terminating... waiting again."
  sleep 5
done

# --- Discover Worker Pods ---
POD_LIST_RAW=$(kubectl get pods -n "$NAMESPACE" -o wide --no-headers=true | grep ${MPIJOB_NAME} |awk -v identifier="$WORKER_POD_IDENTIFIER_STRING" '$1 ~ identifier {print $1}')

if [ -z "$POD_LIST_RAW" ]; then
  echo_warn "No worker pods found containing '$WORKER_POD_IDENTIFIER_STRING' in namespace '$NAMESPACE'."
  exit 1
fi

# Convert POD_LIST_RAW to an array
readarray -t POD_ARRAY <<<"$POD_LIST_RAW"
TOTAL_PODS_FOUND=${#POD_ARRAY[@]}
echo_info "Found $TOTAL_PODS_FOUND pod(s) to test."

# --- Arrays to store job information ---
declare -a pids_array
declare -a pod_names_array
declare -a host_ips_array
declare -a host_names_array
declare -a temp_files_array

# --- Launch tests in parallel ---
echo "================================================================================"
echo_info "Launching tests..."
echo "================================================================================"
job_count=0
for POD_NAME in "${POD_ARRAY[@]}"; do
  HOST_IP=$(kubectl get pod -n "$NAMESPACE" "$POD_NAME" -o jsonpath='{.status.hostIP}' 2>/dev/null || echo "N/A")
  HOST_NAME=$(kubectl get pod -n "$NAMESPACE" "$POD_NAME" -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "N/A")
  TEMP_FILE="$TMP_DIR/${POD_NAME}_output.txt"

  # Store info before launching
  pod_names_array+=("$POD_NAME")
  host_ips_array+=("$HOST_IP")
  host_names_array+=("$HOST_NAME")
  temp_files_array+=("$TEMP_FILE")

  # Launch kubectl exec in the background
  ( kubectl exec -n "$NAMESPACE" "$POD_NAME" -- bash -c "$CMD" > "$TEMP_FILE" 2>&1 ) &
  pids_array+=($!) # Store the PID of the background kubectl exec

  echo_info "  Launched test for $POD_NAME (PID ${pids_array[-1]}), output to $TEMP_FILE"

  # Limit concurrency
  job_count=$((job_count + 1))
  if (( job_count % MAX_PARALLEL_JOBS == 0 )); then
    echo_info "  Reached max parallel jobs ($MAX_PARALLEL_JOBS), waiting for some to complete..."
    # Wait for the oldest $MAX_PARALLEL_JOBS jobs to avoid too many open PIDs
    # More sophisticated job management could be used here (e.g., `jobs -p` and specific waits)
    # For simplicity, this waits for *any* job to complete if we used `wait -n` (bash 4.3+)
    # Or simply wait for all launched so far in this batch
    for ((idx = job_count - MAX_PARALLEL_JOBS; idx < job_count; idx++)); do
        wait "${pids_array[idx]}" # Wait for specific PIDs in the current batch
    done
    echo_info "  Some jobs completed, continuing..."
  fi
done

# Wait for any remaining background jobs to complete
echo "================================================================================"
echo_info "Waiting for all remaining launched tests to complete..."
echo "================================================================================"
for pid in "${pids_array[@]}"; do
  wait "$pid"
done
echo_info "All tests completed."
echo "------------------------------------------------------------------------------------"
echo "Processing TFLOP/s/GPU results..."
printf "%-40s | %-15s | %-15s | %-10s | %-10s | %-10s | %-10s\n" "Pod Name" "Host NAME" "Host IP" "Avg" "Min" "Max" "StdDev"
echo "-------------------------------------------------------------------------------------------------------------"

ACTUAL_PARSED_COUNT=0

# Function: compute stddev from a list of numbers
compute_stddev() {
  local values=("$@")
  local sum=0
  local count=${#values[@]}

  for val in "${values[@]}"; do
    sum=$(echo "$sum + $val" | bc -l)
  done
  local avg=$(echo "scale=6; $sum / $count" | bc -l)

  local sumsq=0
  for val in "${values[@]}"; do
    diff=$(echo "$val - $avg" | bc -l)
    sumsq=$(echo "$sumsq + ($diff * $diff)" | bc -l)
  done

  stddev=$(echo "scale=6; sqrt($sumsq / $count)" | bc -l)
  echo "$stddev"
}

for i in "${!pod_names_array[@]}"; do
  POD_NAME="${pod_names_array[i]}"
  HOST_IP="${host_ips_array[i]}"
  HOST_NAME="${host_names_array[i]}"
  TEMP_FILE="${temp_files_array[i]}"
  TFLOPS_VALUES=()

  if [ -f "$TEMP_FILE" ] && [ -s "$TEMP_FILE" ]; then
    # 去除不可见字符（如颜色码）
    OUTPUT=$(cat "$TEMP_FILE" | sed -r "s/\x1B\[[0-9;]*[mGK]//g")

    # 提取所有 TFLOP/s/GPU 数值
    while IFS= read -r line; do
      # 使用更宽松的正则匹配 TFLOP 值
      val=$(echo "$line" | awk -F'throughput per GPU \\(TFLOP/s/GPU\\):' '{if (NF>1) print $2}' | awk '{print $1}')

      if [[ -n "$val" && "$val" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        TFLOPS_VALUES+=("$val")
    fi
    done <<< "$OUTPUT"

    if [ ${#TFLOPS_VALUES[@]} -gt 0 ]; then
      ACTUAL_PARSED_COUNT=$((ACTUAL_PARSED_COUNT + 1))
      # 统计
      sum=0
      min=${TFLOPS_VALUES[0]}
      max=${TFLOPS_VALUES[0]}
      for v in "${TFLOPS_VALUES[@]}"; do
        sum=$(echo "$sum + $v" | bc -l)
        if (( $(echo "$v < $min" | bc -l) )); then min=$v; fi
        if (( $(echo "$v > $max" | bc -l) )); then max=$v; fi
      done
      avg=$(echo "scale=3; $sum / ${#TFLOPS_VALUES[@]}" | bc -l)
      stddev=$(compute_stddev "${TFLOPS_VALUES[@]}")

      printf "%-40s | %-15s | %-15s | %-10s | %-10s | %-10s | %-10s\n" "$POD_NAME" "$HOST_NAME" "$HOST_IP" "$avg" "$min" "$max" "$stddev"
    else
      printf "%-40s | %-15s | %-15s | %-10s | %-10s | %-10s | %-10s\n" "$POD_NAME" "$HOST_NAME" "$HOST_IP" "N/A" "N/A" "N/A" "N/A"
    fi
  else
    printf "%-40s | %-15s | %-15s | %-10s | %-10s | %-10s | %-10s\n" "$POD_NAME" "$HOST_NAME" "$HOST_IP" "NoLog" "NoLog" "NoLog" "NoLog"
  fi
done

echo "========================================================================="
echo_info "Stop Job '$JOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "========================================================================="