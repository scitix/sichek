/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/sirupsen/logrus"
)

type CPUArchInfo struct {
	// The CPU's architecture (e.g., `x86_64`, `arm64`).
	Architecture string `json:"architecture"`
	// The model name of the CPU (e.g., `Intel(R) Xeon(R) Gold 6254 CPU @ 3.10GHz`).
	ModelName string `json:"model_name"`
	// The vendor ID of the CPU (e.g., `GenuineIntel`, `AuthenticAMD`).
	VendorID string `json:"vendor_id"`
	// The family of the CPU, typically denoting a generation or series.
	Family string `json:"family"`
	// Number of CPU sockets.
	Sockets int `json:"socket"`
	// Number of cores per CPU socket.
	CoresPerSocket int `json:"cores_per_socket"`
	// Number of threads per CPU core.
	ThreadPerCore int `json:"threads_per_core"`
	// Total number of NUMA (Non-Uniform Memory Access) nodes.
	NumaNum int `json:"numa_num"`
	// Details about each NUMA node.
	NumaNodeInfo map[int]*NumaNodeInfo `json:"numa_node_info"`
}

type Detail struct {
	CPUID      int32   `json:"cpu_id"`
	PhysicalID string  `json:"physical_id"`
	CoreID     string  `json:"core_id"`
	Mhz        float64 `json:"mhz"`
	CacheSize  int32   `json:"cache_size"`
	Microcode  string  `json:"microcode"`
}

type NumaNodeInfo struct {
	// ID of the NUMA node.
	ID int `json:"id"`
	// CPUs assigned to the NUMA node
	CPUs string `json:"cpus"`
}

type CPUStateInfo struct {
	// power modes for the CPU (e.g., `performance`, `powersave`).
	PowerMode map[string]string `json:"power_mode"`
}

type Usage struct {
	// The total number of processes running on the system.
	RunnableTaskCount int64 `json:"runnable_task_count"`
	// The total number of threads on the system.
	TotalThreadCount int `json:"total_thread_count"`
	//  CPU usage, in seconds.
	CPUUsageTime float32 `json:"usage_time"`
	// CPU load average over the last 1 minute.
	CpuLoadAvg1m float64 `json:"cpu_load_1m"`
	// CPU load average over the last 5 minutes.
	CpuLoadAvg5m float64 `json:"cpu_load_5m"`
	// CPU load average over the last 15 minutes.
	CpuLoadAvg15m float64 `json:"cpu_load_15m"`
	// Total number of processes created since boot
	SystemProcessesTotal int64 `json:"system_processes_total"`
	// Number of currently running processes
	SystemProcsRunning int64 `json:"system_procs_running"`
	// Number of processes currently blocked
	SystemProcsBlocked int64 `json:"system_procs_blocked"`
	// Total number of interrupts serviced (cumulative)
	SystemInterruptsTotal int64 `json:"system_interrupts_total"`
}

