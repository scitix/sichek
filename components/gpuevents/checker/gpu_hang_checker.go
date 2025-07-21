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

	"github.com/scitix/sichek/pkg/k8s"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuevents/collector"
	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type GpuHangChecker struct {
	name string
	cfg  *config.GpuCostomEventsUserConfig
	spec *config.GpuEventRule

	indicatorStates map[string]*IndicatorStates
	LastUpdate      time.Time // Timestamp of the last update

	HighSampleRateStatus        bool
	originalQueryInterval       common.Duration
	originalNvidiaQueryInterval common.Duration
	abnormalDetectedTimes       uint32
	podResourceMapper           *k8s.PodResourceMapper
}

func NewGpuHangChecker(cfg *config.GpuCostomEventsUserConfig, spec *config.GpuEventRule) common.Checker {
	podResourceMapper := k8s.NewPodResourceMapper()
	return &GpuHangChecker{
		name:                        config.GPUHangCheckerName,
		cfg:                         cfg,
		spec:                        spec,
		indicatorStates:             make(map[string]*IndicatorStates),
		LastUpdate:                  time.Now(),
		HighSampleRateStatus:        false,
		originalQueryInterval:       cfg.UserConfig.QueryInterval,
		originalNvidiaQueryInterval: common.Duration{Duration: 30 * time.Second},
		abnormalDetectedTimes:       0,
		podResourceMapper:           podResourceMapper,
	}
}

func (c *GpuHangChecker) Name() string {
	return c.name
}

func (c *GpuHangChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.DeviceIndicatorValues)
	if !ok {
		return nil, fmt.Errorf("wrong input of HangChecker")
	}
	c.OnData(info)
	var raw string
	abnormalIndicatorNum := make(map[string]int64)
	for uuid, devIndicatorStates := range c.indicatorStates {
		for indicatorName, indicator := range devIndicatorStates.Indicators {
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
	var suggest string
	var gpuAbNum = 0
	devices := make([]string, 0)
	var deviceToPodMap map[string]*k8s.PodInfo
	var err error
	if len(abnormalIndicatorNum) > 0 {
		deviceToPodMap, err = c.podResourceMapper.GetDeviceToPodMap()
		if err != nil {
			return nil, err
		}
	}
	result := &common.CheckerResult{
		Name:        c.spec.Name,
		Description: c.spec.Description,
		Device:      "",
		Spec:        "",
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
			suggest = fmt.Sprintf("%ssuggest check gpu device=%s which probably hang\n", suggest, uuid)
			var devicePod string
			if _, found := deviceToPodMap[uuid]; found {
				devicePod = fmt.Sprintf("%s:%s", uuid, deviceToPodMap[uuid])
				nameSpace := deviceToPodMap[uuid].Namespace
				if _, exist := c.cfg.UserConfig.ProcessedIgnoreNamespace[nameSpace]; exist {
					result.Level = consts.LevelInfo
					logrus.WithField("component", "gpuevents").Warningf("device=%s probably hang in pod=%+v", uuid, deviceToPodMap[uuid])
				}
			} else {
				devicePod = fmt.Sprintf("%s:", uuid)
			}
			devices = append(devices, devicePod)
		}
	}

	abnormalDetected := len(devices) > 0
	switch {
	case abnormalDetected && !c.HighSampleRateStatus:
		// First time an abnormal state is detected, increase the sampling rate without immediately returning an abnormal result
		c.HighSampleRateStatus = true
		c.originalQueryInterval = c.cfg.UserConfig.QueryInterval
		freqController := common.GetFreqController()
		c.originalNvidiaQueryInterval = freqController.GetModuleQueryInterval(consts.ComponentNameNvidia)
		if c.originalNvidiaQueryInterval.Duration == 0 {
			c.originalNvidiaQueryInterval = common.Duration{Duration: 30 * time.Second}
		}
		freqController.SetModuleQueryInterval(consts.ComponentNameGpuEvents, c.spec.QueryIntervalAfterAbnormal)
		freqController.SetModuleQueryInterval(consts.ComponentNameNvidia, c.spec.QueryIntervalAfterAbnormal)
		c.abnormalDetectedTimes += 1
		status = consts.StatusNormal
		suggest = fmt.Sprintf("GPU hang suspected for %v, increase sample rate to %s", devices, c.spec.QueryIntervalAfterAbnormal.Duration)
		logrus.WithField("checker", "gpuevents").Warnf("%s", suggest)
	case abnormalDetected && c.HighSampleRateStatus:
		c.abnormalDetectedTimes++
		// If the abnormal state persists for `c.spec.AbnormalDetectedTimes`` consecutive checks, consider it a real GPU hang
		if c.abnormalDetectedTimes >= c.spec.AbnormalDetectedTimes {
			logrus.WithField("checker", "gpuevents").Errorf("GPU hang confirmed after %d checks", c.abnormalDetectedTimes)
			status = consts.StatusAbnormal
		} else {
			logrus.WithField("checker", "gpuevents").Errorf("GPU hang suspected after %d checks: %s", c.abnormalDetectedTimes, devices)
			status = consts.StatusNormal
		}
	case !abnormalDetected && c.HighSampleRateStatus:
		// Abnormal state recovered, reset status
		c.HighSampleRateStatus = false
		c.abnormalDetectedTimes = 0
		freqController := common.GetFreqController()
		freqController.SetModuleQueryInterval(consts.ComponentNameGpuEvents, c.originalQueryInterval)
		freqController.SetModuleQueryInterval(consts.ComponentNameNvidia, c.originalNvidiaQueryInterval)
		logrus.WithField("checker", "gpuevents").Infof("GPU hang status resolved, restoring hang query interval to %s, nviida query interval to %s.",
			c.originalQueryInterval.Duration, c.originalNvidiaQueryInterval.Duration)
	}

	result.Device = strings.Join(devices, ",")
	result.Curr = strconv.Itoa(gpuAbNum)
	result.Status = status
	result.Detail = raw
	result.Suggestion = suggest
	return result, nil
}

func (c *GpuHangChecker) OnData(IndicatorSnapshot *collector.DeviceIndicatorValues) {
	for gpuId, curIndicatorValues := range IndicatorSnapshot.Indicators {
		if _, ok := c.indicatorStates[gpuId]; !ok {
			// Initialize the state of device if it doesn't exist
			c.indicatorStates[gpuId] = &IndicatorStates{
				Indicators: make(map[string]*IndicatorState),
				LastUpdate: time.Time{},
			}
		}
		IndicatorStates := c.indicatorStates[gpuId].Indicators
		preIndicatorStates := IndicatorStates

		for indicatorName := range c.spec.Indicators {
			if _, ok := IndicatorStates[indicatorName]; !ok {
				// Initialize the state of indicator if it doesn't exist
				IndicatorStates[indicatorName] = &IndicatorState{
					Active:   false,
					Value:    0,
					Duration: 0,
				}
			}
			// Some indicators require post-processing before evaluation.
			// For example, PCIe bandwidth fluctuation over time
			// may be more meaningful than absolute values.
			// Instead of using the raw current value, their status should be determined
			// based on the difference (delta) between the current and previous values.
			var infoValue int64
			switch indicatorName {
			case "rxpci", "txpci":
				infoValue = absDiff(curIndicatorValues.Indicators[indicatorName], preIndicatorStates[indicatorName].Value)
				// fmt.Printf("%s: cur %s = %d, previous %s = %d\n", gpuId, indicatorName, curIndicatorValues.Indicators[indicatorName].Value, indicatorName, preIndicatorStates[indicatorName].Value)
			default:
				infoValue = curIndicatorValues.Indicators[indicatorName]
			}
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
