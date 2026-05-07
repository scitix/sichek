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
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/consts"
)

const MemoryECCUncorrectedCheckerName = "memory-ecc-uncorrected"

// MemoryECCUncorrectedChecker checks for uncorrectable memory ECC errors via EDAC.
type MemoryECCUncorrectedChecker struct {
	name string
}

func NewMemoryECCUncorrectedChecker() (common.Checker, error) {
	return &MemoryECCUncorrectedChecker{
		name: MemoryECCUncorrectedCheckerName,
	}, nil
}

func (c *MemoryECCUncorrectedChecker) Name() string {
	return c.name
}

func (c *MemoryECCUncorrectedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *MemoryECCUncorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	output, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.Output")
	}

	result := config.MemoryCheckItems[MemoryECCUncorrectedCheckerName]

	if !output.EDAC.Available {
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Curr = "N/A"
		result.Detail = "EDAC monitoring not available"
		result.Suggestion = ""
		return &result, nil
	}

	if output.EDAC.TotalUCE > 0 {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%d", output.EDAC.TotalUCE)
		result.Detail = fmt.Sprintf("Uncorrectable memory ECC errors detected: count=%d", output.EDAC.TotalUCE)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "0"
		result.Detail = "No uncorrectable memory ECC errors detected"
		result.Suggestion = ""
	}

	return &result, nil
}

const MemoryECCCorrectedCheckerName = "memory-ecc-corrected"

// MemoryECCCorrectedChecker checks whether correctable ECC errors exceed a threshold.
type MemoryECCCorrectedChecker struct {
	name      string
	threshold int64
}

func NewMemoryECCCorrectedChecker(threshold int64) (common.Checker, error) {
	return &MemoryECCCorrectedChecker{
		name:      MemoryECCCorrectedCheckerName,
		threshold: threshold,
	}, nil
}

func (c *MemoryECCCorrectedChecker) Name() string {
	return c.name
}

func (c *MemoryECCCorrectedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *MemoryECCCorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	output, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.Output")
	}

	result := config.MemoryCheckItems[MemoryECCCorrectedCheckerName]

	if !output.EDAC.Available {
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Curr = "N/A"
		result.Detail = "EDAC monitoring not available"
		result.Suggestion = ""
		return &result, nil
	}

	if output.EDAC.TotalCE >= c.threshold {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%d", output.EDAC.TotalCE)
		result.Detail = fmt.Sprintf("Correctable memory ECC count %d exceeds threshold %d", output.EDAC.TotalCE, c.threshold)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = fmt.Sprintf("%d", output.EDAC.TotalCE)
		result.Detail = fmt.Sprintf("Correctable memory ECC count %d is below threshold %d", output.EDAC.TotalCE, c.threshold)
		result.Suggestion = ""
	}

	return &result, nil
}
