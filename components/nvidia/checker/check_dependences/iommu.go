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
	"os"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

type IOMMUChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewIOMMUChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &IOMMUChecker{
		name: config.IOMMUCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *IOMMUChecker) Name() string {
	return c.name
}

// Check Checks if IOMMU is closed
func (c *IOMMUChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// checks if IOMMU groups are present in /sys/kernel/iommu_groups
	const iommuPath = "/sys/kernel/iommu_groups"

	// Check if the path exists
	_, err := os.Stat(iommuPath)
	if os.IsNotExist(err) {
		fmt.Printf("%v is not exist", iommuPath) // IOMMU is likely disabled
	} else if err != nil {
		fmt.Printf("failed to access IOMMU groups: %v", err) // IOMMU is likely disabled
	}

	// Check if there are subdirectories (groups)
	groups, err := os.ReadDir(iommuPath)
	if err != nil {
		fmt.Printf("failed to read IOMMU groups: %v", err) // IOMMU is likely disabled
	}
	isIOMMUClosed := len(groups) == 0
	result := config.GPUCheckItems[config.IOMMUCheckerName]

	if !isIOMMUClosed && c.cfg.Dependence.Iommu == "on" {
		result.Status = consts.StatusAbnormal
		result.Detail = "IOMMU is ON, while it should be OFF"
		result.Suggestion = "Please turn off IOMMU"
	} else {
		result.Status = consts.StatusNormal
		if isIOMMUClosed {
			result.Curr = "OFF"
		} else {
			result.Curr = "ON"
		}
		result.Detail = fmt.Sprintf("IOMMU is %s", result.Curr)
		result.Suggestion = ""
	}
	return &result, nil
}
