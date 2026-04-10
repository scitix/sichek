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
	"github.com/sirupsen/logrus"
)

type RemmapedRowsPendingChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewRemmapedRowsPendingChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &RemmapedRowsPendingChecker{
		name: config.RemmapedRowsPendingCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *RemmapedRowsPendingChecker) Name() string {
	return c.name
}

func (c *RemmapedRowsPendingChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	// Perform type assertion to convert data to NvidiaInfo
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.RemmapedRowsPendingCheckerName]

	var failedGpus []string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if device.MemoryErrors.RemappedRows.RemappingPending {
			devicePodName := fmt.Sprintf("%d", device.Index)
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			failedGpus = append(failedGpus, fmt.Sprintf("%d:%s", device.Index, device.UUID))
		}
	}
	if len(failedGpuidPodnames) > 0 {
		logrus.WithFields(logrus.Fields{
			"checker": c.Name(),
			"failed_gpus": failedGpus,
		}).Errorf("Remapped Rows Pending detected on GPU(s)")
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Remapped Rows Pending detected on GPU(s): %v", failedGpus)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
	}
	return &result, nil
}
