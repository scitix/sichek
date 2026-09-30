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
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type IBPCIETreeWidthChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPCIETreeWidthChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPCIETreeWidthChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIETreeWidth,
		spec: specCfg,
	}, nil
}

func (c *IBPCIETreeWidthChecker) Name() string {
	return c.name
}

func (c *IBPCIETreeWidthChecker) Description() string {
	return c.description
}

func (c *IBPCIETreeWidthChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIETreeWidthChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
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

		// Compute the path-level cap: minimum parseable cap across all links.
		// Links whose max is unparseable are excluded from the path cap (not
		// constraining the path).  If no link yields a parseable cap, skip
		// the whole NIC silently.
		pathCap := ""
		for _, link := range hwInfo.PCIETreeLinks {
			cap := minNumericSpeed(link.ParentMaxWidth, link.ChildMaxWidth)
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
			continue
		}

		// Flag each link whose current width falls below the path cap.
		for _, link := range hwInfo.PCIETreeLinks {
			if !pcieSpeedLessThan(link.CurWidth, pathCap) {
				continue
			}
			result.Status = consts.StatusAbnormal
			// No comma inside a single device string: the exporter splits the
			// joined Device field on "," to emit one series per failed device.
			devInfo := fmt.Sprintf("%s(%s bottleneck@%s->%s)",
				hwInfo.IBDev, hwInfo.PCIEBDF, link.ParentBDF, link.ChildBDF)
			failedDevices = append(failedDevices, devInfo)
			failedCurr = append(failedCurr, link.CurWidth)
			failedCap = append(failedCap, pathCap)
			detailLines = append(detailLines, fmt.Sprintf(
				"%s upstream link %s->%s current width x%s < cap x%s",
				hwInfo.IBDev, link.ParentBDF, link.ChildBDF, link.CurWidth, pathCap))
			suggestionLines = append(suggestionLines, fmt.Sprintf(
				"Check upstream PCIe link %s->%s for %s, current width x%s is below link capability x%s (min of both endpoints' max).",
				link.ParentBDF, link.ChildBDF, hwInfo.IBDev, link.CurWidth, pathCap))
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
