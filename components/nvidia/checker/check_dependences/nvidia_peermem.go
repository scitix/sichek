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
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type NvPeerMemChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewNvPeerMemChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &NvPeerMemChecker{
		name: config.NvPeerMemCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *NvPeerMemChecker) Name() string {
	return c.name
}

func (c *NvPeerMemChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Check if ib_core and nvidia_peermem, and ib_core is using nvidia_peermem
	// Note: for Driver Version 470.199.02, nvidia_peermem is holder by ib_uverbs
	usingPeermem, err := utils.IsKernalModuleHolder("ib_uverbs", "nvidia_peermem")
	if err != nil {
		return nil, fmt.Errorf("failed to check %s is in %s", "ib_uverbs", "nvidia_peermem")
	}
	if !usingPeermem {
		usingPeermem, err = utils.IsKernalModuleHolder("ib_core", "nvidia_peermem")
		if err != nil {
			return nil, fmt.Errorf("failed to check %s is in %s", "ib_core", "nvidia_peermem")
		}
	}

	result := config.GPUCheckItems[config.NvPeerMemCheckerName]

	if !usingPeermem {
		_, err := utils.ExecCommand(ctx, "modprobe", "nvidia_peermem")
		if err == nil {
			result.Status = consts.StatusNormal
			result.Curr = "LoadedOnline"
			result.Detail = "nvidia_peermem is not loaded. It has been loaded online successfully"
			result.Suggestion = ""
		} else {
			result.Status = consts.StatusAbnormal
			result.Curr = "NotLoaded"
			result.Detail = fmt.Sprintf("nvidia_peermem is not loaded correctly. Failed to load nvidia_peermem online: %v", err)
		}
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "Loaded"
		result.Detail = "nvidia_peermem is loaded correctly"
		result.Suggestion = ""
	}
	return &result, nil
}
