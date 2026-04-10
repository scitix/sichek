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

type IBPCIETreeSpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPCIETreeSpeedChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPCIETreeSpeedChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIETreeSpeed,
		spec: specCfg,
	}, nil
}

func (c *IBPCIETreeSpeedChecker) Name() string {
	return c.name
}

func (c *IBPCIETreeSpeedChecker) Description() string {
	return c.description
}

func (c *IBPCIETreeSpeedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIETreeSpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
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

	infinibandInfo.RLock()
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			logrus.WithField("component", "infiniband").Warnf("HCA %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		// Extract numeric speed from spec (e.g., "32.0 GT/s PCIe" -> "32.0")
		expectedSpeed := extractNumericSpeed(hcaSpec.Hardware.PCIESpeed)
		spec = append(spec, hcaSpec.Hardware.PCIESpeed)

		treeSpeedMin := hwInfo.PCIETreeSpeedMin
		if treeSpeedMin == "" {
			// No upstream tree info available (e.g., direct to CPU), skip
			curr = append(curr, hwInfo.PCIESpeed)
			continue
		}
		curr = append(curr, treeSpeedMin)

		if treeSpeedMin != expectedSpeed {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			failedHcasSpec = append(failedHcasSpec, hcaSpec.Hardware.PCIESpeed)
			failedHcasCurr = append(failedHcasCurr, treeSpeedMin)
		}
	}
	infinibandInfo.RUnlock()

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PCIETreeSpeed check fail: %s upstream path min speed %s, expect %s", strings.Join(failedHcas, ","), failedHcasCurr, failedHcasSpec)
		result.Suggestion = fmt.Sprintf("Check upstream PCIe switch/bridge speed for %s, expected %s but found %s in path to root complex", strings.Join(failedHcas, ","), strings.Join(failedHcasSpec, ","), strings.Join(failedHcasCurr, ","))
	}

	return &result, nil
}

// extractNumericSpeed extracts the numeric part from a PCIe speed string.
// e.g., "32.0 GT/s PCIe" -> "32.0", "16.0" -> "16.0"
func extractNumericSpeed(speed string) string {
	parts := strings.Fields(speed)
	if len(parts) == 0 {
		return speed
	}
	return parts[0]
}
