#!/usr/bin/env bash
# Behavior tests for dcgm-prof-watchdog.sh using PATH-stubbed curl/kubectl.
# No external test framework: exits non-zero if any case fails.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/dcgm-prof-watchdog.sh"
fail=0

setup() {
  TMP="$(mktemp -d)"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  CALLS="$TMP/kubectl_calls"; : > "$CALLS"
  BODY="$TMP/body"; : > "$BODY"
  SEQ="$TMP/seq"; mkdir -p "$SEQ"
  # stub curl: the script fetches via STDOUT (curl -w '\n%{http_code}'), never
  # `-o <file>`. This stub emits <body> then a newline then <code> on STDOUT and
  # ignores any -o argument. It also guards the mount-namespace regression: if the
  # script ever went back to reading an -o temp file, the cases below would fail
  # because this stub writes nothing to any file.
  #   STUB_SEQ_DIR set -> per-call body/code from $SEQ/<n>.body / <n>.code (n from 0);
  #                       $SEQ/<n>.hang => 200 with empty body (timed-out mid-body)
  #   else            -> single STUB_BODY file + STUB_CODE
  cat > "$BIN/curl" <<'CURL'
#!/usr/bin/env bash
if [ -n "${STUB_SEQ_DIR:-}" ]; then
  cf="$STUB_SEQ_DIR/.n"; n=0; [ -f "$cf" ] && n="$(cat "$cf")"; echo $((n+1)) > "$cf"
  if [ -f "$STUB_SEQ_DIR/$n.hang" ]; then printf '\n200'; exit 0; fi
  [ -f "$STUB_SEQ_DIR/$n.body" ] && cat "$STUB_SEQ_DIR/$n.body"
  code=200; [ -f "$STUB_SEQ_DIR/$n.code" ] && code="$(cat "$STUB_SEQ_DIR/$n.code")"
  printf '\n%s' "$code"
else
  [ -f "${STUB_BODY:-/dev/null}" ] && cat "${STUB_BODY:-/dev/null}"
  printf '\n%s' "${STUB_CODE:-200}"
fi
CURL
  # stub kubectl: record only delete invocations; answer `get ... -o name` with a
  # fake pod name (unless STUB_NO_POD=1, then print nothing).
  cat > "$BIN/kubectl" <<KUBECTL
#!/usr/bin/env bash
if [[ "\$*" == *" delete "* ]]; then
  echo "\$@" >> "$CALLS"
elif [[ "\$*" == *" get "* ]]; then
  [ -n "\${STUB_NO_POD:-}" ] || echo "pod/dcgm-exporter-stub"
fi
KUBECTL
  # stub nvidia-smi: report GPU model name (default a PROF-capable model)
  cat > "$BIN/nvidia-smi" <<'NVSMI'
#!/usr/bin/env bash
echo "${STUB_GPU:-NVIDIA H200}"
NVSMI
  chmod +x "$BIN/curl" "$BIN/kubectl" "$BIN/nvidia-smi"
}
teardown() { rm -rf "$TMP"; }
calls() { wc -l < "$CALLS" | tr -d ' '; }
prof_line() { printf 'DCGM_FI_PROF_PCIE_TX_BYTES{gpu="0"} 123\n'; }
check() { # name expected actual
  if [ "$2" = "$3" ]; then echo "PASS $1"; else echo "FAIL $1 (expected $2, got $3)"; fail=1; fi
}

# A: endpoint 200 with PROF present -> never restarts
setup
prof_line > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 WATCHDOG_MAX_LOOPS=5 MISS_THRESHOLD=3 \
    bash "$SCRIPT" >/dev/null 2>&1
check A 0 "$(calls)"; teardown

# B: endpoint down (non-200) -> never restarts, miss never accrues
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=503 WATCHDOG_MAX_LOOPS=6 MISS_THRESHOLD=3 \
    bash "$SCRIPT" >/dev/null 2>&1
check B 0 "$(calls)"; teardown

# C: endpoint 200 but PROF absent for MISS_THRESHOLD polls -> exactly one restart
setup
: > "$BODY"   # 200, zero PROF lines
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 WATCHDOG_MAX_LOOPS=3 MISS_THRESHOLD=3 MAX_PER_HOUR=6 \
    bash "$SCRIPT" >/dev/null 2>&1
check C 1 "$(calls)"; teardown

# D: per-hour cap limits restarts (threshold 1, cap 2, 6 loops -> 2 restarts)
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 WATCHDOG_MAX_LOOPS=6 MISS_THRESHOLD=1 MAX_PER_HOUR=2 MAX_INEFFECTIVE_RESTARTS=99 \
    bash "$SCRIPT" >/dev/null 2>&1
