#!/usr/bin/env python3
"""
NCCL Benchmark Script (Single Node) with SwanLab Integration

Runs NCCL tests on each worker pod in parallel and parses bandwidth metrics.
"""

import os
import sys
from typing import Dict, List, Optional

try:
    import swanlab
except ImportError:
    print("swanlab not installed, installing...")
    import subprocess
    subprocess.check_call(["pip", "install", "-q", "swanlab"])
    import swanlab

from common import (
    echo_info,
    echo_warn,
    parse_nccltest_bandwidth,
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


def print_results(results: List[PodResult]):
    """Print formatted results table"""
    print("\n" + "=" * 100)
    print("Processing results...")
    print("-" * 100)
    print(f"{'Pod Name':<40} | {'Host IP':<15} | {'Host Name':<20} | {'Avg Bus Bandwidth (GB/s)':<20}")
    print("-" * 100)
    
    for r in results:
        bandwidth = parse_nccltest_bandwidth(r.output)
        bandwidth_str = f"{bandwidth:.3f}" if bandwidth is not None else (r.error or "Error/No Output")
        print(f"{r.pod_name:<40} | {r.host_ip:<15} | {r.host_name:<20} | {bandwidth_str:<20}")
    
    print("-" * 100)


def calculate_statistics(results: List[PodResult]) -> Dict:
    """Calculate statistics from results"""
    bandwidths = [parse_nccltest_bandwidth(r.output) for r in results]
    bandwidths = [b for b in bandwidths if b is not None]
    
    if not bandwidths:
        return {
            "count": 0,
            "average": None,
            "min": None,
            "max": None,
            "total_pods": len(results)
        }
    
    return {
        "count": len(bandwidths),
        "average": sum(bandwidths) / len(bandwidths),
        "min": min(bandwidths),
        "max": max(bandwidths),
        "total_pods": len(results)
    }


def print_summary(stats: Dict):
    """Print summary statistics"""
    print("\n" + "=" * 100)
    print("Summary of Avg Bus Bandwidth Results:")
    print("-" * 100)
    
    if stats["count"] == 0:
        print(f"No bandwidth values were successfully parsed from {stats['total_pods']} pod(s) found.")
        return
    
    print(f"Number of pods successfully parsed: {stats['count']} / {stats['total_pods']}")
    print(f"Average Bus Bandwidth (overall):    {stats['average']:.3f} GB/s")
    print(f"Minimum Bus Bandwidth:              {stats['min']:.3f} GB/s")
    print(f"Maximum Bus Bandwidth:              {stats['max']:.3f} GB/s")
    print("-" * 100)


def log_to_swanlab(stats: Dict, swan_run):
    """Log results to SwanLab"""
    if not swan_run or stats["average"] is None:
        return
    
    swanlab.log({
        "bandwidth/average": stats["average"],
        "bandwidth/min": stats["min"],
        "bandwidth/max": stats["max"],
        "bandwidth/success_count": stats["count"],
        "bandwidth/total_pods": stats["total_pods"],
    })


def main():
    parser = create_mpijob_arg_parser("NCCL benchmark (single node) with SwanLab")
    parser.description = (
        "Runs NCCL tests on each worker pod in parallel and parses bandwidth metrics."
    )
    parser.add_argument("--image-repo", default=None, help="Container image repository")
    parser.add_argument("--image-tag", default=None, help="Container image tag")
    parser.add_argument("--request-gpu", action="store_true", help="Request GPU resources for each worker pod")
    args = parser.parse_args()
    
    config = load_user_config()
    
    image_repo = pick_value(args.image_repo, config, "image_repo", "registry-us-east.scitix.ai/hisys/sichek")
    image_tag = pick_value(args.image_tag, config, "image_tag", "latest")
    
    if not args.cmd:
        cmd = "NCCL_DEBUG=INFO /usr/local/sihpc/libexec/nccl-tests/nccl_test -g 8"
    else:
        cmd = args.cmd
    
    hostnames = parse_hostnames(
        args.hostfile if args.hostfile != "None" else None,
        args.host if args.host != "None" else None,
    )
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)
    
    if args.job_name is None:
        args.job_name = "sichek-nccltest-single"
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
        request_gpu=args.request_gpu,
    )
    
    runner = MPIJobRunner(mpijob_config)
    
    swan_run = None
    if os.getenv("SWANLAB_API_KEY") and swanlab:
        swan_run = swanlab.init(
            experiment_name=mpijob_config.job_name,
            description=f"NCCL benchmark ({len(mpijob_config.hostnames)} workers)",
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
        
        results = runner.execute_on_pods(pods, mpijob_config.cmd, parse_nccltest_bandwidth)
        
        print_results(results)
        stats = calculate_statistics(results)
        print_summary(stats)
        log_to_swanlab(stats, swan_run)
        
    finally:
        runner.cleanup()


if __name__ == "__main__":
    main()
