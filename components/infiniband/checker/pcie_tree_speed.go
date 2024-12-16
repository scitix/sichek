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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"
)

type IBPCIETreeSpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewBPCIETreeSpeedChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &IBPCIETreeSpeedChecker{
		id:          commonCfg.CheckerIDInfinibandFW,
		name:        config.CheckPCIETreeSpeed,
		spec:        specCfg,
		description: "check the nic pcie tree",
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
	var (
		errDevice         []string
		detailTmp         []string
		spec, suggestions string
		level             string = commonCfg.LevelInfo
		detail            string = config.InfinibandCheckItems[c.name].Detail
	)

	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	status := commonCfg.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result := config.InfinibandCheckItems[c.name]
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	for _, hwSpec := range c.spec.HWSpec {
		for _, pcieTree := range hwSpec.Specifications.PcieTreeSpeed {
			if strings.Contains(hostname, pcieTree.NodeName) {
				spec = pcieTree.PCIESpeed
				break
			}
		}
	}

	curr := make(map[string]string)
	errDeviceSet := make(map[string]struct{})

	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		for _, pcieTree := range hwInfo.PCIETreeSpeed {
			curr[pcieTree.BDF] = pcieTree.Speed

			if strings.Contains(pcieTree.Speed, spec) {
				continue
			}
			//FIXME
			if strings.Contains(pcieTree.Speed, "5.0") {
				continue
			}

			detailTmp = append(detailTmp, fmt.Sprintf("%s(%s : %s)", hwInfo.IBDev, pcieTree.BDF, pcieTree.Speed))

			if _, exists := errDeviceSet[hwInfo.IBDev]; !exists {
				errDevice = append(errDevice, hwInfo.IBDev)
				errDeviceSet[hwInfo.IBDev] = struct{}{}
			}
		}
	}

	jsonData, err := json.Marshal(curr)
	if err != nil {
		return nil, err
	}

	if len(errDevice) != 0 {
		status = commonCfg.StatusAbnormal
		level = config.InfinibandCheckItems[c.name].Level
		detail = fmt.Sprintf("%s is not right pcie speed,curr:%s, spec:%s", strings.Join(detailTmp, ","), string(jsonData), spec)
		suggestions = fmt.Sprintf("set the %s to write pcie speed", strings.Join(errDevice, ","))
	}

	result := config.InfinibandCheckItems[c.name]
	result.Curr = string(jsonData)
	result.Spec = spec
	result.Status = status
	result.Level = level
	result.Detail = detail
	result.Device = strings.Join(errDevice, ",")
	result.Suggestion = suggestions

	return &result, nil
}
