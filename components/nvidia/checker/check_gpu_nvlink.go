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
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

var NOTSUPPORT = "Not Supported"

type NvlinkChecker struct {
	name string
	cfg  *config.NvidiaSpecItem
}

func NewNvlinkChecker(cfg *config.NvidiaSpecItem) (common.Checker, error) {
	return &NvlinkChecker{
		name: config.NvlinkCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *NvlinkChecker) Name() string {
	return c.name
}

func (c *NvlinkChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.NvlinkCheckerName]

	if !c.cfg.Nvlink.NVlinkSupported {
		result.Status = consts.StatusNormal
		result.Curr = NOTSUPPORT
		result.Detail = "Nvlink Not supported"
		result.Suggestion = ""
		return &result, nil
	}
	// Check if all the Nvidia GPUs Nvlink are active
	var failedGpuidPodnames []string
	var failedReason []string
	for _, device := range nvidiaInfo.DevicesInfo {
		var devicePodName string
		if _, found := nvidiaInfo.DeviceToPodMap[device.UUID]; found {
			devicePodName = fmt.Sprintf("%s:%s", device.UUID, nvidiaInfo.DeviceToPodMap[device.UUID])
		} else {
			devicePodName = fmt.Sprintf("%s:", device.UUID)
		}
		if device.NVLinkStates.NVlinkSupported != c.cfg.Nvlink.NVlinkSupported {
			failedReason = append(failedReason, fmt.Sprintf("GPU %d: NVlinkSupported is `%t`, while expected `%t`\n",
				device.Index, device.NVLinkStates.NVlinkSupported, c.cfg.Nvlink.NVlinkSupported))
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			continue
		}
		if device.NVLinkStates.NvlinkNum != c.cfg.Nvlink.NvlinkNum {
			failedReason = append(failedReason, fmt.Sprintf("GPU %d: NVlinkNum is `%d`, while expected `%d`\n",
				device.Index, device.NVLinkStates.NvlinkNum, c.cfg.Nvlink.NvlinkNum))
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			continue
		}
		if !device.NVLinkStates.AllFeatureEnabled {
			failedReason = append(failedReason, fmt.Sprintf("GPU %d: Not All NVlink Features Are Enabled\n", device.Index))
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			continue
		}

	}
	if len(failedGpuidPodnames) > 0 {
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(failedGpuidPodnames, ",")
		result.Detail = strings.Join(failedReason, "")
		if c.cfg.Nvlink.NVlinkSupported {
			result.Curr = "Error Detected"
		} else {
			result.Curr = NOTSUPPORT
		}
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		if c.cfg.Nvlink.NVlinkSupported {
			result.Detail = "All GPUs Nvlink are active"
			result.Curr = "Active"
		} else {
			result.Curr = NOTSUPPORT
			result.Detail = "Nvlink Not supported"
		}
	}
	return &result, nil
}
