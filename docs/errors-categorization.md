# HealthCheck Fatal Error

Sichek categorizes errors into three main types: **Fatal**, **Critical**, and **Warning**. Each error type includes a description and recommended actions.
- **Fatal**: Stop the task immediately and resubmit it.
- **Critical**: Cordon the node and fix hardware or software issues as soon as possible.
- **Warning**: Cordon the node and schedule hardware or software fixes at a convenient time.


## Fatal Errors

| **Error Type** | **Error Name**                    | **Devices**          | **Description**                                | **Suggestion**                                                                            |
|---------------|------------------------------------|---------------------|------------------------------------------------|-------------------------------------------------------------------------------------------|
| **NCCL**      | NCCLTimeout                        | :pytorch-master-0        | NCCL timeout occurred during user task.                    | Terminate the task and restart restart.                                                                  |
| **Hang**      | GPUHang                            | GPU-UUID:pytorch-master-0                 | GPU is hanging.                               | Terminate the task and restart restart.                                                                  |
| **GPU**       | GPULost                            | GPU-UUID:pytorch-master-0                 | GPU is lost.                             | Perform a cold reset.                                                                       |
|               | xid79-GPULost                      | GPU-UUID:pytorch-master-0                 | GPU disconnected from PCIe bus.      | Perform a cold reset.                                                                       |


## Critical Error

| **Error Type** | **Error Name**                    | **Devices**          | **Description**                                | **Suggestion**                                                                            |
|---------------|------------------------------------|---------------------|------------------------------------------------|-------------------------------------------------------------------------------------------|
| **GPU**       | PCIeACSNotClosed                   | -           | PCIe ACS is disabled.                         | Run `for i in $(lspci \| cut -f 1 -d " "); do setpci -v -s $i ecap_acs+6.w=0; done` to close ACS. |
|               | IOMMUNotClosed                     | -            | IOMMU is enabled.                             | Add `iommu=off` to `GRUB_CMDLINE_LINUX_DEFAULT` in `/etc/default/grub`, then reboot.         |
|               | NvidiaPeerMemNotLoaded             | -                 | `nvidia_peermem` module is not loaded.        | Run `modprobe nvidia_peermem` to load the module.                                           |
|               | NvidiaFabricManagerNotActive       | -                 | `nvidia-fabricmanager` is not running.        | Run `systemctl restart nvidia-fabricmanager`.                                               |
|               | ClockThrottleEvent                  | GPU-UUID:pytorch-master-0                 | Critical clock events in GPUs detected.       | Diagnose the GPU for potential hardware issues.                                                           |
|               | NvlinkNotActive                    | GPU-UUID:pytorch-master-0         | Nvlink connections are inactive.              | Reboot the system.                                                                           |
|               | RemmapedRowsPending                | GPU-UUID:pytorch-master-0                 | Pending remapped memory rows detected.                | Reset the GPU.                                                                               |
|               | RemmapedRowsFailure                | GPU-UUID:pytorch-master-0                 | Remapping failure in GPU memory rows.                 | Replace the GPU.                                                                             |
|               | SRAMVolatileUncorrectableErrors          | GPU-UUID:pytorch-master-0                 | Uncorrectable ECC errors in SRAM.                | Reset the GPU.                                                                               |
|               | HighSRAMAggregateUncorrectableErrors     | GPU-UUID:pytorch-master-0                 | Hign aggregate number of uncorrectable SRAM errors.          | Replace the GPU.                                                                             |
|               | xid31-GPUMemoryPageFault           | GPU-UUID:pytorch-master-0                 | GPU memory page faults detected.              | Reset the GPU or verify the app for illegal memory access.                                 |
|               | xid48-GPUMemoryDBE                 | GPU-UUID:pytorch-master-0                 | Double-bit ECC errors detected.               | Reset the GPU.                                                                               |
|               | xid63-UcePending                   | GPU-UUID:pytorch-master-0                 | Pending ECC page retirements.                 | Reset the GPU.                                                                               |
|               | xid64-ECCRowremapperFailure        | GPU-UUID:pytorch-master-0                 | Row remapper failure detected.                | Reset the GPU.                                                                               |
|               | xid74-NVLinkError                  | GPU-UUID:pytorch-master-0         | NVLink connection error detected.             | Reset the GPU.                                                                               |
|               | xid92-HighSingleBitECCErrorRate    | GPU-UUID:pytorch-master-0                 | High single-bit ECC error rate.               | Replace the GPU.                                                                             |
|               | xid94-ContainedECCError            | GPU-UUID:pytorch-master-0                 | Contained ECC errors detected.                | Reset the GPU.                                                                               |
|               | xid95-UncontainedECCError          | GPU-UUID:pytorch-master-0                 | Uncontained ECC errors detected.              | Replace the GPU.                                                                             |
| **Infiniband**      | IBDeviceCountMismatch               | mlx5_0,mlx5_1            | IB device count mismatch or missing          | Check PCIe status or IB NIC connectivity                                                                                                        |
|               | IBStateNotActive                  | mlx5_0,mlx5_1            | Some IB ports are not in ACTIVE state.              | Check OpenSM and IB connection                                                                                                                |
|               | IBPhyStateNotLinkUp                | mlx5_0,mlx5_1            | Some IB ports are in LINK_DOWN physical state           | Verify IB cable and link status                                                                                             |
|               | IBPortSpeedNotMax                | mlx5_0,mlx5_1            | IB ports are not operating at maximum speed           | Ensure IB speed settings are correct in firmware                                                                                             |
|               | PCIEMRRIncorrect                | mlx5_0,mlx5_1            | PCIe MRR setting is incorrect (expected: 4096)           | Ensure PCIe slot and firmware support correct speed status                                                                                             |
|               | PCIELinkSpeedDegraded                | mlx5_0,mlx5_1            | PCIe link speed has degraded           | Verify IB cable and link status                                                                                             |
|               | PCIELinkWidthIncorrect                | mlx5_0,mlx5_1            | PCIe link width is lower than specification           | Verify PCIe lane configuration in BIOS                                                                                            |
|               | PCIETreeSpeedDegraded                | -            | PCIe path to root complex has degraded speed           | Check upstream PCIe device speed and configuration                                                                                             |
|               | PCIETreeWidthIncorrect                | -            | PCIe path has degraded link width.           | Check PCIe switch and topology configuration                                                                                            |
|               | IBKernelModulesNotAllInstalled                | -            | Some IB kernel modules are missing           | Install or reload missing kernel modules                                                                                            |
| **GPFS**      | QuorumConnectionDown               | -            | Quorum node connection is down          | Check GPFS daemon network                                                                                                        |
|               | FilesystemUnmount                  | -            | Filesystem is unmounted on this node.              | Check GPFS status                                                                                                                |
|               | ExpelledFromCluster                | -            | Node has been expelled from the GPFS cluster           | Check GPFS daemon network and status                                                                                             |


