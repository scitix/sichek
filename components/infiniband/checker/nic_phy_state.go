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
	commonCfg "github.com/scitix/sichek/config"
)

type IBPhyStateChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPhyStateChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPhyStateChecker{
		id:   commonCfg.CheckerIDInfinibandFW,
		name: config.ChekIBPhyState,
		spec: specCfg,
	}, nil
}

func (c *IBPhyStateChecker) Name() string {
	return c.name
}

func (c *IBPhyStateChecker) Description() string {
	return c.description
}

func (c *IBPhyStateChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPhyStateChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {

	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedHcas := make([]string, 0)
	spec := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.PhyState)
		curr = append(curr, hwInfo.PhyState)
		if !strings.Contains(hwInfo.PhyState, hcaSpec.PhyState) {
			result.Status = commonCfg.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")

	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PhyState check fail: %s NOT LinkUp",strings.Join(failedHcas, ";"))
		result.Suggestion = fmt.Sprintf("Check and Up %s pyhical state", strings.Join(failedHcas, ";"))
	}

	return &result, nil
}
