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
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils"
)

type PCIEACSChecker struct {
	id          string
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewPCIEACSChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
	return &PCIEACSChecker{
		id:   commonCfg.CheckerIDInfinibandFW,
		name: config.CheckPCIEACS,
		spec: specCfg,
	}, nil
}

func (c *PCIEACSChecker) Name() string {
	return c.name
}

func (c *PCIEACSChecker) Description() string {
	return c.description
}

func (c *PCIEACSChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *PCIEACSChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	var (
		errBDFs  = make(map[string]string, 0)
		failBDFs []string
	)

	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Spec = c.spec.SoftwareDependencies.PcieACS

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Level = config.InfinibandCheckItems[c.name].Level
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	notDisabledACS, err := utils.IsAllACSDisabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run IsAllACSDisabled, err: %v", err)
	}

	if len(notDisabledACS) > 0 {
		disabledACSFail, err := utils.DisableAllACS(ctx)
		if err != nil {
			result.Status = commonCfg.StatusAbnormal
			result.Curr = "NotDisabled"
			result.Level = config.InfinibandCheckItems[c.name].Level
			result.Detail = fmt.Sprintf("Not All PCIe ACS are disabled: %v", err)
			result.Suggestion = `use shell cmd "for i in $(lspci | cut -f 1 -d ' '); do setpci -v -s $i ecap_acs+6.w=0; done" disable acs`
		} else {
			for _, setFail := range disabledACSFail {
				errBDFs[setFail.BDF] = setFail.ACSStatus
				failBDFs = append(failBDFs, setFail.BDF)
			}
			result.Status = commonCfg.StatusNormal
			result.Curr = "DisabledOnline"
			result.Level = commonCfg.LevelInfo
			result.Device = strings.Join(failBDFs, ",")
			result.Suggestion = ""
			result.Detail = fmt.Sprintf("need to disable acs, curr:%s", errBDFs)
		}

	} else {
		result.Status = commonCfg.StatusNormal
		result.Curr = "Disabled"
		result.Level = commonCfg.LevelInfo
		result.Suggestion = ""
		result.Detail = "All PCIe ACS are disabled"
	}

	return &result, nil
}
