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

// UtilizationInfo ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g540824faa6cef45500e0d1dc2f50b321
type UtilizationInfo struct {
	// Percent of time over the past sample period during which one or more kernels was executing on the GPU.
	GPUUsagePercent uint32 `json:"gpu_usage_percent"`
	// Percent of time over the past sample period during which global (device) memory was being read or written.
	MemoryUsagePercent uint32 `json:"memory_usage_percent"`
}

func (util *UtilizationInfo) JSON() ([]byte, error) {
	return common.JSON(util)
}

// ToString Convert struct to JSON (pretty-printed)
func (util *UtilizationInfo) ToString() string {
	return common.ToString(util)
}

func (util *UtilizationInfo) Get(device nvml.Device, uuid string) error {
	// Each sample period may be between 1 second and 1/6 second, depending on the product being queried.
	utilization, err := device.GetUtilizationRates()
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get utilization rates for GPU %v : %v", uuid, nvml.ErrorString(err))
	}
	util.GPUUsagePercent = utilization.Gpu
	util.MemoryUsagePercent = utilization.Memory

	return nil
}
