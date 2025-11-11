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
	"github.com/sirupsen/logrus"
)

type IBPCIESpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPCIESpeedChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPCIESpeedChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIESpeed,
		spec: specCfg,
	}, nil
}

func (c *IBPCIESpeedChecker) Name() string {
	return c.name
}

func (c *IBPCIESpeedChecker) Description() string {
	return c.description
}

func (c *IBPCIESpeedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIESpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}
	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedHcas := make([]string, 0)
	spec := make([]string, 0, hwInfoLen)
	curr := make([]string, 0, hwInfoLen)
	var failedHcasSpec []string
	var failedHcasCurr []string
	var devicesToUpdate []string
	// 先加读锁读取所有数据
	infinibandInfo.RLock()
	for dev, hwInfo := range infinibandInfo.IBHardWareInfo {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			logrus.WithField("checker", c.name).Debugf("HCA %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.Hardware.PCIESpeed)
		curr = append(curr, hwInfo.PCIESpeed)
		if !strings.Contains(hwInfo.PCIESpeed, hcaSpec.Hardware.PCIESpeed) {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			failedHcasSpec = append(failedHcasSpec, hcaSpec.Hardware.PCIESpeed)
			failedHcasCurr = append(failedHcasCurr, hwInfo.PCIESpeed)
			devicesToUpdate = append(devicesToUpdate, dev)
		}
	}
	infinibandInfo.RUnlock()
	// 批量更新状态（使用写锁）
	if len(devicesToUpdate) > 0 {
		infinibandInfo.Lock()
		for _, dev := range devicesToUpdate {
			tmp := infinibandInfo.IBHardWareInfo[dev]
			tmp.PCIESpeedState = "1"
			infinibandInfo.IBHardWareInfo[dev] = tmp
		}
		infinibandInfo.Unlock()
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PCIESpeed check fail: %s expect %s, but get %s", strings.Join(failedHcas, ","), failedHcasSpec, failedHcasCurr)
		result.Suggestion = fmt.Sprintf("Set %s with PCIe MaxReadReq %s", strings.Join(failedHcas, ","), strings.Join(failedHcasSpec, ","))
	}

	return &result, nil
}
