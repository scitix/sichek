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
	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
)

type SRAMAggUncorrectableChecker struct {
	name string
	cfg  *nvidia.NvidiaSpecItem
}

func NewSRAMAggUncorrectableChecker(cfg *nvidia.NvidiaSpecItem) (common.Checker, error) {
	return &SRAMAggUncorrectableChecker{
		name: nvidia.SRAMAggUncorrectableCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *SRAMAggUncorrectableChecker) Name() string {
	return c.name
}

// func (c *SRAMAggUncorrectableChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

func (c *SRAMAggUncorrectableChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := nvidia.GPUCheckItems[nvidia.SRAMAggUncorrectableCheckerName]

	var falied_gpuid_podnames []string
	var memory_error_events map[int]string
	for _, device := range nvidiaInfo.DevicesInfo {
		// suggestion action : replace GPU
		if device.MemoryErrors.AggregateECC.SRAM.Uncorrected > c.cfg.MemoryErrorThreshold.SRAMAggregateUncorrectableErrors {
			if memory_error_events == nil {
				memory_error_events = make(map[int]string)
			}
			memory_error_events[device.Index] = fmt.Sprintf(
				"GPU %d:%s SRAM Aggregate Uncorrectable Detected: %d, Threshold: %d\n",
				device.Index, device.UUID,
				device.MemoryErrors.AggregateECC.SRAM.Uncorrected,
				c.cfg.MemoryErrorThreshold.SRAMAggregateUncorrectableErrors)
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
		result.Detail = fmt.Sprintf("%v", memory_error_events)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
