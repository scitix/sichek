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
	spec        *config.InfinibandHCASpec
	description string
}

func NewIBPhyStateChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
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

	var (
		spec, suggestions string
		errDevice         []string
		level             string = commonCfg.LevelInfo
		detail            string = config.InfinibandCheckItems[c.name].Detail
	)

	status := commonCfg.StatusNormal
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaType := hwInfo.HCAType
		state := hwInfo.PhyStat
		curr = append(curr, state)
		found := false
		for _, hwSpec := range c.spec.HWSpec {
			if hcaType != hwSpec.Type {
				continue
			}
			spec = hwSpec.Specifications.PhyState
			if strings.Contains(state, spec) {
				found = true
				break
			}
		}
		if !found {
			errDevice = append(errDevice, hwInfo.IBDev)
		}
	}

	if len(errDevice) != 0 {
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("%s status is not LinkUp, curr:%s", strings.Join(errDevice, ","), strings.Join(curr, ","))
		suggestions = fmt.Sprintf("check nic to up %s link status", strings.Join(errDevice, ","))
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = strings.Join(curr, ",")
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Device = strings.Join(errDevice, ",")
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
