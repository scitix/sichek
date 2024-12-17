#!/bin/bash
color_green="\033[1;32m"
color_yellow="\033[1;33m"
color_purple="\033[1;35m"
color_reset="\033[0m"

echo_back() {
    local _cmdLog=${1}
    printf "[${color_purple}EXEC${color_reset}] ${_cmdLog}\n"
    eval ${_cmdLog}
}

echo_info() {
    local _cmdLog=${1}
    printf "[${color_green}INFO${color_reset}] ${_cmdLog}\n"
}

echo_warn() {
    local _cmdLog=${1}
    printf "[${color_yellow}WARN${color_reset}] ${_cmdLog}\n"
}

############################################################################

# set -e
# set -x

E2EDIR=$(dirname "$(realpath "$0")")
WORKSPACE_DIR="$E2EDIR/../.."
WORKSPACE_DIR=$(realpath "$WORKSPACE_DIR")
K8S_DIR="$WORKSPACE_DIR/k8s"

echo "========================================================================="
echo_info "start sichek daemonset"
echo "========================================================================="
echo_info "E2EDIR: $E2EDIR"
echo_info "K8S_DIR: $K8S_DIR"
echo_back "helm uninstall sichek"
echo_back "helm install sichek $K8S_DIR/sichek"
echo_info "$(kubectl get pod |grep sichek)"
echo ""

echo "========================================================================="
echo_info "start TaskGuard Service"
echo "========================================================================="
TASKGUARD="taskguard-service"
checktaskguard=$(kubectl get deployment $TASKGUARD)
if [[ "$checktaskguard" == *"NotFound"* ]]; then
    pass
else
    echo_warn "$TASKGUARD already exists. Delete it first."
    echo_back "kubectl delete -f $WORKSPACE_DIR/test/e2e/taskguard-service.yaml"
fi
echo_back "kubectl apply -f $WORKSPACE_DIR/test/e2e/taskguard-service.yaml"
echo_back "sleep 5"
taskguardpod=$(kubectl get pod |grep $TASKGUARD |awk '{print $1}')
echo_info "TaskGuard pod: $taskguardpod"


echo ""
echo "========================================================================="
PYTORCHJOB="sichek-taskguard-test"
echo_info "start pytorchjob $PYTORCHJOB"
echo "========================================================================="
checkjob=$(kubectl get pytorchjob $PYTORCHJOB)
if [[ "$checkjob" == *"NotFound"* ]]; then
    pass
else
    echo_warn "Pytorchjob $PYTORCHJOB already exists. Delete it first."
    echo_back "kubectl delete -f pytorchjob.yaml"
    echo_back "sleep 5"
fi
echo_back "kubectl apply -f pytorchjob.yaml"

echo_info "Waiting for pytorchjob $PYTORCHJOB to Running state."
timeout=300
interval=5
elapsed=0
while (( elapsed < timeout )); do
    status=$(kubectl get pytorchjob "$PYTORCHJOB" | grep -v NAME |awk '{print $2}')
    if [[ "$status" == "Running" ]]; then
        echo_info "Pytorchjob $PYTORCHJOB are in Running state."
        break
    else
        echo_info "Pytorchjob $PYTORCHJOB is not in Running state yet. Retrying..."
        echo_back "sleep $interval"
        (( elapsed += interval ))
    fi
done

if (( elapsed >= timeout )); then
    echo_warn "Timeout reached while waiting for pytorchjob $PYTORCHJOB to reach Running state."
fi

echo ""
echo "========================================================================="
echo_info "Select sichek-taskguard-test-worker-0 to mock Hang case."
pytorchjob_worker_0_node=$(kubectl get pod sichek-taskguard-test-worker-0 -oyaml |grep "nodeName:" |awk '{print $2}')
echo_info "sichek-taskguard-test-worker-0 running on node $pytorchjob_worker_0_node"
echo_info "Show the scitix.ai/sichek annotation of node $pytorchjob_worker_0_node"
echo_back "kubectl get node $pytorchjob_worker_0_node -oyaml |grep scitix.ai/sichek"
echo_info "The events showing in scitix.ai/sichek annotation of node $pytorchjob_worker_0_node is 'Null'."

