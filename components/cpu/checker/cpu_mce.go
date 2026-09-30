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
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"
)

const CPUMCEUncorrectedCheckerName = "cpu-mce-uncorrected"

type CPUMCEUncorrectedChecker struct {
	name string
}

func NewCPUMCEUncorrectedChecker() (common.Checker, error) {
	return &CPUMCEUncorrectedChecker{
		name: CPUMCEUncorrectedCheckerName,
	}, nil
}

func (c *CPUMCEUncorrectedChecker) Name() string {
	return c.name
}

func (c *CPUMCEUncorrectedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *CPUMCEUncorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuOutput, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.CPUOutput")
	}

	result := config.CPUCheckItems[CPUMCEUncorrectedCheckerName]

	if !cpuOutput.MCEInfo.Available {
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Curr = "N/A"
		result.Detail = "MCE monitoring not available"
		result.Suggestion = ""
		return &result, nil
	}

	if cpuOutput.MCEInfo.UncorrectedCount > 0 {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%d", cpuOutput.MCEInfo.UncorrectedCount)
		result.Detail = fmt.Sprintf("Uncorrected MCE detected: count=%d", cpuOutput.MCEInfo.UncorrectedCount)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "0"
		result.Detail = "No uncorrected MCE detected"
		result.Suggestion = ""
	}

	return &result, nil
}

const CPUMCECorrectedCheckerName = "cpu-mce-corrected"

type CPUMCECorrectedChecker struct {
	name      string
	threshold int64
}

func NewCPUMCECorrectedChecker(threshold int64) (common.Checker, error) {
	return &CPUMCECorrectedChecker{
		name:      CPUMCECorrectedCheckerName,
		threshold: threshold,
	}, nil
}

func (c *CPUMCECorrectedChecker) Name() string {
	return c.name
}

func (c *CPUMCECorrectedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *CPUMCECorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuOutput, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.CPUOutput")
	}

	result := config.CPUCheckItems[CPUMCECorrectedCheckerName]

	if !cpuOutput.MCEInfo.Available {
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Curr = "N/A"
		result.Detail = "MCE monitoring not available"
		result.Suggestion = ""
		return &result, nil
	}

	if cpuOutput.MCEInfo.CorrectedCount >= c.threshold {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%d", cpuOutput.MCEInfo.CorrectedCount)
		result.Detail = fmt.Sprintf("Corrected MCE count %d exceeds threshold %d", cpuOutput.MCEInfo.CorrectedCount, c.threshold)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = fmt.Sprintf("%d", cpuOutput.MCEInfo.CorrectedCount)
		result.Detail = fmt.Sprintf("Corrected MCE count %d is below threshold %d", cpuOutput.MCEInfo.CorrectedCount, c.threshold)
		result.Suggestion = ""
	}

	return &result, nil
}
