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
JOB_NAME=${1:-"nccl-test-1"}
NAMESPACE=${2:-"default"}
NODE_SELECTOR=${3:-"xlliu-test=test"}
NUM_WORKERS=${4:-2}
# !!! IMPORTANT: Define the exact command to run inside each pod !!!
NCCL_COMMAND_IN_POD=${5:-"/usr/local/sihpc/libexec/nccl-tests/nccl_test -g 8"}
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
echo_info "Command to be executed in each pod: $NCCL_COMMAND_IN_POD"
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
  --set mpijob.numWorkers=${NUM_WORKERS}"

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
POD_LIST_RAW=$(kubectl get pods -n "$NAMESPACE" -o wide --no-headers=true | grep ${JOB_NAME} |awk -v identifier="$WORKER_POD_IDENTIFIER_STRING" '$1 ~ identifier {print $1}')

if [ -z "$POD_LIST_RAW" ]; then
  echo "No worker pods found containing '$WORKER_POD_IDENTIFIER_STRING' in namespace '$NAMESPACE'."
  exit 1
fi

# Convert POD_LIST_RAW to an array
readarray -t POD_ARRAY <<<"$POD_LIST_RAW"
TOTAL_PODS_FOUND=${#POD_ARRAY[@]}
echo "Found $TOTAL_PODS_FOUND pod(s) to test."

# --- Arrays to store job information ---
declare -a pids_array
declare -a pod_names_array
declare -a host_ips_array
declare -a temp_files_array

# --- Launch tests in parallel ---
echo "Launching tests..."
job_count=0
for POD_NAME in "${POD_ARRAY[@]}"; do
  HOST_IP=$(kubectl get pod -n "$NAMESPACE" "$POD_NAME" -o jsonpath='{.status.hostIP}' 2>/dev/null || echo "N/A")
  TEMP_FILE="$TMP_DIR/${POD_NAME}_output.txt"

  # Store info before launching
  pod_names_array+=("$POD_NAME")
  host_ips_array+=("$HOST_IP")
  temp_files_array+=("$TEMP_FILE")

  # Launch kubectl exec in the background
  ( kubectl exec -n "$NAMESPACE" "$POD_NAME" -- bash -c "$NCCL_COMMAND_IN_POD" > "$TEMP_FILE" 2>&1 ) &
  pids_array+=($!) # Store the PID of the background kubectl exec

  echo "  Launched test for $POD_NAME (PID ${pids_array[-1]}), output to $TEMP_FILE"

  # Limit concurrency
  job_count=$((job_count + 1))
  if (( job_count % MAX_PARALLEL_JOBS == 0 )); then
    echo "  Reached max parallel jobs ($MAX_PARALLEL_JOBS), waiting for some to complete..."
    # Wait for the oldest $MAX_PARALLEL_JOBS jobs to avoid too many open PIDs
    # More sophisticated job management could be used here (e.g., `jobs -p` and specific waits)
    # For simplicity, this waits for *any* job to complete if we used `wait -n` (bash 4.3+)
    # Or simply wait for all launched so far in this batch
    for ((idx = job_count - MAX_PARALLEL_JOBS; idx < job_count; idx++)); do
        wait "${pids_array[idx]}" # Wait for specific PIDs in the current batch
    done
    echo "  Some jobs completed, continuing..."
  fi
done

# Wait for any remaining background jobs to complete
echo "Waiting for all remaining launched tests to complete..."
for pid in "${pids_array[@]}"; do
  wait "$pid"
done
echo "All tests completed."
echo "------------------------------------------------------------------------------------"

# --- Process results from temporary files ---
echo "Processing results..."
printf "%-40s | %-15s | %-20s\n" "Pod Name" "Host IP" "Avg Bus Bandwidth (GB/s)"
echo "------------------------------------------------------------------------------------"

ACTUAL_PARSED_COUNT=0
for i in "${!pod_names_array[@]}"; do
  POD_NAME="${pod_names_array[i]}"
  HOST_IP="${host_ips_array[i]}"
  TEMP_FILE="${temp_files_array[i]}"
  AVG_BW_RESULT="Error/No Output" # Default

  if [ -f "$TEMP_FILE" ] && [ -s "$TEMP_FILE" ]; then # Check if file exists and is not empty
    OUTPUT=$(cat "$TEMP_FILE")
    PARSED_BW_VALUE=$(echo "$OUTPUT" | grep '# Avg bus bandwidth' | awk '{print $NF}')

    if [ -n "$PARSED_BW_VALUE" ] && [[ "$PARSED_BW_VALUE" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
      AVG_BW_RESULT="$PARSED_BW_VALUE"
      bandwidth_values+=("$AVG_BW_RESULT")
      ACTUAL_PARSED_COUNT=$((ACTUAL_PARSED_COUNT + 1))
    else
      AVG_BW_RESULT="Parse Error"
    fi
  else
      AVG_BW_RESULT="No Output File"
  fi

  printf "%-40s | %-15s | %-20s\n" "$POD_NAME" "$HOST_IP" "$AVG_BW_RESULT"
  pod_final_results["$POD_NAME"]="Host IP: $HOST_IP, Bandwidth: $AVG_BW_RESULT"
done

echo "------------------------------------------------------------------------------------"
echo "Summary of Avg Bus Bandwidth Results:"
echo "------------------------------------------------------------------------------------"

# --- Summarization ---
if [ ${#bandwidth_values[@]} -eq 0 ]; then
  echo "No bandwidth values were successfully parsed from $TOTAL_PODS_FOUND pod(s) found."
  exit 0
fi

sum=0
min_bw=${bandwidth_values[0]}
max_bw=${bandwidth_values[0]}

for bw_str in "${bandwidth_values[@]}"; do
    sum=$(echo "$sum + $bw_str" | bc)
    if (( $(echo "$bw_str < $min_bw" | bc -l) )); then
      min_bw=$bw_str
    fi
    if (( $(echo "$bw_str > $max_bw" | bc -l) )); then
      max_bw=$bw_str
    fi
done

if [ "$ACTUAL_PARSED_COUNT" -gt 0 ]; then
  average_bw=$(echo "scale=3; $sum / $ACTUAL_PARSED_COUNT" | bc)
  echo "Number of pods successfully parsed: $ACTUAL_PARSED_COUNT / $TOTAL_PODS_FOUND"
  echo "Average Bus Bandwidth (overall):    $average_bw GB/s"
  echo "Minimum Bus Bandwidth:              $min_bw GB/s"
  echo "Maximum Bus Bandwidth:              $max_bw GB/s"
else
  echo "No valid numeric bandwidth values were collected for summary from $TOTAL_PODS_FOUND pod(s) found."
fi
echo "------------------------------------------------------------------------------------"
# The cleanup function will run on script exit to remove $TMP_DIR