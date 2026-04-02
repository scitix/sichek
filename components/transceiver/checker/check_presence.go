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

// PresenceChecker verifies that each transceiver module is physically present.
// A missing module on a business network is fatal; on a management network it is a warning.
type PresenceChecker struct {
	spec *config.TransceiverSpec
}

func (c *PresenceChecker) Name() string { return config.PresenceCheckerName }

func (c *PresenceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for PresenceChecker")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: "Check transceiver module presence",
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if module.Present {
			continue
		}

		result.Status = consts.StatusAbnormal

		if module.NetworkType == "business" {
			result.Level = consts.LevelFatal
			result.ErrorName = "ModuleAbsent"
			result.Detail += fmt.Sprintf(
				"Interface %s transceiver module is not present (business network — fatal).\n",
				module.Interface,
			)
		} else {
			if result.Level != consts.LevelFatal {
				result.Level = consts.LevelWarning
			}
			result.ErrorName = "ModuleAbsent"
			result.Detail += fmt.Sprintf(
				"Interface %s transceiver module is not present.\n",
				module.Interface,
			)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = "Re-seat or replace the missing transceiver module."
	}

	return result, nil
}
