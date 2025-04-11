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
	"errors"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NVLinkState struct {
	NVlinkSupported      bool `json:"nvlink_supported"`
	FeatureEnabled       bool `json:"feature_enabled"`
	LinkNo               int  `json:"link_no"`
}

type NVLinkStates struct {
	NVlinkSupported   bool          `json:"nvlink_supported"`
	AllFeatureEnabled bool          `json:"all_feature_enabled,omitempty"`
	NvlinkNum         int           `json:"active_nvlink_num"`
	NVLinkStates      []NVLinkState `json:"nvlink_state,omitempty"`
}

func (nvlinkState *NVLinkState) JSON() ([]byte, error) {
	return common.JSON(nvlinkState)
}

// ToString Convert struct to JSON (pretty-printed)
func (nvlinkState *NVLinkState) ToString() string {
	return common.ToString(nvlinkState)
}

func (nvlinkStates *NVLinkStates) JSON() ([]byte, error) {
	return json.Marshal(nvlinkStates)
}

// ToString convert nvlinkStates to a pretty-printed JSON string
func (nvlinkStates *NVLinkStates) ToString() string {
	jsonData, err := json.MarshalIndent(nvlinkStates, "", "  ")
	if err != nil {
		fmt.Printf("Error converting struct to JSON: %v\n", err)
		return ""
	}
	return string(jsonData)
}

func (nvlinkStates *NVLinkStates) Get(dev nvml.Device, uuid string) error {
	for link := 0; link < int(nvml.NVLINK_MAX_LINKS); link++ {
		var nvlinkState NVLinkState
		err := nvlinkState.Get(dev, uuid, link)
		if !errors.Is(err, nvml.SUCCESS) {
			if errors.Is(err, nvml.ERROR_INVALID_ARGUMENT) {
				// No more nvlink links to query
				break
			}
			if errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
				// No nvlink support or No more nvlink links to query
				break
			}
			return fmt.Errorf("error getting nvlink state: %v", err)
		} else {
			if nvlinkState.NVlinkSupported {
				nvlinkStates.NVLinkStates = append(nvlinkStates.NVLinkStates, nvlinkState)
			}
		}
	}
	nvlinkStates.NvlinkNum = len(nvlinkStates.NVLinkStates)
	nvlinkStates.NVlinkSupported = nvlinkStates.NvlinkNum > 0
	nvlinkStates.AllFeatureEnabled = nvlinkStates.getAllFeatureEnabled()
	return nil
}

func (nvlinkStates *NVLinkStates) getAllFeatureEnabled() bool {
	for _, nvlinkState := range nvlinkStates.NVLinkStates {
		if !nvlinkState.FeatureEnabled {
			return false
		}
	}
	return true
}

func (nvlinkState *NVLinkState) Get(device nvml.Device, uuid string, linkNo int) nvml.Return {
	nvlinkState.LinkNo = linkNo
	// Get NVLink feature enabled status, like nvidia-smi nvlink --status
	state, err := device.GetNvLinkState(linkNo)
	if errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		nvlinkState.NVlinkSupported = false
		return err
	} else if !errors.Is(err, nvml.SUCCESS) {
		// Ignore invalid argument error for non-existing links
		if !errors.Is(err, nvml.ERROR_INVALID_ARGUMENT) {
			logrus.Errorf("failed to get NVLink state for GPU %v link %d: %v", uuid, linkNo, err.String())
		}
		return err
	}
	nvlinkState.NVlinkSupported = true
	nvlinkState.FeatureEnabled = state == nvml.FEATURE_ENABLED
	return nvml.SUCCESS
}
