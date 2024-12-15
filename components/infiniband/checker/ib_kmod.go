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

type IBKmodChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewIBKmodChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &IBKmodChecker{
		id:   commonCfg.CheckerIDInfinibandFW,
		name: config.CheckIBKmod,
		spec: specCfg,
	}, nil
}

func (c *IBKmodChecker) Name() string {
	return c.name
}

func (c *IBKmodChecker) Description() string {
	return c.description
}

func (c *IBKmodChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBKmodChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	var (
		spec, curr, suggestions string
		notInstalled            []string
		level                   string = commonCfg.LevelInfo
		detail                  string = config.InfinibandCheckItems[c.name].Detail
	)

	status := commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Detail = config.NOIBFOUND
		result.Suggestion = config.InfinibandCheckItems[c.name].Suggestion
		return &result, fmt.Errorf("fail to get the IB device")
	}

	spec = strings.Join(c.spec.SoftwareDependencies.KernelModule, ",")
	curr = strings.Join(infinibandInfo.IBSoftWareInfo.KernelModule, ",")

	for _, dependency := range c.spec.SoftwareDependencies.KernelModule {
		found := false
		for _, module := range infinibandInfo.IBSoftWareInfo.KernelModule {
			if strings.Contains(dependency, module) {
				found = true
				break
			}
		}
		if !found {
			notInstalled = append(notInstalled, dependency)
		}
	}

	if len(notInstalled) != 0 {
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("need to install kmod:%s", strings.Join(notInstalled, ","))
		suggestions = fmt.Sprintf("use modprobe to install kmod:%s", strings.Join(notInstalled, ","))
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = curr
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
