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
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils"
)

type GPUPCIeACSChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewGPUPCIeACSChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &GPUPCIeACSChecker{
		name: config.GPUPCIeACSCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GPUPCIeACSChecker) Name() string {
	return c.name
}

func (c *GPUPCIeACSChecker) GetSpec() common.CheckerSpec {
	return c.cfg
}

// checks if PCIe ACS is disabled for all NVIDIA GPU
func (c *GPUPCIeACSChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	disabledACS, err := utils.IsAllACSDisabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run IsAllACSDisabled, err: %v", err)
	}

	result := config.GPUCheckItems[config.GPUPCIeACSCheckerName]

	if len(disabledACS) > 0 {
		disabledACSFail, err := utils.DisableAllACS(ctx)
		if err == nil {
			result.Status = commonCfg.StatusNormal
			result.Curr = "DisabledOnline"
			result.Detail = "Detect Not All PCIe ACS are disabled. They have been disabled online successfully"
		} else {
			var failBDFs []string
			errBDFs := make(map[string]string, 0)
			for _, setFail := range disabledACSFail {
				errBDFs[setFail.BDF] = setFail.ACSStatus
				failBDFs = append(failBDFs, setFail.BDF)
			}
			result.Device = strings.Join(failBDFs, ",")
			result.Status = commonCfg.StatusAbnormal
			result.Curr = "NotDisabled"
			result.Detail = fmt.Sprintf("Not All PCIe ACS are disabled: %s", errBDFs)
		}
	} else {
		result.Status = commonCfg.StatusNormal
		result.Curr = "Disabled"
		result.Detail = "All PCIe ACS are disabled"
	}
	return &result, nil
}
