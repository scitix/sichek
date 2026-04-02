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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

// VoltageChecker checks supply voltage against module built-in alarm thresholds.
type VoltageChecker struct {
	spec *config.TransceiverSpec
}

func (c *VoltageChecker) Name() string { return config.VoltageCheckerName }

func (c *VoltageChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VoltageChecker")
	}

	tmpl := config.GetCheckItem(c.Name(), "business")
	result := &common.CheckerResult{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}

		volt := module.Voltage
		low := module.VoltageLowAlarm
		high := module.VoltageHighAlarm

		// Skip if no valid thresholds available (module doesn't report them)
		if low == 0 && high == 0 {
			continue
		}

		if volt < low || volt > high {
			result.Status = consts.StatusAbnormal
			itemLevel := config.GetCheckItem(c.Name(), module.NetworkType).Level
			if consts.LevelPriority[itemLevel] > consts.LevelPriority[result.Level] {
				result.Level = itemLevel
			}
			result.ErrorName = tmpl.ErrorName
			result.Detail += fmt.Sprintf(
				"Interface %s voltage %.3f V out of range [%.3f, %.3f] V.\n",
				module.Interface, volt, low, high,
			)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = tmpl.Suggestion
	}

	return result, nil
}
