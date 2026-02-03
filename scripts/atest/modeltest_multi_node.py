#!/usr/bin/env python3
"""
Llama2 70B Benchmark Script (Multi Node) with SwanLab Integration
"""

import os
import argparse
import shlex
import signal
import sys
import time
from datetime import datetime
from pathlib import Path

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
    echo_info,
    echo_warn,
    run_cmd,
    run_cmd_check,
    parse_hostnames,
    parse_megatron_tflops_values,
    summarize,
    load_user_config,
    pick_value,
    start_kubectl_log_stream,
    wait_for_pods_ready,
)


def main():
    parser = argparse.ArgumentParser(description="Llama2 70B benchmark (multi node) with SwanLab")
    parser.add_argument("--job-name", default=None)
    parser.add_argument("--namespace", default="default")
    parser.add_argument("--cmd", default="", help="command to run in each pod: PP=1 MBS=4 bash /workspace/ai4s-job-system/mcore_trainer/demos/llama/train_llama2_70b_bf16.sh by default")
    parser.add_argument("--image-repo", default=None)
    parser.add_argument("--image-tag", default=None)
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--scheduler-name", default=None)
    parser.add_argument("--roce-shared-mode", default=None)
    parser.add_argument("--hostfile", default="None")
    parser.add_argument("--host", default="None")
    parser.add_argument("--max-steps", type=int, default=128)
    parser.add_argument("--mbs", type=int, default=None, help="micro batch size")
    parser.add_argument("--tp", type=int, default=None, help="Tensor Parallelism size")
    parser.add_argument("--pp", type=int, default=None, help="Pipeline Parallelism size")
    parser.add_argument("--ep", type=int, default=None, help="Expert Parallelism size")
    parser.add_argument("--host-dir", default=None, help="host directory to mount in pytorchjob pods")
    parser.add_argument("--gpu-type", default=None, help="GPU type")

    args = parser.parse_args()

    config = load_user_config()

    args.image_repo = pick_value(args.image_repo, config, "pytorchjob_image_repo", "registry-us-east.scitix.ai/hisys/mcore")

    args.image_tag = pick_value(args.image_tag, config, "pytorchjob_image_tag", "v2.1-cudnn9.14-te2.8-cuda_arch_10.0_at")

    args.scheduler_name = pick_value(args.scheduler_name, config, "scheduler", "si-scheduler")
    args.roce_shared_mode = pick_value(args.roce_shared_mode, config, "roce_shared_mode", "none")
    args.gpu_type = pick_value(args.gpu_type, config, "gpu_type", "h100")

    hostnames = parse_hostnames(args.hostfile, args.host)
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)

    num_workers = len(hostnames)
    date_str = datetime.now().strftime("%Y%m%d%H%M%S")
    if args.job_name is None:
        args.job_name = f"sichek-modeltest-multi-n{num_workers}-{date_str}"
    else:
        args.job_name = f"{args.job_name}-n{num_workers}-{date_str}"
    if args.gpu_type.lower() == "b200":
        default_cmd = (
            "PP=1 MBS=4 bash /workspace/ai4s-job-system/mcore_trainer/demos/llama/train_llama2_70b_bf16.sh"
        )
    elif args.gpu_type.lower() == "h200":
        default_cmd = (
            "PP=2 bash /workspace/ai4s-job-system/mcore_trainer/demos/llama/train_llama2_70b_bf16.sh"
        )
    else:
        default_cmd = (
            "bash /workspace/ai4s-job-system/mcore_trainer/demos/llama/train_llama2_70b_bf16.sh"
        )

    gbs = 128 * num_workers
    if not args.cmd:
        args.cmd = default_cmd
    cmd = f"GBS={gbs} MAX_STEPS={args.max_steps} {args.cmd}"
    if args.mbs is not None:
        cmd = f"MBS={args.mbs} {cmd}"
    if args.tp is not None:
        cmd = f"TP={args.tp} {cmd}"
    if args.pp is not None:
        cmd = f"PP={args.pp} {cmd}"
    if args.ep is not None:
        cmd = f"EP={args.ep} {cmd}"
    if args.host_dir is not None:
        cmd = f"OLMO_CORE_DIR={args.host_dir} {cmd}"
    if os.getenv("SWANLAB_API_KEY") is not None:
        cmd = (
            f"export SWANLAB_API_KEY={os.getenv('SWANLAB_API_KEY')} && "
            f"export SWANLAB_WORKSPACE={os.getenv('SWANLAB_WORKSPACE')} && "
            f"export SWANLAB_PROJ_NAME={os.getenv('SWANLAB_PROJ_NAME')} && "
            f"export SWANLAB_EXP_NAME={args.job_name} && "
            f"{cmd}"
        )
    else:
        cmd = (
            f"export SWANLAB_MODE=disabled && "
            f"{cmd}"
        )
    
    scripts_dir = Path(__file__).parent.resolve()
    helm_dir = scripts_dir.parent.parent / "k8s" / "sichek"

    log_proc = None

    def cleanup():
        echo_info(f"Cleaning up Helm release: {args.job_name}")
        run_cmd(f"helm uninstall {args.job_name} -n {args.namespace} || true")
        run_cmd(f"kubectl delete pytorchjob {args.job_name} -n {args.namespace} --ignore-not-found")
        if log_proc:
            try:
                log_proc.terminate()
                log_proc.wait(timeout=5)
            except Exception:
                pass

    def signal_handler(sig, frame):
        echo_info("Interrupted, cleaning up...")
        cleanup()
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    swan_run = None
    if os.getenv("SWANLAB_API_KEY") and swanlab is not None:
        swan_run = swanlab.init(
            experiment_name=args.job_name,
            description=f"Llama2 70B benchmark ({num_workers} workers)",
            config={
                "job_name": args.job_name,
                "namespace": args.namespace,
                "num_workers": num_workers,
                "command": cmd,
                "image": f"{args.image_repo}:{args.image_tag}",
                "timeout": args.timeout,
                "scheduler": args.scheduler_name,
                "roce_mode": args.roce_shared_mode,
                "hosts": hostnames,
                "tp": args.tp,
                "pp": args.pp,
                "ep": args.ep,
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
        
        # First, wait for pods to be created (poll until they exist)
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
            echo_warn(f"Timeout waiting for worker pods to be created after {max_wait} seconds")
            return
        
        # Now wait for all worker pods to be ready
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
        
        # Wait for master pod to be ready
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

        # Find last worker pod
        pod_cmd = (
            f"kubectl get pod -n {args.namespace} -l training.kubeflow.org/replica-type=worker,"
            f"training.kubeflow.org/job-name={args.job_name} "
            f"-o jsonpath='{{.items[*].metadata.name}}'"
        )
        rc, out, _ = run_cmd(pod_cmd, quiet=True)
        if rc != 0 or not out.strip():
            echo_warn(f"No worker pods found for job '{args.job_name}'")
            return
        pods = sorted(out.strip().split())
        last_pod = pods[-1]
        echo_info(f"Last worker pod name: {last_pod}")

        # Find master pod
        master_pod_cmd = (
            f"kubectl get pod -n {args.namespace} -l training.kubeflow.org/replica-type=master,"
            f"training.kubeflow.org/job-name={args.job_name} "
            f"-o jsonpath='{{.items[*].metadata.name}}'"
        )
        rc, out, _ = run_cmd(master_pod_cmd, quiet=True)
        if rc != 0 or not out.strip():
            echo_warn(f"No master pod found for job '{args.job_name}'")
            return

        master_pods = out.strip().split()
        if len(master_pods) != 1:
            echo_warn(f"Unexpected number of master pods: {master_pods}")

        master_pod = master_pods[0]
        echo_info(f"Master pod name: {master_pod}")

        if "olmo" in args.job_name.lower():
            log_proc, _ = start_kubectl_log_stream(args.namespace, master_pod, "master-log")
        else:
            log_proc, _ = start_kubectl_log_stream(args.namespace, last_pod, "worker-log")

        # Wait for completion
        echo_info("=" * 80)
        echo_info(f"Waiting for PyTorchJob {args.job_name} to complete...")
        echo_info("=" * 80)
        
        timeout = args.timeout
        interval = 10
        elapsed = 0
        
        while elapsed < timeout:
            # Check PyTorchJob status
            rc, out, _ = run_cmd(
                f"kubectl get pytorchjob {args.job_name} -n {args.namespace} | "
                f"grep -v NAME | awk '{{print $2}}'", quiet=True
            )
            status = out.strip() if rc == 0 else ""
            
            if status in ("Succeeded", "Failed"):
                echo_info(f"PyTorchJob Status: {status}")
                break
            
            # Check last pod status and print last log line if Running
            rc2, out2, _ = run_cmd(
                f"kubectl get pod {last_pod} -n {args.namespace} | "
                f"grep -v NAME | awk '{{print $3}}'", quiet=True
            )
            pod_status = out2.strip() if rc2 == 0 else ""
            
            if pod_status == "Running":
                rc3, last_log, _ = run_cmd(
                    f"kubectl logs -n {args.namespace} {last_pod} | tail -n 1",
                    quiet=True
                )
                if rc3 == 0 and last_log.strip():
                    echo_info(last_log.strip())
            
            time.sleep(interval)
            elapsed += interval
        
        if elapsed >= timeout:
            echo_warn(f"Timeout waiting for PyTorchJob {args.job_name} to reach complete state.")

        # Parse TFLOPS from logs
        rc, logs, _ = run_cmd(f"kubectl logs -n {args.namespace} {last_pod}")
        values = parse_megatron_tflops_values(logs)
        avg, mn, mx, std = summarize(values)

        print(f"{'Job Name':<30} | {'Avg':<9} | {'Min':<9} | {'Max':<9} | {'StdDev':<9}")
        if avg is None:
            print(f"{args.job_name:<30} | {'N/A':<9} | {'N/A':<9} | {'N/A':<9} | {'N/A':<9}")
        else:
            print(f"{args.job_name:<30} | {avg:<9.2f} | {mn:<9.2f} | {mx:<9.2f} | {std:<9.2f}")

        if swan_run and avg is not None:
            swanlab.log(
                {
                    "tflops/avg": avg,
                    "tflops/min": mn,
                    "tflops/max": mx,
                    "tflops/stddev": std,
                    "tflops/count": len(values),
                }
            )

    finally:
        cleanup()


if __name__ == "__main__":
    main()
