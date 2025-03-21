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

type SRAMHighcorrectableChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewSRAMHighcorrectableChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &SRAMHighcorrectableChecker{
		name: config.SRAMHighcorrectableCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *SRAMHighcorrectableChecker) Name() string {
	return c.name
}

func (c *SRAMHighcorrectableChecker) GetSpec() common.CheckerSpec {
	return c.cfg
}

func (c *SRAMHighcorrectableChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.SRAMHighcorrectableCheckerName]

	var memory_error_events map[string]string
	var falied_gpuid_podnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.MemoryErrors.AggregateECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors ||
			device.MemoryErrors.VolatileECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors {
			if memory_error_events == nil {
				memory_error_events = make(map[string]string)
			}
			var device_pod_name string
			if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
				device_pod_name = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
			} else {
				device_pod_name = fmt.Sprintf("%s:", device.UUID)
			}
			falied_gpuid_podnames = append(falied_gpuid_podnames, device_pod_name)
		}

		if device.MemoryErrors.AggregateECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors {
			memory_error_events[device.UUID] = fmt.Sprintf(
				"GPU %d:%s SRAM High AggregateECC Correctable error count detected: %d, Threshold: %d",
				device.Index, device.UUID,
				device.MemoryErrors.AggregateECC.SRAM.Corrected, c.cfg.MemoryErrorThreshold.SRAMAggregateCorrectableErrors)
		}
		if device.MemoryErrors.VolatileECC.SRAM.Corrected > c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors {
			memory_error_events[device.UUID] = fmt.Sprintf(
				"GPU %d:%s SRAM High VolitileECC Correctable error count detected: %d, Threshold: %d",
				device.Index, device.UUID,
				device.MemoryErrors.VolatileECC.SRAM.Corrected, c.cfg.MemoryErrorThreshold.SRAMVolatileCorrectableErrors)
		}
	}
	if len(memory_error_events) > 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Detail = fmt.Sprintf("%v", memory_error_events)
		result.Device = strings.Join(falied_gpuid_podnames, ",")
	} else {
		result.Status = commonCfg.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
