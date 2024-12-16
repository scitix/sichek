# GPFS Check

*GPFS Component* scans **GPFS (General Parallel File System)** logs for specific patterns and assesses the criticality of events. For each identified event, it classifies it into different categories according to the effect of job:

- **Critical**: Requires immediate action (e.g., filesystem unmount or expulsion from the cluster).
- **Warning**: Needs attention but is not immediately critical (e.g., system time-clock error).

## Detailed Events

### 1. Time Clock

- Ensures that the system's time synchronization is consistent across the GPFS cluster. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=messages-6027-2955-w)
- Criticality: Warning
- Suggestion: Reset the time clock with ntp.

### 2. OS Lockup

- Detects operating system stalls or freezes that impact GPFS operations. [Learn more](https://www.ibm.com/docs/en/powervc/2.0.3?topic=kps-cpu-soft-lockup-messages-console-dmesg-output-powervc-version-203)
- Criticality: Warning
- Suggestion: Fix OS kernel and driver bugs.

### 3. RDMA Status

- Checks the status of Remote Direct Memory Access (RDMA) for inter-node communication. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=events-network)
- Criticality: Warning
- Suggestion: Check RDMA network and device.

### 4. Quorum Connection

- Monitors quorum connection stability in the cluster. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=issues-quorum-loss)
- Criticality: Critical
- Suggestion: Check GPFS daemon network.

### 5. TCP State

- Checks TCP connection states between nodes. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=messages-6027-1760-e)
- Criticality: Warning
- Suggestion: Check GPFS daemon network.

### 6. Filesystem Unmount

- Detects unexpected unmounting of GPFS filesystems. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=fsfu-gpfs-error-messages-file-system-forced-unmount-problems)
- Criticality: Critical
- Suggestion: Check GPFS status

### 7. Expelled from Cluster

 - Identifies nodes that have been expelled from the GPFS cluster. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=messages-6027-766-n)
 - Criticality: Critical
 - Suggestion: Check the network connection between this node and the node specified above.

 ### 8. Unauthorized

 - Remote filesystem is unauthorized. [Learn more](https://www.ibm.com/docs/en/storage-scale/5.1.9?topic=issues-file-system-fails-mount)
 - Criticality: Warning
 - Suggestion: Contact the administrator and request access.

 By analyzing these events, system administrators maintain the health and reliability of GPFS filesystems on node.
