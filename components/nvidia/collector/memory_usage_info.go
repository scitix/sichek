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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
)

// MemoryUsageInfo is GPU framebuffer (VRAM) capacity usage in bytes, as reported
// by nvmlDeviceGetMemoryInfo. This is distinct from UtilizationInfo.MemoryUsagePercent,
// which is memory-bandwidth busy time, not capacity used.
// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g2dfeb1db82aa1de91aa6edf941c85ca8
type MemoryUsageInfo struct {
	// Total framebuffer memory installed on the device.
	TotalBytes uint64 `json:"memory_total_bytes" yaml:"memory_total_bytes"`
	// Unallocated framebuffer memory.
	FreeBytes uint64 `json:"memory_free_bytes" yaml:"memory_free_bytes"`
	// Allocated framebuffer memory (sum of all allocations, includes driver reserved).
	UsedBytes uint64 `json:"memory_used_bytes" yaml:"memory_used_bytes"`
}

func (info *MemoryUsageInfo) JSON() ([]byte, error) {
	return common.JSON(info)
}

// ToString Convert struct to JSON (pretty-printed)
func (info *MemoryUsageInfo) ToString() string {
	return common.ToString(info)
}

func (info *MemoryUsageInfo) Get(device nvml.Device, uuid string) error {
	memory, err := device.GetMemoryInfo()
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get memory info for GPU %v : %v", uuid, nvml.ErrorString(err))
	}
	info.TotalBytes = memory.Total
	info.FreeBytes = memory.Free
	info.UsedBytes = memory.Used

	return nil
}
