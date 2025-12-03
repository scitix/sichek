#!/bin/bash

set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 [OPTIONS]

Options:
  -j, --job-name NAME          Name of the job (default: comm-stress-test)
  -n, --namespace NAMESPACE    Kubernetes namespace (default: default)
  -s, --node-selector SELECTOR Node selector (default: None)
  -w, --num-workers NUM        Number of workers (default: 0, auto-derived from hostnames)
  -c, --test-cmd COMMAND       Test command to execute (default: \"echo 'No test command specified'\")
  -r, --image-repo REPO        Image repository (default: registry-us-east.scitix.ai/hisys/sichek)
  -t, --image-tag TAG          Image tag (default: latest)
  -T, --timeout SECONDS        Timeout in seconds (default: 600)
  -S, --scheduler NAME         Scheduler name (default: si-scheduler)
  -m, --roce-mode MODE         RoCE shared mode (default: none)
  -f, --hostfile FILE          File containing hostnames, one per line (default: None)
  -h, --host HOSTS             Comma-separated hostnames (default: None)
  --help                        Show this help message

Examples:
  $0 -j \"my-test\" -h \"node1,node2\" -c \"mpirun --version\"
  $0 -f /tmp/hosts.txt -c \"/usr/local/sihpc/bin/mpirun --allow-run-as-root /usr/local/sihpc/libexec/nccl-tests/nccl_test\"
  $0 --job-name \"my-test\" --host \"node1,node2\" --test-cmd \"mpirun --version\"
"

# Default parameter values
JOB_NAME="comm-stress-test"
NAMESPACE="default"
NODE_SELECTOR="None"
NUM_WORKERS=0
TEST_CMD="echo 'No test command specified'"
IMAGE_REPO="registry-us-east.scitix.ai/hisys/sichek"
IMAGE_TAG="latest"
TIMEOUT_TO_COMPLETE=600
SCHEDULER_NAME="si-scheduler"
ROCE_SHARED_MODE="none"
HOSTFILE="None"
HOST="None"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -j|--job-name)
      JOB_NAME="$2"
      shift 2
      ;;
    -n|--namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    -s|--node-selector)
      NODE_SELECTOR="$2"
      shift 2
      ;;
    -w|--num-workers)
      NUM_WORKERS="$2"
      shift 2
      ;;
    -c|--test-cmd)
      TEST_CMD="$2"
      shift 2
      ;;
    -r|--image-repo)
      IMAGE_REPO="$2"
      shift 2
      ;;
    -t|--image-tag)
      IMAGE_TAG="$2"
      shift 2
      ;;
    -T|--timeout)
      TIMEOUT_TO_COMPLETE="$2"
      shift 2
      ;;
    -S|--scheduler)
      SCHEDULER_NAME="$2"
      shift 2
      ;;
    -m|--roce-mode)
      ROCE_SHARED_MODE="$2"
      shift 2
      ;;
    -f|--hostfile)
      HOSTFILE="$2"
      shift 2
      ;;
    -h|--host)
      HOST="$2"
      shift 2
      ;;
    --help)
      echo "$USAGE"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "$USAGE"
      exit 1
      ;;
  esac
done

WORKER_POD_IDENTIFIER_STRING="worker"
MAX_PARALLEL_JOBS=200

# Use functions from common.sh to process hostfile and host parameters
setup_host_labels "$HOSTFILE" "$HOST" "$NODE_SELECTOR"

NODE_SELECTOR_ARGS="--set nodeSelector.$NODE_SELECTOR"

# --- Initialization ---
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
  cleanup_labels  # Clean up temporary labels
  [[ -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
  exit 0
}
trap cleanup EXIT        # Call on script exit
trap cleanup INT         # Ctrl+C interrupt
trap cleanup TERM        # When killed
trap cleanup ERR         # Also cleanup on script error (optional)

echo "================================================================================"
echo "Launching Communication Stress Test '$JOB_NAME' with $NUM_WORKERS workers in namespace '$NAMESPACE'"
echo "NodeSelector: $NODE_SELECTOR"
if [ ${#HOSTNAMES[@]} -gt 0 ]; then
  echo_info "Target hostnames: ${HOSTNAMES[*]}"
fi
echo "Image: $IMAGE_REPO:$IMAGE_TAG"
echo "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "Test Command: $TEST_CMD"
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

# Locate launcher Pod (first from status, then name grep)
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

# Collect Worker Pod list and their nodes
echo
echo "Test machines - Worker Pods and their nodes:"
WORKER_INFO=$(
  kubectl get pods \
  -l training.kubeflow.org/replica-type=worker,training.kubeflow.org/job-name="$MPIJOB_NAME" \
  -o 'jsonpath={range .items[*]}{.metadata.name}{" on "}{.spec.nodeName}{"\n"}{end}'
)
if [ -z "$WORKER_INFO" ]; then
  echo "  - no worker pods found"
else
  echo "$WORKER_INFO" | sed 's/^/  - /'
fi

# Execute user-specified test command
echo
echo "================================================================================"
echo "Executing Communication Stress Test Command"
echo "================================================================================"
echo "Command: $TEST_CMD"
echo "Timeout: $TIMEOUT_TO_COMPLETE seconds"
echo "================================================================================"

TMP_LOG="$TMP_DIR/communication_test_output.txt"

# Use timeout command to wrap test command execution, output to both console and file
echo "Starting test execution..."
echo "================================================================================"
echo "REAL-TIME OUTPUT FROM LAUNCHER POD:"
echo "================================================================================"

# Use tee to output to both console and file
if ! kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "timeout $TIMEOUT_TO_COMPLETE $TEST_CMD" 2>&1 | tee $TMP_LOG ; then
  echo "WARNING: Test command failed or timed out after $TIMEOUT_TO_COMPLETE seconds"
fi

echo "================================================================================"

# Check command execution status
if kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "echo \$?" > /dev/null 2>&1; then
  exit_code=$(kubectl -n "$NAMESPACE" exec "$LAUNCHER_POD" -- /bin/bash -c "echo \$?" 2>/dev/null | tail -n1)
  if [ "$exit_code" = "0" ]; then
    echo "✅ Test command completed successfully - exit code: $exit_code"
  else
    echo "❌ Test command failed - exit code: $exit_code"
  fi
else
  echo "⚠️  Could not determine exit code"
fi

echo "================================================================================"
echo "Communication Stress Test completed for $MPIJOB_NAME"
echo "================================================================================"

echo "================================================================================"
echo "Stop Job '$JOB_NAME' in namespace '$NAMESPACE' by helm ..."
echo "================================================================================"
