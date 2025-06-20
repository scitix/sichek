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
	"github.com/scitix/sichek/pkg/utils"
)

type IBKmodChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBKmodChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBKmodChecker{
		id:   consts.CheckerIDInfinibandFW,
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
	)

	detail := config.InfinibandCheckItems[c.name].Detail
	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	spec = strings.Join(c.spec.IBSoftWareInfo.KernelModule, ",")
	if utils.IsNvidiaGPUExist() {
		spec += ",nvidia_peermem"
	}
	curr = strings.Join(infinibandInfo.IBSoftWareInfo.KernelModule, ",")

	for _, dependency := range c.spec.IBSoftWareInfo.KernelModule {
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
		result.Status = consts.StatusAbnormal
		detail = fmt.Sprintf("need to install kmod:%s", strings.Join(notInstalled, ","))
		suggestions = fmt.Sprintf("use modprobe to install kmod:%s", strings.Join(notInstalled, ","))
	}

	result.Curr = curr
	result.Spec = spec
	result.Detail = detail
	result.Suggestion = suggestions

	return &result, nil
}
