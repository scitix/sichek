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

type DeviceInfo struct {
	Name         string          `json:"name"`
	Index        int             `json:"Index"`
	UUID         string          `json:"uuid"`
	Serial       string          `json:"serial"`
	VBIOSVersion string          `json:"vbios_version"`
	PCIeInfo     PCIeInfo        `json:"pcie_info"`
	States       StatesInfo      `json:"states_info"`
	Clock        ClockInfo       `json:"clock_info"`
	ClockEvents  ClockEvents     `json:"clock_events"`
	Power        PowerInfo       `json:"power_info"`
	Temperature  TemperatureInfo `json:"temperature_info"`
	Utilization  UtilizationInfo `json:"utilization_info"`
	NVLinkStates NVLinkStates    `json:"nvlink_state"`
	MemoryErrors MemoryErrors    `json:"ecc_event"`
}

func (deviceInfo *DeviceInfo) JSON() ([]byte, error) {
	return common.JSON(deviceInfo)
}

// Convert struct to JSON (pretty-printed)
func (deviceInfo *DeviceInfo) ToString() string {
	return common.ToString(deviceInfo)
}

func (deviceInfo *DeviceInfo) Get(device nvml.Device, index int) error {
	// Get GPU Name
	name, err := device.GetName()
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get name for GPU %d: %v", index, nvml.ErrorString(err))
	}
	deviceInfo.Name = name

	// Get GPU Index
	// moduleId, err := device.GetModuleId()
	minorNumber, err := device.GetMinorNumber()
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get index for GPU %d: %v", index, nvml.ErrorString(err))
	}
	deviceInfo.Index = minorNumber

	// Get GPU UUID
	uuid, err := device.GetUUID()
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get UUID for GPU %d: %v", index, nvml.ErrorString(err))
	}
	deviceInfo.UUID = uuid

	// Get GPU Serial
	serial, err := device.GetSerial()
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get serial for GPU %v: %v", uuid, nvml.ErrorString(err))
	}
	deviceInfo.Serial = serial

	// Get the VBIOS version, may differ between GPUs
	vbiosVersion, err := device.GetVbiosVersion()
	if err != nvml.SUCCESS {
		return fmt.Errorf("failed to get VBIOS version for device 0: %v", nvml.ErrorString(err))
	}
	deviceInfo.VBIOSVersion = vbiosVersion

	err2 := deviceInfo.States.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get states info for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.PCIeInfo.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get pcie info for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.Clock.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get clock %v: %v", uuid, err2)
	}

	err2 = deviceInfo.ClockEvents.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get clock events for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.Power.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get Power for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.Temperature.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get temperature events for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.MemoryErrors.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get sensor info for device %v: %v", uuid, err2)
	}

	err2 = deviceInfo.NVLinkStates.Get(device, uuid)
	if err2 != nil {
		return fmt.Errorf("failed to get nvlink states for device %v: %v", uuid, err2)
	}
	return nil
}
