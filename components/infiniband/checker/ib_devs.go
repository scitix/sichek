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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type IBDevsChecker struct {
	name        string
	spec        *config.InfinibandSpecItem
	description string
}

func NewIBDevsChecker(specCfg *config.InfinibandSpecItem) (common.Checker, error) {
	return &IBDevsChecker{
		name: config.CheckIBDevs,
		spec: specCfg,
	}, nil
}

func (c *IBDevsChecker) Name() string {
	return c.name
}

func (c *IBDevsChecker) Description() string {
	return c.description
}

func (c *IBDevsChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBDevsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]

	for mlxSpecDev := range c.spec.IBDevs {
		netDev, found := infinibandInfo.IBDevs[mlxSpecDev]
		if !found {
			result.Detail = fmt.Sprintf("IB device %s has unexpected value %s, expected %s", mlxSpecDev, netDev, mlxSpecDev)
			return &result, nil
		} else {
			expectedNetDev := c.spec.IBDevs[mlxSpecDev]
			if netDev != expectedNetDev {
				result.Status = consts.StatusAbnormal
				result.Detail = fmt.Sprintf("IB Net device %s has unexpected value %s, expected %s", infinibandInfo.IBDevs[mlxSpecDev], netDev, expectedNetDev)
				return &result, nil
			}
		}
	}
	fmt.Println(result)
	result.Status = consts.StatusNormal
	result.Detail = fmt.Sprintf("IB device %s is normal", c.spec.IBDevs)
	return &result, nil
}
