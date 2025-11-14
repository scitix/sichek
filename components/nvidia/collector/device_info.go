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
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

type DeviceInfo struct {
	Name          string          `json:"name" yaml:"name"`
	Index         int             `json:"Index" yaml:"Index"`
	UUID          string          `json:"uuid" yaml:"uuid"`
	Serial        string          `json:"serial" yaml:"serial"`
	VBIOSVersion  string          `json:"vbios_version" yaml:"vbios_version"`
	PCIeInfo      PCIeInfo        `json:"pcie_info" yaml:"pcie_info"`
	States        StatesInfo      `json:"states_info" yaml:"states_info"`
	Clock         ClockInfo       `json:"clock_info" yaml:"clock_info"`
	ClockEvents   ClockEvents     `json:"clock_events" yaml:"clock_events"`
	Power         PowerInfo       `json:"power_info" yaml:"power_info"`
	Temperature   TemperatureInfo `json:"temperature_info" yaml:"temperature_info"`
	Utilization   UtilizationInfo `json:"utilization_info" yaml:"utilization_info"`
	NVLinkStates  NVLinkStates    `json:"nvlink_state" yaml:"nvlink_state"`
	MemoryErrors  MemoryErrors    `json:"ecc_event" yaml:"ecc_event"`
	NProcess      int             `json:"nprocess" yaml:"nprocess"`
	PartialErrors []string        `json:"partial_errors,omitempty" yaml:"partial_errors,omitempty"`
}

func (deviceInfo *DeviceInfo) JSON() ([]byte, error) {
	return common.JSON(deviceInfo)
}

// ToString Convert struct to JSON (pretty-printed)
func (deviceInfo *DeviceInfo) ToString() string {
	return common.ToString(deviceInfo)
}

func (deviceInfo *DeviceInfo) Get(device nvml.Device, index int, driverVersion string) error {
	deviceInfo.PartialErrors = make([]string, 0)

	// Get GPU Name
	name, err := device.GetName()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get gpu name: %v", nvml.ErrorString(err)))
	}
	deviceInfo.Name = name

	// Get GPU Index
	// moduleId, err := device.GetModuleId()
	minorNumber, err := device.GetMinorNumber()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get gpu index: %v", nvml.ErrorString(err)))
	}
	deviceInfo.Index = minorNumber

	// Get GPU UUID
	uuid, err := device.GetUUID()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get gpu uuid: %v", nvml.ErrorString(err)))
	}
	deviceInfo.UUID = uuid

	// Get GPU Serial
	serial, err := device.GetSerial()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get serial: %v", nvml.ErrorString(err)))
	} else {
		deviceInfo.Serial = serial
	}

	// Get the VBIOS version, may differ between GPUs
	vbiosVersion, err := device.GetVbiosVersion()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get VBIOS version: %v", nvml.ErrorString(err)))
	} else {
		deviceInfo.VBIOSVersion = vbiosVersion
	}

	// Get States info
	err2 := deviceInfo.States.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get states info: %v", err2))
	}

	// Get PCIe info
	err2 = deviceInfo.PCIeInfo.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get pcie info: %v", err2))
	}

	// Get Clock info
	err2 = deviceInfo.Clock.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get clock info: %v", err2))
	}

	// clock events are supported in version 535 and above
	// otherwise, the function GetCurrentClocksEventReasons() will exits with undefined symbol: nvmlGetCurrentClocksEventReasons
	isSupported, err3 := isDriverVersionSupportedClkEvents(driverVersion, 535)
	if err3 != nil {
		logrus.WithField("component", "nvidia-collector-device-info").Warnf("failed to check if driver version %v is supported for clock events: %v", driverVersion, err3)
	}
	deviceInfo.ClockEvents.IsSupported = isSupported
	if isSupported {
		err2 = deviceInfo.ClockEvents.Get(device, uuid)
		if err2 != nil {
			deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get clock events: %v", err2))
		}
	}

	// Get Power info
	err2 = deviceInfo.Power.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get power info: %v", err2))
	}

	// Get Temperature info (skip for L40)
	deviceID := fmt.Sprintf("0x%x", deviceInfo.PCIeInfo.DEVID)
	if deviceID != "0x26b510de" { // skip temperature events for L40
		err2 = deviceInfo.Temperature.Get(device, uuid)
		if err2 != nil {
			deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get temperature info: %v", err2))
		}
	}

	// Get Utilization info
	err2 = deviceInfo.Utilization.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get utilization info: %v", err2))
	}

	// Get MemoryErrors info
	err2 = deviceInfo.MemoryErrors.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get memory errors info: %v", err2))
	}

	// Get NVLinkStates info
	err2 = deviceInfo.NVLinkStates.Get(device, uuid)
	if err2 != nil {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get nvlink states: %v", err2))
	}

	// Get the number of processes using the GPU
	processInfo, err := device.GetComputeRunningProcesses()
	if !errors.Is(err, nvml.SUCCESS) {
		deviceInfo.PartialErrors = append(deviceInfo.PartialErrors, fmt.Sprintf("failed to get processes: %v", nvml.ErrorString(err)))
		deviceInfo.NProcess = 0
	} else {
		deviceInfo.NProcess = len(processInfo)
	}

	if len(deviceInfo.PartialErrors) > 0 {
		return fmt.Errorf("failed to get device info: %d errors occurred", len(deviceInfo.PartialErrors))
	}
	return nil
}

func isDriverVersionSupportedClkEvents(driverVersion string, requiredMajor int) (bool, error) {
	// Split the driver version string by "."
	parts := strings.Split(driverVersion, ".")
	if len(parts) < 1 {
		return false, fmt.Errorf("invalid driver version format: %s", driverVersion)
	}

	// Parse the major version (first part of the string)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, fmt.Errorf("invalid major version in driver version: %s", driverVersion)
	}

	// Compare the major version
	return major >= requiredMajor, nil
}
