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

// GpuRecoveryActionChecker flags GPUs the driver has marked as needing a
// recovery action (Reset/Reboot/Drain...). This is a snapshot check: unlike the
// XidEventPoller — which only fires while the daemon is online at the instant a
// critical Xid occurs — it reports the persistent "reset required" state on
// every poll, so it catches a GPU that faulted (e.g. Xid 95 uncontained ECC)
// while sichek was offline and is still stuck awaiting a reset.
type GpuRecoveryActionChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewGpuRecoveryActionChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &GpuRecoveryActionChecker{
		name: config.GpuRecoveryActionCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *GpuRecoveryActionChecker) Name() string {
	return c.name
}

func (c *GpuRecoveryActionChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.GpuRecoveryActionCheckerName]

	var failedGpus []string
	var failedGpuidPodnames []string
	for _, device := range nvidiaInfo.DevicesInfo {
		if collector.NeedsRecoveryAction(device.RecoveryAction) {
			devicePodName := fmt.Sprintf("%d", device.Index)
			failedGpuidPodnames = append(failedGpuidPodnames, devicePodName)
			failedGpus = append(failedGpus, fmt.Sprintf("%d:%s(action=%s)", device.Index, device.UUID, device.RecoveryAction))
		}
	}
	if len(failedGpuidPodnames) > 0 {
		logrus.WithFields(logrus.Fields{
			"checker":     c.Name(),
			"failed_gpus": failedGpus,
		}).Errorf("GPU recovery action required")
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("GPU(s) flagged by driver as needing a recovery action: %v", failedGpus)
		result.Device = strings.Join(failedGpuidPodnames, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
	}
	return &result, nil
}
