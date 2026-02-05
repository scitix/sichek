#!/usr/bin/env python3
"""
DeepEP Tuning Test Script (Single Node)

Runs DeepEP intranode test on each worker pod and parses the three Best results
(FP8 dispatch, BF16 dispatch, combine) per node.
"""

import os
import re
import sys
from typing import Any, Dict, List, Optional

swanlab = None
try:
    import swanlab
except ImportError:
    pass

from common import (
    echo_warn,
    parse_hostnames,
    load_user_config,
    pick_value,
)

from mpijob_helper import (
    MPIJobConfig,
    MPIJobRunner,
    PodResult,
    create_mpijob_arg_parser,
    generate_job_name,
)


# [tuning] Best dispatch (FP8): SMs 24, NVL chunk 32, 349.28 GB/s (NVL), t: 693.48 us
# [tuning] Best dispatch (BF16): SMs 24, NVL chunk 22, 488.13 GB/s (NVL), t: 962.38 us
# [tuning] Best combine: SMs 24, NVL chunk 15: 451.55 GB/s (NVL), t: 1040.33 us
_RE_BEST_DISPATCH = re.compile(
    r"\[tuning\]\s+Best dispatch \((FP8|BF16)\):\s+SMs (\d+),\s+NVL chunk (\d+),\s+([\d.]+) GB/s \(NVL\),\s+t:\s+([\d.]+) us"
)
_RE_BEST_COMBINE = re.compile(
    r"\[tuning\]\s+Best combine:\s+SMs (\d+),\s+NVL chunk (\d+):\s+([\d.]+) GB/s \(NVL\),\s+t:\s+([\d.]+) us"
)


def parse_deepep_best_results(output: str) -> List[Dict[str, Any]]:
    """
    Parse DeepEP tuning log and extract the three Best lines per node.
    Returns list of dicts: [{"kind": "FP8", "sms": 24, "chunk": 32, "gb_s": 349.28, "us": 693.48}, ...]
    """
    bests = []
    for m in _RE_BEST_DISPATCH.finditer(output):
        kind, sms, chunk, gb_s, us = m.group(1), int(m.group(2)), int(m.group(3)), float(m.group(4)), float(m.group(5))
        bests.append({"kind": kind, "sms": sms, "chunk": chunk, "gb_s": gb_s, "us": us})
    for m in _RE_BEST_COMBINE.finditer(output):
        sms, chunk, gb_s, us = int(m.group(1)), int(m.group(2)), float(m.group(3)), float(m.group(4))
        bests.append({"kind": "combine", "sms": sms, "chunk": chunk, "gb_s": gb_s, "us": us})
    return bests


def parse_deepep_for_runner(output: str) -> Optional[float]:
    """Parse fn for execute_on_pods: return number of bests found, or None."""
    bests = parse_deepep_best_results(output)
    if len(bests) >= 3:
        return float(len(bests))
    return None


def print_deepep_results(results: List[PodResult]) -> None:
    """Print per-node summary of the three Best results (FP8, BF16, combine)."""
    print("\n" + "-" * 120)
    print("DeepEP tuning: Best per node (FP8 dispatch, BF16 dispatch, combine)")
    print(
        f"{'Pod Name':<62} | {'Host Name':<14} | {'Best FP8 (GB/s)':<16} | {'t(us)':<10} | "
        f"{'Best BF16 (GB/s)':<17} | {'t(us)':<10} | {'Best combine (GB/s)':<20} | {'t(us)':<10}"
    )
    print("-" * 120)

    for r in results:
        bests = parse_deepep_best_results(r.output)
        fp8 = next((b for b in bests if b["kind"] == "FP8"), None)
        bf16 = next((b for b in bests if b["kind"] == "BF16"), None)
        combine = next((b for b in bests if b["kind"] == "combine"), None)

        def row(b: Optional[Dict]) -> tuple:
            if b is None:
                return "N/A", "N/A"
            return f"{b['gb_s']:.2f}", f"{b['us']:.2f}"

        fp8_gb, fp8_us = row(fp8)
        bf16_gb, bf16_us = row(bf16)
        co_gb, co_us = row(combine)

        print(
            f"{r.pod_name:<62} | {r.host_name:<14} | {fp8_gb:>14} | {fp8_us:>8} | "
            f"{bf16_gb:>15} | {bf16_us:>8} | {co_gb:>18} | {co_us:>8}"
        )

    print("-" * 120)


