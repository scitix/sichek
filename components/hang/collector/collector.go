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
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/hang/config"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

// IndicatorState represents the real-time status of a specific hang indicator,
// including whether the condition is currently met and how long it's been active.
type IndicatorState struct {
	Active   bool          // Whether the indicator currently meets the hang condition
	Value    int64         // The current value of the indicator
	Duration time.Duration // Accumulated duration during which the condition is met
}

// IndicatorStates tracks the status of all hang indicators for a single device.
type IndicatorStates struct {
	Indicators map[string]*IndicatorState
	LastUpdate time.Time // Last update timestamp for this device's indicators
}

// DeviceIndicatorStates tracks all hang indicators for all GPU device.
type DeviceIndicatorStates struct {
	Indicators map[string]*IndicatorStates // DeviceID -> IndicatorStates
	LastUpdate time.Time                   // Last update timestamp for all devices' indicators
}

func (s *DeviceIndicatorStates) JSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type HangCollector struct {
	name string

	cfg  *config.HangUserConfig
	spec *config.HangEventRule

	devIndicatorStates *DeviceIndicatorStates
	LastUpdate         time.Time // Timestamp of the last update

	nvidiaComponent common.Component
}

func NewHangCollector(cfg *config.HangUserConfig, spec *config.HangEventRule) (*HangCollector, error) {
	var nvidiaComponent common.Component
	var err error
	if !cfg.Hang.Mock {
		if nvidiaComponent, err = nvidia.GetComponent(); err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to GetComponent")
			return nil, err
		}
	} else {
		if nvidiaComponent, err = NewMockNvidiaComponent(""); err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to NewMockNvidiaComponent")
			return nil, err
		}
	}

	return &HangCollector{
		name: consts.ComponentNameHang,
		cfg:  cfg,
		spec: spec,
		devIndicatorStates: &DeviceIndicatorStates{
			Indicators: make(map[string]*IndicatorStates),
		},
		LastUpdate:      time.Now(),
		nvidiaComponent: nvidiaComponent,
	}, nil
}

func (c *HangCollector) Name() string {
	return c.name
}

func (c *HangCollector) Collect(ctx context.Context) (common.Info, error) {
	var curDeviceIndicatorStates *DeviceIndicatorStates
	if !c.cfg.Hang.NVSMI {
		curDeviceIndicatorStates = c.getInfobyLatestInfo(ctx)
	} else {
		curDeviceIndicatorStates = getInfobyNvidiaSmi(ctx)
	}
	if curDeviceIndicatorStates == nil {
		return nil, fmt.Errorf("failed to get device indicator states")
	}

	devIndicatorStates := c.devIndicatorStates.Indicators
	for gpuId, curIndicatorStates := range curDeviceIndicatorStates.Indicators {
		if _, ok := devIndicatorStates[gpuId]; !ok {
			// Initialize the state of device if it doesn't exist
			devIndicatorStates[gpuId] = &IndicatorStates{
				Indicators: make(map[string]*IndicatorState),
				LastUpdate: time.Time{},
			}
		}
		IndicatorStates := devIndicatorStates[gpuId].Indicators
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
				infoValue = absDiff(curIndicatorStates.Indicators[indicatorName].Value, preIndicatorStates[indicatorName].Value)
				// fmt.Printf("%s: cur %s = %d, previous %s = %d\n", gpuId, indicatorName, curIndicatorStates.Indicators[indicatorName].Value, indicatorName, preIndicatorStates[indicatorName].Value)
			default:
				infoValue = curIndicatorStates.Indicators[indicatorName].Value
			}
			duration := c.getDuration(indicatorName, infoValue, curIndicatorStates.LastUpdate)
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

	c.LastUpdate = curDeviceIndicatorStates.LastUpdate
	return c.devIndicatorStates, nil
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

func (c *HangCollector) getDuration(indicatorName string, infoValue int64, now time.Time) int64 {
	var res int64 = 0
	if c.spec.Indicators[indicatorName] == nil {
		logrus.WithField("collector", "hanggetter").Errorf("failed to get hang spec of %s", indicatorName)
		return res
	}
	indicator := c.spec.Indicators[indicatorName]

	if (infoValue < indicator.Threshold && indicator.CompareType == string(config.CompareLow)) ||
		(infoValue > indicator.Threshold && indicator.CompareType == string(config.CompareHigh)) {
		if !c.LastUpdate.IsZero() {
			res = int64(now.Sub(c.LastUpdate).Seconds())
		}
	}
	return res
}
