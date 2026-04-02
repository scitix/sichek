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

// RxPowerChecker checks Rx optical power per lane against module built-in alarm
// thresholds plus a configurable margin from the spec.
type RxPowerChecker struct {
	spec *config.TransceiverSpec
}

func (c *RxPowerChecker) Name() string { return config.RxPowerCheckerName }

func (c *RxPowerChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for RxPowerChecker")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: "Check transceiver Rx optical power per lane",
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}

		netSpec := getNetworkSpec(c.spec, module.NetworkType)
		var margin float64
		if netSpec != nil {
			margin = netSpec.Thresholds.RxPowerMarginDB
		}

		// Skip if no valid alarm thresholds from module
		if module.RxPowerLowAlarm == 0 && module.RxPowerHighAlarm == 0 {
			continue
		}

		isBusiness := module.NetworkType == "business"

		for i, rxPow := range module.RxPower {
			lane := i + 1
			low := module.RxPowerLowAlarm + margin
			high := module.RxPowerHighAlarm - margin

			if rxPow < low || rxPow > high {
				result.Status = consts.StatusAbnormal
				if isBusiness {
					result.Level = consts.LevelCritical
				} else {
					if result.Level != consts.LevelCritical {
						result.Level = consts.LevelWarning
					}
				}
				result.ErrorName = "RxPowerOutOfRange"
				result.Detail += fmt.Sprintf(
					"Interface %s lane %d Rx power %.2f dBm out of range [%.2f, %.2f] dBm (alarm±margin).\n",
					module.Interface, lane, rxPow, low, high,
				)
			}
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = "Check fiber connections, transceiver seating, or replace faulty transceiver."
	}

	return result, nil
}
