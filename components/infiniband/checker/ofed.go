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

type IBOFEDChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewIBOFEDChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &IBOFEDChecker{
		id:   commonCfg.CheckerIDInfinibandOFED,
		name: config.ChekIBOFED,
		spec: specCfg,
	}, nil
}

func (c *IBOFEDChecker) Name() string {
	return c.name
}

func (c *IBOFEDChecker) Description() string {
	return c.description
}

func (c *IBOFEDChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBOFEDChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	var (
		curr, spec, suggestion string
		level                  = commonCfg.LevelInfo
		detail                 = config.InfinibandCheckItems[c.name].Detail
	)

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	curr = infinibandInfo.IBSoftWareInfo.OFEDVer
	status := commonCfg.StatusNormal

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	hostFound := false
	for _, host := range c.spec.SoftwareDependencies.OFED {
		if strings.Contains(hostname, host.NodeName) {
			spec = host.OFEDVer
			hostFound = true
			break
		}
	}
	if !hostFound {
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("host name does not match any spec, curr:%s", hostname)
		suggestion = "check the default configuration"
	} else if !strings.Contains(curr, spec) { // 如果主机名匹配，但 OFED 版本不匹配
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("OFED version mismatch, expected:%s, current:%s", spec, curr)
		suggestion = "update the OFED version"
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = curr
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Detail = detail
	result.Suggestion = suggestion

	return &result, nil
}
