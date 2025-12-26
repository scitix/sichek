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
	"errors"
	"fmt"
	"time"
	"os"
    "strings"
    "bufio"
	"path/filepath"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/pkg/k8s"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

type NvidiaCollector struct {
	// only collect once as it is collected by `nvidia-smi -q -i 0`
	softwareInfo        SoftwareInfo
	ExpectedDeviceCount int
	// collect DeviceUUIDs until all the expected num are valid, otherwise, it will be collected every Collect(ctx) call
	UUIDAllValidFlag bool
	// record all the expected num of UUIDs, in case some of them are invalid later
	DeviceUUIDs       map[int]string
	nvmlInst          *nvml.Interface // Shared pointer to NVML instance
	podResourceMapper *k8s.PodResourceMapper
}

func NewNvidiaCollector(ctx context.Context, nvmlInstPtr *nvml.Interface, expectedDeviceCount int, expectedDeviceName string) (*NvidiaCollector, error) {
	podResourceMapper := k8s.NewPodResourceMapper()
	if podResourceMapper == nil {
		err := fmt.Errorf("failed to create PodResourceMapper")
		logrus.WithField("component", "NVIDIA-Collector").Errorf("%v", err)
		return nil, err
	}
	collector := &NvidiaCollector{nvmlInst: nvmlInstPtr, podResourceMapper: podResourceMapper}
	var err error
	for i := 0; i < expectedDeviceCount; i++ {
		err = collector.softwareInfo.Get(ctx, i)
		if err != nil {
			logrus.WithField("component", "NVIDIA-Collector-getSWInfo").Errorf("%v", err)
		} else {
			break
		}
	}
	if err == nil {
		// Default to expectedDeviceCount
		collector.ExpectedDeviceCount = expectedDeviceCount

		// TODO(xdxiong): Remove this workaround after spec is changed to use machine model ID corresponding spec.
		// This logic adjusts ExpectedDeviceCount based on NVML DeviceGetCount() for A100-PCIE-40GB,
		// which should be handled by the spec configuration instead.
		// Get device count and adjust ExpectedDeviceCount if needed
		numDevices, err2 := (*collector.nvmlInst).DeviceGetCount()
		if !errors.Is(err2, nvml.SUCCESS) {
			if invalidErr := utils.IsNvmlInvalidError(err2); invalidErr != nil {
				return nil, invalidErr
			}
			logrus.WithField("component", "NVIDIA-Collector").Warnf("failed to get device count: %v", err2)
		} else {
			// If device count is 4 and expected device name is NVIDIA A100-PCIE-40GB, set ExpectedDeviceCount to 4
			if numDevices == 4 && expectedDeviceName == "NVIDIA A100-PCIE-40GB" {
				logrus.WithField("component", "NVIDIA-Collector").Warnf("adjust ExpectedDeviceCount to 4 for NVIDIA A100-PCIE-40GB")
				collector.ExpectedDeviceCount = 4
			}
		}
		collector.DeviceUUIDs = make(map[int]string, collector.ExpectedDeviceCount)
		if err := collector.getUUID(); err != nil {
			return nil, fmt.Errorf("failed to get UUID during collector initialization: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to NewNvidiaCollector: %v", err)
	}
	return collector, nil
}

func (collector *NvidiaCollector) getUUID() error {
	collector.UUIDAllValidFlag = true
	for i := 0; i < collector.ExpectedDeviceCount; i++ {
		device, err := (*collector.nvmlInst).DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			if invalidErr := utils.IsNvmlInvalidError(err); invalidErr != nil {
				return invalidErr
			}
			collector.UUIDAllValidFlag = false
			logrus.WithField("component", "NVIDIA-Collector-getUUID").Errorf("failed to get Nvidia GPU device %d: %v", i, err)
			return nil
		}
		uuid, err := device.GetUUID()
		if !errors.Is(err, nvml.SUCCESS) {
			if invalidErr := utils.IsNvmlInvalidError(err); invalidErr != nil {
				return invalidErr
			}
			logrus.WithField("component", "NVIDIA-Collector-getUUID").Errorf("failed to get UUID for GPU %d: %v", i, nvml.ErrorString(err))
			collector.UUIDAllValidFlag = false
		}
		collector.DeviceUUIDs[i] = uuid
	}
	return nil
}

func (collector *NvidiaCollector) Name() string {
	return "NvidiaCollector"
}

func (collector *NvidiaCollector) GetCfg() common.ComponentUserConfig {
	return nil
}

