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
	commonCfg "github.com/scitix/sichek/config"
	"github.com/sirupsen/logrus"
)

type IBOFEDChecker struct {
	id          string
	name        string
	spec        config.InfinibandSpec
	description string
}

func NewIBOFEDChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBOFEDChecker{
		id:          commonCfg.CheckerIDInfinibandOFED,
		name:        config.ChekIBOFED,
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

	result := config.InfinibandCheckItems[c.name]
	result.Status = commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
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
		result.Status = commonCfg.StatusAbnormal
		result.Detail = fmt.Sprintf("OFED version mismatch, expected:%s  current:%s", spec, curr)
		result.Suggestion = "update the OFED version"
	}

	result.Curr = curr
	result.Spec = spec
	logrus.WithField("component", "infiniband").Infof("get the ofed check: %v", result)

	return &result, nil
}