check D 2 "$(calls)"; teardown

# E: PROF recovers before threshold -> miss resets, no restart
setup
: > "$SEQ/0.body"                 # absent
: > "$SEQ/1.body"                 # absent
prof_line > "$SEQ/2.body"         # present -> reset
: > "$SEQ/3.body"                 # absent (miss back to 1)
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_SEQ_DIR="$SEQ" WATCHDOG_MAX_LOOPS=4 MISS_THRESHOLD=3 \
    bash "$SCRIPT" >/dev/null 2>&1
check E 0 "$(calls)"; teardown

# F: 200 header then hung/empty body must be treated as PROF-absent (not stale-healthy)
setup
prof_line > "$SEQ/0.body"        # healthy
: > "$SEQ/1.hang"; : > "$SEQ/2.hang"; : > "$SEQ/3.hang"   # 3 hangs
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_SEQ_DIR="$SEQ" WATCHDOG_MAX_LOOPS=4 MISS_THRESHOLD=3 MAX_PER_HOUR=6 \
    bash "$SCRIPT" >/dev/null 2>&1
check F 1 "$(calls)"; teardown

# G: selector matches no pod -> never issues a delete
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 STUB_NO_POD=1 WATCHDOG_MAX_LOOPS=4 MISS_THRESHOLD=3 \
    bash "$SCRIPT" >/dev/null 2>&1
check G 0 "$(calls)"; teardown

# H: the delete command targets the right ns/label/node
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 WATCHDOG_MAX_LOOPS=3 MISS_THRESHOLD=3 \
    bash "$SCRIPT" >/dev/null 2>&1
if grep -q -- "-n monitoring" "$CALLS" && grep -q -- "-l app.kubernetes.io/name=dcgm-exporter" "$CALLS" && grep -q -- "--field-selector spec.nodeName=n" "$CALLS"; then echo "PASS H"; else echo "FAIL H: $(cat "$CALLS")"; fail=1; fi
teardown

# I: PROF never returns -> give up after MAX_INEFFECTIVE_RESTARTS restarts (bounded churn)
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 WATCHDOG_MAX_LOOPS=12 MISS_THRESHOLD=1 MAX_PER_HOUR=99 MAX_INEFFECTIVE_RESTARTS=2 \
    bash "$SCRIPT" >/dev/null 2>&1
check I 2 "$(calls)"; teardown

# J: after giving up, PROF returning re-arms the watchdog (ineffective resets)
setup
: > "$SEQ/0.body"; : > "$SEQ/1.body"; : > "$SEQ/2.body"   # absent x3 -> 2 restarts then give up
prof_line > "$SEQ/3.body"                                 # present -> re-arm
: > "$SEQ/4.body"                                         # absent -> restart again
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_SEQ_DIR="$SEQ" WATCHDOG_MAX_LOOPS=5 MISS_THRESHOLD=1 MAX_PER_HOUR=99 MAX_INEFFECTIVE_RESTARTS=2 \
    bash "$SCRIPT" >/dev/null 2>&1
check J 3 "$(calls)"; teardown

# K: PROF absent but GPU is a no-profiling model (NVIDIA L40) -> never restarts
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 STUB_GPU="NVIDIA L40" WATCHDOG_MAX_LOOPS=6 MISS_THRESHOLD=1 \
    bash "$SCRIPT" >/dev/null 2>&1
check K 0 "$(calls)"; teardown

# L: PROF absent but GPU is a no-profiling model (RTX 5090) -> never restarts
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 STUB_GPU="NVIDIA GeForce RTX 5090" WATCHDOG_MAX_LOOPS=6 MISS_THRESHOLD=1 \
    bash "$SCRIPT" >/dev/null 2>&1
check L 0 "$(calls)"; teardown

# M: PROF absent on a PROF-capable model (H200) still restarts (gate does not over-block)
setup
: > "$BODY"
env HOST_CMD="" PATH="$BIN:$PATH" NODE_NAME=n POLL_SECONDS=0 COOLDOWN_SECONDS=0 \
    STUB_BODY="$BODY" STUB_CODE=200 STUB_GPU="NVIDIA H200" WATCHDOG_MAX_LOOPS=3 MISS_THRESHOLD=3 MAX_PER_HOUR=6 \
    bash "$SCRIPT" >/dev/null 2>&1
check M 1 "$(calls)"; teardown

if [ "$fail" -ne 0 ]; then echo "TESTS FAILED"; exit 1; fi
echo "ALL TESTS PASSED"
