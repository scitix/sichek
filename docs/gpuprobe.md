# GPU Probe Check

*GPUProbe Component* is an **active** GPU compute self-test. Every other GPU-related checker in sichek (nvidia, gpuevents, hang, dcgm profiling watchdog, …) is passive: it reads counters, Xid events, or NVML state and infers health from them. None of that catches a GPU that "looks fine but computes wrong" — a card that reports healthy clocks, healthy temps, healthy Xid history, and idle utilization, yet silently produces incorrect results (or hangs) the moment a real kernel launches on it. GPUProbe closes that gap by actually running a tiny CUDA kernel on the GPU and checking its output, but only when the GPU is verified idle so the probe never competes with real workloads.

## Default OFF

GPUProbe is **not** in `consts.DefaultComponents`, so it will not run under `sichek all`, `sichek daemon start`, or any config-driven auto-selection just because `config/default_user_config.yaml` has a `gpuprobe:` section (that section only exists so the component's config loader doesn't error out when the component *is* asked to run).

To run it, opt in explicitly:

```bash
# one-shot standalone check
sichek gpu_probe -c config/default_user_config.yaml -v
# (alias: sichek gpuprobe)

# fold it into a daemon / all-in-one run alongside the default set
sichek daemon start -E gpuprobe
sichek all -E gpuprobe
```

## The probe binary

The actual compute test is a separate precompiled ELF, not Go code:

- Path on the node: `/var/sichek/bin/gpu_probe` (spec key `probe_binary_path`).
- Source: `components/gpuprobe/probe/gpu_probe.cu`, built via `components/gpuprobe/probe/Makefile` (see that directory's `README.md` for build/toolchain details — CUDA `nvcc`, static `libcudart_static` link so it only depends on the driver's `libcuda.so.1` at runtime).
- Shipped as a precompiled binary inside the sichek package (deb/tar.gz/docker) rather than built by `make`, so the main Go build stays offline-capable.
- The collector execs it per-GPU as a subprocess: `gpu_probe -d <device_id> [--min-free-pct <pct>]`.

Exit codes (collector decides pass/fail/skip purely from these, plus a wrapper-level timeout):

| Exit code | Meaning | Trigger |
|---|---|---|
| `0` | PASS | Kernel result matched expected output element-wise — GPU computes correctly. |
| `1` | FAIL | Result mismatch — the GPU computed wrong. This is the real fault signal. |
| `2` | SKIP | Probe itself backed off (free-mem% below `--min-free-pct`, or the device turned out busy/OOM after all — `cudaErrorMemoryAllocation` / `cudaErrorDevicesUnavailable`). |
| `3` | ENV_ERR | Any other CUDA API failure (driver/init error, invalid device index, etc.) — a CUDA environment problem, not a compute fault. |

The collector additionally recognizes two conditions the exit code alone can't express: `exec_err` (couldn't even launch the subprocess) and `timeout` (probe didn't exit within `probe_timeout_sec` and was killed).

## Idle gating

A GPU is only probed if **all three** signals agree it is idle:

1. **No compute processes** on the device (`nvidia-smi`/NVML process list is empty), gated by `skip_if_compute_apps` (default `true`).
2. **Free memory** ≥ `min_free_mem_pct` of total (default `50`).
3. **GPU utilization** ≤ `max_gpu_util_pct` (default `10`).

If any signal says busy, the GPU is **skipped, never probed** — GPUProbe will not interfere with running workloads. MIG-mode GPUs are also always skipped (`skip_mig`, default `true`) since MIG instances aren't addressable the way the probe expects. A skip is reported as `StatusNormal` / Info, not a failure.

## Level mapping

| Outcome (idle GPU) | Level | ErrorName |
|---|---|---|
| FAIL (kernel mismatch) or timeout | Critical (`fail_level`, default `consts.LevelCritical`) | `GPUProbeFailed` |
| ENV_ERR / exec error | Warning (`env_error_level`, default `consts.LevelWarning`) | `GPUProbeEnvError` |
| SKIP (busy / MIG) | Normal (Info) | — |

A FAIL on an idle GPU is treated as a real compute fault — this is the class of failure that motivated the component, so there is no debounce fuzz by default: `fail_consecutive_threshold` (default `1`) means the very first observed FAIL is reported. Raise it if a given fleet needs N consecutive FAILs before alerting.

## Resource safety

Actively execing kernels from a health-check daemon has its own failure modes, so GPUProbe bounds every step:

- **Per-probe timeout** — `probe_timeout_sec` (default `30`). The collector runs the probe in its own process group and, on timeout, kills the whole group rather than just the parent.
- **Hang abandonment** — if the probe process wedges in D-state and won't die even after the kill signal, the collector gives up waiting after `kill_grace_sec` (default `5`) rather than blocking the daemon's tick loop indefinitely. A GPU stuck this way surfaces as `timeout`, not a hung sichek daemon.
- **Bounded gating probe** — the idle-check itself (the `nvidia-smi`/NVML calls used for gating) is likewise time-bounded, so a GPU that's already unresponsive can't wedge the gating step either.

## Spec knobs

Configured via `components/gpuprobe/config/spec.go` (`GpuProbeSpec`, loaded by `config.LoadSpec`; falls back to `config.DefaultSpec()` if the spec file is missing/invalid):

| Field | Default | Meaning |
|---|---|---|
| `probe_binary_path` | `/var/sichek/bin/gpu_probe` | Path to the precompiled probe ELF. |
| `probe_timeout_sec` | `30` | Max seconds a single probe run may take before being killed. |
| `kill_grace_sec` | `5` | Extra seconds to wait for a killed probe to actually exit before giving up on it. |
| `min_free_mem_pct` | `50` | Minimum free-memory percentage required to consider a GPU idle. |
| `max_gpu_util_pct` | `10` | Maximum utilization percentage required to consider a GPU idle. |
| `skip_if_compute_apps` | `true` | Skip a GPU that has any compute process attached, regardless of the mem/util numbers. |
| `skip_mig` | `true` | Skip GPUs running in MIG mode. |
| `fail_consecutive_threshold` | `1` | Number of consecutive FAILs (on an idle GPU) required before reporting Critical. |
| `fail_level` | `consts.LevelCritical` | Level reported for a confirmed FAIL. |
| `env_error_level` | `consts.LevelWarning` | Level reported for ENV_ERR / exec errors. |

## Metrics

When `enable_metrics: true` (user config), GPUProbe exports, per GPU:

- `sichek_gpuprobe_probe_status{gpu,bdf}` — outcome code: `0`=pass, `1`=fail, `2`=skip, `3`=env_err, `4`=exec_err, `5`=timeout.
- `sichek_gpuprobe_duration_ms{gpu,bdf}` — how long the probe run took, in milliseconds.
