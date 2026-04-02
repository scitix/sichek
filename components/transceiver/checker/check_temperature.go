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

// TemperatureChecker checks module temperature against warning and critical thresholds
// from the spec.
type TemperatureChecker struct {
	spec *config.TransceiverSpec
}

func (c *TemperatureChecker) Name() string { return config.TemperatureCheckerName }

func (c *TemperatureChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for TemperatureChecker")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: "Check transceiver module temperature",
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}

		netSpec := getNetworkSpec(c.spec, module.NetworkType)
		if netSpec == nil {
			continue
		}

		temp := module.Temperature
		warnThresh := netSpec.Thresholds.TemperatureWarningC
		critThresh := netSpec.Thresholds.TemperatureCriticalC

		if temp >= critThresh {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "TemperatureCritical"
			result.Detail += fmt.Sprintf(
				"Interface %s temperature %.1f°C exceeds critical threshold %.1f°C.\n",
				module.Interface, temp, critThresh,
			)
		} else if temp >= warnThresh {
			result.Status = consts.StatusAbnormal
			if result.Level != consts.LevelCritical {
				result.Level = consts.LevelWarning
			}
			result.ErrorName = "TemperatureWarning"
			result.Detail += fmt.Sprintf(
				"Interface %s temperature %.1f°C exceeds warning threshold %.1f°C.\n",
				module.Interface, temp, warnThresh,
			)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = "Check airflow, ambient temperature, and cooling. Consider replacing if temperature remains high."
	}

	return result, nil
}
