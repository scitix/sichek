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
	"github.com/scitix/sichek/components/hang/collector"
	"github.com/scitix/sichek/components/hang/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type HangChecker struct {
	id                          string
	name                        string
	cfg                         *config.HangUserConfig
	spec                        *config.HangSpec
	HighSampleRateStatus        bool
	originalQueryInterval       time.Duration
	originalNvidiaQueryInterval time.Duration
	abnormalDetectedTimes       uint32
	podResourceMapper           *k8s.PodResourceMapper
}

func NewHangChecker(cfg *config.HangUserConfig, spec *config.HangSpec) common.Checker {
	podResourceMapper := k8s.NewPodResourceMapper()
	return &HangChecker{
		id:                          consts.CheckerIDHang,
		name:                        "GPUHangChecker",
		cfg:                         cfg,
		spec:                        spec,
		HighSampleRateStatus:        false,
		originalQueryInterval:       cfg.Hang.QueryInterval,
		originalNvidiaQueryInterval: 30 * time.Second,
		abnormalDetectedTimes:       0,
		podResourceMapper:           podResourceMapper,
	}
}

func (c *HangChecker) Name() string {
	return c.name
}

func (c *HangChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.DeviceIndicatorStates)
	if !ok {
		return nil, fmt.Errorf("wrong input of HangChecker")
	}

	var raw string
	hangNum := make(map[string]int64)
	for uuid, curIndicatorStates := range info.Indicators {
		for indicateName, indicator := range curIndicatorStates.Indicators {
			if indicator.Duration >= c.spec.DurationThreshold.Duration {
				raw = fmt.Sprintf("%sdevice=%s, indicateName=%s, hang_duration=%s, hang_duration_threshold=%s\n",
					raw, uuid, indicateName, indicator.Duration, c.spec.DurationThreshold)
				hangNum[uuid]++
			}
		}
	}
	status := consts.StatusNormal
	var suggest string
	var gpuAbNum = 0
	devices := make([]string, 0)
	var deviceToPodMap map[string]*k8s.PodInfo
	var err error
	if len(hangNum) > 0 {
		deviceToPodMap, err = c.podResourceMapper.GetDeviceToPodMap()
		if err != nil {
			return nil, err
		}
	}
	result := config.HangCheckItems["GPUHang"]

	for name, num := range hangNum {
		if num == int64(len(c.spec.Indicators)) {
			gpuAbNum++
			status = consts.StatusAbnormal
			suggest = fmt.Sprintf("%ssuggest check gpu device=%s which probably hang\n", suggest, name)
			var devicePod string
			if _, found := deviceToPodMap[name]; found {
				devicePod = fmt.Sprintf("%s:%s", name, deviceToPodMap[name])
				nameSpace := deviceToPodMap[name].Namespace
				if _, exist := c.cfg.Hang.ProcessedIgnoreNamespace[nameSpace]; exist {
					result.Level = consts.LevelInfo
					logrus.WithField("component", "hang").Warningf("device=%s probably hang in pod=%+v", name, deviceToPodMap[name])
				}
			} else {
				devicePod = fmt.Sprintf("%s:", name)
			}
			devices = append(devices, devicePod)
		}
	}

	abnormalDetected := len(devices) > 0
	switch {
	case abnormalDetected && !c.HighSampleRateStatus:
		// First time an abnormal state is detected, increase the sampling rate without immediately returning an abnormal result
		c.HighSampleRateStatus = true
		c.originalQueryInterval = c.cfg.Hang.QueryInterval
		freqController := common.GetFreqController()
		c.originalNvidiaQueryInterval = freqController.GetModuleQueryInterval(consts.ComponentNameNvidia)
		if c.originalNvidiaQueryInterval == 0 {
			c.originalNvidiaQueryInterval = 30
		}
		freqController.SetModuleQueryInterval(consts.ComponentNameHang, 10)
		freqController.SetModuleQueryInterval(consts.ComponentNameNvidia, 10)
		c.abnormalDetectedTimes += 1
		status = consts.StatusNormal
		suggest = "GPU hang suspected, increase sample rate to 10s"
		logrus.WithField("checker", "hang").Warn("GPU hang suspected, increase sample rate to 10s")
	case abnormalDetected && c.HighSampleRateStatus:
		c.abnormalDetectedTimes++
		// If the abnormal state persists for 3 consecutive checks, consider it a real GPU hang
		if c.abnormalDetectedTimes >= 3 {
			logrus.WithField("checker", "hang").Errorf("GPU hang confirmed after %d checks", c.abnormalDetectedTimes)
			status = consts.StatusAbnormal
		}
	case !abnormalDetected && c.HighSampleRateStatus:
		// Abnormal state recovered, reset status
		c.HighSampleRateStatus = false
		c.abnormalDetectedTimes = 0
		freqController := common.GetFreqController()
		freqController.SetModuleQueryInterval(consts.ComponentNameHang, c.originalQueryInterval)
		freqController.SetModuleQueryInterval(consts.ComponentNameNvidia, c.originalNvidiaQueryInterval)
		logrus.WithField("checker", "hang").Infof("GPU hang status resolved, restoring hang query interval to %d, nviida query interval to %d.", c.originalQueryInterval, c.originalNvidiaQueryInterval)
	}

	result.Device = strings.Join(devices, ",")
	result.Curr = strconv.Itoa(gpuAbNum)
	result.Status = status
	result.Detail = raw
	result.Suggestion = suggest
	return &result, nil
}
