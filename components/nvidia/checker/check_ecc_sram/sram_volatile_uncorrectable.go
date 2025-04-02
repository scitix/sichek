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

type SRAMVolatileUncorrectableChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewSRAMVolatileUncorrectableChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &SRAMVolatileUncorrectableChecker{
		name: config.SRAMVolatileUncorrectableCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *SRAMVolatileUncorrectableChecker) Name() string {
	return c.name
}

func (c *SRAMVolatileUncorrectableChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.SRAMVolatileUncorrectableCheckerName]

	var failedGpuidPodnames []string
	var memoryErrorEvents map[int]string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.MemoryErrors.VolatileECC.SRAM.Uncorrected > c.cfg.MemoryErrorThreshold.SRAMVolatileUncorrectableErrors {
			if memoryErrorEvents == nil {
				memoryErrorEvents = make(map[int]string)
			}
			var devicePodName string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				devicePodName = fmt.Sprintf("%s:", device.UUID)
			}
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			memoryErrorEvents[device.Index] = fmt.Sprintf(
				"GPU %d:%s SRAM Volatile Uncorrectable Detected: %d, Threshold: %d\n",
				device.Index, device.UUID,
				device.MemoryErrors.VolatileECC.SRAM.Uncorrected,
				c.cfg.MemoryErrorThreshold.SRAMVolatileUncorrectableErrors)
		}
	}
	if len(failedGpuidPodnames) > 0 {
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