func (collector *NvidiaCollector) Collect(ctx context.Context) (*NvidiaInfo, error) {
	// Note: GPUAvailability, LostGPUErrors, and DevicesInfo are all sized based on ExpectedDeviceCount.
	// Even if some GPUs are lost, GPUAvailability and LostGPUErrors will still contain entries for all
	// expected device indices (0 to ExpectedDeviceCount-1), with lost GPUs marked as unavailable.
	// The collection loop iterates through all ExpectedDeviceCount devices, ensuring complete coverage.
	if !collector.UUIDAllValidFlag {
		if err := collector.getUUID(); err != nil {
			return nil, err
		}
	}

	nvidia := &NvidiaInfo{
		Time:                time.Now(),
		SoftwareInfo:        collector.softwareInfo,
		ValiddeviceUUIDFlag: collector.UUIDAllValidFlag,
		DeviceUUIDs:         collector.DeviceUUIDs,
		GPUAvailability:     make(map[int]bool, collector.ExpectedDeviceCount),
		LostGPUErrors:       make(map[int]string, collector.ExpectedDeviceCount),
		IbgdaEnable:         collector.getDriverParams(),
		IbgdaConfigCount:    collector.getIBGDAConfigCount(), 
		P2PStatusMatrix:     collector.getP2PStatusMatrix(),
	}

	// Get the number of devices
	numDevices, err := (*collector.nvmlInst).DeviceGetCount()
	if !errors.Is(err, nvml.SUCCESS) {
		// Check if this is an error that indicates NVML is invalid
		if invalidErr := utils.IsNvmlInvalidError(err); invalidErr != nil {
			return nil, invalidErr
		}
		return nil, fmt.Errorf("failed to get Nvidia GPU device count: %v", err)
	}
	nvidia.DeviceCount = numDevices

	// Check GPU availability for all expected devices and get the device info
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g4cc7ff5253d53cc97b1afb606d614888
	nvidia.DevicesInfo = make([]DeviceInfo, 0)
	nvidia.DeviceUsedCount = 0
	for i := 0; i < collector.ExpectedDeviceCount; i++ {
		device, err := (*collector.nvmlInst).DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			if invalidErr := utils.IsNvmlInvalidError(err); invalidErr != nil {
				return nil, invalidErr
			}
			nvidia.GPUAvailability[i] = false
			nvidia.LostGPUErrors[i] = nvml.ErrorString(err)
			logrus.WithField("component", "NVIDIA-Collector-Collect").Errorf("GPU %d is not accessible: %s", i, nvidia.LostGPUErrors[i])
			continue
		}
		nvidia.GPUAvailability[i] = true
		var deviceInfo DeviceInfo
		err2 := deviceInfo.Get(device, i, collector.softwareInfo.DriverVersion)
		if err2 != nil {
			logger := logrus.WithField("component", "NVIDIA-Collector-Collect")
			logger.Errorf("GPU %d: %s", i, err2.Error())
			for j, partialErr := range deviceInfo.PartialErrors {
				logger.Errorf("GPU %d:   %d. %s", i, j+1, partialErr)
			}
			nvidia.GPUAvailability[i] = false
			nvidia.LostGPUErrors[i] = err2.Error()
		}
		// Only add successfully collected device info to the list
		nvidia.DevicesInfo = append(nvidia.DevicesInfo, deviceInfo)
		if deviceInfo.NProcess > 0 {
			nvidia.DeviceUsedCount++
		}
	}

	// Get the device to pod map
	deviceToPodMap, err2 := collector.podResourceMapper.GetDeviceToPodMap()
	if err2 != nil {
		logrus.WithField("component", "NVIDIA-Collector").Errorf("failed to get device to pod map: %v", err2)
	}
	nvidia.DeviceToPodMap = deviceToPodMap
	return nvidia, nil
}

func (collector *NvidiaCollector) getDriverParams() map[string]string {
    params := make(map[string]string)
    path := "/proc/driver/nvidia/params"
    
    file, err := os.Open(path)
    if err != nil {
        logrus.WithField("component", "NvidiaCollector").Debugf("failed to open driver params: %v", err)
        return nil 
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        
        parts := strings.SplitN(line, ":", 2)
        if len(parts) == 2 {
            key := strings.TrimSpace(parts[0])
            val := strings.TrimSpace(parts[1])
            params[key] = val
        }
    }
    
    return params
}

func (collector *NvidiaCollector) getIBGDAConfigCount() int {
	matches, err := filepath.Glob("/etc/modprobe.d/*.conf")
	if err != nil { return 0 }

	countOps := 0
	countPeer := 0

	for _, match := range matches {
		file, err := os.Open(match)
		if err != nil { continue }
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "options nvidia") {
				continue
			}
			if strings.Contains(line, "EnableStreamMemOPs") {
				countOps++
			}
			if strings.Contains(line, "PeerMappingOverride") {
				countPeer++
			}
		}
		file.Close()
	}

	if countOps > countPeer {
		return countOps
	}
	return countPeer
}


func (collector *NvidiaCollector) getP2PStatusMatrix() map[string]bool {
	if collector.nvmlInst == nil {
		return nil
	}

	matrix := make(map[string]bool)
	count := collector.ExpectedDeviceCount

	for i := 0; i < count; i++ {
		handle1, ret1 := (*collector.nvmlInst).DeviceGetHandleByIndex(i)
		if ret1 != nvml.SUCCESS {
			continue
		}

		for j := 0; j < count; j++ {
			if i == j {
				continue
			}

			handle2, ret2 := (*collector.nvmlInst).DeviceGetHandleByIndex(j)
			if ret2 != nvml.SUCCESS {
				continue
			}

			status, ret := handle1.GetP2PStatus(handle2, nvml.P2P_CAPS_INDEX_READ)
			
			key := fmt.Sprintf("%d-%d", i, j)
			
			if ret == nvml.SUCCESS && status == nvml.P2P_STATUS_OK {
				matrix[key] = true
			} else {
				matrix[key] = false
			}
		}
	}
	return matrix
}