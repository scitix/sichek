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
	"context"
	"testing"

	"github.com/scitix/sichek/components/common"
)

func TestCPUArchInfo_Get(t *testing.T) {
	ctx := context.Background()
	cpuInfo := &CPUArchInfo{}

	err := cpuInfo.Get(ctx)
	if err != nil {
		t.Errorf("Failed to get CPUArchInfo: %v", err)
	}
	t.Logf("CPUArchInfo: %v", common.ToString(cpuInfo))
}

func TestUsage_Get(t *testing.T) {
	usage := &Usage{}
	err := usage.Get()
	if err != nil {
		t.Errorf("Failed to get CPU usage: %v", err)
	}
	t.Logf("CPU Usage: %v", common.ToString(usage))
}


func TestGetTotalThreads(t *testing.T) {
	threadCount, err := GetTotalThreads()
	if err != nil {
		t.Errorf("Failed to get total threads: %v", err)
	}
	t.Logf("Total Threads: %d", threadCount)
}

func TestCPUArchInfo_getCPUArchInfo(t *testing.T) {
	ctx := context.Background()
	cpuInfo := &CPUArchInfo{}

	err := cpuInfo.getCPUArchInfo(ctx)
	if err != nil {
		t.Errorf("Failed to get CPU architecture info: %v", err)
	}
	t.Logf("CPUArchInfo: %v", common.ToString(cpuInfo))
}

func TestCPUArchInfo_getNumaNodeInfo(t *testing.T) {
	ctx := context.Background()
	cpuInfo := &CPUArchInfo{}

	err := cpuInfo.getNumaNodeInfo(ctx)
	if err != nil {
		t.Errorf("Failed to get NUMA node info: %v", err)
	}
	t.Logf("NumaNodeInfo: %v", common.ToString(cpuInfo.NumaNodeInfo))
}

func TestUsage_getLoadAvg(t *testing.T) {
	usage := &Usage{}
	err := usage.getLoadAvg()
	if err != nil {
		t.Errorf("Failed to get load average: %v", err)
	}
	t.Logf("LoadAvg: 1m=%f, 5m=%f, 15m=%f", usage.CpuLoadAvg1m, usage.CpuLoadAvg5m, usage.CpuLoadAvg15m)
}

func TestUsage_getProcStats(t *testing.T) {
	usage := &Usage{}
	err := usage.getProcStats()
	if err != nil {
		t.Errorf("Failed to get proc stats: %v", err)
	}
	t.Logf("ProcStats: Running=%d, Blocked=%d, Interrupts=%d", usage.SystemProcsRunning, usage.SystemProcsBlocked, usage.SystemInterruptsTotal)
}

