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

type GpuPStateChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewGpuPStateChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &GpuPStateChecker{
		name: config.GpuPStateCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuPStateChecker) Name() string {
	return c.name
}

// func (c *GpuPStateChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

// Check if the Nvidia GPU performance state is in state 0 -- Maximum Performance.
func (c *GpuPStateChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.GpuPStateCheckerName]

	// Check if all the Nvidia GPUs are in pstate 0
	var info string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.States.GpuPstate > c.cfg.State.GpuPstate {
			info += fmt.Sprintf("GPU %d: unexpeced pstate P%d, expected pstate P%d\n", device.Index, device.States.GpuPstate, c.cfg.State.GpuPstate)
			var device_pod_name string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				device_pod_name = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				device_pod_name = fmt.Sprintf("%s:", device.UUID)
			}
			falied_gpuid_podnames = append(falied_gpuid_podnames, device_pod_name)
		}
	}
	if len(falied_gpuid_podnames) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("The following GPUs pstates less than P%d:\n %v", c.cfg.State.GpuPstate, info)
		result.Curr = fmt.Sprintf("Above P%d", c.cfg.State.GpuPstate)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
		if c.cfg.State.GpuPstate != 0 {
			result.Curr = fmt.Sprintf("Below or Is P%d", c.cfg.State.GpuPstate)
		} else {
			result.Curr = "P0"
		}
		result.Detail = fmt.Sprintf("All GPUs Pstats are Below P%d", c.cfg.State.GpuPstate)
	}
	return &result, nil
}