echo ""
echo "========================================================================="
echo_info "Waiting for training processes to start."
while true; do
    ppid=$(kubectl exec sichek-taskguard-test-worker-0 -- pgrep torchrun |grep -v Default)
    # Remove spaces
    ppid=${ppid// /}
    # contains only numbers (digits)
    if [[ "$ppid" =~ ^[0-9]+$ ]]; then
        echo_info "The training processes are started. sleep more 30s ..."
        echo_back "sleep 30"
        echo_back "kubectl logs sichek-taskguard-test-worker-0 |tail"
        break
    else
        echo_info "The training processes are not started yet. Retrying..."
        echo_back "sleep 5"
    fi
done
echo "========================================================================="


echo ""
echo "========================================================================="
echo_info "Send SIGINT to a RANK main process of pytorchjob sichek-taskguard-test-worker-0."
pytorchjob_worker_0_ppid=$(kubectl exec sichek-taskguard-test-worker-0 -- pgrep torchrun |grep -v Default)
pytorchjob_worker_0_pids=$(kubectl exec sichek-taskguard-test-worker-0 -- ps --ppid $ppid |grep python |awk '{print $1}')
echo_info "sichek-taskguard-test-worker-0 running on node $pytorchjob_worker_0_node , its ranks main process are: $pytorchjob_worker_0_pids"
pytorchjob_worker_0_pid_to_SIGINT=$(echo $pytorchjob_worker_0_pids | awk '{print $1}')
echo_warn "killed SIGINT to one of the rank main process : $pytorchjob_worker_0_pid_to_SIGINT"
echo_back "kubectl exec sichek-taskguard-test-worker-0 -- kill -SIGINT $pytorchjob_worker_0_pid_to_SIGINT"

echo ""
echo "========================================================================="
sleep 5
echo_warn "The logs of sichek-taskguard-test-worker-0 show the failed status, while the pytorchjob are still running."
echo_back "kubectl logs sichek-taskguard-test-worker-0 |tail"
echo_warn "The pytorchjob will Hang for 10 minutes and then exit."
echo "========================================================================="

echo ""
echo "========================================================================="
echo_info "Sichek should detect the Hang case and update the status of scitix.ai/sichek annotation with 'GPUHang'."
echo_warn "Waiting for Sichek detect the Hang case after 60s."
timeout=300
elapsed=0
interval=5

while (( elapsed < timeout )); do
    sichek_annotation=$(kubectl get node $pytorchjob_worker_0_node -oyaml |grep scitix.ai/sichek)
    all_running=true
    if [[ "$sichek_annotation" == *"GPUHang"* ]]; then
        echo_warn "Sichek detect the Hang case and update the status of scitix.ai/sichek annotation with 'GPUHang'"
        echo_back "kubectl get node $pytorchjob_worker_0_node -oyaml |grep scitix.ai/sichek"
        break
    else
        echo_info "Waiting for Sichek detect the Hang case. Retrying..."
        echo_back "sleep $interval"
        (( elapsed += interval ))
    fi
done

if (( elapsed >= timeout )); then
    echo_warn "Timeout reached while waiting for Sichek to detect the Hang case."
fi
echo "========================================================================="

echo ""
echo "========================================================================="
echo_info "Check the logs of TaskGuard. It is expected to resubmit the pytorchjob."
taskguardpod=$(kubectl get pod |grep $TASKGUARD |awk '{print $1}')
timeout=300
elapsed=0
interval=5

while (( elapsed < timeout )); do
    taskguard_resubmit_succeed=$(kubectl logs $taskguardpod |grep "resubmit succeed")
    if [[ -n "$taskguard_resubmit_succeed" ]]; then
        echo_warn "The pytorchjob $PYTORCHJOB is resubmitted and running again."
        echo_back "kubectl get pytorchjob |grep $PYTORCHJOB"
        break
    else
        echo_info "Waiting for taskguard resubmit the pytorchjob $PYTORCHJOB..."
        echo_back "sleep $interval"
        (( elapsed += interval ))
    fi
done
if (( elapsed >= timeout )); then
    echo_warn "Timeout reached while waiting for TaskGuard to resubmit the pytorchjob."
fi

echo_back "sleep 30"
echo_back "kubectl logs sichek-taskguard-test-1-worker-0 |tail"
echo_back "kubectl get pytorchjob |grep $PYTORCHJOB |awk 'system(\"kubectl delete pytorchjob \" \$1)'"
echo "========================================================================="
