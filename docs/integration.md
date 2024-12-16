# Sichek Integration

This document outlines the integration of **Sichek** with two key components of the Kubernetes ecosystem: **TaskGuard** and **Prometheus/Grafana**.

**TaskGuard**: TaskGuard is a component that manages task scheduling and remediation actions based on issues reported by Sichek. Its implementation may vary depending on the task management system. This document provides a demo of its functionality. TaskGuard orchestrates alerts and recovery actions at the cluster level, responding to node health issues reported by the Problem Notifier.

**Prometheus & Grafana**: These components aggregate metrics from all nodes to provide insights into cluster-wide health.

## Integration with TaskGuard

Sichek integrates with TaskGuard to notify and manage node issues at the cluster level. TaskGuard orchestrates recovery actions and ensures cluster stability by leveraging the data and alerts provided by Sichek.

### Integration Workflow

1. **Health Issue Detection**

- Sichek's health checkers detect node-level issues such as GPU faults, network failures, and file system errors, etc.

2. Notification to Kubernetes API Server and TaskGuard

- Detected issues are sent from the Problem Notifier to the Kubernetes API Server, updating node annotations (default key: `sichek.ai/sichek`).

- The annotation key can be set by running param (`--annotation-key`), the default key is `sichek.ai/sichek`; annotation value is a json struct of all components.


```
{
    // when there are no error events, this component value is null
    "nccl": null,
    "hang": {
        "fatal": [
             // hang component's device is a list joined by `,`
             // device has the format of `GPU_ID:POD_ID`, when there is not pod affected, POD_ID is ''
             {"error_name":"GPUHang","device":"GPU-bc73c94f-41eb-07bf-e827-d100f0e09da8:,GPU-eea0ccff-fe84-946c-68e0-d9880c3401d7:gpt3-5b-2k-bf16-gbs128-ckpt0-n8-1203t0554-master-0,GPU-14fe8e83-6a8c-0c45-534c-d41dbd4e1a25:gpt3-5b-2k-bf16-gbs128-ckpt0-n8-1203t0554-master-0,GPU-78b3964c-cb99-36b8-3001-bb8bec809325:gpt3-5b-2k-bf16-gbs128-ckpt0-n8-1203t0554-master-0,GPU-065e72ac-0cc4-38b6-b704-17ae6122da44:gpt3-5b-2k-bf16-gbs128-ckpt0-n8-1203t0554-master-0,GPU-e7ab46e8-b0cb-6e88-a4e2-3b2f30a0699e:"}
        ]
    },
    // each componnet result map has `fatal, critical, warning` keys
    "nvidia": {
        "fatal": [
            {"error_name":"GPULost", "device":"GPU-bc73c94f-41eb-07bf-e827-d100f0e09da8,GPU-eea0ccff-fe84-946c-68e0-d9880c3401d7"},
        ],
        "critical": [
            {"error_name":"IOMMUNotClosed", "device":"GPU-bc73c94f-41eb-07bf-e827-d100f0e09da8,GPU-eea0ccff-fe84-946c-68e0-d9880c3401d7"},
        ],
    },
    "infiniband": null,
    "ethernet": null,
    "gpfs": {
        "critical": [
            // for some node level component, device is ''
            {"error_name":"FilesystemUnmount", "device":""},
        ],
        "warning": [
            {"error_name":"BadTcpState", "device":""},
        ]
    },
    "cpu": null,
    "memory": null,
    "dmesg": null,
}
```

- TaskGuard monitors the node annotation `sichek.ai/sichek` to retrieve details about the issues.

3. Action Orchestration

TaskGuard processes the node annotations and initiates corrective actions, such as:

- Draining the affected node.

- Evicting pods from the node.

- Notifying administrators of critical issues.


By integrating Sichek’s notifications, TaskGuard ensures a proactive response to node issues, reducing potential disruptions in the cluster.

## Integration with Kubernetes Prometheus and Grafana

Sichek integrates with Prometheus to export node-level metrics and health data, enabling detailed monitoring and visualization of cluster health in Grafana.

### Integration Workflow

- **Metrics Collection**

    - Metrics is collected by opensource exporters (DCGM_exporter and node_exporter), you should deploy these in your k8s cluster first.

    - Data is stored in the Prometheus time-series database.

- **Grafana Visualization**

    - Metrics are visualized using Grafana, providing dashboards and trend analysis for detailed insights into cluster health.

    - Demo grafana json file is integration/grafana/node_health.json, you can import it in grafana web.
