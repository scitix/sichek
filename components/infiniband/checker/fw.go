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
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type IBFirmwareChecker struct {
	id          string
	name        string
	spec        infiniband.InfinibandSpecItem
	description string
}

func NewFirmwareChecker(specCfg *infiniband.InfinibandSpecItem) (common.Checker, error) {
	return &IBFirmwareChecker{
		id:          consts.CheckerIDInfinibandFW,
		name:        infiniband.ChekIBFW,
		spec:        *specCfg,
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

	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := infiniband.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = infiniband.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	var detail []string
	failedHcas := make([]string, 0)
	var spec []string
	var curr []string
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.FWVer)
		curr = append(curr, hwInfo.FWVer)
		pass := common.CompareVersion(hcaSpec.FWVer, hwInfo.FWVer)
		if !pass {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			errMsg := fmt.Sprintf("fw check fail: hca:%s psid:%s curr:%s, spec:%v", hwInfo.IBDev, hwInfo.BoardID, hwInfo.FWVer, hcaSpec.FWVer)
			logrus.WithField("component", "infiniband").Warnf("%s", errMsg)
			detail = append(detail, errMsg)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	result.Detail = strings.Join(detail, ",")

	return &result, nil
}
