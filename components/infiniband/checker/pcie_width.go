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
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/consts"
)

type IBPCIEWidthChecker struct {
	id          string
	name        string
	spec        *infiniband.InfinibandSpecItem
	description string
}

func NewIBPCIEWidthChecker(specCfg *infiniband.InfinibandSpecItem) (common.Checker, error) {
	return &IBPCIEWidthChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: infiniband.CheckPCIEWidth,
		spec: specCfg,
	}, nil
}

func (c *IBPCIEWidthChecker) Name() string {
	return c.name
}

func (c *IBPCIEWidthChecker) Description() string {
	return c.description
}

func (c *IBPCIEWidthChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIEWidthChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := infiniband.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = infiniband.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedHcas := make([]string, 0)
	spec := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	var faiedHcasSpec []string
	var faiedHcasCurr []string
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.PCIEWidth)
		curr = append(curr, hwInfo.PCIEWidth)
		if hwInfo.PCIEWidth != hcaSpec.PCIEWidth {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			faiedHcasSpec = append(faiedHcasSpec, hcaSpec.PCIEWidth)
			faiedHcasCurr = append(faiedHcasCurr, hwInfo.PCIEWidth)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PCIEWidth check fail: %s expect %s, but get %s", strings.Join(failedHcas, ","), faiedHcasSpec, faiedHcasCurr)
		result.Suggestion = fmt.Sprintf("Set %s with %s", strings.Join(failedHcas, ","), strings.Join(faiedHcasSpec, ","))
	}

	return &result, nil
}
