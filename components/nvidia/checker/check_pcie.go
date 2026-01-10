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

type PCIeChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewPCIeChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &PCIeChecker{
		name: config.PCIeCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *PCIeChecker) Name() string {
	return c.name
}

func (c *PCIeChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.PCIeCheckerName]

	// Check if any degraded PCIe link is detected
	info := ""
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		// For device `NVIDIA L40` and `NVIDIA 5090`, PCIe link generation may not be its maximum when pstate is not P0
		if device.PCIeInfo.PCILinkGen != device.PCIeInfo.PCILinkGenMAX &&
			(!device.ClockEvents.IsSupported || (device.ClockEvents.IsSupported && device.States.GpuPstate == 0)) {
			info += fmt.Sprintf("GPU %d: %v PCIe link gen is %v, expected gen is %d\n",
				device.Index, device.PCIeInfo.BDFID, device.PCIeInfo.PCILinkGen, device.PCIeInfo.PCILinkGenMAX)
			result.Status = consts.StatusAbnormal
		}
		if device.PCIeInfo.PCILinkWidth != device.PCIeInfo.PCILinkWidthMAX {
			info += fmt.Sprintf("GPU %d: %v PCIe link width is %d, expected width is %d\n",
				device.Index, device.PCIeInfo.BDFID, device.PCIeInfo.PCILinkWidth, device.PCIeInfo.PCILinkWidthMAX)
			result.Status = consts.StatusAbnormal
		}

		if device.PCIeInfo.PCILinkGen != device.PCIeInfo.PCILinkGenMAX || device.PCIeInfo.PCILinkWidth != device.PCIeInfo.PCILinkWidthMAX {
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
		}
	}
	if result.Status == consts.StatusAbnormal {
		result.Detail = info
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
	}
	return &result, nil
}
