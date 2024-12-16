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
	"encoding/binary"
	"fmt"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type TemperatureInfo struct {
	GPUCurTemperature                  uint32 `json:"current_temperature_C"`
	GPUThreadholdTemperature           uint32 `json:"threadhold_temperature_C"`
	GPUThreadholdTemperatureShutdown   uint32 `json:"threadhold_temperature_shutdown_C"`
	GPUThreadholdTemperatureSlowdown   uint32 `json:"threadhold_temperature_slowdown_C"`
	MemoryCurTemperature               uint32 `json:"current_memory_temperature_C"`
	MemoryMaxOperationLimitTemperature uint32 `json:"max_memory_operation_temperature_C"`
}

func (p *TemperatureInfo) JSON() ([]byte, error) {
	return common.JSON(p)
}

// Convert struct to JSON (pretty-printed)
func (p *TemperatureInfo) ToString() string {
	return common.ToString(p)
}

func (info *TemperatureInfo) Get(device nvml.Device, uuid string) error {
	// Get the current GPU temperature
	gpuTemp, err := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get GPU temperature: %v", nvml.ErrorString(err))
	}
	info.GPUCurTemperature = gpuTemp

	// Get the GPU temperature thresholds
	gpuTempThreshold, err := device.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_GPU_MAX)
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get GPU temperature threshold: %v", nvml.ErrorString(err))
	}
	info.GPUThreadholdTemperature = gpuTempThreshold

	gpuTempShutdown, err := device.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_SHUTDOWN)
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get GPU temperature shutdown threshold: %v", nvml.ErrorString(err))
	}
	info.GPUThreadholdTemperatureShutdown = gpuTempShutdown

	gpuTempSlowdown, err := device.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_SLOWDOWN)
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get GPU temperature slowdown threshold: %v", nvml.ErrorString(err))
	}
	info.GPUThreadholdTemperatureSlowdown = gpuTempSlowdown

	// Get the current memory temperature
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlFieldValueQueries.html#group__nvmlFieldValueQueries_1g0b02941a262ee4327eb82831f91a1bc0
	values := []nvml.FieldValue{
		{FieldId: nvml.FI_DEV_MEMORY_TEMP}, // Memory temperature for the device
	}

	err = device.GetFieldValues(values)
	if err == nvml.SUCCESS {
		info.MemoryCurTemperature = uint32(binary.NativeEndian.Uint64(values[0].Value[:]))
	} else {
		info.MemoryCurTemperature = 0 //"N/A"
	}

	// Get the maximum memory operation limit temperature
	memMaxTemp, err := device.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_MEM_MAX)
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get memory max operation temperature: %v", nvml.ErrorString(err))
	}
	info.MemoryMaxOperationLimitTemperature = memMaxTemp

	return nil
}
