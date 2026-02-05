#!/usr/bin/env python3
"""
DeepEP Tuning Test Script (Multi Node)

Runs the same DeepEP command as deepeptest_single_node on multiple nodes (PyTorchJob).
Each node outputs three Best lines (FP8/BF16/combine) with rdma+nvl lat (us), RDMA GB/s, NVL GB/s.
Extracts and prints these per node (SM, NVL chunk, RDMA chunk are parsed but not shown).
"""

import argparse
import os
import re
import shlex
import signal
import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

swanlab = None
try:
    import swanlab
except ImportError:
    # Swanlab is optional; continue without experiment tracking if it's not installed.
    print("Warning: 'swanlab' package not found; proceeding without Swanlab integration.", file=sys.stderr)

from common import (
    echo_info,
    echo_warn,
    run_cmd,
    run_cmd_check,
    parse_hostnames,
    load_user_config,
    pick_value,
    start_kubectl_log_stream,
    wait_for_pods_ready,
)


# Multi-node format:
# [tuning] Best dispatch (FP8): SMs 24, NVL chunk 24, RDMA chunk 8: 437 + 2412 us, 50.08 GB/s (RDMA), 99.93 GB/s (NVL)
# [tuning] Best dispatch (BF16): SMs 24, NVL chunk 20, RDMA chunk 4: 731 + 4471 us, 52.40 GB/s (RDMA), 104.55 GB/s (NVL)
# [tuning] Best combine: SMs 24, NVL chunk 2, RDMA chunk 8, 570.70 + 4566.00 us, 51.31 GB/s (RDMA), 102.37 GB/s (NVL)
_RE_BEST_DISPATCH_MULTI = re.compile(
    r"\[tuning\]\s+Best dispatch \((FP8|BF16)\):\s+SMs (\d+),\s+NVL chunk (\d+),\s+RDMA chunk (\d+):\s+"
    r"([\d.]+) \+ ([\d.]+) us,\s+([\d.]+) GB/s \(RDMA\),\s+([\d.]+) GB/s \(NVL\)"
)
_RE_BEST_COMBINE_MULTI = re.compile(
    r"\[tuning\]\s+Best combine:\s+SMs (\d+),\s+NVL chunk (\d+),\s+RDMA chunk (\d+),\s+"
    r"([\d.]+) \+ ([\d.]+) us,\s+([\d.]+) GB/s \(RDMA\),\s+([\d.]+) GB/s \(NVL\)"
)


def parse_deepep_multi_best_results(output: str) -> List[Dict[str, Any]]:
    """
    Parse DeepEP multi-node tuning log. Returns list of dicts with:
    kind, sms, nvl_chunk, rdma_chunk, latency_us (rdma + nvl lat, us), latency_str, rdma_gbps, nvl_gbps.
    """
    bests = []
    for m in _RE_BEST_DISPATCH_MULTI.finditer(output):
        kind = m.group(1)
        sms = int(m.group(2))
        nvl_chunk = int(m.group(3))
        rdma_chunk = int(m.group(4))
        lat_a, lat_b = float(m.group(5)), float(m.group(6))
        rdma_gbps = float(m.group(7))
        nvl_gbps = float(m.group(8))
        bests.append({
            "kind": kind,
            "sms": sms,
            "nvl_chunk": nvl_chunk,
            "rdma_chunk": rdma_chunk,
            "latency_us": lat_a + lat_b,
            "latency_str": f"{lat_a:.2f} + {lat_b:.2f}",
            "rdma_gbps": rdma_gbps,
            "nvl_gbps": nvl_gbps,
        })
    for m in _RE_BEST_COMBINE_MULTI.finditer(output):
        sms = int(m.group(1))
        nvl_chunk = int(m.group(2))
        rdma_chunk = int(m.group(3))
        lat_a, lat_b = float(m.group(4)), float(m.group(5))
        rdma_gbps = float(m.group(6))
        nvl_gbps = float(m.group(7))
        bests.append({
            "kind": "combine",
            "sms": sms,
            "nvl_chunk": nvl_chunk,
            "rdma_chunk": rdma_chunk,
            "latency_us": lat_a + lat_b,
            "latency_str": f"{lat_a:.2f} + {lat_b:.2f}",
            "rdma_gbps": rdma_gbps,
            "nvl_gbps": nvl_gbps,
        })
    return bests


