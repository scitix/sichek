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
	"math"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/consts"
)

const MemoryCapacityCheckerName = "memory-capacity"

// MemoryCapacityChecker verifies that installed memory matches the expected value.
type MemoryCapacityChecker struct {
	name         string
	expectedGB   float64
	tolerancePct float64
}

func NewMemoryCapacityChecker(expectedGB, tolerancePct float64) (common.Checker, error) {
	return &MemoryCapacityChecker{
		name:         MemoryCapacityCheckerName,
		expectedGB:   expectedGB,
		tolerancePct: tolerancePct,
	}, nil
}

func (c *MemoryCapacityChecker) Name() string {
	return c.name
}

func (c *MemoryCapacityChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *MemoryCapacityChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	output, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.Output")
	}

	result := config.MemoryCheckItems[MemoryCapacityCheckerName]

	if c.expectedGB <= 0 {
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Curr = fmt.Sprintf("%.2f GB", output.Capacity.TotalGB)
		result.Detail = "No expected memory capacity configured, skipping check"
		result.Suggestion = ""
		return &result, nil
	}

	actualGB := output.Capacity.TotalGB
	diffPct := math.Abs(actualGB-c.expectedGB) / c.expectedGB * 100

	if diffPct > c.tolerancePct {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%.2f GB", actualGB)
		result.Spec = fmt.Sprintf("%.2f GB", c.expectedGB)
		result.Detail = fmt.Sprintf("Memory capacity %.2f GB deviates %.1f%% from expected %.2f GB (tolerance %.1f%%)",
			actualGB, diffPct, c.expectedGB, c.tolerancePct)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = fmt.Sprintf("%.2f GB", actualGB)
		result.Spec = fmt.Sprintf("%.2f GB", c.expectedGB)
		result.Detail = fmt.Sprintf("Memory capacity %.2f GB is within %.1f%% tolerance of expected %.2f GB",
			actualGB, c.tolerancePct, c.expectedGB)
		result.Suggestion = ""
	}

	return &result, nil
}
