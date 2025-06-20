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
	"github.com/scitix/sichek/pkg/utils"
)

type GpuPersistenceChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewGpuPersistenceChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &GpuPersistenceChecker{
		name: config.GpuPersistencedCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuPersistenceChecker) Name() string {
	return c.name
}

// Check verifies if the Nvidia GPU persistence mode is enabled and working correctly.
// It takes a context and data of type NvidiaInfo, and returns a CheckerResult and an error.
// The data parameter is expected to be of type collector.NvidiaInfo, which contains information about Nvidia devices.
// Possible error conditions include:
// - If the data type is not collector.NvidiaInfo, an error is returned.
// - If any GPU does not have persistence mode enabled, the result status is set to "abnormal" and an error code is provided.
func (c *GpuPersistenceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.GpuPersistencedCheckerName]

	// Check if all the Nvidia GPUs have persistence mode enabled
	var disableGpus []string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.States.GpuPersistenceM != c.cfg.State.GpuPersistenceM {
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			disableGpus = append(disableGpus, fmt.Sprintf("GPU %d", device.Index))
			_, err := utils.ExecCommand(ctx, "nvidia-persistenced")
			if err != nil {
				result.Detail += fmt.Sprintf("GPU %d:  Failed to enable persistence mode: %s\n", device.Index, err.Error())
				failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			} else {
				result.Detail += fmt.Sprintf("GPU %d:  Persistence mode has been enabled\n", device.Index)
			}
		}
	}
	result.Status = consts.StatusNormal
	if len(disableGpus) == 0 {
		result.Status = consts.StatusNormal
		result.Detail = "All Nvidia GPUs have persistence mode enabled"
		result.Curr = "Enabled"
		result.Suggestion = ""
	} else {
		if len(failedGpuidPodnames) == 0 {
			result.Status = consts.StatusNormal
			result.Curr = "EnabledOnline"
			result.Suggestion = ""
		} else {
			result.Status = consts.StatusAbnormal
			result.Curr = "Disabled"
			result.Device = fmt.Sprintf("%v", strings.Join(failedGpuidPodnames, ","))
		}
	}
	return &result, nil
}
