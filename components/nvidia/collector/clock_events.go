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
	"fmt"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type ClockEvent struct {
	Name               string `json:"name"`
	ClockEventReasonId uint64 `json:"clock_event_reason_id"`
	Description        string `json:"description"`
}

// ref. https://github.com/NVIDIA/go-nvml/blob/main/pkg/nvml/nvml.h#L2276
var WarningClockEvents = map[uint64]ClockEvent{
	/** SwPowerCap
	*
	* The clocks have been optimized to ensure not to exceed currently set power limits
	* @see nvmlDeviceGetPowerUsage
	* @see nvmlDeviceSetPowerManagementLimit
	* @see nvmlDeviceGetPowerManagementLimit
	 */
	0x00000000000000004: {"SwPowerCap", 0x00000000000000004, "SwPowerCap is engaged due to high power"},

	/** SW Thermal Slowdown
	*
	* The current clocks have been optimized to ensure the the following is true:
	*  - Current GPU temperature does not exceed GPU Max Operating Temperature
	*  - Current memory temperature does not exceeed Memory Max Operating Temperature
	*
	 */
	0x00000000000000020: {"SW Thermal Slowdown", 0x00000000000000020, "SW Thermal Slowdown is engaged due to high temperature"},
}

// ref. https://github.com/NVIDIA/go-nvml/blob/main/pkg/nvml/nvml.h#L2322
var CriticalClockEvents = map[uint64]ClockEvent{

	/** HW Thermal Slowdown (reducing the core clocks by a factor of 2 or more) is engaged
	*
	* This is an indicator of:
	*   - temperature being too high
	*
	* @see nvmlDeviceGetTemperature
	* @see nvmlDeviceGetTemperatureThreshold
	* @see nvmlDeviceGetPowerUsage
	 */
	0x0000000000000040: {"HW Thermal Slowdown", 0x0000000000000040, "HW Thermal Slowdown is engaged due to high temperature, reducing the core clocks by a factor of 2 or more"},

	/** HW Power Brake Slowdown (reducing the core clocks by a factor of 2 or more) is engaged
	*
	* This is an indicator of:
	*   - External Power Brake Assertion being triggered (e.g. by the system power supply)
	*
	* @see nvmlDeviceGetTemperature
	* @see nvmlDeviceGetTemperatureThreshold
	* @see nvmlDeviceGetPowerUsage
	 */
	0x0000000000000080: {"HW Power Brake Slowdown", 0x0000000000000080, "HW Power Brake Slowdown is engaged due to External Power Brake Assertion being triggered (e.g. by the system power supply), reducing the core clocks by a factor of 2 or more"},
}

var gpuIdleId uint64 = 0x0000000000000001

type ClockEvents struct {
	GpuIdle             bool         `json:"gpu_idle"`
	CriticalClockEvents []ClockEvent `json:"critical_clock_events"`
	WarningClockEvents  []ClockEvent `json:"warning_clock_events"`
}

func (clk *ClockEvents) JSON() ([]byte, error) {
	return common.JSON(clk)
}

// Convert struct to JSON (pretty-printed)
func (clk *ClockEvents) ToString() string {
	return common.ToString(clk)
}

// https://docs.nvidia.com/deploy/nvml-api/group__nvmlClocksEventReasons.html#group__nvmlClocksEventReasons
func (clk *ClockEvents) Get(device nvml.Device, uuid string) error {
	reasons, ret := device.GetCurrentClocksEventReasons()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to get device clock event reasons: %v", nvml.ErrorString(ret))
	}

	clk.GpuIdle = reasons&gpuIdleId != 0

	for id, event := range CriticalClockEvents {
		if reasons&id != 0 {
			clk.CriticalClockEvents = append(clk.CriticalClockEvents, event)
		}
	}

	for id, event := range WarningClockEvents {
		if reasons&id != 0 {
			clk.WarningClockEvents = append(clk.WarningClockEvents, event)
		}
	}

	return nil
}
