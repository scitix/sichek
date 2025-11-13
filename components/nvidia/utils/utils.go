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
package utils

import (
	"errors"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

func GetDeviceID() (string, error) {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); !errors.Is(ret, nvml.SUCCESS) {
		return "", fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))

	}
	defer nvmlInst.Shutdown()

	// In case of GPU error, iterate through all GPUs to find the first valid one
	deviceCount, err := nvmlInst.DeviceGetCount()
	if !errors.Is(err, nvml.SUCCESS) {
		return "", fmt.Errorf("failed to get device count: %s", nvml.ErrorString(err))
	}
	var deviceID string
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Errorf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		pciInfo, err := device.GetPciInfo()
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Errorf("failed to get PCIe Info  for NVIDIA GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		deviceID = fmt.Sprintf("0x%x", pciInfo.PciDeviceId)
		return deviceID, nil
	}
	return "", fmt.Errorf("failed to get product name for NVIDIA GPU")
}

func GetComputeCapability(index int) (int, int, error) {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); !errors.Is(ret, nvml.SUCCESS) {
		return 0, 0, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))

	}
	defer nvmlInst.Shutdown()

	device, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return 0, 0, fmt.Errorf("failed to get device handle: %v", nvml.ErrorString(ret))
	}

	// Get Compute Capability
	major, minor, ret := device.GetCudaComputeCapability()
	if ret != nvml.SUCCESS {
		return 0, 0, fmt.Errorf("failed to get compute capability: %v", nvml.ErrorString(ret))
	}

	return major, minor, nil
}

// IsNvmlInvalidError checks if the error indicates NVML instance is invalid
// Returns an error if invalid, nil otherwise
func IsNvmlInvalidError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, nvml.ERROR_UNINITIALIZED) || errors.Is(err, nvml.ERROR_DRIVER_NOT_LOADED) {
		return fmt.Errorf("NVML invalid error: %w", err)
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		if errors.Is(unwrapped, nvml.ERROR_UNINITIALIZED) || errors.Is(unwrapped, nvml.ERROR_DRIVER_NOT_LOADED) {
			return fmt.Errorf("NVML invalid error: %w", unwrapped)
		}
	}
	return nil
}

// CheckNvmlInvalidError checks if the error indicates NVML instance is invalid and needs reinitialization
// Returns true for errors that indicate NVML is uninitialized or driver is not loaded
func CheckNvmlInvalidError(err error) bool {
	if err == nil {
		return false
	}
	return IsNvmlInvalidError(err) != nil
}
