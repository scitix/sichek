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
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type PCIeACSChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewPCIeACSChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &PCIeACSChecker{
		name: config.PCIeACSCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *PCIeACSChecker) Name() string {
	return c.name
}

// func (c *PCIeACSChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

// checks if PCIe ACS is disabled for all NVIDIA GPU
func (c *PCIeACSChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	enabledACS, err := utils.GetACSEnabledDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run GetACSEnabledDevices, err: %v", err)
	}

	result := config.GPUCheckItems[config.PCIeACSCheckerName]

	if len(enabledACS) > 0 {
		failedDevices, err := utils.BatchDisableACS(ctx, enabledACS)
		if err == nil && len(failedDevices) == 0 {
			result.Status = consts.StatusNormal
			result.Curr = "DisabledOnline"
			result.Detail = "Detect Not All PCIe ACS are disabled. They have been disabled online successfully"
		} else {
			var failedBDFs []string
			for _, failedDevice := range failedDevices {
				failedBDFs = append(failedBDFs, failedDevice.BDF)
			}
			result.Device = strings.Join(failedBDFs, ",")
			result.Status = consts.StatusAbnormal
			result.Curr = "NotAllDisabled"
			result.Detail = fmt.Sprintf("Not All PCIe ACS are disabled. Failed to disable online: %v", failedBDFs)
		}
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "Disabled"
		result.Detail = "All PCIe ACS are disabled"
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
