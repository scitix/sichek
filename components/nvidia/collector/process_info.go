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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type ComputeProcesses []ComputeProcess

type ComputeProcess struct {
	Pid              uint32 `json:"pid" yaml:"pid"`
	Comm             string `json:"comm,omitempty" yaml:"comm,omitempty"`
	UsedGpuMemoryMiB uint64 `json:"used_gpu_memory_mib" yaml:"used_gpu_memory_mib"`
}

func (p ComputeProcesses) JSON() ([]byte, error) {
	return common.JSON(p)
}

func (p ComputeProcesses) ToString() string {
	return common.ToString(p)
}

func (p *ComputeProcesses) Get(device nvml.Device, uuid string) error {
	*p = make(ComputeProcesses, 0)
	procs, ret := device.GetComputeRunningProcesses()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get compute running processes for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}

	out := make(ComputeProcesses, 0, len(procs))
	for _, pi := range procs {
		out = append(out, ComputeProcess{
			Pid:              pi.Pid,
			Comm:             readProcComm(pi.Pid),
			UsedGpuMemoryMiB: pi.UsedGpuMemory / (1024 * 1024),
		})
	}
	*p = out
	return nil
}

// readProcComm reads /proc/<pid>/comm and returns the process name without
// the trailing newline. Returns an empty string on any failure (process exited,
// PID not visible from this namespace, permission denied) — callers fall back
// to the raw PID.
func readProcComm(pid uint32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}
