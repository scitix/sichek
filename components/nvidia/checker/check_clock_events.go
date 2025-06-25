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

type ClockEventsChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewClockEventsChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &ClockEventsChecker{
		name: config.ClockEventsCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *ClockEventsChecker) Name() string {
	return c.name
}

func (c *ClockEventsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.ClockEventsCheckerName]

	// Check if any critical clock event is engaged in any Nvidia GPU
	var devClockEvents map[int]string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if !device.ClockEvents.IsSupported {
			logrus.WithField("component", "Nvidia-Clock-Events-Checker").Warnf("device is not supported for clock events")
			return nil, nil
		}
		if len(device.ClockEvents.CriticalClockEvents) > 0 {
			if devClockEvents == nil {
				devClockEvents = make(map[int]string)
			}
			devClockEvents[device.Index] = device.ClockEvents.ToString()
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
		}
	}
	if len(devClockEvents) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Critical clock events engaged: \n%v", devClockEvents)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
	}
	return &result, nil
}
