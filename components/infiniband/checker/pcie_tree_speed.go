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
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type IBPCIETreeSpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPCIETreeSpeedChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPCIETreeSpeedChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIETreeSpeed,
		spec: specCfg,
	}, nil
}

func (c *IBPCIETreeSpeedChecker) Name() string {
	return c.name
}

func (c *IBPCIETreeSpeedChecker) Description() string {
	return c.description
}

func (c *IBPCIETreeSpeedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIETreeSpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	infinibandInfo.RLock()
	hws := uniqueByDev(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	failedDevices := make([]string, 0)
	failedCurr := make([]string, 0)
	failedCap := make([]string, 0)
	detailLines := make([]string, 0)
	suggestionLines := make([]string, 0)

	for _, hwInfo := range hws {
		if len(hwInfo.PCIETreeLinks) == 0 {
			// Direct-to-CPU or sysfs unavailable; treat as normal.
			continue
		}

		// Compute the path-level capability: the minimum parseable cap across
		// all links.  A link whose max is unparseable is excluded from the
		// path cap calculation (treated as ∞ for this purpose).  If no link
		// yields a parseable cap, skip the whole NIC.
		pathCap := ""
		for _, link := range hwInfo.PCIETreeLinks {
			cap := minNumericSpeed(link.ParentMaxSpeed, link.ChildMaxSpeed)
			if cap == "" {
				continue
			}
			if pathCap == "" {
				pathCap = cap
			} else {
				pathCap = minNumericSpeed(pathCap, cap)
			}
		}
		if pathCap == "" {
			// No parseable caps on any link; skip silently.
			continue
		}

		// Flag each link whose current speed falls below the path cap — these
		// are the true bottlenecks.  A link running at the path cap (because
		// its own max matches the path cap) is expected and not flagged.
		for _, link := range hwInfo.PCIETreeLinks {
			if !pcieSpeedLessThan(link.CurSpeed, pathCap) {
				continue
			}
			result.Status = consts.StatusAbnormal
			devInfo := fmt.Sprintf("%s(%s, bottleneck@%s->%s)",
				hwInfo.IBDev, hwInfo.PCIEBDF, link.ParentBDF, link.ChildBDF)
			failedDevices = append(failedDevices, devInfo)
			failedCurr = append(failedCurr, link.CurSpeed)
			failedCap = append(failedCap, pathCap)
			detailLines = append(detailLines, fmt.Sprintf(
				"%s upstream link %s->%s current %s < cap %s",
				hwInfo.IBDev, link.ParentBDF, link.ChildBDF, link.CurSpeed, pathCap))
			suggestionLines = append(suggestionLines, fmt.Sprintf(
				"Check upstream PCIe link %s->%s for %s, current %s is below link capability %s (min of both endpoints' max).",
				link.ParentBDF, link.ChildBDF, hwInfo.IBDev, link.CurSpeed, pathCap))
		}
	}

	result.Curr = strings.Join(failedCurr, ",")
	result.Spec = strings.Join(failedCap, ",")
	result.Device = strings.Join(failedDevices, ",")
	if len(failedDevices) != 0 {
		result.Detail = strings.Join(detailLines, "\n")
		result.Suggestion = strings.Join(suggestionLines, "\n")
	}

	return &result, nil
}

// extractNumericSpeed extracts the numeric part from a PCIe speed string.
// e.g., "32.0 GT/s PCIe" -> "32.0", "16.0" -> "16.0"
func extractNumericSpeed(speed string) string {
	parts := strings.Fields(speed)
	if len(parts) == 0 {
		return speed
	}
	return parts[0]
}

// pcieSpeedLessThan returns true iff a < b after extracting the leading numeric
// part of each (so "16.0 GT/s PCIe" parses as 16.0). Returns false when either
// value cannot be parsed — callers must treat "unknown" as "not less" so the
// checker stays normal on unreadable sysfs entries.
func pcieSpeedLessThan(a, b string) bool {
	af, errA := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(a)), 64)
	bf, errB := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(b)), 64)
	if errA != nil || errB != nil {
		return false
	}
	return af < bf-1e-9
}

// minNumericSpeed returns whichever of a or b parses to the smaller numeric
// value, preserving the raw string form. If either side is unparseable
// returns "" so the checker can skip the link rather than emit a noisy
// "unknown" comparison.
func minNumericSpeed(a, b string) string {
	af, errA := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(a)), 64)
	bf, errB := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(b)), 64)
	if errA != nil || errB != nil {
		return ""
	}
	if af <= bf {
		return a
	}
	return b
}
