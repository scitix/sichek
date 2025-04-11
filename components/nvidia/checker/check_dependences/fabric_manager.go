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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/systemd"
)

type NVFabricManagerChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewNVFabricManagerChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &NVFabricManagerChecker{
		name: config.NVFabricManagerCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *NVFabricManagerChecker) Name() string {
	return c.name
}

func (c *NVFabricManagerChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	result := config.GPUCheckItems[config.NVFabricManagerCheckerName]
	if c.cfg.Dependence.FabricManager == "Not Required" {
		result.Status = consts.StatusNormal
		result.Curr = "Not Required"
		result.Suggestion = ""
		result.ErrorName = ""
		return &result, nil
	}

	active, _ := systemd.IsActive("nvidia-fabricmanager")

	if !active {
		result.Status = consts.StatusAbnormal
		result.Detail = "Nvidia FabricManager is not active, please check to restart Nvidia FabricManager"
		// err := systemd.RestartSystemdService("nvidia-fabricmanager")
		// if err == nil {
		// 	result.Status = consts.StatusNormal
		// 	result.Curr = "Restarted"
		// 	result.Detail = "Nvidia FabricManager is not active. It has been restarted successfully"
		// } else {
		// 	result.Status = consts.StatusAbnormal
		// 	result.Curr = "NotActive"
		// 	result.Detail = fmt.Sprintf("Nvidia FabricManager is not active. Failed to try to restart Nvidia FabricManager: %v", err)
		// }
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "Active"
		result.Detail = "Nvidia FabricManager is active"
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
