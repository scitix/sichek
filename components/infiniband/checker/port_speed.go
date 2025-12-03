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

type IBPortSpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPortSpeedChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPortSpeedChecker{
		id:   consts.CheckerIDInfinibandPortSpeed,
		name: config.CheckIBPortSpeed,
		spec: specCfg,
	}, nil
}

func (c *IBPortSpeedChecker) Name() string {
	return c.name
}

func (c *IBPortSpeedChecker) Description() string {
	return c.description
}

func (c *IBPortSpeedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPortSpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
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
	// First acquire read lock to read all data
	infinibandInfo.RLock()
	for dev, hwInfo := range infinibandInfo.IBHardWareInfo {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			logrus.WithField("component", "infiniband").Warnf("HCA spec for board ID %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.Hardware.PortSpeed)
		curr = append(curr, hwInfo.PortSpeed)
		if hwInfo.PortSpeed != hcaSpec.Hardware.PortSpeed {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			failedHcasSpec = append(failedHcasSpec, hcaSpec.Hardware.PortSpeed)
			failedHcasCurr = append(failedHcasCurr, hwInfo.PortSpeed)
			devicesToUpdate = append(devicesToUpdate, dev)
		}
	}
	infinibandInfo.RUnlock()
	// Batch update status (using write lock)
	if len(devicesToUpdate) > 0 {
		infinibandInfo.Lock()
		for _, dev := range devicesToUpdate {
			tmp := infinibandInfo.IBHardWareInfo[dev]
			tmp.PortSpeedState = "1"
			infinibandInfo.IBHardWareInfo[dev] = tmp
		}
		infinibandInfo.Unlock()
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PortSpeed check fail: %s expect %s, but get %s", strings.Join(failedHcas, ","), failedHcasSpec, failedHcasCurr)
		result.Suggestion = fmt.Sprintf("Set %s with %s", strings.Join(failedHcas, ","), strings.Join(failedHcasSpec, ","))
	}

	return &result, nil
}
