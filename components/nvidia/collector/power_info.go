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
	"errors"
	"fmt"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type PowerInfo struct {
	PowerUsage         uint32  `json:"power_usage" yaml:"power_usage"`
	CurPowerLimit      float32 `json:"cur_power_limit_W" yaml:"cur_power_limit_W"`
	DefaultPowerLimit  float32 `json:"default_power_limit_W" yaml:"default_power_limit_W"`
	EnforcedPowerLimit float32 `json:"enforced_power_limit_W" yaml:"enforced_power_limit_W"`
	MinPowerLimit      float32 `json:"min_power_limit_W" yaml:"min_power_limit_W"`
	MaxPowerLimit      float32 `json:"max_power_limit_W" yaml:"max_power_limit_W"`
	PowerViolations    uint64  `json:"power_violations" yaml:"power_violations"`
	ThermalViolations  uint64  `json:"thermal_violations" yaml:"thermal_violations"`
}

func (info *PowerInfo) JSON() ([]byte, error) {
	return common.JSON(info)
}

// ToString Convert struct to JSON (pretty-printed)
func (info *PowerInfo) ToString() string {
	return common.ToString(info)
}

func (info *PowerInfo) Get(device nvml.Device, uuid string) error {

	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g7ef7dff0ff14238d08a19ad7fb23fc87
	powerUsage, ret := device.GetPowerUsage()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get device power usage: %v", nvml.ErrorString(ret))
	}
	info.PowerUsage = powerUsage

	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1gf754f109beca3a4a8c8c1cd650d7d66c
	curPowerLimit, ret := device.GetPowerManagementLimit()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get device power limit: %v", nvml.ErrorString(ret))
	}
	info.CurPowerLimit = float32(curPowerLimit) / 1000.0

	defaultPowerLimit, ret := device.GetPowerManagementDefaultLimit()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get device default power limit: %v", nvml.ErrorString(ret))
	}
	info.DefaultPowerLimit = float32(defaultPowerLimit) / 1000.0

	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g263b5bf552d5ec7fcd29a088264d10ad
	enforcedPowerLimit, ret := device.GetEnforcedPowerLimit()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get device power limit: %v", nvml.ErrorString(ret))
	}
	info.EnforcedPowerLimit = float32(enforcedPowerLimit) / 1000.0

	// Get power limit constraints
	minPowerLimit, maxPowerLimit, ret := device.GetPowerManagementLimitConstraints()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get power limit constraints for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	info.MinPowerLimit = float32(minPowerLimit) / 1000.0
	info.MaxPowerLimit = float32(maxPowerLimit) / 1000.0

	// Get power violations
	pviol, ret := device.GetViolationStatus(nvml.PERF_POLICY_POWER)
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get power violation status: %v", nvml.ErrorString(ret))
	}
	info.PowerViolations = pviol.ViolationTime

	// Get Thermal violations
	tviol, ret := device.GetViolationStatus(nvml.PERF_POLICY_THERMAL)
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get thermal violation status: %v", nvml.ErrorString(ret))
	}
	info.PowerViolations = tviol.ViolationTime

	return nil
}
