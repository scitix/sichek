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

type NetOperstateChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpecItem
	description string
}

func NewNetOperstateChecker(specCfg *config.InfinibandSpecItem) (common.Checker, error) {
	return &NetOperstateChecker{
		id:   consts.CheckerNetOperstate,
		name: config.CheckNetOperstate,
		spec: specCfg,
	}, nil
}

func (c *NetOperstateChecker) Name() string {
	return c.name
}

func (c *NetOperstateChecker) Description() string {
	return c.description
}

func (c *NetOperstateChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *NetOperstateChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedHcas := make([]string, 0)
	spec := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.NetOperstate)
		curr = append(curr, hwInfo.NetOperstate)
		if hwInfo.NetOperstate != hcaSpec.NetOperstate {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")

	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("NetOperstate check fail: %s expected state = %s, current state = %s", strings.Join(failedHcas, ","), result.Spec, result.Curr)
		result.Suggestion = fmt.Sprintf("check opensm to active %s status", strings.Join(failedHcas, ","))
	}

	return &result, nil
}
