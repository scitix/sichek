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
// Package checker provides functionality to check the Nvidia GPU persistence mode.
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
)

type GpuTemperatureChecker struct {
	name string
	cfg  *nvidia.NvidiaSpecItem
}

func NewGpuTemperatureChecker(cfg *nvidia.NvidiaSpecItem) (common.Checker, error) {
	return &GpuPStateChecker{
		name: nvidia.GpuTemperatureCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuTemperatureChecker) Name() string {
	return c.name
}

// func (c *GpuTemperatureChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

// Check verifies if the Nvidia GPU temperature is > specified C, e.g 75 C .
func (c *GpuTemperatureChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := nvidia.GPUCheckItems[nvidia.GpuTemperatureCheckerName]

	var unexpected_gpus_temp map[int]string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.Temperature.GPUCurTemperature > uint32(c.cfg.TemperatureThreshold.Gpu) ||
			device.Temperature.MemoryCurTemperature > uint32(c.cfg.TemperatureThreshold.Memory) {
			if unexpected_gpus_temp == nil {
				unexpected_gpus_temp = make(map[int]string)
			}
			unexpected_gpus_temp[device.Index] = fmt.Sprintf(
				"GPU %d:%s has high Temperature: GPUCurTemperature = %d C (expected < %d), MemoryCurTemperature = %d C (expected < %d)\n",
				device.Index, device.UUID,
				device.Temperature.GPUCurTemperature,
				c.cfg.TemperatureThreshold.Gpu,
				device.Temperature.MemoryCurTemperature,
				c.cfg.TemperatureThreshold.Memory,
			)
			var device_pod_name string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				device_pod_name = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				device_pod_name = fmt.Sprintf("%s:", device.UUID)
			}
			falied_gpuid_podnames = append(falied_gpuid_podnames, device_pod_name)
		}
	}
	if len(unexpected_gpus_temp) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("%v", unexpected_gpus_temp)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
