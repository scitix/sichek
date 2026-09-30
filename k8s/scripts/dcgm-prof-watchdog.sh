#!/usr/bin/env bash
# DCGM profiling-metric watchdog.
# Detects "dcgm-exporter up but DCGM_FI_PROF_* absent" on THIS node and restarts
# the local dcgm-exporter pod so DCGM re-establishes its profiling watch.
#
# Source of truth for the dcgm-prof-watchdog sidecar. This file is inlined
# verbatim into args of that sidecar in k8s/deploy.yaml and
# k8s/devops_deploy.cuda128.yaml — keep the three copies in sync.
set -uo pipefail

DCGM_PORT="${DCGM_PORT:-9400}"
DCGM_NS="${DCGM_NS:-monitoring}"
DCGM_LABEL="${DCGM_LABEL:-app.kubernetes.io/name=dcgm-exporter}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-/var/sichek/run/current/kubeconfig}"
NODE_NAME="${NODE_NAME:-}"
POLL_SECONDS="${POLL_SECONDS:-30}"
MISS_THRESHOLD="${MISS_THRESHOLD:-3}"
COOLDOWN_SECONDS="${COOLDOWN_SECONDS:-300}"
MAX_PER_HOUR="${MAX_PER_HOUR:-6}"
# After this many consecutive restarts fail to bring DCGM_FI_PROF_* back, give up
# on the node and alert instead of churning forever (e.g. nodes with no profiling
# support, or a persistent evictor a restart cannot fix). Reset when PROF returns.
MAX_INEFFECTIVE_RESTARTS="${MAX_INEFFECTIVE_RESTARTS:-2}"
# GPU models that legitimately never expose DCGM_FI_PROF_* — absence is expected,
# so never restart dcgm on these (comma-separated, exact nvidia-smi names).
PROF_UNSUPPORTED_GPUS="${PROF_UNSUPPORTED_GPUS:-NVIDIA L40,NVIDIA GeForce RTX 5090}"
# HOST_CMD runs host binaries via nsenter in-cluster; tests set it empty.
HOST_CMD="${HOST_CMD:-nsenter -t 1 -m -p -n -u -i --}"
# WATCHDOG_MAX_LOOPS bounds the loop for tests; empty = run forever.
WATCHDOG_MAX_LOOPS="${WATCHDOG_MAX_LOOPS:-}"
# PROBE_LOG=true logs one heartbeat line per poll (and during cooldown) so the
# watchdog is visibly alive; set to any other value to log only actions/changes.
PROBE_LOG="${PROBE_LOG:-true}"

log() { echo "$(date '+%Y-%m-%dT%H:%M:%S%z') [dcgm-prof-watchdog] $*"; }

# Fetch metrics via STDOUT (never `-o <file>`): curl runs in the host mount
# namespace via HOST_CMD/nsenter, so a temp file would be written in the host
# filesystem but read back in the container filesystem — always empty, making
# PROF look absent on every node. stdout crosses the nsenter boundary cleanly.
# Prints the response body, then a final line containing the HTTP code. If curl
# cannot reach 127.0.0.1:${DCGM_PORT} at all, the caller sees code 000 and skips.
fetch() {
  $HOST_CMD curl -s -m 4 -w $'\n%{http_code}' \
    "http://127.0.0.1:${DCGM_PORT}/metrics" 2>/dev/null
}

restart_dcgm() {
  local pods
  pods="$($HOST_CMD kubectl --kubeconfig="${KUBECONFIG_PATH}" --request-timeout=10s \
    get pod -n "${DCGM_NS}" -l "${DCGM_LABEL}" \
    --field-selector "spec.nodeName=${NODE_NAME}" -o name 2>/dev/null)"
  if [ -z "$pods" ]; then
    log "WARN no dcgm-exporter pod matched (ns=${DCGM_NS} label=${DCGM_LABEL} node=${NODE_NAME}); not restarting"
    return 1
  fi
  log "deleting matched dcgm-exporter pod(s): ${pods//$'\n'/ }"
  $HOST_CMD kubectl --kubeconfig="${KUBECONFIG_PATH}" --request-timeout=10s \
    delete pod -n "${DCGM_NS}" -l "${DCGM_LABEL}" \
    --field-selector "spec.nodeName=${NODE_NAME}"
}

# sliding 1h window of restart epoch seconds
restart_times=()
recent_restarts() {
  local now t c=0 keep=()
  now="$(date +%s)"
  for t in "${restart_times[@]:-}"; do
    if [ -n "$t" ] && [ $((now - t)) -lt 3600 ]; then keep+=("$t"); c=$((c + 1)); fi
  done
  restart_times=("${keep[@]:-}")
  echo "$c"
}

# Sleep the cooldown in chunks, emitting a heartbeat, so the log does not go
# silent for the whole cooldown (which looks like the watchdog has hung).
cooldown_wait() {
  local left="$COOLDOWN_SECONDS"
  while [ "$left" -gt 0 ]; do
    if [ "$left" -le 30 ]; then
      sleep "$left"; left=0
    else
      sleep 30; left=$((left - 30))
      [ "$PROBE_LOG" = true ] && log "cooldown: ${left}s remaining"
    fi
  done
}

