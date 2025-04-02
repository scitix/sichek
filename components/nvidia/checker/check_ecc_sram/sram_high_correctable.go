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

type SRAMHighcorrectableChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewSRAMHighcorrectableChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &SRAMHighcorrectableChecker{
		name: config.SRAMHighcorrectableCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *SRAMHighcorrectableChecker) Name() string {
	return c.name
}

func (c *SRAMHighcorrectableChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.SRAMHighcorrectableCheckerName]

	var memoryErrorEvents map[string]string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.MemoryErrors.AggregateECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors ||
			device.MemoryErrors.VolatileECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors {
			if memoryErrorEvents == nil {
				memoryErrorEvents = make(map[string]string)
			}
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
		}

		if device.MemoryErrors.AggregateECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors {
			memoryErrorEvents[device.UUID] = fmt.Sprintf(
				"GPU %d:%s SRAM High AggregateECC Correctable error count detected: %d, Threshold: %d",
				device.Index, device.UUID,
				device.MemoryErrors.AggregateECC.SRAM.Corrected, c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors)
		}
		if device.MemoryErrors.VolatileECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors {
			memoryErrorEvents[device.UUID] = fmt.Sprintf(
				"GPU %d:%s SRAM High VolitileECC Correctable error count detected: %d, Threshold: %d",
				device.Index, device.UUID,
				device.MemoryErrors.VolatileECC.SRAM.Corrected, c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors)
		}
	}
	if len(memoryErrorEvents) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("%v", memoryErrorEvents)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
