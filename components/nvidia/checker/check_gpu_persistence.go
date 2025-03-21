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
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils"
)

type GpuPersistenceChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewGpuPersistenceChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &GpuPersistenceChecker{
		name: config.GpuPersistenceCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuPersistenceChecker) Name() string {
	return c.name
}

func (c *GpuPersistenceChecker) GetSpec() common.CheckerSpec {
	return c.cfg
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

	result := config.GPUCheckItems[config.GpuPersistenceCheckerName]

	// Check if all the Nvidia GPUs have persistence mode enabled
	var disable_gpus []string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.States.GpuPersistenced != c.cfg.State.GpuPersistenced {
			var device_pod_name string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				device_pod_name = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				device_pod_name = fmt.Sprintf("%s:", device.UUID)
			}
			disable_gpus = append(disable_gpus, fmt.Sprintf("GPU %d", device.Index))
			_, err := utils.ExecCommand(ctx, "nvidia-smi", "-i", fmt.Sprintf("%d", device.Index), "-pm", "1")
			if err != nil {
				result.Detail += fmt.Sprintf("GPU %d:  Failed to enable persistence mode: %s\n", device.Index, err.Error())
				falied_gpuid_podnames = append(falied_gpuid_podnames, device_pod_name)
			} else {
				result.Detail += fmt.Sprintf("GPU %d:  Persistence mode has been enabled\n", device.Index)
			}
		}
	}
	result.Status = commonCfg.StatusNormal
	if len(disable_gpus) == 0 {
		result.Status = commonCfg.StatusNormal
		result.Detail = "All Nvidia GPUs have persistence mode enabled"
		result.Curr = "Enabled"
		result.Suggestion = ""
		result.ErrorName = ""
	} else {
		if len(falied_gpuid_podnames) == 0 {
			result.Status = commonCfg.StatusNormal
			result.Curr = "EnabledOnline"
			result.Suggestion = ""
			result.ErrorName = ""
		} else {
			result.Status = commonCfg.StatusAbnormal
			result.Curr = "Disabled"
			result.Device = fmt.Sprintf("%v", strings.Join(falied_gpuid_podnames, ","))
		}
	}
	return &result, nil
}