func (cpuArchInfo *CPUArchInfo) Get(ctx context.Context) error {
	err := cpuArchInfo.getCPUArchInfo(ctx)
	if err != nil {
		return err
	}
	err = cpuArchInfo.getNumaNodeInfo(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (cpuArchInfo *CPUArchInfo) getCPUArchInfo(ctx context.Context) error {
	// Get using gopsutil
	arch, err := host.KernelArch()
	if err != nil {
		return err
	}
	cpuArchInfo.Architecture = arch

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return err
	}

	if len(cpuInfo) == 0 {
		return fmt.Errorf("no cpu info found")
	}
	cpuArchInfo.ModelName = cpuInfo[0].ModelName
	cpuArchInfo.VendorID = cpuInfo[0].VendorID
	cpuArchInfo.Family = cpuInfo[0].Family
	cpuArchInfo.Sockets = 0
	cpuArchInfo.CoresPerSocket = int(cpuInfo[0].Cores)
	// cpuInfo.Details = make([]*Detail, 0)
	seenSockets := make(map[string]bool)
	for _, detail := range cpuInfo {
		if !seenSockets[detail.PhysicalID] {
			seenSockets[detail.PhysicalID] = true
			cpuArchInfo.Sockets++
		}
	}
	logicalCores, err := cpu.CountsWithContext(ctx, true)
	if err == nil && cpuArchInfo.Sockets > 0 && cpuArchInfo.CoresPerSocket > 0 {
		cpuArchInfo.ThreadPerCore = logicalCores / (cpuArchInfo.Sockets * cpuArchInfo.CoresPerSocket)
	}

	if err != nil {
		return err
	}
	return nil
}

func (cpuArchInfo *CPUArchInfo) getNumaNodeInfo(ctx context.Context) error {
	// Get NUMA node info and count using lscpu
	cmd := exec.CommandContext(ctx, "lscpu")
	lscpuOutput, err := cmd.Output()
	if err != nil {
		return err
	}

	cpuArchInfo.NumaNodeInfo = make(map[int]*NumaNodeInfo)
	lines := strings.Split(string(lscpuOutput), "\n")
	for _, line := range lines {
		if strings.Contains(line, "NUMA node(s):") {
			fields := strings.Fields(line)
			if len(fields) > 2 {
				cpuArchInfo.NumaNum, err = strconv.Atoi(fields[2])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
			}
		}
		if strings.Contains(line, "NUMA node") && strings.Contains(line, "CPU(s):") {
			fields := strings.Split(line, ":")
			if len(fields) == 2 {
				subfields := strings.Split(fields[0], " ")
				nodeID := strings.TrimPrefix(subfields[1], "node")
				nodeID = strings.TrimSpace(nodeID)
				id, err := strconv.Atoi(nodeID)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				cpus := strings.TrimSpace(fields[1])
				cpuArchInfo.NumaNodeInfo[id] = &NumaNodeInfo{
					ID:   id,
					CPUs: cpus,
				}
			}
		}
	}
	return nil
}

func (usage *Usage) Get() error {
	err := usage.getLoadAvg()
	if err != nil {
		return err
	}
	err = usage.getProcStats()
	if err != nil {
		return err
	}
	usage.TotalThreadCount, err = GetTotalThreads()
	if err != nil {
		return err
	}
	return nil
}

// getLoadAvg parses /proc/loadavg to get CPU load averages and runnable tasks
func (usage *Usage) getLoadAvg() error {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return fmt.Errorf("failed to open /proc/loadavg: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logrus.WithField("component", "cpu").Errorf("Error closing file: %v\n", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			usage.CpuLoadAvg1m, err = strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return fmt.Errorf("fail to conv err:%w", err)
			}
			usage.CpuLoadAvg5m, err = strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return fmt.Errorf("fail to conv err:%w", err)
			}
			usage.CpuLoadAvg15m, err = strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return fmt.Errorf("fail to conv err:%w", err)
			}
		}
		if len(fields) >= 4 {
			taskInfo := strings.Split(fields[3], "/")
			if len(taskInfo) == 2 {
				// The number of currently running processes (in the run queue).
				usage.RunnableTaskCount, err = strconv.ParseInt(taskInfo[0], 10, 64)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				// The total number of processes on the system (including sleeping, running, and idle)
				usage.SystemProcessesTotal, err = strconv.ParseInt(taskInfo[1], 10, 64)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
			}
		}
	}
	return scanner.Err()
}

// Read /proc/stat for usage statistics
func (usage *Usage) getProcStats() error {
	return usage.getProcStats_("/proc/stat")
}

func (usage *Usage) getProcStats_(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %v: %v", filename, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logrus.WithField("component", "cpu").Errorf("Error closing file: %v\n", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		switch fields[0] {
		case "procs_running":
			if len(fields) > 1 {
				usage.SystemProcsRunning, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
			}
		case "procs_blocked":
			if len(fields) > 1 {
				usage.SystemProcsBlocked, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
			}
		case "intr":
			if len(fields) > 1 {
				usage.SystemInterruptsTotal, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
			}
		case "cpu":
			if len(fields) > 7 {
				user, err := strconv.Atoi(fields[1])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				nice, err := strconv.Atoi(fields[2])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				system, err := strconv.Atoi(fields[3])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				irq, err := strconv.Atoi(fields[6])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				softirq, err := strconv.Atoi(fields[7])
				if err != nil {
					return fmt.Errorf("fail to conv err:%w", err)
				}
				// FIXME: This may be not the correct way to calculate CPU usage
				// CPU usage in jiffies (approximate to seconds if scaled by jiffy time)
				usage.CPUUsageTime = float32(user+nice+system+irq+softirq) / 100.0
				// cpuInfo.Usage.CPUUsageTime = float32(user + nice + system + idle + iowait + irq + softirq)
			}
		}
	}
	return scanner.Err()
}

func GetTotalThreads() (int, error) {
	threadCount := 0
	procDir := "/proc"

	// Read all directories in /proc
	dirEntries, err := os.ReadDir(procDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range dirEntries {
		// Only look at numeric directories (which correspond to process IDs)
		if isNumeric(entry.Name()) && entry.IsDir() {
			taskDir := filepath.Join(procDir, entry.Name(), "task")
			taskEntries, err := os.ReadDir(taskDir)
			if err == nil {
				// Each subdirectory in /proc/[pid]/task corresponds to a thread
				threadCount += len(taskEntries)
			}
		}
	}

	return threadCount, nil
}

// Helper function to check if a string is numeric
func isNumeric(s string) bool {
	if len(s) > 0 && s[0] == '-' {
		s = s[1:]
	}
	_, err := strconv.Atoi(s)
	return err == nil
}
