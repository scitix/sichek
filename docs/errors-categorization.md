# HealthCheck Fatal Error

Sichek categorizes errors into three main types: **Fatal**, **Critical**, and **Warning**. Each error type includes a description and suggested corrective actions
- **Fatal**: Stop the task immediately and resubmit it.
- **Critical**: Cordon the node and fix hardware or software issues as soon as possible.
- **Warning**: Cordon the node and schedule hardware or software fixes at a convenient time.


## Fatal Errors

| **Error Type** | **Error Name**                    | **Devices**          | **Description**                                | **Suggestion**                                                                            |
|---------------|------------------------------------|---------------------|------------------------------------------------|-------------------------------------------------------------------------------------------|
| **NCCL**      | NCCLTimeout                        | :pytorch-master-0        | NCCL timeout in user task.                    | Fail the task and restart.                                                                  |
| **Hang**      | GPUHang                            | GPU-UUID:pytorch-master-0                 | GPU is hanging.                               | Fail the task and restart.                                                                  |
| **GPU**       | GPULost                            | GPU-UUID:pytorch-master-0                 | GPU disconnected.                             | Perform a cold reset.                                                                       |
|               | xid79-GPULost                      | GPU-UUID:pytorch-master-0                 | GPU disconnection from the bus detected.      | Perform a cold reset.                                                                       |


## Critical Error

| **Error Type** | **Error Name**                    | **Devices**          | **Description**                                | **Suggestion**                                                                            |
|---------------|------------------------------------|---------------------|------------------------------------------------|-------------------------------------------------------------------------------------------|
| **GPU**       | PCIeACSNotClosed                   | -           | PCIe ACS is disabled.                         | Run `for i in $(lspci \| cut -f 1 -d " "); do setpci -v -s $i ecap_acs+6.w=0; done` to close ACS. |
|               | IOMMUNotClosed                     | -            | IOMMU is enabled.                             | Add `iommu=off` to `GRUB_CMDLINE_LINUX_DEFAULT` in `/etc/default/grub`, then reboot.         |
|               | NvidiaPeerMemNotLoaded             | -                 | `nvidia_peermem` module is not loaded.        | Run `modprobe nvidia_peermem` to load the module.                                           |
|               | NvidiaFabricManagerNotActive       | -                 | `nvidia-fabricmanager` is not running.        | Run `systemctl restart nvidia-fabricmanager`.                                               |
|               | ClockEventEngaged                  | GPU-UUID:pytorch-master-0                 | Critical clock events in GPUs detected.       | Diagnose GPU for hardware issues.                                                           |
|               | NvlinkNotActive                    | GPU-UUID:pytorch-master-0         | Nvlink connections are inactive.              | Reboot the system.                                                                           |
|               | RemmapedRowsPending                | GPU-UUID:pytorch-master-0                 | Pending remapped rows in GPUs.                | Reset the GPU.                                                                               |
|               | RemmapedRowsFailure                | GPU-UUID:pytorch-master-0                 | Remapped rows failure on GPU.                 | Replace the GPU.                                                                             |
|               | SRAMVolatileUncorrectable          | GPU-UUID:pytorch-master-0                 | Uncorrectable ECC SRAM errors.                | Reset the GPU.                                                                               |
|               | HighSRAMAggregateUncorrectable     | GPU-UUID:pytorch-master-0                 | Aggregate uncorrectable SRAM errors.          | Replace the GPU.                                                                             |
|               | xid31-GPUMemoryPageFault           | GPU-UUID:pytorch-master-0                 | GPU memory page faults detected.              | Reset the GPU or verify the app for illegal memory access.                                 |
|               | xid48-GPUMemoryDBE                 | GPU-UUID:pytorch-master-0                 | Double-bit ECC errors detected.               | Reset the GPU.                                                                               |
|               | xid63-UcePending                   | GPU-UUID:pytorch-master-0                 | Pending ECC page retirements.                 | Reset the GPU.                                                                               |
|               | xid64-ECCRowremapperFailure        | GPU-UUID:pytorch-master-0                 | Row remapper failure detected.                | Reset the GPU.                                                                               |
|               | xid74-NVLinkError                  | GPU-UUID:pytorch-master-0         | NVLink connection error detected.             | Reset the GPU.                                                                               |
|               | xid92-HighSingleBitECCErrorRate    | GPU-UUID:pytorch-master-0                 | High single-bit ECC error rate.               | Replace the GPU.                                                                             |
|               | xid94-ContainedECCError            | GPU-UUID:pytorch-master-0                 | Contained ECC errors detected.                | Reset the GPU.                                                                               |
|               | xid95-UncontainedECCError          | GPU-UUID:pytorch-master-0                 | Uncontained ECC errors detected.              | Replace the GPU.                                                                             |
| **GPFS**      | QuorumConnectionDown               | -            | connection with quorum node down          | Check GPFS daemon network                                                                                                        |
|               | FilesystemUnmount                  | -            | node filesystem is unmounted              | Check GPFS status                                                                                                                |
|               | ExpelledFromCluster                | -            | node expelled from GPFS cluster           | Check GPFS daemon network and status                                                                                             |


## Warning

| **Error Type** | **Error Name**                  | **Device**         | **Description**                                | **Suggestion**                                            |
|----------------|----------------------------------|--------------------|------------------------------------------------|----------------------------------------------------------|
| **GPU**        | PCIeLinkDegraded                 | GPU-UUID:pytorch-master-0               | Detect degraded PCIe links.                    | Reboot the system.                                        |
|                | GPUPersistenceModeNotEnabled     | GPU-UUID:pytorch-master-0                | GPU persistence mode is not active.       | Run `nvidia-smi -pm 1` to enable persistence mode.       |
|                | GPUStateNotMaxPerformance        | GPU-UUID:pytorch-master-0                | GPU is not in maximum performance mode.  | Reset the GPU.                                            |
|                | AppClocksNotMax                  | GPU-UUID:pytorch-master-0                | GPU application clocks are not set to max.| Run `nvidia-smi -rac` to set clocks to maximum.          |
|                | SoftwareVersionNotCorrect        | -                | software versions donot  match expectations.   | Update to the correct version.                           |
|                | HighTemperature                  | -                | GPU temperature exceeds the limit.    | Monitor application performance.                         |
|                | HighRemmapedRowsUncorrectable    | -                | Detect high remapped rows with errors.         | Diagnose the GPU for hardware issues.                    |
|                | HighSRAMCorrectable              | -                | Detect high correctable SRAM errors.           | Diagnose the GPU for hardware issues.                    |
| **CPU**        | CPUPerfModeNotEnabled            | -                | Not all CPUs are in performance mode.     | Run `echo performance > /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor` to enable it. |
| **GPFS**      | TimeClockError                    | -            | Time-of-day may have jumped back.               | Reset the time clock with ntp                                                                                               |
|               | OSLockup                          | -            | OS lockup, may cause GPFS heartbeat fail and unmount | Fix OS kernel and driver bugs                                                                                                |
|               | RDMAStatusError                   | -            | node RDMA network down                           | Check RDMA network and device                                                                                               |
|               | BadTcpState                       | -            | node TCP connection down                         | Check GPFS daemon network                                                                                                   |
|               | Unauthorized                      | -            | node unauthorized for remote cluster             | Check GPFS authorization status                                                                                             |
