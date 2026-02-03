#!/usr/bin/env python3
"""
Model Benchmark Script (Single Node) with SwanLab Integration

Runs model training benchmarks (e.g., Llama2 13B) on each worker pod
and parses TFLOPS/GPU metrics.
"""

import os
import sys
from statistics import mean
from typing import List, Optional

swanlab = None
try:
    import swanlab
except ImportError:
    print("swanlab not installed, installing...")
    import subprocess
    try:
        subprocess.check_call(["pip", "install", "-q", "swanlab"])
        import swanlab
    except Exception:
        print("swanlab not installed online, skipping...")
        pass

from common import (
    echo_warn,
    parse_megatron_tflops_values,
    parse_hostnames,
    summarize,
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


def parse_tflops(output: str) -> Optional[float]:
    """Parse TFLOPS values from Megatron output and return average"""
    values = parse_megatron_tflops_values(output)
    if not values:
        return None
    return mean(values)


def print_results(results: List[PodResult]):
    """Print formatted results table"""
    print("\n" + "-" * 110)
    print("Processing TFLOP/s/GPU results...")
    print(
        f"{'Pod Name':<40} | {'Host Name':<15} | {'Host IP':<15} | "
        f"{'Avg':<10} | {'Min':<10} | {'Max':<10} | {'StdDev':<10}"
    )
    print("-" * 110)
    
    for r in results:
        values = parse_megatron_tflops_values(r.output)
        avg, mn, mx, std = summarize(values)
        
        def fmt(v: Optional[float]) -> str:
            return f"{v:.3f}" if v is not None else "N/A"
        
        print(
            f"{r.pod_name:<40} | {r.host_name:<15} | {r.host_ip:<15} | "
            f"{fmt(avg):<10} | {fmt(mn):<10} | {fmt(mx):<10} | {fmt(std):<10}"
        )
    
    print("-" * 110)


def log_to_swanlab(results: List[PodResult], swan_run):
    """Log results to SwanLab"""
    if not swan_run:
        return
    
    for idx, r in enumerate(results):
        values = parse_megatron_tflops_values(r.output)
        avg, mn, mx, std = summarize(values)
        if avg is not None:
            swanlab.log({
                f"pod_{idx}/avg": avg,
                f"pod_{idx}/min": mn,
                f"pod_{idx}/max": mx,
                f"pod_{idx}/stddev": std,
            }, step=idx)
    
    all_avgs = []
    for r in results:
        values = parse_megatron_tflops_values(r.output)
        if values:
            all_avgs.append(mean(values))
    
    if all_avgs:
        swanlab.log({
            "summary/avg": mean(all_avgs),
            "summary/min": min(all_avgs),
            "summary/max": max(all_avgs),
            "summary/count": len(all_avgs),
        })


def main():
    parser = create_mpijob_arg_parser("Model benchmark (single node) with SwanLab")
    parser.description = (
        "Runs model training benchmarks on each worker pod and parses TFLOPS/GPU metrics."
    )
    parser.add_argument("--image-repo", default=None, help="Container image repository")
    parser.add_argument("--image-tag", default=None, help="Container image tag")
    parser.add_argument("--max-steps", type=int, default=128)
    parser.add_argument("--mbs", type=int, default=None, help="micro batch size")
    parser.add_argument("--host-dir", default=None, help="host directory to mount in pytorchjob pods")

    args = parser.parse_args()
    
    config = load_user_config()
    
    default_cmd = (
        "bash /workspace/ai4s-job-system/mcore_trainer/demos/llama/train_llama2_13b_bf16.sh"
    )
    if not args.cmd:
        args.cmd = default_cmd
    cmd = f"MAX_STEPS={args.max_steps} {args.cmd}"
    if args.mbs is not None:
        cmd = f"MBS={args.mbs} {cmd}"
    if args.host_dir is not None:
        cmd = f"OLMO_CORE_DIR={args.host_dir} {cmd}"
    
    image_repo = pick_value(args.image_repo, config, "pytorchjob_image_repo", "registry-us-east.scitix.ai/hisys/mcore")
    image_tag = pick_value(args.image_tag, config, "pytorchjob_image_tag", "v2.1-cudnn9.14-te2.8-cuda_arch_10.0_at")
    
    hostnames = parse_hostnames(
        args.hostfile if args.hostfile != "None" else None,
        args.host if args.host != "None" else None,
    )
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)
    
    if args.job_name is None:
        args.job_name = "sichek-modeltest-single"
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
            description=f"Model benchmark ({len(mpijob_config.hostnames)} workers)",
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
        # Deploy and run
        runner.deploy()
        runner.wait_for_pods_ready()
        pods = runner.get_worker_pods()
        
        if not pods:
            echo_warn("No worker pods found, exiting...")
            return
        
        results = runner.execute_on_pods(pods, mpijob_config.cmd, parse_tflops)
        
        print_results(results)
        log_to_swanlab(results, swan_run)
        
    finally:
        runner.cleanup()


if __name__ == "__main__":
    main()

