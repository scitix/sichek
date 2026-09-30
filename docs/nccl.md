# NCCL Errors

In large-scale AI training tasks, a common issue is **NCCL (Nvidia Collective Communication Library) timeout**. This problem arises when processes experience communication delays but fail to exit promptly, causing **Kubernetes PyTorchjobs** to hang without reporting any abnormal status.

Tracking all pods in such large-scale tasks becomes increasingly challenging due to the sheer number of nodes involved. These unresolved issues can result in wasted computational resources and extended training timelines.

To address this, we analyzed NCCL and PyTorch code and developed the `SiChek NCCL` component. This tool performs real-time log analysis for all pods to detect NCCL timeout scenarios. When a timeout is identified, it pinpoints the specific pod and reports the issue as a **Fatal Error**.