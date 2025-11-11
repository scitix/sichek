#!/bin/bash

set -euo pipefail

source "$(dirname "$0")/common.sh"

SCRIPTS_DIR=$(dirname "$(realpath "$0")")
SICHEK_ROOTDIR=$(realpath "$SCRIPTS_DIR/..")
SICHEK_HELM_DIR="$SICHEK_ROOTDIR/k8s/sichek"

USAGE="Usage: $0 [namespace] [image-repo] [image-tag] [default-spec] [hostfile] [host]
Defaults:
  namespace               = hi-sys-monitor
  imageRepository         = registry-us-east.scitix.ai/hisys/sichek
  imageTag                = latest
  defaultSpec             = hercules_spec.yaml
  hostfile                = None (file containing hostnames, one per line)
  host                    = None (comma-separated hostnames)
  operatingSystem         = ubuntu (ubuntu or centos)
Note: Number of workers will be automatically derived from hostfile or host parameter.
"

# 参数解析
NAMESPACE=${1:-"hi-sys-monitor"}
IMAGE_REPO=${2:-"registry-us-east.scitix.ai/hisys/sichek"}
IMAGE_TAG=${3:-"latest"}
DEFAULT_SPEC=${4:-"hercules_spec.yaml"}
HOSTFILE=${5:-"None"}
HOST=${6:-"None"}
OPERATING_SYSTEM=${7:-"ubuntu"}
# 使用common.sh中的函数处理hostfile和host参数
setup_host_labels "$HOSTFILE" "$HOST" "None"

# 从解析的hostnames中推导worker数量
NUM_WORKERS=${#HOSTNAMES[@]}
if [ $NUM_WORKERS -eq 0 ]; then
  echo_warn "No hostnames provided, exiting..."
  exit 1
fi

echo "========================================================================="
echo_info "Starting sichek install with the following configuration:"
echo "  Namespace: $NAMESPACE"
echo "  Number of Workers: $NUM_WORKERS (auto-derived from hostnames)"
echo "  Image Repository: $IMAGE_REPO"
echo "  Image Tag: $IMAGE_TAG"
echo "  Default Spec: $DEFAULT_SPEC"
echo "  Hostfile: $HOSTFILE"
echo "  Host: $HOST"
if [ $NUM_WORKERS -gt 0 ]; then
  echo "  Target Nodes: ${HOSTNAMES[*]}"
fi
echo "========================================================================="

# 构建helm命令参数
HELM_ARGS=(
  "upgrade" "--install" "install-all" "/var/sichek/k8s/sichek/"
  "--atomic"
  "--set" "mode=install-all"
  "--set" "image.repository=$IMAGE_REPO"
  "--set" "image.tag=$IMAGE_TAG"
  "--set" "batchjob.parallelism=$NUM_WORKERS"
  "--set" "batchjob.completions=$NUM_WORKERS"
  "--set" "defaultSpec=$DEFAULT_SPEC"
  "--set" "namespace=$NAMESPACE"
  "--set" "nodeSelector.$NODE_SELECTOR"
  "--set" "os=$OPERATING_SYSTEM"
)
# Cleanup function to handle script exit
cleanup() {
  echo_info "Cleaning up sichek install..."
  # 清理临时labels
  cleanup_labels
  exit 0
}
trap cleanup EXIT        # 脚本退出时调用
trap cleanup INT         # Ctrl+C 中断
trap cleanup TERM        # 被 kill 时
trap cleanup ERR         # 脚本出错也清理（可选）

echo_info "Running helm command: helm ${HELM_ARGS[*]}"

# 执行helm install
if helm "${HELM_ARGS[@]}"; then
  echo "========================================================================="
  echo_info "✅ sichek install completed successfully!"
  echo "========================================================================="
else
  echo "========================================================================="
  echo_warn "❌ sichek install failed!"
  echo "========================================================================="
  exit 1
fi

echo "========================================================================="
echo_info "🎉 sichek install process completed!"
echo "========================================================================="
