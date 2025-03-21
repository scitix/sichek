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
	commonCfg "github.com/scitix/sichek/config"
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

func (c *ClockEventsChecker) GetSpec() common.CheckerSpec {
	return c.cfg
}

func (c *ClockEventsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.ClockEventsCheckerName]

	// Check if any critical clock event is engaged in any Nvidia GPU
	var dev_clock_events map[int]string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if !device.ClockEvents.IsSupported {
			return nil, nil
		}
		if len(device.ClockEvents.CriticalClockEvents) > 0 {
			if dev_clock_events == nil {
				dev_clock_events = make(map[int]string)
			}
			dev_clock_events[device.Index] = device.ClockEvents.ToString()
			var device_pod_name string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				device_pod_name = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				device_pod_name = fmt.Sprintf("%s:", device.UUID)
			}
			falied_gpuid_podnames = append(falied_gpuid_podnames, device_pod_name)
		}
	}
	if len(dev_clock_events) > 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Detail = fmt.Sprintf("Critical clock events engaged: \n%v", dev_clock_events)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = commonCfg.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
