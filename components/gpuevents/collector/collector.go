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
	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

// IndicatorValues tracks the values of all indicators for a single device.
type IndicatorValues struct {
	Indicators map[string]int64
	LastUpdate time.Time // Last update timestamp for this device's indicators
}

// DeviceIndicatorValues tracks all gpu indicators for all GPU device.
type DeviceIndicatorValues struct {
	Indicators map[string]*IndicatorValues // DeviceID -> IndicatorValues
	LastUpdate time.Time                   // Last update timestamp for all devices' indicators
}

func (s *DeviceIndicatorValues) JSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type GpuIndicatorSnapshot struct {
	name string

	cfg *config.GpuCostomEventsUserConfig

	devIndicatorValues *DeviceIndicatorValues
	LastUpdate         time.Time // Timestamp of the last update

	nvidiaComponent common.Component
}

func NewGpuIndicatorSnapshot(cfg *config.GpuCostomEventsUserConfig) (*GpuIndicatorSnapshot, error) {
	var nvidiaComponent common.Component
	var err error
	if !cfg.UserConfig.Mock {
		if nvidiaComponent, err = nvidia.GetComponent(); err != nil {
			logrus.WithField("collector", "gpuevents").WithError(err).Errorf("failed to GetComponent")
			return nil, err
		}
	} else {
		if nvidiaComponent, err = NewMockNvidiaComponent(""); err != nil {
			logrus.WithField("collector", "gpuevents").WithError(err).Errorf("failed to NewMockNvidiaComponent")
			return nil, err
		}
	}

	return &GpuIndicatorSnapshot{
		name: consts.ComponentNameGpuEvents,
		cfg:  cfg,
		devIndicatorValues: &DeviceIndicatorValues{
			Indicators: make(map[string]*IndicatorValues),
		},
		LastUpdate:      time.Now(),
		nvidiaComponent: nvidiaComponent,
	}, nil
}

func (c *GpuIndicatorSnapshot) Name() string {
	return c.name
}

func (c *GpuIndicatorSnapshot) Collect(ctx context.Context) (common.Info, error) {
	var curDeviceIndicatorValues *DeviceIndicatorValues
	if !c.cfg.UserConfig.NVSMI {
		curDeviceIndicatorValues = c.getInfobyLatestInfo(ctx)
	} else {
		curDeviceIndicatorValues = getInfobyNvidiaSmi(ctx)
	}
	if curDeviceIndicatorValues == nil {
		return nil, fmt.Errorf("failed to get device indicator states")
	}
	return curDeviceIndicatorValues, nil
}
