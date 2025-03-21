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
	"encoding/json"
	// "fmt"
	"time"

	"github.com/scitix/sichek/components/common"

	// "github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NvidiaInfo struct {
	Time                time.Time
	SoftwareInfo        SoftwareInfo      `json:"software_info"`
	ValiddeviceUUIDFlag bool              `json:"valid_device_uuid_flag"`
	DeviceUUIDs         map[int]string    `json:"device_uuids"`
	DeviceCount         int               `json:"device_count"`
	DevicesInfo         []DeviceInfo      `json:"gpu_devices"`
	DeviceToPodMap      map[string]string `json:"device_to_pod_map"`
}

func (nvidia NvidiaInfo) JSON() (string, error) {
	data, err := json.Marshal(nvidia)
	return string(data), err
}

// func (nvidia *NvidiaInfo) JSON() ([]byte, error) {
// 	return common.JSON(nvidia)
// }

// Convert struct to JSON (pretty-printed)
func (nvidia *NvidiaInfo) ToString() string {
	return common.ToString(nvidia)
}

// func (nvidia *NvidiaInfo) Get(nvmlInst nvml.Interface) error {
// 	nvidia.Time = time.Now()
// 	// Get the software info
// 	err := nvidia.SoftwareInfo.Get(0)
// 	if err != nil {
// 		return fmt.Errorf("failed to get Nvidia GPU software info: %v", err)
// 	}

// 	// Get the number of devices
// 	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1ga93623b195bff04bbe3490ca33c8a42d
// 	numDevices, err := nvmlInst.DeviceGetCount()
// 	if err != nvml.SUCCESS {
// 		return fmt.Errorf("failed to get Nvidia GPU device count: %v", err)
// 	}
// 	nvidia.DeviceCount = numDevices

// 	// Get the device info
// 	var deviceInfo DeviceInfo
// 	for i := 0; i < numDevices; i++ {
// 		device, err := nvmlInst.DeviceGetHandleByIndex(i)
// 		if err != nvml.SUCCESS {
// 			continue
// 			// return fmt.Errorf("failed to get Nvidia GPU device %d: %v", i, err)
// 		}
// 		err2 := deviceInfo.Get(device, i, nvidia.SoftwareInfo.DriverVersion)
// 		if err2 != nil {
// 			continue
// 			// return fmt.Errorf("failed to get Nvidia GPU deviceInfo %d: %v", i, err2)
// 		}
// 		nvidia.DevicesInfo = append(nvidia.DevicesInfo, deviceInfo)
// 	}
// 	return nil
// }
