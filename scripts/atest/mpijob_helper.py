#!/usr/bin/env python3
"""
MPIJob Helper - Common utilities for deploying and running MPIJobs.

This module provides a unified interface for:
- Deploying MPIJobs via Helm
- Waiting for worker pods to be ready
- Executing commands in each worker pod in parallel
- Streaming pod logs in real-time
- Cleanup resources
"""

import os
import shlex
import shutil
import signal
import sys
import tempfile
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Tuple

from common import (
    echo_info,
    echo_warn,
    run_cmd,
    run_cmd_check,
    parse_hostnames,
    load_user_config,
    pick_value,
    start_kubectl_log_stream,
)


@dataclass
class PodInfo:
    """Information about a worker pod"""
    name: str
    host_ip: str
    host_name: str


@dataclass
class PodResult:
    """Result from executing a command in a pod"""
    pod_name: str
    host_ip: str
    host_name: str
    output: str = ""
    error: Optional[str] = None
    parsed_value: Optional[float] = None
    extra: Dict[str, Any] = field(default_factory=dict)


@dataclass
class MPIJobConfig:
    """Configuration for an MPIJob"""
    job_name: str
    namespace: str
    hostnames: List[str]
    image_repo: str
    image_tag: str
    scheduler_name: str
    roce_shared_mode: str
    timeout: int
    max_parallel_jobs: int = 200
    cmd: str = ""

