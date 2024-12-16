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
	"os"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"
)

type IBPCIEWidthChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewIBPCIEWidthChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &IBPCIEWidthChecker{
		id:   commonCfg.CheckerIDInfinibandFW,
		name: config.CheckPCIEWidth,
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

	var (
		errDevice         []string
		spec, suggestions string
		level             string = commonCfg.LevelInfo
		detail            string = config.InfinibandCheckItems[c.name].Detail
	)
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))

	status := commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	for _, hwSpec := range c.spec.HWSpec {
		for _, width := range hwSpec.Specifications.PcieWidth {
			if strings.Contains(hostname, width.NodeName) {
				spec = width.PCIEWidth
				break
			}
		}
	}

	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		curr = append(curr, hwInfo.PCIEWidth)
		if !strings.Contains(hwInfo.PCIEWidth, spec) {
			errDevice = append(errDevice, hwInfo.IBDev)
		}
	}

	if len(errDevice) != 0 {
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("%s is not right pcie width,curr:%s, spec:%s", strings.Join(errDevice, ","), strings.Join(curr, ","), spec)
		suggestions = fmt.Sprintf("set the %s to right pcie width", strings.Join(errDevice, ","))
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = strings.Join(curr, ",")
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
