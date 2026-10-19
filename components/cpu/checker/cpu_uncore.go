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
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"
)

const CPUUncoreFreqCheckerName = "cpu-uncore-freq"

// CPUUncoreFreqChecker flags Intel physical hosts whose uncore frequency band is not
// collapsed onto its ceiling. Uncore is a separate frequency domain from the cores and is
// invisible to core-frequency metrics: a node can look healthy while L3/memory-controller
// clocks sit at the PCU floor. The verdict is on the configured band, never on
// current_freq_khz, which is instantaneous and reads low on a healthy idle node.
type CPUUncoreFreqChecker struct {
	name string
}

func NewCPUUncoreFreqChecker() (common.Checker, error) {
	return &CPUUncoreFreqChecker{
		name: CPUUncoreFreqCheckerName,
	}, nil
}

func (c *CPUUncoreFreqChecker) Name() string {
	return c.name
}

func (c *CPUUncoreFreqChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *CPUUncoreFreqChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuOutput, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.CPUOutput")
	}

	result := config.CPUCheckItems[CPUUncoreFreqCheckerName]
	uncore := cpuOutput.UncoreInfo

	if !uncore.Applicable {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = fmt.Sprintf("Uncore frequency check skipped: %s", uncore.SkipReason)
		result.Suggestion = ""
		return &result, nil
	}

	if !uncore.DriverLoaded {
		result.Status = consts.StatusAbnormal
		result.Curr = "DriverNotLoaded"
		result.Detail = "intel_uncore_frequency driver not loaded, uncore frequency is unmanaged and left to the PCU"
		return &result, nil
	}

	if len(uncore.Domains) == 0 {
		result.Status = consts.StatusAbnormal
		result.Curr = "NoDomain"
		result.Detail = "intel_uncore_frequency is present but exposes no readable domain"
		return &result, nil
	}

	var band, loose []string
	for _, d := range uncore.Domains {
		band = append(band, fmt.Sprintf("%s:%d-%d", d.Name, d.MinFreqKHz/1000, d.MaxFreqKHz/1000))
		if !d.Pinned() {
			loose = append(loose, fmt.Sprintf("%s(min=%d,max=%d,ceiling=%d MHz)",
				d.Name, d.MinFreqKHz/1000, d.MaxFreqKHz/1000, d.InitialMaxFreqKHz/1000))
		}
	}
	result.Curr = fmt.Sprintf("%s %s MHz", uncore.Interface, strings.Join(band, ","))

	if len(loose) > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Uncore band not pinned to ceiling on %d/%d %s domain(s): %s",
			len(loose), len(uncore.Domains), uncore.Interface, strings.Join(loose, ", "))
	} else {
		result.Status = consts.StatusNormal
		result.Detail = fmt.Sprintf("Uncore band pinned to ceiling on all %d %s domain(s)", len(uncore.Domains), uncore.Interface)
		result.Suggestion = ""
	}
	return &result, nil
}
