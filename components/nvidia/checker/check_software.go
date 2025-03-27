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
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
)

type SoftwareChecker struct {
	name string
	cfg  *nvidia.NvidiaSpecItem
}

func NewSoftwareChecker(cfg *nvidia.NvidiaSpecItem) (common.Checker, error) {
	return &SoftwareChecker{
		name: nvidia.SoftwareCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *SoftwareChecker) Name() string {
	return c.name
}

// func (c *SoftwareChecker) GetSpec() common.CheckerSpec {
// 	return c.cfg
// }

func (c *SoftwareChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := nvidia.GPUCheckItems[nvidia.SoftwareCheckerName]

	info := ""

	if nvidiaInfo.SoftwareInfo.DriverVersion != c.cfg.Software.DriverVersion {
		info += fmt.Sprintf("Driver version is %s, expected version is %s\n", nvidiaInfo.SoftwareInfo.DriverVersion, c.cfg.Software.DriverVersion)
		result.Status = consts.StatusAbnormal
	}
	if nvidiaInfo.SoftwareInfo.CUDAVersion != c.cfg.Software.CUDAVersion {
		info += fmt.Sprintf("CUDA version is %s, expected version is %s\n", nvidiaInfo.SoftwareInfo.CUDAVersion, c.cfg.Software.CUDAVersion)
		result.Status = consts.StatusAbnormal
	}
	// if nvidiaInfo.SoftwareInfo.VBIOSVersion != c.cfg.Software.VBIOSVersion {
	// 	info += fmt.Sprintf("Driver version is %s, expected version is %s\n", nvidiaInfo.SoftwareInfo.VBIOSVersion, c.cfg.Software.VBIOSVersion)
	// 	result.Status = commonCfg.StatusAbnormal
	// }
	// if nvidiaInfo.SoftwareInfo.FabricManagerVersion != c.cfg.Software.FabricManagerVersion {
	// 	info += fmt.Sprintf("Driver version is %s, expected version is %s\n", nvidiaInfo.SoftwareInfo.FabricManagerVersion, c.cfg.Software.FabricManagerVersion)
	// 	result.Status = commonCfg.StatusAbnormal
	// }
	if result.Status == consts.StatusAbnormal {
		result.Detail = info
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		result.ErrorName = ""
	}
	return &result, nil
}
