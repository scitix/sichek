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
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuevents/collector"
	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type SmClkStuckLowChecker struct {
	name string
	cfg  *config.GpuCostomEventsUserConfig
	spec *config.GpuEventRule

	indicatorStates map[string]*IndicatorStates
	LastUpdate      time.Time // Timestamp of the last update

}

func NewSmClkStuckLowChecker(cfg *config.GpuCostomEventsUserConfig, spec *config.GpuEventRule) common.Checker {
	return &SmClkStuckLowChecker{
		name:            config.SmClkStuckLowCheckerName,
		cfg:             cfg,
		spec:            spec,
		indicatorStates: make(map[string]*IndicatorStates),
		LastUpdate:      time.Now(),
	}
}

func (c *SmClkStuckLowChecker) Name() string {
	return c.name
}

func (c *SmClkStuckLowChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.DeviceIndicatorValues)
	if !ok {
		return nil, fmt.Errorf("wrong input of SmClkStuckLowChecker")
	}
	c.OnData(info)
	var raw string
	abnormalIndicatorNum := make(map[string]int64)
	var SmClkLowThreshold int64
	for uuid, devIndicatorStates := range c.indicatorStates {
		for indicatorName, indicator := range devIndicatorStates.Indicators {
			SmClkLowThreshold = c.spec.Indicators[indicatorName].Threshold
			// fmt.Printf("device=%s, indicatorName=%s, value=%d, spec=%ser-than-%d, hang_duration=%s, duration_threshold=%s\n",
			// 	uuid, indicatorName, indicator.Value, c.spec.Indicators[indicatorName].CompareType, c.spec.Indicators[indicatorName].Threshold, indicator.Duration, c.spec.DurationThreshold)
			if indicator.Duration >= c.spec.DurationThreshold.Duration {
				raw = fmt.Sprintf("%sdevice=%s, indicatorName=%s, value=%d, spec=%ser-than-%d, hang_duration=%s, duration_threshold=%s\n",
					raw, uuid, indicatorName, indicator.Value, c.spec.Indicators[indicatorName].CompareType, c.spec.Indicators[indicatorName].Threshold, indicator.Duration, c.spec.DurationThreshold)
				abnormalIndicatorNum[uuid]++
			}
		}
	}
	status := consts.StatusNormal
	var gpuAbNum = 0
	devices := make([]string, 0)
	result := &common.CheckerResult{
		Name:        c.spec.Name,
		Description: c.spec.Description,
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       c.spec.Level,
		Detail:      "",
		ErrorName:   c.spec.Name,
		Suggestion:  "",
	}

	// Check if all abnormal indicators exceeds their's threshold
	for uuid, num := range abnormalIndicatorNum {
		if num == int64(len(c.spec.Indicators)) {
			gpuAbNum++
			status = consts.StatusAbnormal
			devices = append(devices, uuid)
		}
	}

	result.Device = strings.Join(devices, ",")
	result.Curr = strconv.Itoa(gpuAbNum)
	result.Status = status
	result.Detail = raw
	return result, nil
}

func (c *SmClkStuckLowChecker) OnData(IndicatorSnapshot *collector.DeviceIndicatorValues) {
	for gpuId, curIndicatorValues := range IndicatorSnapshot.Indicators {
		if _, ok := c.indicatorStates[gpuId]; !ok {
			// Initialize the state of device if it doesn't exist
			c.indicatorStates[gpuId] = &IndicatorStates{
				Indicators: make(map[string]*IndicatorState),
				LastUpdate: time.Time{},
			}
		}
		IndicatorStates := c.indicatorStates[gpuId].Indicators

		for indicatorName := range c.spec.Indicators {
			if indicatorName != "smclk" && indicatorName != "gpuidle" {
				logrus.WithField("checker", "gpuevents").Warnf("Unexpected indicator %s for smclk stuck low checker", indicatorName)
				continue
			}

			if _, ok := IndicatorStates[indicatorName]; !ok {
				// Initialize the state of indicator if it doesn't exist
				IndicatorStates[indicatorName] = &IndicatorState{
					Active:   false,
					Value:    0,
					Duration: 0,
				}
			}
			infoValue := curIndicatorValues.Indicators[indicatorName]
			duration := GetIndicatorDuration(indicatorName, infoValue, c.spec, curIndicatorValues.LastUpdate, c.LastUpdate)
			if duration == 0 {
				IndicatorStates[indicatorName] = &IndicatorState{
					Active:   false,
					Value:    infoValue,
					Duration: 0,
				}
			} else {
				IndicatorStates[indicatorName].Active = true
				IndicatorStates[indicatorName].Value = infoValue
				IndicatorStates[indicatorName].Duration += time.Duration(duration) * time.Second
			}
		}
	}

	c.LastUpdate = IndicatorSnapshot.LastUpdate
}
