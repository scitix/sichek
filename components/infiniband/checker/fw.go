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

type IBFirmwareChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewFirmwareChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &IBFirmwareChecker{
		id:          commonCfg.CheckerIDInfinibandFW,
		name:        config.ChekIBFW,
		spec:        specCfg,
		description: "check the nic fw",
	}, nil
}

func (c *IBFirmwareChecker) Name() string {
	return c.name
}

func (c *IBFirmwareChecker) Description() string {
	return c.description
}

func (c *IBFirmwareChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBFirmwareChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	var (
		spec, suggestions string
		errDev            []string
		allDevErr         []struct {
			device  string
			devType string
		}
		level  string = commonCfg.LevelInfo
		detail string = config.InfinibandCheckItems[c.name].Detail
	)

	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))

	status := commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Suggestion = config.InfinibandCheckItems[c.name].Suggestion
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaType := hwInfo.HCAType
		fw := hwInfo.FWVer
		curr = append(curr, fw)
		found := false
		for _, hwSpec := range c.spec.HWSpec {
			if hcaType != hwSpec.Type {
				continue
			}
			spec = hwSpec.Specifications.FwVersion
			if strings.Contains(spec, fw) {
				found = true
				break
			}
		}
		if !found {
			allDevErr = append(allDevErr, struct {
				device  string
				devType string
			}{
				device:  hwInfo.IBDev,
				devType: hcaType,
			})
		}
	}

	if len(allDevErr) != 0 {
		for _, dev := range allDevErr {
			errDev = append(errDev, dev.device)
			for _, hwSpec := range c.spec.HWSpec {
				if dev.devType != hwSpec.Type {
					continue
				}
				spec = hwSpec.Specifications.FwVersion
				break
			}
		}
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("%s fw is not in the spec, curr:%s, spec:%s", strings.Join(errDev, ","), strings.Join(curr, ","), spec)
		suggestions = fmt.Sprintf("use flint tool to burn %s fw ", strings.Join(errDev, ","))
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = strings.Join(curr, ",")
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Device = strings.Join(errDev, ",")
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
