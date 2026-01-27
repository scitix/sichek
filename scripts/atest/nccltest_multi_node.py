#!/usr/bin/env python3
"""
NCCL Benchmark Script (Multi Node) with SwanLab Integration
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
from typing import Dict, List, Optional, Tuple

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
    run_cmd,
    run_cmd_check,
    parse_hostnames,
    parse_nccltest_bandwidth,
    load_user_config,
    pick_value,
    start_kubectl_log_stream,
)


def main():
    parser = argparse.ArgumentParser(description="NCCL benchmark (multi node) with SwanLab")
    parser.add_argument("--job-name", default=None)
    parser.add_argument("--namespace", default="default")
    parser.add_argument("--cmd", default="", 
                        help="choose from [allreduce, allgather, reducescatter, alltoall, all], \
                        all means all of the above, default is allreduce")
    parser.add_argument("--image-repo", default=None)
    parser.add_argument("--image-tag", default=None)
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--scheduler-name", default=None)
    parser.add_argument("--roce-shared-mode", default=None)
    parser.add_argument("--hostfile", default="None")
    parser.add_argument("--host", default="None")
    args = parser.parse_args()

    config = load_user_config()
    args.image_repo = pick_value(args.image_repo, config, "image_repo", "registry-us-east.scitix.ai/hisys/sichek")
    args.image_tag = pick_value(args.image_tag, config, "image_tag", "latest")
    args.scheduler_name = pick_value(args.scheduler_name, config, "scheduler", "si-scheduler")
    args.roce_shared_mode = pick_value(args.roce_shared_mode, config, "roce_shared_mode", "none")

    hostnames = parse_hostnames(args.hostfile, args.host)
    if not hostnames:
        echo_warn("No hostnames provided, exiting...")
        sys.exit(1)

    num_workers = len(hostnames)

    # Generate job name if not provided
    if args.job_name is None:
        date_str = datetime.now().strftime("%Y%m%d%H%M%S")
        args.job_name = f"sichek-nccltest-multi-n{num_workers}-{date_str}"
    scripts_dir = Path(__file__).parent.resolve()
    helm_dir = scripts_dir.parent.parent / "k8s" / "sichek"

    log_proc = None

    def cleanup():
        echo_info(f"Cleaning up Helm release: {args.job_name}")
        run_cmd(f"helm uninstall {args.job_name} -n {args.namespace} || true")
        run_cmd(f"kubectl delete mpijob {args.job_name} -n {args.namespace} --ignore-not-found")
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
    if os.getenv("SWANLAB_API_KEY") and swanlab:
        swan_run = swanlab.init(
           experiment_name=args.job_name,
            description=f"NCCL benchmark multi-node ({num_workers} workers)",
            config={
                "job_name": args.job_name,
                "namespace": args.namespace,
                "num_workers": num_workers,
                "command": args.cmd,
                "image": f"{args.image_repo}:{args.image_tag}",
                "timeout": args.timeout,
                "scheduler": args.scheduler_name,
                "roce_mode": args.roce_shared_mode,
                "hosts": hostnames,
            },
        )

    try:
        echo_info(f"Launching MPIJob '{args.job_name}' with {num_workers} workers in namespace '{args.namespace}'")
        echo_info(f"Node affinity hosts: {', '.join(hostnames)}")
        echo_info(f"Image: {args.image_repo}:{args.image_tag}")
        echo_info(f"Timeout: {args.timeout} seconds")

        host_csv = ",".join(hostnames)
        helm_cmd = (
            f"helm upgrade --install {shlex.quote(args.job_name)} {shlex.quote(str(helm_dir))} "
            f"--atomic "
            f"--set namespace={shlex.quote(args.namespace)} "
            f"--set mode=mpijob "
            f"--set schedulerName={shlex.quote(args.scheduler_name)} "
            f"--set roceSharedMode={shlex.quote(args.roce_shared_mode)} "
            f"--set image.repository={shlex.quote(args.image_repo)} "
            f"--set image.tag={shlex.quote(args.image_tag)} "
            f"--set mpijob.name={shlex.quote(args.job_name)} "
            f"--set mpijob.numWorkers={num_workers} "
            f"--set 'mpijob.nodeAffinityHosts={{{host_csv}}}'"
        )
        run_cmd_check(helm_cmd)

        echo_info("Waiting for all pods to be ready...")
        time.sleep(5)
        
        max_attempts = args.timeout // 5
        attempt = 0
        
        while attempt < max_attempts:
            wait_cmd = (
                f"kubectl wait --for=condition=Ready pod "
                f"-l training.kubeflow.org/job-name={args.job_name} "
                f"-n {args.namespace} --timeout={args.timeout}s"
            )
            rc, _, _ = run_cmd(wait_cmd, quiet=True)
            
            # Check if any pods are still Terminating
            check_cmd = (
                f"kubectl get pod -n {args.namespace} | "
                f"grep {args.job_name} | grep Terminating"
            )
            rc2, _, _ = run_cmd(check_cmd, quiet=True)
            
            if rc == 0 and rc2 != 0:  # Wait succeeded and no terminating pods
                echo_info("All pods are ready and no pods are terminating.")
                break
            
            if rc2 == 0:  # Found terminating pods
                echo_info("Some pods are still Terminating... waiting again.")
            elif rc != 0:  # Wait failed (pods might not be created yet)
                echo_info("Waiting for pods to be created and ready...")
            else:
                echo_info("Waiting for pods to be ready...")
            
            time.sleep(5)
            attempt += 1
        
        if attempt >= max_attempts:
            echo_warn(f"Timeout waiting for pods after {args.timeout} seconds")

        # Find launcher pod
        rc, out, _ = run_cmd(
            f"kubectl get mpijob {args.job_name} -n {args.namespace} "
            f"-o jsonpath='{{.status.launcherStatus.podName}}'"
        )
        launcher_pod = out.strip() if rc == 0 else ""
        if not launcher_pod:
            rc, out, _ = run_cmd(
                f"kubectl get pods -n {args.namespace} -o name | "
                f"grep '{args.job_name}-launcher' | head -n1 | sed 's|pods/||'"
            )
            launcher_pod = out.strip()
        if not launcher_pod:
            echo_warn("Error: cannot find launcher Pod")
            return
        echo_info(f"Found launcher pod: {launcher_pod}")
        log_proc, _ = start_kubectl_log_stream(args.namespace, launcher_pod, "launcher")

        # Display worker pods and nodes
        rc, worker_info, _ = run_cmd(
            f"kubectl get pods -n {args.namespace} "
            f"-l training.kubeflow.org/replica-type=worker,training.kubeflow.org/job-name={args.job_name} "
            f"-o jsonpath='{{range .items[*]}}{{.metadata.name}} on {{.spec.nodeName}}{{\"\\n\"}}{{end}}'"
        )
        if worker_info.strip():
            echo_info("Test machines (Worker Pods and their nodes):")
            for line in worker_info.strip().splitlines():
                print(f"  - {line}")

        # NCCL test commands
        default_labels = ["all_reduce", "all_gather", "reduce_scatter", "all2all"]
        default_cmds = ["allreduce", "allgather", "reducescatter", "alltoall"]
        test_labels = default_labels
        test_cmds = default_cmds

        if args.cmd:
            if re.search(r"(allreduce|allgather|reducescatter|alltoall)", args.cmd):
                test_cmds = [args.cmd]
                test_labels = ["custom_nccltest"]
            elif args.cmd == "all":
                test_cmds = default_cmds
                test_labels = default_labels
            else:
                echo_warn(f"Invalid command: {args.cmd}")
                return
        else:
            test_cmds = ["all_reduce -N 20"]
            test_labels = ["all_reduce"]

        results: Dict[str, float] = {}

        for label, cmd in zip(test_labels, test_cmds):
            # Wait for launcher pod to be running
            max_wait = 300
            waited = 0
            while True:
                rc, status, _ = run_cmd(
                    f"kubectl -n {args.namespace} get {launcher_pod} -o jsonpath='{{.status.phase}}'"
                )
                if status.strip() == "Running":
                    break
                if waited >= max_wait:
                    echo_warn(f"{launcher_pod} did not reach Running state after {max_wait} seconds.")
                    return
                time.sleep(5)
                waited += 5

            run_cmd_line = (
                "timeout {timeout} /usr/local/sihpc/bin/mpirun "
                "--allow-run-as-root --map-by ppr:8:node "
                "--mca oob_tcp_if_include eth0 --mca pml ^ucx --mca btl self,tcp "
                "--mca btl_tcp_if_include eth0 --mca routed direct --mca plm_rsh_no_tree_spawn 1 "
                "-x UCX_TLS=tcp -x NCCL_MIN_NCHANNELS=32 -x NCCL_IB_QPS_PER_CONNECTION=8 "
                "/usr/local/sihpc/libexec/nccl-tests/nccl_test -l{cmd}"
            ).format(timeout=args.timeout, cmd=cmd)

            echo_info(f"Running NCCL test: {label}")
            # Wrap command to tee output to container's main process stdout
            # This makes output visible in `kubectl logs` while also capturing it
            wrapped_cmd = f"bash -c {shlex.quote(f'({run_cmd_line}) 2>&1 | tee /proc/1/fd/1')}"
            rc, out, _ = run_cmd(f"kubectl -n {args.namespace} exec {launcher_pod} -- {wrapped_cmd}")
            if "timeout: command terminated" in out:
                echo_warn(f"Command timed out after {args.timeout} seconds")
                results[label] = 0.0
                continue

            bw = parse_nccltest_bandwidth(out)
            if bw is None:
                echo_warn(f"Failed to parse '{label}' bandwidth, set to 0")
                bw = 0.0
            results[label] = bw

        print("\n========== NCCL Benchmark Summary ==========")
        print(f"{'Test':<20} {'GB/s':>10}")
        print("-" * 32)
        for label in test_labels:
            print(f"{label:<20} {results.get(label, 0):>10.2f}")
        print("=" * 40)

        if swan_run:
            for label, bw in results.items():
                swanlab.log({f"bandwidth/{label}": bw})

    finally:
        cleanup()


if __name__ == "__main__":
    main()
