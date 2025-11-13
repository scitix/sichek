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

	"github.com/sirupsen/logrus"
)

type HardwareChecker struct {
	name string
	spec *config.NvidiaSpec
}

func NewHardwareChecker(spec *config.NvidiaSpec) (common.Checker, error) {
	return &HardwareChecker{
		name: config.HardwareCheckerName,
		spec: spec,
	}, nil
}

func (c *HardwareChecker) Name() string {
	return c.name
}

func (c *HardwareChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.HardwareCheckerName]

	// Check if any Nvidia GPU is lost
	lostGPUs, lostReasons := c.checkGPUbyIndex(nvidiaInfo)
	lostGPUNums := len(lostGPUs)
	curGPUNums := c.spec.GpuNums - lostGPUNums
	if lostGPUNums != 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Expected GPU number: %d, Current GPU number: %d, Lost GPU: %v\t\t\n%v",
			c.spec.GpuNums, curGPUNums, lostGPUNums, strings.Join(lostReasons, "\n"))
		result.Device = strings.Join(lostGPUs, ",")
	} else {
		result.Status = consts.StatusNormal

	}
	return &result, nil
}

// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g4cc7ff5253d53cc97b1afb606d614888
func (c *HardwareChecker) checkGPUbyIndex(nvidiaInfo *collector.NvidiaInfo) ([]string, []string) {
	var lostDeviceIDs []string
	var lostDeviceIDErrs []string
	for index := 0; index < c.spec.GpuNums; index++ {
		if available := nvidiaInfo.GPUAvailability[index]; !available {
			errMsg := nvidiaInfo.LostGPUErrors[index]
			lostDeviceIDErrs = append(lostDeviceIDErrs, fmt.Sprintf("NVIDIA GPU %d Error: %s\n", index, errMsg))

			var devicePodName string
			if nvidiaInfo.ValiddeviceUUIDFlag {
				if lostUUID, uuidExists := nvidiaInfo.DeviceUUIDs[index]; uuidExists {
					if _, found := nvidiaInfo.DeviceToPodMap[lostUUID]; found {
						devicePodName = fmt.Sprintf("%s:%d", lostUUID, index)
					} else {
						devicePodName = fmt.Sprintf("%s:", lostUUID)
					}
				} else {
					devicePodName = fmt.Sprintf("%d:", index)
				}
			} else {
				// if the device UUID is not valid, use the index as the device UUID
				devicePodName = fmt.Sprintf("%d:", index)
			}
			lostDeviceIDs = append(lostDeviceIDs, devicePodName)
			logrus.WithField("component", "nvidia").Infof("GPU %d is lost/inaccessible: %s", index, errMsg)
		}
	}
	return lostDeviceIDs, lostDeviceIDErrs
}