def get_pod_host(namespace: str, pod_name: str) -> str:
    """Get node/host name for a pod."""
    rc, out, _ = run_cmd(
        f"kubectl get pod {pod_name} -n {namespace} -o jsonpath='{{.spec.nodeName}}'",
        quiet=True,
    )
    return out.strip() if rc == 0 and out.strip() else ""


# Width for "rdma + nvl" latency string (e.g. "206.07 + 3653.00")
_LAT_STR_W = 22

def _row_fmt(b: Optional[Dict], w: int = 10) -> str:
    """Format one Best row: rdma+nvl lat (a + b us), RDMA_GB/s, NVL_GB/s."""
    if b is None:
        return f"{'N/A':<{_LAT_STR_W}} {'N/A':<{w}} {'N/A':<{w}}"
    return (
        f"{b['latency_str']:<{_LAT_STR_W}} {b['rdma_gbps']:<{w}.2f} {b['nvl_gbps']:<{w}.2f}"
    )


def print_deepep_multi_results(
    pod_logs: List[Tuple[str, str, str]],
) -> None:
    """Print per-node table: Pod, Host, and for each Best (FP8, BF16, combine): rdma+nvl lat (a + b us), RDMA GB/s, NVL GB/s."""
    w = 10
    col_block = _LAT_STR_W + 2 * w + 4  # lat_str, RDMA, NVL per Best
    sep_len = 52 + 18 + 3 * col_block + 8
    print("\n" + "-" * sep_len)
    print("DeepEP multi-node tuning: Best per node (rdma+nvl lat a+b us, RDMA GB/s, NVL GB/s)")
    print(
        f"{'Pod Name':<52} | {'Host':<18} | "
        f"{'FP8 (lat_us RDMA NVL)':<{col_block}} | "
        f"{'BF16 (lat_us RDMA NVL)':<{col_block}} | "
        f"{'combine (lat_us RDMA NVL)':<{col_block}}"
    )
    print("-" * sep_len)

    for pod_name, host, logs in pod_logs:
        bests = parse_deepep_multi_best_results(logs)
        fp8 = next((b for b in bests if b["kind"] == "FP8"), None)
        bf16 = next((b for b in bests if b["kind"] == "BF16"), None)
        combine = next((b for b in bests if b["kind"] == "combine"), None)
        fp8_str = _row_fmt(fp8, w).rstrip()
        bf16_str = _row_fmt(bf16, w).rstrip()
        co_str = _row_fmt(combine, w).rstrip()
        print(f"{pod_name:<52} | {host:<18} | {fp8_str} | {bf16_str} | {co_str}")

    print("-" * sep_len)


