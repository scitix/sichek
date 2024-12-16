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
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	commonCfg "github.com/scitix/sichek/config"
)

type EthPhyStateChecker struct {
	id          string
	name        string
	spec        *config.EthernetSpec
	description string
}

func NewEthPhyStateChecker(specCfg *config.EthernetSpec) (common.Checker, error) {
	return &EthPhyStateChecker{
		id:   commonCfg.CheckerIDEthPhyState,
		name: config.ChekEthPhyState,
		spec: specCfg,
	}, nil
}

func (c *EthPhyStateChecker) Name() string {
	return c.name
}

func (c *EthPhyStateChecker) Description() string {
	return c.description
}

func (c *EthPhyStateChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *EthPhyStateChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	ethernetInfo, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid ethernet type")
	}

	if len(ethernetInfo.EthDevs) == 0 {
		result := config.EthCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.EthCheckItems[c.name].Level
		result.Detail = "No eth device found"
		return &result, fmt.Errorf("fail to get the eth device")
	}

	var (
		errDevice []string
		spec      string
		level     string = commonCfg.LevelInfo
		detail    string = config.EthCheckItems[c.name].Detail
	)

	status := commonCfg.StatusNormal
	suggestions := " "

	curr := make([]string, 0, len(ethernetInfo.EthHardWareInfo))
	for _, stat := range ethernetInfo.EthHardWareInfo {
		curr = append(curr, stat.PhyStat)
	}

	for _, hwInfo := range ethernetInfo.EthHardWareInfo {
		state := hwInfo.PhyStat
		curr = append(curr, state)
		for _, hwSpec := range c.spec.HWSpec {
			spec = hwSpec.Specifications.PhyState
			if strings.Contains(state, spec) {
				continue
			}
			if len(hwInfo.EthDev) != 0 {
				errDevice = append(errDevice, hwInfo.EthDev)
			}
		}
	}

	if len(errDevice) != 0 {
		status = commonCfg.StatusAbnormal
		level = config.EthCheckItems[c.name].Level
		detail = fmt.Sprintf("%s status is not right status", strings.Join(errDevice, ","))
		suggestions = fmt.Sprintf("check nic to up %s link status", strings.Join(errDevice, ","))
	}

	result := config.EthCheckItems[c.name]
	result.Curr = strings.Join(curr, ",")
	result.Spec = spec
	result.Level = level
	result.Status = status
	result.Device = strings.Join(errDevice, ",")
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