# GPU-model gate: some models never expose DCGM_FI_PROF_*, so their absence is
# expected and must NOT trigger a restart.
NODE_GPU_NAMES=""
gpu_names() {   # unique GPU model names (newline-separated), cached
  if [ -z "$NODE_GPU_NAMES" ]; then
    NODE_GPU_NAMES="$($HOST_CMD nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | sed 's/[[:space:]]*$//' | sort -u)"
  fi
  printf '%s' "$NODE_GPU_NAMES"
}
gpu_prof_unsupported() {   # returns 0 (true) if EVERY GPU model is in PROF_UNSUPPORTED_GPUS
  local names n rc=0 oldIFS="${IFS:-}"
  names="$(gpu_names)"
  [ -n "$names" ] || return 1   # unknown model -> treat as supported (do not skip)
  IFS=$'\n'
  for n in $names; do
    [ -n "$n" ] || continue
    case ",${PROF_UNSUPPORTED_GPUS}," in
      *",${n},"*) ;;            # this model legitimately has no PROF
      *) rc=1; break ;;         # a PROF-capable model is present -> not skippable
    esac
  done
  IFS="$oldIFS"
  return "$rc"
}

log "starting: node=${NODE_NAME} port=${DCGM_PORT} poll=${POLL_SECONDS}s miss=${MISS_THRESHOLD} cooldown=${COOLDOWN_SECONDS}s cap=${MAX_PER_HOUR}/h giveup_after=${MAX_INEFFECTIVE_RESTARTS}"
miss=0
loops=0
ineffective=0        # consecutive restarts since PROF was last seen present
gaveup=false         # true once we stop restarting a node whose PROF never returns
gpu_unsup_logged=false  # log the "GPU has no profiling support" note only once
while true; do
  resp="$(fetch)"
  code="${resp##*$'\n'}"
  body="${resp%$'\n'*}"
  [ -n "$code" ] || code="000"
  if [ "$code" = "200" ]; then
    pc="$(printf '%s\n' "$body" | grep -c '^DCGM_FI_PROF_')"
  else
    pc="-"
  fi
  [ "$PROBE_LOG" = true ] && log "probe: http=${code} prof=${pc}$([ "$gaveup" = true ] && printf ' (gaveup)')"
  if [ "$code" != "200" ]; then
    # curl could not reach 127.0.0.1:${DCGM_PORT} (or non-200): ignore, do not
    # probe further, do not restart.
    miss=0
  else
    if [ "$pc" -gt 0 ]; then
      [ "$miss" -gt 0 ] && log "DCGM_FI_PROF_* present again (${pc} lines), miss reset"
      if [ "$gaveup" = true ]; then
        log "DCGM_FI_PROF_* restored (${pc} lines); re-arming watchdog"
        gaveup=false
      fi
      miss=0
      ineffective=0
    elif gpu_prof_unsupported; then
      # This node's GPU model(s) never expose DCGM_FI_PROF_*; absence is expected.
      [ "$gpu_unsup_logged" = true ] || {
        log "DCGM_FI_PROF_* absent but node GPU(s) [$(gpu_names | paste -sd',' -)] have no profiling support; NOT restarting"
        gpu_unsup_logged=true
      }
      miss=0
    else
      miss=$((miss + 1))
      [ "$gaveup" = true ] || log "DCGM_FI_PROF_* absent (miss ${miss}/${MISS_THRESHOLD})"
      if [ "$miss" -ge "$MISS_THRESHOLD" ]; then
        if [ "$gaveup" = true ]; then
          :   # already gave up on this node; stay quiet until PROF returns
        elif [ "$ineffective" -ge "$MAX_INEFFECTIVE_RESTARTS" ]; then
          gaveup=true
          log "ALERT: restarted dcgm-exporter ${ineffective}x on ${NODE_NAME} but DCGM_FI_PROF_* did not return; giving up until PROF reappears (likely no profiling support or a persistent evictor a restart cannot fix — investigate)"
        else
          n="$(recent_restarts)"
          if [ "$n" -ge "$MAX_PER_HOUR" ]; then
            log "WARN per-hour restart cap reached (${n}/${MAX_PER_HOUR}); NOT restarting"
          else
            log "restarting dcgm-exporter pod on ${NODE_NAME} (miss=${miss}, ineffective=${ineffective}/${MAX_INEFFECTIVE_RESTARTS})"
            if restart_dcgm; then
              restart_times+=("$(date +%s)")
              ineffective=$((ineffective + 1))
              miss=0
              log "restart issued; cooldown ${COOLDOWN_SECONDS}s"
              cooldown_wait
            else
              log "WARN kubectl delete failed; will retry next cycle"
            fi
          fi
        fi
      fi
    fi
  fi
  loops=$((loops + 1))
  if [ -n "$WATCHDOG_MAX_LOOPS" ] && [ "$loops" -ge "$WATCHDOG_MAX_LOOPS" ]; then
    log "reached WATCHDOG_MAX_LOOPS=${WATCHDOG_MAX_LOOPS}, exiting"
    break
  fi
  sleep "$POLL_SECONDS"
done
