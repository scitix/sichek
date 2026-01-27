#!/usr/bin/env python3
"""
Shared helpers for acceptance-test scripts.
"""

import os
import re
import subprocess
import sys
import threading
from statistics import mean, pstdev
from typing import Dict, List, Optional, Tuple


class Colors:
    GREEN = "\033[1;32m"
    YELLOW = "\033[1;33m"
    PURPLE = "\033[1;35m"
    RESET = "\033[0m"


def echo_info(msg: str):
    print(f"[{Colors.GREEN}INFO{Colors.RESET}] {msg}")


def echo_warn(msg: str):
    print(f"[{Colors.YELLOW}WARN{Colors.RESET}] {msg}")

def run_cmd(
    cmd: str,
    check: bool = False,
    exit_on_error: bool = False,
    quiet: bool = False,
) -> Tuple[int, str, str]:
    """
    Run a shell command, stream stdout/stderr to screen in real time,
    and capture them for post-processing.
    
    Args:
        cmd: Command to execute
        check: If True, print warning on non-zero exit
        exit_on_error: If True, exit program on failure
        quiet: If True, don't print command or output to screen
    """
    if not quiet:
        print(f"[EXEC] {cmd}")

    proc = subprocess.Popen(
        cmd,
        shell=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,              # line buffered
        universal_newlines=True
    )

    stdout_lines = []
    stderr_lines = []

    # 同时读 stdout / stderr，避免阻塞
    from threading import Thread

    def read_stdout():
        for line in proc.stdout:
            if not quiet:
                print(line, end="")
            stdout_lines.append(line)

    def read_stderr():
        for line in proc.stderr:
            if not quiet:
                print(line, end="", file=sys.stderr)
            stderr_lines.append(line)

    t_out = Thread(target=read_stdout)
    t_err = Thread(target=read_stderr)

    t_out.start()
    t_err.start()

    proc.wait()
    t_out.join()
    t_err.join()

    stdout = "".join(stdout_lines)
    stderr = "".join(stderr_lines)
    rc = proc.returncode

    if rc != 0 and (check or exit_on_error):
        if not quiet:
            print(f"[WARN] Command failed with exit code {rc}")
            if stderr:
                print(f"[WARN] stderr:\n{stderr}", file=sys.stderr)
        if exit_on_error:
            sys.exit(rc)

    return rc, stdout, stderr

def run_cmd_check(cmd: str, exit_on_error: bool = True) -> Tuple[int, str, str]:
    """
    Run a shell command and check for errors.
    
    Args:
        cmd: Command to execute
        exit_on_error: If True, exit program on failure (default: True)
    
    Returns:
        (returncode, stdout, stderr)
    """
    return run_cmd(cmd, check=True, exit_on_error=exit_on_error)


def load_user_config(config_path: Optional[str] = None) -> Dict[str, str]:
    if config_path is None:
        config_path = os.path.expanduser("~/.sichek/config.yaml")

    if not os.path.isfile(config_path):
        return {}

    config: Dict[str, str] = {}
    try:
        with open(config_path, "r") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if ":" not in line:
                    continue
                key, value = line.split(":", 1)
                key = key.strip()
                value = value.strip()
                if not key:
                    continue
                # strip surrounding quotes
                if (value.startswith("'") and value.endswith("'")) or (value.startswith('"') and value.endswith('"')):
                    value = value[1:-1]
                config[key] = value
    except Exception as exc:
        echo_warn(f"Failed to read config {config_path}: {exc}")
        return {}
    set_swanlab_env(config)
    return config

def set_swanlab_env(config: Dict[str, str]):
    # Helper to check if a value is valid (not None, empty, or string "None")
    def is_valid_value(value):
        if value is None:
            return False
        if isinstance(value, str):
            value = value.strip()
            if value == "" or value.lower() == "none":
                return False
        return True
    
    # Handle swanlab_api_key: set if valid, clear if explicitly set to None/empty
    if "swanlab_api_key" in config:
        if is_valid_value(config["swanlab_api_key"]):
            os.environ["SWANLAB_API_KEY"] = config["swanlab_api_key"]
        else:
            os.environ.pop("SWANLAB_API_KEY", None)
    
    if "swanlab_workspace" in config:
        os.environ["SWANLAB_WORKSPACE"] = config["swanlab_workspace"]
    if "swanlab_proj_name" in config:
        os.environ["SWANLAB_PROJ_NAME"] = config["swanlab_proj_name"]

def pick_value(cli_value: Optional[str], config: Dict[str, str], key: str, default: str) -> str:
    if cli_value not in (None, ""):
        return cli_value
    cfg_value = config.get(key)
    if cfg_value not in (None, ""):
        return cfg_value
    return default

def parse_hostnames(hostfile: Optional[str], host: Optional[str]) -> List[str]:
    hostnames = []
    if host and host.lower() != "none":
        echo_info(f"Parsing hostnames from parameter: {host}")
        hostnames = [h.strip() for h in host.split(",") if h.strip()]
    elif hostfile and hostfile.lower() != "none" and os.path.isfile(hostfile):
        echo_info(f"Reading hostnames from file: {hostfile}")
        with open(hostfile, "r") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    hostname = line.split()[0].split(":")[0]
                    if hostname:
                        hostnames.append(hostname)
    return hostnames


def parse_nccltest_bandwidth(output: str) -> Optional[float]:
    match = re.search(r"Avg bus bandwidth\s*:\s*([0-9]+(?:\.[0-9]+)?)", output)
    if not match:
        return None
    try:
        return float(match.group(1))
    except ValueError:
        return None


def parse_megatron_tflops_values(output: str) -> List[float]:
    clean = re.sub(r"\x1B\[[0-9;]*[mGK]", "", output)
    pattern = r"throughput per GPU \(TFLOP/s/GPU\):\s*([0-9]+(?:\.[0-9]+)?)"
    return [float(m) for m in re.findall(pattern, clean)]


def summarize(values: List[float]) -> Tuple[Optional[float], Optional[float], Optional[float], Optional[float]]:
    if not values:
        return None, None, None, None
    avg = mean(values)
    mn = min(values)
    mx = max(values)
    std = pstdev(values) if len(values) > 1 else 0.0
    return avg, mn, mx, std


def start_kubectl_log_stream(namespace: str, pod_name: str, prefix: str) -> Tuple[Optional[subprocess.Popen], Optional[threading.Thread]]:
    """Stream logs from a pod in a background thread.
    
    Note: It's recommended to wait for the pod to be ready before calling this function.
    """
    if not pod_name:
        return None, None
    
    try:
        proc = subprocess.Popen(
            ["kubectl", "logs", "-n", namespace, "-f", pod_name, "--timestamps"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,  # Redirect stderr to stdout to capture error messages
            text=True,
        )
    except Exception as e:
        echo_warn(f"Failed to start log stream for {pod_name}: {e}")
        return None, None

    def _stream():
        if not proc.stdout:
            return
        try:
            for line in proc.stdout:
                print(f"[{prefix}] {line.rstrip()}")
        except Exception as e:
            echo_warn(f"Error reading logs from {pod_name}: {e}")

    thread = threading.Thread(target=_stream, daemon=True)
    thread.start()
    return proc, thread