def log_to_swanlab(results: List[PodResult], swan_run) -> None:
    """Log DeepEP best metrics to SwanLab."""
    if not swan_run or swanlab is None:
        return
    for idx, r in enumerate(results):
        bests = parse_deepep_best_results(r.output)
        fp8 = next((b for b in bests if b["kind"] == "FP8"), None)
        bf16 = next((b for b in bests if b["kind"] == "BF16"), None)
        combine = next((b for b in bests if b["kind"] == "combine"), None)
        swanlab.log({
            f"pod_{idx}/best_fp8_gbps": fp8["gb_s"] if fp8 else 0,
            f"pod_{idx}/best_bf16_gbps": bf16["gb_s"] if bf16 else 0,
            f"pod_{idx}/best_combine_gbps": combine["gb_s"] if combine else 0,
        }, step=idx)


def main() -> None:
    parser = create_mpijob_arg_parser("DeepEP tuning test (single node)")
    parser.description = (
        "Runs DeepEP intranode test on each worker pod and prints the three Best results per node."
    )
    parser.add_argument("--image-repo", default="registry-taihua.siflow.cn/hisys/mcore", help="Container image repository")
    parser.add_argument("--image-tag", default="pytorch25.11-cuda13-cudnn9.17-te-main-v1", help="Container image tag")
    parser.add_argument(
        "--host-dir",
        default=None,
        help="Host directory to mount (e.g. /tmp/DeepEP for DeepEP code)",
    )

    args = parser.parse_args()
    config = load_user_config()

    default_cmd = (
        "python /tmp/DeepEP/tests/test_intranode.py "
        "--num-processes 8 --num-tokens 4096 --hidden 7168 --num-topk 8 --num-experts 8"
    )
    cmd = args.cmd if args.cmd else default_cmd
    if args.host_dir is not None:
        cmd = f"DEEPEP_HOST_DIR={args.host_dir} {cmd}"

    image_repo = pick_value(
        args.image_repo, config, "pytorchjob_image_repo", "registry-us-east.scitix.ai/hisys/mcore"
    )
    image_tag = pick_value(
        args.image_tag, config, "pytorchjob_image_tag", "v2.1-cudnn9.14-te2.8-cuda_arch_10.0_at"
    )

    hostnames = parse_hostnames(
        args.hostfile if args.hostfile != "None" else None,
        args.host if args.host != "None" else None,
    )
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)

    if args.job_name is None:
        args.job_name = "sichek-deepeptest-single"

    mpijob_config = MPIJobConfig(
        job_name=generate_job_name(args.job_name),
        namespace=args.namespace,
        hostnames=hostnames,
        image_repo=image_repo,
        image_tag=image_tag,
        scheduler_name=pick_value(args.scheduler_name, config, "scheduler", "si-scheduler"),
        roce_shared_mode=pick_value(args.roce_shared_mode, config, "roce_shared_mode", "none"),
        timeout=args.timeout,
        max_parallel_jobs=args.max_parallel_jobs,
        cmd=cmd,
    )

    runner = MPIJobRunner(mpijob_config)

    swan_run = None
    if os.getenv("SWANLAB_API_KEY") and swanlab is not None:
        swan_run = swanlab.init(
            experiment_name=mpijob_config.job_name,
            description=f"DeepEP tuning ({len(mpijob_config.hostnames)} workers)",
            config={
                "job_name": mpijob_config.job_name,
                "namespace": mpijob_config.namespace,
                "num_workers": len(mpijob_config.hostnames),
                "command": mpijob_config.cmd,
                "image": f"{mpijob_config.image_repo}:{mpijob_config.image_tag}",
                "timeout": mpijob_config.timeout,
                "hosts": mpijob_config.hostnames,
            },
        )

    try:
        runner.deploy()
        runner.wait_for_pods_ready()
        pods = runner.get_worker_pods()
        if not pods:
            echo_warn("No worker pods found, exiting...")
            return
        results = runner.execute_on_pods(pods, mpijob_config.cmd, parse_deepep_for_runner)
        print_deepep_results(results)
        log_to_swanlab(results, swan_run)
    finally:
        runner.cleanup()


if __name__ == "__main__":
    main()
