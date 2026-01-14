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
	"strconv"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type IBLostChecker struct {
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBLostChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBLostChecker{
		name: config.CheckIBLost,
		spec: specCfg,
	}, nil
}

func (c *IBLostChecker) Name() string {
	return c.name
}

func (c *IBLostChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	if len(infinibandInfo.IBPCIDevs) != infinibandInfo.HCAPCINum {
		result.Status = consts.StatusAbnormal
		result.Detail = "IBCapablePCIDevs: "
		for pciDev, _ := range infinibandInfo.IBPCIDevs {
			result.Detail += pciDev + ","
		}
		result.Detail += "\nHCAPCINum: " + strconv.Itoa(infinibandInfo.HCAPCINum)
		result.Detail += "\nIBPFDevs: "
		for ibDev, _ := range infinibandInfo.IBPFDevs {
			result.Detail += ibDev + ","
		}
	}
	infinibandInfo.RUnlock()
	return &result, nil
}
