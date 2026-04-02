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

// BiasCurrentChecker checks laser bias current per lane.
// A value <= 0 indicates the laser is off or data is invalid.
// Management network interfaces are skipped.
type BiasCurrentChecker struct {
	spec *config.TransceiverSpec
}

func (c *BiasCurrentChecker) Name() string { return config.BiasCurrentCheckerName }

func (c *BiasCurrentChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for BiasCurrentChecker")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: "Check transceiver laser bias current per lane",
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}
		// Skip management network
		if module.NetworkType == "management" {
			continue
		}

		for i, bias := range module.BiasCurrent {
			lane := i + 1
			if bias <= 0 {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "BiasCurrentZero"
				result.Detail += fmt.Sprintf(
					"Interface %s lane %d bias current %.3f mA is <= 0 (laser may be off or faulty).\n",
					module.Interface, lane, bias,
				)
			}
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = "Verify transceiver is properly seated and laser is enabled. Replace transceiver if bias current remains zero."
	}

	return result, nil
}