## Warning

| **Error Type** | **Error Name**                  | **Device**         | **Description**                                | **Suggestion**                                            |
|----------------|----------------------------------|--------------------|------------------------------------------------|----------------------------------------------------------|
| **Nvidia**        | PCIeLinkDegraded                 | GPU-UUID:pytorch-master-0               | PCIe link degradation detected.                    | Reboot the system.                                        |
|                | GPUPersistencedModeNotEnabled     | GPU-UUID:pytorch-master-0                | GPU persistence mode is disabled.       | Run `nvidia-persistenced` to enable persistence mode.       |
|                | GPUStateNotMaxPerformance        | GPU-UUID:pytorch-master-0                | GPU is not in maximum performance mode.  | Reset the GPU.                                            |
|                | AppClocksNotMax                  | GPU-UUID:pytorch-master-0                | GPU application clocks are not set to max.| Run `nvidia-smi -rac` to set clocks to maximum.          |
|                | SoftwareVersionIncorrect        | -                | software versions do not  match expectations.   | Update to the correct version.                           |
|                | HighTemperature                  | -                | GPU temperature exceeds the limit.    | Monitor application performance.                         |
|                | HighRemmapedRowsUncorrectableErrors    | -                | Detect high remapped rows with errors.         | Diagnose the GPU for hardware issues.                    |
|                | HighSRAMCorrectableErrors              | -                | Detect high correctable SRAM errors.           | Diagnose the GPU for hardware issues.                    |
| **Infiniband**        | OFEDVersionMismatch                 | -               | IB OFED version does not match expected version.                    | Reinstall or upgrade OFED to the required version.                                        |
|                | IBFirmwareVersionMismatch     | -                | IB firmware version is inconsistent.       | Update firmware to match the specification.       |
|                | IBDeviceNameMismatch        | mlx5_0, mlx5_1                | IB device names do not match expected values.  | Verify udev rules or device naming configuration.                                            |
| **CPU**        | CPUPerfModeNotEnabled            | -                | Not all CPUs are in performance mode.     | Run `echo performance > /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor` to enable it. |
| **GPFS**      | TimeClockError                    | -            | Time-of-day may have jumped back.               | Reset the time clock with ntp                                                                                               |
|               | OSLockup                          | -            | OS lockup, may cause GPFS heartbeat fail and unmount | Fix OS kernel and driver bugs                                                                                                |
|               | RDMAStatusError                   | -            | RDMA network is down                           | Check RDMA network and device                                                                                               |
|               | BadTcpState                       | -            | TCP connection is down                         | Check GPFS daemon network                                                                                                   |
|               | Unauthorized                      | -            | Node is unauthorized for the remote cluster             | Check GPFS authorization status                                                                                             |
