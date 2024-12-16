# System Components

*System Components* collects pre-defined health-related metrics from system sub-components. It currently supports the following components:

* Host
* CPU
* Memory

## Detailed Metrics

### Host

The following pre-defined metrics are collected from `Host` component:

* `hostname`: Hostname of the system 
* `os_version`: Operating system version (e.g. `cos 73-11647.217.0`)
* `kernel_version`: kernel version (e.g. `4.14.127+`).
* `host_uptime`: System uptime in seconds. 

### CPU

The following pre-defined metrics are collected from `cpu` component:

* `Architecture`: The CPU's architecture (e.g., `x86_64`, `arm64`).
* `ModelName`: The model name of the CPU (e.g., `Intel(R) Xeon(R) Gold 6254 CPU @ 3.10GHz`).
* `VendorID`: The vendor ID of the CPU (e.g., `GenuineIntel`, `AuthenticAMD`).
* `Family`: The family of the CPU, typically denoting a generation or series.
* `Sockets`: The total number of physical CPU sockets in the system.
* `CorePerSocket`: The number of cores in each CPU socket.
* `ThreadPerCore`: The number of threads per core.
* `PowerMode`: Available power modes for the CPU (e.g., `performance`, `powersave`).
* `NumaNum`: Total number of NUMA (Non-Uniform Memory Access) nodes.
* `NumaNodeInfo`: Details about each NUMA node, including:
    * `ID`: ID of the NUMA node.
    * `CPUs`: CPUs assigned to the NUMA node.
* `Usage`: Usage statistics providing information about how the CPU is utilized.

* `RunnableTaskCount`: The average number of runnable tasks in the run-queue during the last minute. Collected from [`/proc/loadavg`][/proc doc].
* `CPUUsageTime`: CPU usage, in seconds. Collected from [CPU state][/proc doc].
* `CpuLoad1m`: CPU load average over the last 1 minute. Collected from [`/proc/loadavg`][/proc doc].
* `CpuLoad5m`: CPU load average over the last 5 minutes. Collected from [`/proc/loadavg`][/proc doc].
* `CpuLoad15m`: CPU load average over the last 15 minutes. Collected from [`/proc/loadavg`][/proc doc].
* `SystemProcessesTotal`: Total number of processes created since boot.
* `SystemProcsRunning`: Number of currently running processes.
* `SystemProcsBlocked`: Number of processes currently blocked.
* `SystemInterruptsTotal`: Total number of interrupts serviced (cumulative).

[/proc doc]: http://man7.org/linux/man-pages/man5/proc.5.html

### Memory

The following pre-defined metrics are collected from `Memory` component:

* `MemTotal`: Total memory, in bytes. 
* `MemUsed`: Total memory used, in bytes. 
* `MemFree`: Total memory free, in bytes. 
* `MemPercentUsed`: The percentage of memory used. 
* `MemAnonymousUsed`: Anonymous memory usage, in Bytes. 
* `PageCacheUsed`: Memory used for the page cache, in bytes.
* `MemUnevictableUsed`: Memory marked as[Unevictable memory][/proc doc], in Bytes.
* `DirtyPageUsed`: Dirty pages usage, in Bytes. 