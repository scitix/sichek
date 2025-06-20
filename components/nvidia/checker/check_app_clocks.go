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

type AppClocksChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewAppClocksChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &AppClocksChecker{
		name: config.AppClocksCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *AppClocksChecker) Name() string {
	return c.name
}

func (c *AppClocksChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.AppClocksCheckerName]

	// Check if all the Nvidia GPUs have set application clocks to max
	var gpusAppClocksStatus map[int]string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.Clock.AppGraphicsClk != device.Clock.MaxGraphicsClk || device.Clock.AppMemoryClk != device.Clock.MaxMemoryClk ||
			device.Clock.AppSMClk != device.Clock.MaxSMClk {
			if gpusAppClocksStatus == nil {
				gpusAppClocksStatus = make(map[int]string)
			}
			gpusAppClocksStatus[device.Index] = fmt.Sprintf(
				"GPU %d:%s AppSMClk: %d Mhz, MaxAppSMClk: %d Mhz, AppGraphicsClk: %d Mhz, MaxGraphicsClk: %d Mhz, AppMemoryClk: %d Mhz, MaxMemoryClk: %d Mhz\n",
				device.Index, device.UUID,
				device.Clock.AppSMClk,
				device.Clock.MaxSMClk,
				device.Clock.AppGraphicsClk,
				device.Clock.MaxGraphicsClk,
				device.Clock.AppMemoryClk,
				device.Clock.MaxMemoryClk,
			)
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID].String())
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
		}
	}
	if len(gpusAppClocksStatus) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Not all GPU application clocks are set to max: \n %v", gpusAppClocksStatus)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
	}
	return &result, nil
}
