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
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/k8s"
)

type NvidiaInfo struct {
	Time                time.Time
	SoftwareInfo        SoftwareInfo            `json:"software_info"`
	ValiddeviceUUIDFlag bool                    `json:"valid_device_uuid_flag"`
	DeviceUUIDs         map[int]string          `json:"device_uuids"`
	DeviceCount         int                     `json:"device_count"`
	DeviceUsedCount     int                     `json:"device_used_count"`
	DevicesInfo         []DeviceInfo            `json:"gpu_devices"`
	DeviceToPodMap      map[string]*k8s.PodInfo `json:"device_to_pod_map"`
}

func (nvidia *NvidiaInfo) JSON() (string, error) {
	data, err := json.Marshal(nvidia)
	return string(data), err
}

// ToString Convert struct to JSON (pretty-printed)
func (nvidia *NvidiaInfo) ToString() string {
	return common.ToString(nvidia)
}