def main() -> None:
    parser = argparse.ArgumentParser(description="DeepEP tuning test (multi node)")
    parser.add_argument("--job-name", default=None)
    parser.add_argument("--namespace", default="default")
    parser.add_argument("--cmd", default="", help="Command to run (same as deepeptest_single_node)")
    parser.add_argument("--image-repo", default="registry-taihua.siflow.cn/hisys/mcore", help="Container image repository")
    parser.add_argument("--image-tag", default="pytorch25.11-cuda13-cudnn9.17-te-main-v1", help="Container image tag")
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--scheduler-name", default=None)
    parser.add_argument("--roce-shared-mode", default=None)
    parser.add_argument("--hostfile", default="None")
    parser.add_argument("--host", default="None")
    parser.add_argument("--host-dir", default=None, help="Host directory to mount (e.g. /tmp/DeepEP)")

    args = parser.parse_args()
    config = load_user_config()

    args.image_repo = pick_value(
        args.image_repo, config, "pytorchjob_image_repo", "registry-taihua.siflow.cn/hisys/mcore"
    )
    args.image_tag = pick_value(
        args.image_tag, config, "pytorchjob_image_tag", "pytorch25.11-cuda13-cudnn9.17-te-main-v1"
    )
    args.scheduler_name = pick_value(args.scheduler_name, config, "scheduler", "si-scheduler")
    args.roce_shared_mode = pick_value(args.roce_shared_mode, config, "roce_shared_mode", "none")

    hostnames = parse_hostnames(
        args.hostfile if args.hostfile != "None" else None,
        args.host if args.host != "None" else None,
    )
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)

    num_workers = len(hostnames)
    date_str = datetime.now().strftime("%Y%m%d%H%M%S")
    if args.job_name is None:
        args.job_name = f"sichek-deepeptest-multi-n{num_workers}-{date_str}"
    else:
        args.job_name = f"{args.job_name}-n{num_workers}-{date_str}"

    num_experts = 8 * num_workers
    default_cmd = (
        "python /tmp/DeepEP/tests/test_internode.py "
        f"--num-processes 8 --num-tokens 4096 --hidden 7168 --num-topk 8 --num-experts {num_experts}"
    )
    cmd = args.cmd if args.cmd else default_cmd
    if args.host_dir is not None:
        cmd = f"DEEPEP_HOST_DIR={args.host_dir} {cmd}"

    scripts_dir = Path(__file__).parent.resolve()
    helm_dir = scripts_dir.parent.parent / "k8s" / "sichek"
    log_proc = None

    def cleanup() -> None:
        echo_info(f"Cleaning up Helm release: {args.job_name}")
        run_cmd(f"helm uninstall {args.job_name} -n {args.namespace} || true")
        run_cmd(f"kubectl delete pytorchjob {args.job_name} -n {args.namespace} --ignore-not-found")
        if log_proc:
            try:
                log_proc.terminate()
                log_proc.wait(timeout=5)
            except Exception:
                pass

    def signal_handler(sig: int, frame: Any) -> None:
        echo_info("Interrupted, cleaning up...")
        cleanup()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    swan_run = None
    if os.getenv("SWANLAB_API_KEY") and swanlab is not None:
        swan_run = swanlab.init(
            experiment_name=args.job_name,
            description=f"DeepEP tuning multi-node ({num_workers} workers)",
            config={
                "job_name": args.job_name,
                "namespace": args.namespace,
                "num_workers": num_workers,
                "command": cmd,
                "hosts": hostnames,
            },
        )

    try:
        echo_info(f"Starting PyTorchJob '{args.job_name}' in namespace '{args.namespace}'")
        host_csv = ",".join(hostnames)
        helm_cmd = (
            f"helm upgrade --install {shlex.quote(args.job_name)} {shlex.quote(str(helm_dir))} "
            f"--atomic "
            f"--set namespace={shlex.quote(args.namespace)} "
            f"--set mode=pytorchjob "
            f"--set schedulerName={shlex.quote(args.scheduler_name)} "
            f"--set roceSharedMode={shlex.quote(args.roce_shared_mode)} "
            f"--set image.repository={shlex.quote(args.image_repo)} "
            f"--set image.tag={shlex.quote(args.image_tag)} "
            f"--set pytorchjob.name={shlex.quote(args.job_name)} "
            f"--set pytorchjob.numWorkers={num_workers} "
            f"--set pytorchjob.cmd={shlex.quote(cmd)} "
            f"--set 'pytorchjob.nodeAffinityHosts={{{host_csv}}}'"
        )
        if args.host_dir is not None:
            helm_cmd += f" --set pytorchjob.hostDir={shlex.quote(args.host_dir)}"
        run_cmd_check(helm_cmd)

        echo_info("Waiting for worker pods to be created...")
        time.sleep(5)
        max_wait = 300
        wait_interval = 5
        waited = 0
        pods_exist = False
        while waited < max_wait:
            pod_cmd = (
                f"kubectl get pod -n {args.namespace} -l training.kubeflow.org/replica-type=worker,"
                f"training.kubeflow.org/job-name={args.job_name} "
                f"-o jsonpath='{{.items[*].metadata.name}}'"
            )
            rc, out, _ = run_cmd(pod_cmd, quiet=True)
            if rc == 0 and out.strip():
                pods_exist = True
                echo_info(f"Found worker pods: {out.strip()}")
                break
            echo_info("Worker pods not created yet, waiting...")
            time.sleep(wait_interval)
            waited += wait_interval

        if not pods_exist:
            echo_warn(f"Timeout waiting for worker pods after {max_wait} seconds")
            return

        wait_for_pods_ready(
            namespace=args.namespace,
            label_selector=(
                f"training.kubeflow.org/replica-type=worker,"
                f"training.kubeflow.org/job-name={args.job_name}"
            ),
            timeout=args.timeout,
            pod_name_filter=args.job_name,
            initial_delay=5,
            check_interval=5,
        )
        wait_for_pods_ready(
            namespace=args.namespace,
            label_selector=(
                f"training.kubeflow.org/replica-type=master,"
                f"training.kubeflow.org/job-name={args.job_name}"
            ),
            timeout=300,
            pod_name_filter=args.job_name,
            initial_delay=0,
            check_interval=5,
        )

        rc, out, _ = run_cmd(
            f"kubectl get pod -n {args.namespace} -l training.kubeflow.org/replica-type=worker,"
            f"training.kubeflow.org/job-name={args.job_name} "
            f"-o jsonpath='{{.items[*].metadata.name}}'",
            quiet=True,
        )
        if rc != 0 or not out.strip():
            echo_warn(f"No worker pods found for job '{args.job_name}'")
            return
        pods = sorted(out.strip().split())
        last_pod = pods[-1]
        echo_info(f"Last worker pod: {last_pod}")

        log_proc, _ = start_kubectl_log_stream(args.namespace, last_pod, "worker-log")
        echo_info("=" * 80)
        echo_info(f"Waiting for PyTorchJob {args.job_name} to complete...")
        echo_info("=" * 80)
        elapsed = 0
        while elapsed < args.timeout:
            rc, out, _ = run_cmd(
                f"kubectl get pytorchjob {args.job_name} -n {args.namespace} | grep -v NAME | awk '{{print $2}}'",
                quiet=True,
            )
            status = out.strip() if rc == 0 else ""
            if status in ("Succeeded", "Failed"):
                echo_info(f"PyTorchJob Status: {status}")
                break
            time.sleep(10)
            elapsed += 10

        if elapsed >= args.timeout:
            echo_warn(f"Timeout waiting for PyTorchJob {args.job_name}")

        # Per-node: get logs from each worker pod and parse Best results
        pod_logs: List[Tuple[str, str, str]] = []
        for pod in pods:
            rc, logs, _ = run_cmd(f"kubectl logs -n {args.namespace} {pod}", quiet=True)
            host = get_pod_host(args.namespace, pod)
            pod_logs.append((pod, host, logs if rc == 0 else ""))

        # Also fetch and parse master pod logs
        rc_m, out_m, _ = run_cmd(
            f"kubectl get pod -n {args.namespace} -l training.kubeflow.org/replica-type=master,"
            f"training.kubeflow.org/job-name={args.job_name} "
            f"-o jsonpath='{{.items[*].metadata.name}}'",
            quiet=True,
        )
        if rc_m == 0 and out_m.strip():
            for master_pod in sorted(out_m.strip().split()):
                rc, logs, _ = run_cmd(f"kubectl logs -n {args.namespace} {master_pod}", quiet=True)
                host = get_pod_host(args.namespace, master_pod)
                pod_logs.append((master_pod, host, logs if rc == 0 else ""))

        print_deepep_multi_results(pod_logs)

        if swan_run and pod_logs:
            for idx, (_, _, logs) in enumerate(pod_logs):
                bests = parse_deepep_multi_best_results(logs)
                fp8 = next((b for b in bests if b["kind"] == "FP8"), None)
                bf16 = next((b for b in bests if b["kind"] == "BF16"), None)
                combine = next((b for b in bests if b["kind"] == "combine"), None)
                swanlab.log({
                    f"pod_{idx}/fp8_rdma_gbps": fp8["rdma_gbps"] if fp8 else 0,
                    f"pod_{idx}/fp8_nvl_gbps": fp8["nvl_gbps"] if fp8 else 0,
                    f"pod_{idx}/bf16_rdma_gbps": bf16["rdma_gbps"] if bf16 else 0,
                    f"pod_{idx}/bf16_nvl_gbps": bf16["nvl_gbps"] if bf16 else 0,
                    f"pod_{idx}/combine_rdma_gbps": combine["rdma_gbps"] if combine else 0,
                    f"pod_{idx}/combine_nvl_gbps": combine["nvl_gbps"] if combine else 0,
                }, step=idx)

    finally:
        cleanup()


if __name__ == "__main__":
    main()
