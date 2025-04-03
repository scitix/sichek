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
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

type GpuTemperatureChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewGpuTemperatureChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &GpuPStateChecker{
		name: config.GpuTemperatureCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuTemperatureChecker) Name() string {
	return c.name
}

// Check verifies if the Nvidia GPU temperature is > specified C, e.g. 75 C .
func (c *GpuTemperatureChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.GpuTemperatureCheckerName]

	var unexpectedGpusTemp map[int]string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.Temperature.GPUCurTemperature > uint32(c.cfg.TemperatureThreshold.Gpu) ||
			device.Temperature.MemoryCurTemperature > uint32(c.cfg.TemperatureThreshold.Memory) {
			if unexpectedGpusTemp == nil {
				unexpectedGpusTemp = make(map[int]string)
			}
			unexpectedGpusTemp[device.Index] = fmt.Sprintf(
				"GPU %d:%s has high Temperature: GPUCurTemperature = %d C (expected < %d), MemoryCurTemperature = %d C (expected < %d)\n",
				device.Index, device.UUID,
				device.Temperature.GPUCurTemperature,
				c.cfg.TemperatureThreshold.Gpu,
				device.Temperature.MemoryCurTemperature,
				c.cfg.TemperatureThreshold.Memory,
			)
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
		}
	}
	if len(unexpectedGpusTemp) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("%v", unexpectedGpusTemp)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
