# Nvidia GPU Check

Nvidia GPU performance and operation can be assessed through comprehensive system metrics. Proactively catching system issues before they affect user workloads is crucial for maintaining cluster stability and high utilization.

## Nvidia GPU specific Checks:

To ensure the proper functioning of Nvidia GPUs, the following checks should be performed:

1. **Configuration Validation**

- **System Settings**:
    - Verify that all **PCIe ACS** (Access Control Services) are disabled.

    - Confirm that **IOMMU** (Input-Output Memory Management Unit) is disabled.

- **Software Requirements**:

    - Ensure the **GPU driver** is installed and loaded correctly.

    - Verify the installation of nv-peermem (required for GPUDirect RDMA). [Learn more](https://developer.nvidia.com/gpudirect)

    - Confirm the gdrdrv module is active (required for GDRCopy). [Learn more](https://github.com/NVIDIA/gdrcopy)

- **For NVSwitch based Systems**

    - Confirm that the **abric Manager** is running to manage NVSwitch operations.

    - Verify that the **NVLink** fabric topology is configured and operating as expected.

- **NVIDIA GPU specific Settings**:

    - Confirm driver persistence setting is correct. Driver persistence should be controlled through Persistence Daemon.[Learn more](https://docs.nvidia.com/deploy/driver-persistence/index.html)

- **Runtime Error and Anomaly Detection**：
    - **GPU Count**: Confirm the number of GPUs matches the expected configuration.
    - **PCIe Link Speed**: Ensure PCIe connections are operating at their desired speeds.
    - **Thermal Throttling**: Verify that no GPUs are being throttled due to high temperatures.
    - **ECC Memory Errors**: Monitor for any uncorrectable memory errors that could impact computations.
    - **XID Errors**: Check for any recent Nvidia XID errors, which may indicate hardware or software faults.

## Key Metrics

Sichek collected the following Key metrics to perform Nvidia GPU specific Chechs:

- **software version** (node-level)
    - Driver Version: Tracks the installed Nvidia GPU driver version to ensure compatibility with the hardware
    - CUDA Version: Identifies the installed CUDA version to verify compatibility
- **GPU Devices Num** (node-level)
    - GPU Devices Num and their UUIDs: Ensures all GPUs are recognized, and their unique identifiers are logged for tracking
    - GPU device to Kubernetes pods mapper: It used to indentify which pods will be affected, once GPU errors detected

- **PCIe Info**  (device-level)
    - PCIe Bus/Device/Function ID: Provides low-level hardware identification for each GPU
    - PCIe DeviceID (device code): Identifies the specific GPU model.
    - PCIe Generation and Width: Ensures GPUs are connected using the correct PCIe generation and lane width
    - PCIe Tx/Rx Bytes: Tracks data transfer rates to detect potential PCIe bandwidth bottlenecks

- **GPU States**  (device-level)
    - persistent mode: Indicates whether persistent mode is enabled for efficient GPU usage across multiple processes
    - pstate: Reports the GPU's performance state, which reflects its power and clock configuration

- **Clock Info**  (device-level)
    - Current/Application/Maximum SMClk
    - Current/Application/Maximum MemoryClk
    - Current/Application/Maximum GraphicsClk

- **Clock Events**  (device-level)
    - GPU Idle: Flags whether the GPU is idle
    - HW Thermal Slowdown: 
    - HW Power Brake Slowdown
    - SwPowerCap
    - SW Thermal Slowdown

- **Power Info**  (device-level)
    - Power Usage: Real-time GPU power consumption
	- Current/Default/Enforced/Maximum/Minimum PowerLimit
	- Power Violations: Logs instances where GPUs exceed power limits
	- Thermal Violations: Records events where GPUs exceed safe operating temperatures
- **Temperature**  (device-level)
    - GPU Temperature: Monitors core GPU temperature for overheating detection.
    - Memory Temperature: Tracks the thermal condition of GPU memory modules

- **Utilization**  (device-level)
    - GPU Usage Percent: Indicates the percentage of GPU compute resources being utilized.
    - Memory Usage Percent:  Shows the proportion of GPU memory currently in use.

- **NVLink States**  (device-level)
    - NVlinkSupported: Indicates if NVLink is available on the system
    - FeatureEnabled: Confirms whether NVLink links are activated
    - Throughput: Measures NVLink data transfer rates.
    - Replay Errors: Detects transmission errors requiring data retransmission.
    - Recovery Errors: ogs errors successfully resolved by NVLink's recovery mechanisms.
    - CRCErrors: racks cyclic redundancy check errors, which indicate potential communication issues.

- **Memory Errors**  (device-level)
    - RemappedDueToCorrectable/Uncorrectable: Identifies memory blocks remapped due to correctable or uncorrectable error
    - RemappingPending: Flags pending memory remapping operations.
    - RemappingFailureOccurred: Logs any failed remapping attempts.
    - DRAM/SRAM Corrected/Uncorrected Errors: Memory ECC errors

- **XID Errors** (node-level): Tracks the NVIDIA GPU Xid errors using the NVIDIA Management Library (NVML)

By systematically collecting these metrics and performing the specified checks, administrators can maintain the health and performance of Nvidia GPUs in their clusters, ensuring reliable and efficient operation.