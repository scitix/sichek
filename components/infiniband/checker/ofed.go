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
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type IBOFEDChecker struct {
	id          string
	name        string
	spec        infiniband.InfinibandSpecItem
	description string
}

func NewIBOFEDChecker(specCfg *infiniband.InfinibandSpecItem) (common.Checker, error) {
	return &IBOFEDChecker{
		id:          consts.CheckerIDInfinibandOFED,
		name:        infiniband.ChekIBOFED,
		spec:        *specCfg,
		description: "check the rdma ofed",
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

	result := infiniband.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = infiniband.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	curr := infinibandInfo.IBSoftWareInfo.OFEDVer
	spec := c.spec.IBSoftWareInfo.OFEDVer
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hca := c.spec.HCAs[hwInfo.BoardID]
		if hca.OFEDVer != "" {
			spec = hca.OFEDVer
			logrus.Warnf("use the IB device's OFED spec to check the system OFED version")
		}
	}
	if curr != spec {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("OFED version mismatch, expected:%s  current:%s", spec, curr)
		result.Suggestion = "update the OFED version"
	}

	result.Curr = curr
	result.Spec = spec

	return &result, nil
}