class MPIJobRunner:
    """
    Helper class for deploying and running command in each pod of MPIJobs.
    
    Usage:
        config = MPIJobConfig(...)
        runner = MPIJobRunner(config)
        
        try:
            runner.deploy()
            runner.wait_for_pods_ready()
            pods = runner.get_worker_pods()
            runner.start_log_stream(pods)
            results = runner.execute_on_pods(pods, my_command, my_parser)
            # Process results...
        finally:
            runner.cleanup()
    """
    
    def __init__(self, config: MPIJobConfig):
        self.config = config
        self.scripts_dir = Path(__file__).parent.resolve()
        self.helm_dir = self.scripts_dir.parent.parent / "k8s" / "sichek"
        self.tmp_dir: Optional[str] = None
        self.log_proc = None
        self._setup_signal_handlers()
    
    def _setup_signal_handlers(self):
        """Setup signal handlers for cleanup on interrupt"""
        def handler(sig, frame):
            echo_info("Interrupted, cleaning up...")
            self.cleanup()
            sys.exit(0)
        
        signal.signal(signal.SIGINT, handler)
        signal.signal(signal.SIGTERM, handler)
    
    def deploy(self):
        """Deploy MPIJob using Helm"""
        cfg = self.config
        
        print("=" * 100)
        echo_info(f"Launching MPIJob '{cfg.job_name}' with {len(cfg.hostnames)} workers in namespace '{cfg.namespace}'")
        echo_info(f"Command to be executed in each pod: {cfg.cmd}")
        echo_info(f"Node affinity hosts: {', '.join(cfg.hostnames)}")
        echo_info(f"Image: {cfg.image_repo}:{cfg.image_tag}")
        echo_info(f"Timeout: {cfg.timeout} seconds")
        print("=" * 100)
        
        host_csv = ",".join(cfg.hostnames)
        helm_cmd = (
            f"helm upgrade --install {shlex.quote(cfg.job_name)} {shlex.quote(str(self.helm_dir))} "
            f"--atomic "
            f"--set namespace={shlex.quote(cfg.namespace)} "
            f"--set mode=mpijob "
            f"--set schedulerName={shlex.quote(cfg.scheduler_name)} "
            f"--set roceSharedMode={shlex.quote(cfg.roce_shared_mode)} "
            f"--set image.repository={shlex.quote(cfg.image_repo)} "
            f"--set image.tag={shlex.quote(cfg.image_tag)} "
            f"--set mpijob.name={shlex.quote(cfg.job_name)} "
            f"--set mpijob.numWorkers={len(cfg.hostnames)} "
            f"--set 'mpijob.nodeAffinityHosts={{{host_csv}}}'"
        )
        run_cmd_check(helm_cmd)
    
    def wait_for_pods_ready(self):
        """Wait for all worker pods to be ready"""
        cfg = self.config
        echo_info("Waiting for all worker pods to be ready...")
        time.sleep(5)
        
        max_attempts = cfg.timeout // 5
        
        for attempt in range(max_attempts):
            wait_cmd = (
                f"kubectl wait --for=condition=Ready "
                f"pod -l training.kubeflow.org/job-name={cfg.job_name} "
                f"-n {cfg.namespace} --timeout=5s"
            )
            rc, _, _ = run_cmd(wait_cmd)
            
            if rc == 0:
                # Check if any pods are still terminating
                check_cmd = f"kubectl get pod -n {cfg.namespace} | grep {cfg.job_name} | grep Terminating"
                rc2, _, _ = run_cmd(check_cmd)
                if rc2 != 0:  # No terminating pods found
                    echo_info("All worker pods are ready.")
                    return
            
            if attempt < max_attempts - 1:
                echo_info("Some pods are not ready yet... waiting.")
                time.sleep(5)
        
        echo_warn(f"Timeout waiting for pods after {cfg.timeout} seconds")
    
    def get_worker_pods(self) -> List[PodInfo]:
        """Get list of worker pods with their details"""
        cfg = self.config
        
        pod_cmd = (
            f"kubectl get pods -n {cfg.namespace} "
            f"-l training.kubeflow.org/job-name={cfg.job_name},training.kubeflow.org/replica-type=worker "
            f"-o jsonpath='{{.items[*].metadata.name}}'"
        )
        rc, out, _ = run_cmd(pod_cmd)
        
        if rc != 0 or not out.strip():
            echo_warn(f"No worker pods found for job '{cfg.job_name}'")
            return []
        
        pod_names = out.strip().split()
        pods = []
        
        for pod_name in pod_names:
            host_ip_cmd = f"kubectl get pod -n {cfg.namespace} {pod_name} -o jsonpath='{{.status.hostIP}}'"
            host_name_cmd = f"kubectl get pod -n {cfg.namespace} {pod_name} -o jsonpath='{{.spec.nodeName}}'"
            
            _, host_ip_out, _ = run_cmd(host_ip_cmd)
            _, host_name_out, _ = run_cmd(host_name_cmd)
            
            pods.append(PodInfo(
                name=pod_name,
                host_ip=host_ip_out.strip() if host_ip_out else "N/A",
                host_name=host_name_out.strip() if host_name_out else "N/A",
            ))
        
        echo_info(f"Found {len(pods)} worker pod(s).")
        return pods
    
    def start_log_stream(self, pods: List[PodInfo], prefix: str = "worker-log"):
        """Start streaming logs from the last worker pod"""
        if not pods:
            echo_warn("No worker pods found for log streaming")
            return
        
        last_pod = pods[-1].name
        echo_info(f"Streaming logs from last worker pod: {last_pod}")
        self.log_proc, _ = start_kubectl_log_stream(self.config.namespace, last_pod, prefix)
    
    def execute_on_pod(
        self,
        pod: PodInfo,
        command: str,
        parse_fn: Optional[Callable[[str], Optional[float]]] = None,
        quiet: bool = True,
    ) -> PodResult:
        """Execute a command in a single pod and return the result.
        
        Output is also written to /proc/1/fd/1 so it appears in `kubectl logs`.
        """
        cfg = self.config
        
        # Wrap command to tee output to container's main process stdout
        # This makes output visible in `kubectl logs` while also capturing it
        wrapped_cmd = f"({command}) 2>&1 | tee /proc/1/fd/1"
        exec_cmd = f"kubectl exec -n {cfg.namespace} {pod.name} -- bash -c {shlex.quote(wrapped_cmd)}"
        rc, stdout, stderr = run_cmd(exec_cmd, quiet=quiet)
        output = (stdout or "") + (stderr or "")
        
        parsed_value = None
        error = None
        
        if rc != 0:
            error = f"Exit code {rc}"
        elif parse_fn:
            try:
                parsed_value = parse_fn(output)
            except Exception as e:
                error = f"Parse error: {e}"
        
        return PodResult(
            pod_name=pod.name,
            host_ip=pod.host_ip,
            host_name=pod.host_name,
            output=output,
            error=error,
            parsed_value=parsed_value,
        )
    
    def execute_on_pods(
        self,
        pods: List[PodInfo],
        command: str,
        parse_fn: Optional[Callable[[str], Optional[float]]] = None,
        show_last_pod_output: bool = True,
    ) -> List[PodResult]:
        """Execute a command in all pods in parallel.
        
        Args:
            pods: List of pods to execute on
            command: Command to execute
            parse_fn: Optional function to parse output
            show_last_pod_output: If True, show output from the last pod only
        """
        cfg = self.config
        echo_info("Launching tests...")
        echo_info(f"Tests will be run in parallel (up to {cfg.max_parallel_jobs} concurrently)")
        
        if not pods:
            return []
        
        results = []
        last_pod = pods[-1]
        other_pods = pods[:-1]
        max_workers = min(cfg.max_parallel_jobs, len(pods))
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            # Submit all pods: others are quiet, last pod shows output
            future_to_pod = {}
            for pod in other_pods:
                future = executor.submit(self.execute_on_pod, pod, command, parse_fn, True)
                future_to_pod[future] = pod
            
            # Last pod: show output if requested
            if show_last_pod_output:
                echo_info(f"Showing output from last pod: {last_pod.name}")
            future = executor.submit(self.execute_on_pod, last_pod, command, parse_fn, not show_last_pod_output)
            future_to_pod[future] = last_pod
            
            for future in as_completed(future_to_pod):
                pod = future_to_pod[future]
                try:
                    result = future.result()
                    results.append(result)
                    
                    if result.parsed_value is not None:
                        echo_info(f"Completed: {result.pod_name} -> {result.parsed_value}")
                    elif result.error:
                        echo_warn(f"Failed: {result.pod_name} -> {result.error}")
                    else:
                        echo_info(f"Completed: {result.pod_name}")
                except Exception as e:
                    echo_warn(f"Exception for {pod.name}: {e}")
                    results.append(PodResult(
                        pod_name=pod.name,
                        host_ip=pod.host_ip,
                        host_name=pod.host_name,
                        error=str(e),
                    ))
        
        return results
    
    def cleanup(self):
        """Cleanup Helm release and resources"""
        cfg = self.config
        echo_info(f"Cleaning up Helm release: {cfg.job_name}")
        run_cmd(f"helm uninstall {cfg.job_name} -n {cfg.namespace} || true")
        run_cmd(f"kubectl delete mpijob {cfg.job_name} -n {cfg.namespace} --ignore-not-found")
        
        if self.tmp_dir and os.path.exists(self.tmp_dir):
            shutil.rmtree(self.tmp_dir)
        
        if self.log_proc:
            try:
                self.log_proc.terminate()
                self.log_proc.wait(timeout=5)
            except Exception:
                pass


def create_mpijob_arg_parser(description: str):
    """Create a standard argument parser for MPIJob scripts"""
    import argparse
    
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument("--job-name", default=None, help="Job name (default: auto-generated)")
    parser.add_argument("--namespace", default="default", help="Kubernetes namespace")
    parser.add_argument("--cmd", default="", help="Command to run in each pod")
    # Note: --image-repo and --image-tag should be added by each script as they differ
    parser.add_argument("--timeout", type=int, default=600, help="Timeout in seconds")
    parser.add_argument("--scheduler-name", default=None, help="Kubernetes scheduler name")
    parser.add_argument("--roce-shared-mode", default=None, help="RoCE shared mode")
    parser.add_argument("--hostfile", default="None", help="File containing hostnames")
    parser.add_argument("--host", default="None", help="Comma-separated hostnames")
    parser.add_argument("--max-parallel-jobs", type=int, default=200, help="Max parallel jobs")
    
    return parser


def generate_job_name(prefix: str) -> str:
    date_str = datetime.now().strftime("%Y%m%d%H%M%S")
    return f"{prefix}-{date_str}"



