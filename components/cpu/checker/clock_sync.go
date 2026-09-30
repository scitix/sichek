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
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"
)

const ClockSyncServiceCheckerName = "clock-sync-service"

type ClockSyncServiceChecker struct {
	name string
}

func NewClockSyncServiceChecker() (common.Checker, error) {
	return &ClockSyncServiceChecker{
		name: ClockSyncServiceCheckerName,
	}, nil
}

func (c *ClockSyncServiceChecker) Name() string {
	return c.name
}

func (c *ClockSyncServiceChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *ClockSyncServiceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuOutput, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.CPUOutput")
	}

	result := config.CPUCheckItems[ClockSyncServiceCheckerName]

	if cpuOutput.PTPInfo.SyncAvailable {
		result.Status = consts.StatusNormal
		svc := "unknown"
		if cpuOutput.PTPInfo.PTPServiceActive {
			svc = "PTP (ptp4l)"
		} else if cpuOutput.PTPInfo.NTPServiceActive {
			svc = "NTP"
		}
		result.Curr = svc
		result.Detail = fmt.Sprintf("Clock sync service is active: %s", svc)
		result.Suggestion = ""
	} else {
		result.Status = consts.StatusAbnormal
		result.Curr = "NotRunning"
		result.Detail = "No clock synchronization service (PTP or NTP) is running"
	}

	return &result, nil
}

const ClockSyncOffsetCheckerName = "clock-sync-offset"

type ClockSyncOffsetChecker struct {
	name       string
	warningMs  float64
	criticalMs float64
}

func NewClockSyncOffsetChecker(warningMs, criticalMs float64) (common.Checker, error) {
	return &ClockSyncOffsetChecker{
		name:       ClockSyncOffsetCheckerName,
		warningMs:  warningMs,
		criticalMs: criticalMs,
	}, nil
}

func (c *ClockSyncOffsetChecker) Name() string {
	return c.name
}

func (c *ClockSyncOffsetChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *ClockSyncOffsetChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuOutput, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.CPUOutput")
	}

	result := config.CPUCheckItems[ClockSyncOffsetCheckerName]

	if !cpuOutput.PTPInfo.SyncAvailable {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = "Clock sync not available; offset check skipped"
		result.Suggestion = ""
		return &result, nil
	}

	offsetMs := cpuOutput.PTPInfo.OffsetMs()

	if offsetMs >= c.criticalMs {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.Curr = fmt.Sprintf("%.3fms", offsetMs)
		result.Detail = fmt.Sprintf("Clock offset %.3fms exceeds critical threshold %.1fms", offsetMs, c.criticalMs)
	} else if offsetMs >= c.warningMs {
		result.Status = consts.StatusAbnormal
		result.Curr = fmt.Sprintf("%.3fms", offsetMs)
		result.Detail = fmt.Sprintf("Clock offset %.3fms exceeds warning threshold %.1fms", offsetMs, c.warningMs)
	} else {
		result.Status = consts.StatusNormal
		result.Curr = fmt.Sprintf("%.3fms", offsetMs)
		result.Detail = fmt.Sprintf("Clock offset %.3fms is within acceptable range", offsetMs)
		result.Suggestion = ""
	}

	return &result, nil
}
