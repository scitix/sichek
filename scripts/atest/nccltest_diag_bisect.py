#!/usr/bin/env python3
"""
NCCL AllGather diag binary search to locate bad nodes.
"""

import argparse
import subprocess
import sys
from typing import Dict, List, Tuple


def parse_mapping(mapping: str) -> Dict[str, str]:
    result: Dict[str, str] = {}
    if not mapping:
        return result
    for pair in mapping.split(","):
        if ":" not in pair:
            continue
        pod, node = pair.split(":", 1)
        if pod and node:
            result[pod.strip()] = node.strip()
    return result


def read_nodes(hostfile: str) -> List[str]:
    try:
        with open(hostfile, "r") as f:
            return [line.split()[0] for line in f if line.strip()]
    except FileNotFoundError:
        print(f"Error: Hostfile {hostfile} not found!")
        sys.exit(1)


def build_hosts(nodes: List[str], slots_per_node: int) -> str:
    return ",".join([f"{n}:{slots_per_node}" for n in nodes])


def run_allgather_test(
    nodes: List[str],
    nccl_test: str,
    mpirun_bin: str,
    interface: str,
    slots_per_node: int,
    timeout_sec: int,
) -> Tuple[bool, str]:
    hosts = build_hosts(nodes, slots_per_node)
    cmd = [
        "timeout",
        str(timeout_sec),
        mpirun_bin,
        "--mca",
        "routed",
        "direct",
        "--mca",
        "plm_rsh_no_tree_spawn",
        "1",
        "--allow-run-as-root",
        "--host",
        hosts,
        "--map-by",
        f"ppr:{slots_per_node}:node",
        "--mca",
        "oob_tcp_if_include",
        interface,
        "--mca",
        "pml",
        "^ucx",
        "--mca",
        "btl",
        "self,tcp",
        "--mca",
        "btl_tcp_if_include",
        interface,
        nccl_test,
        "-lallgather",
        "-b",
        "8",
        "-e",
        "256M",
        "-f",
        "2",
    ]
    proc = subprocess.run(cmd, capture_output=True, text=True)
    output = (proc.stdout or "") + (proc.stderr or "")
    passed = "Avg bus bandwidth" in output
    return passed, output


def bisect_nodes(nodes: List[str], test_func, min_group: int, good_nodes: List[str] = None) -> List[str]:
    """
    Binary search to find bad nodes.
    
    When group size <= min_group, use exclusion method:
    - Try removing each node one by one
    - If remaining nodes pass, the removed node is bad
    - If all exclusions still fail, all nodes in group are marked bad
    """
    bad: List[str] = []
    stack = [nodes]
    cache: Dict[Tuple[str, ...], bool] = {}
    
    # Track known good nodes for exclusion testing
    known_good: List[str] = list(good_nodes) if good_nodes else []

    while stack:
        group = stack.pop()
        if not group:
            continue
        key = tuple(sorted(group))
        if key in cache:
            passed = cache[key]
        else:
            passed = test_func(group)
            cache[key] = passed

        if passed:
            # Mark these nodes as known good for future exclusion tests
            for n in group:
                if n not in known_good:
                    known_good.append(n)
            continue
            
        if len(group) <= min_group:
            # Use exclusion method: try removing each node to identify the bad one
            identified_bad = []
            for i, suspect in enumerate(group):
                remaining = [n for j, n in enumerate(group) if j != i]
                if len(remaining) < 2:
                    # Need at least 2 nodes for NCCL test, try with a known good node
                    if known_good:
                        remaining = remaining + [known_good[0]]
                    else:
                        # Can't test single node, mark as suspect
                        continue
                
                remaining_key = tuple(sorted(remaining))
                if remaining_key in cache:
                    remaining_passed = cache[remaining_key]
                else:
                    remaining_passed = test_func(remaining)
                    cache[remaining_key] = remaining_passed
                
                if remaining_passed:
                    # Removing this node fixed the issue
                    identified_bad.append(suspect)
            
            if identified_bad:
                bad.extend(identified_bad)
            else:
                # Could not isolate, mark all as suspect
                bad.extend(group)
            continue
            
        mid = len(group) // 2
        stack.append(group[:mid])
        stack.append(group[mid:])

    return list(set(bad))  # Remove duplicates


def main():
    parser = argparse.ArgumentParser(description="NCCL AllGather diag bisect")
    parser.add_argument("--hostfile", default="/etc/mpi/hostfile")
    parser.add_argument("--nccl-test", default="/usr/local/sihpc/libexec/nccl-tests/nccl_test")
    parser.add_argument("--mpirun-bin", default="/usr/local/sihpc/bin/mpirun")
    parser.add_argument("--interface", default="eth0")
    parser.add_argument("--slots-per-node", type=int, default=8)
    parser.add_argument("--mpirun-timeout", type=int, default=120)
    parser.add_argument("--pod-node-mapping", default="")
    parser.add_argument("--min-group", type=int, default=2)
    args = parser.parse_args()

    nodes = read_nodes(args.hostfile)
    if not nodes:
        print("No nodes found in hostfile.")
        sys.exit(1)

    mapping = parse_mapping(args.pod_node_mapping)

    print(f"🔍 Starting diag search over {len(nodes)} nodes...")
    if mapping:
        print("ℹ️  Using pod-to-node mapping for display")

    def test_func(group: List[str]) -> bool:
        hosts = build_hosts(group, args.slots_per_node)
        print(f"Checking: {hosts} ... ", end="", flush=True)
        passed, _ = run_allgather_test(
            group,
            args.nccl_test,
            args.mpirun_bin,
            args.interface,
            args.slots_per_node,
            args.mpirun_timeout,
        )
        print("✅ PASS" if passed else "❌ FAIL")
        return passed

    bad_nodes = bisect_nodes(nodes, test_func, args.min_group)

    print("\n==========================================")
    if not bad_nodes:
        print("🎉 All nodes passed NCCL AllGather check!")
    else:
        print("❌ The following nodes failed:")
        for n in bad_nodes:
            name = mapping.get(n, n)
            print(f"   - {name}")


if __name__ == "__main__":
    main()

