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
	cfg  *config.NvidiaSpecItem
}

func NewAppClocksChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &AppClocksChecker{
		name: config.AppClocksCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *AppClocksChecker) Name() string {
	return c.name
}

// func (c *AppClocksChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

func (c *AppClocksChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.AppClocksCheckerName]

	// Check if all the Nvidia GPUs have set application clocks to max
	var gpus_app_clocks_status map[int]string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.Clock.AppGraphicsClk != device.Clock.MaxGraphicsClk || device.Clock.AppMemoryClk != device.Clock.MaxMemoryClk ||
			device.Clock.AppSMClk != device.Clock.MaxSMClk {
			if gpus_app_clocks_status == nil {
				gpus_app_clocks_status = make(map[int]string)
			}
			gpus_app_clocks_status[device.Index] = fmt.Sprintf(
				"GPU %d:%s AppSMClk: %s, MaxAppSMClk: %s, AppGraphicsClk: %s, MaxGraphicsClk: %s, AppMemoryClk: %s, MaxMemoryClk: %s\n",
				device.Index, device.UUID,
				device.Clock.AppSMClk,
				device.Clock.MaxSMClk,
				device.Clock.AppGraphicsClk,
				device.Clock.MaxGraphicsClk,
				device.Clock.AppMemoryClk,
				device.Clock.MaxMemoryClk,
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
	if len(gpus_app_clocks_status) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Not all GPU application clocks are set to max: \n %v", gpus_app_clocks_status)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
