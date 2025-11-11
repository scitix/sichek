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
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type NVLinkState struct {
	NVlinkSupported bool `json:"nvlink_supported" yaml:"nvlink_supported"`
	FeatureEnabled  bool `json:"feature_enabled" yaml:"feature_enabled"`
	LinkNo          int  `json:"link_no" yaml:"link_no"`
}

type NVLinkStates struct {
	NVlinkSupported   bool          `json:"nvlink_supported" yaml:"nvlink_supported"`
	AllFeatureEnabled bool          `json:"all_feature_enabled,omitempty" yaml:"all_feature_enabled,omitempty"`
	NvlinkNum         int           `json:"active_nvlink_num" yaml:"active_nvlink_num"`
	NVLinkStates      []NVLinkState `json:"nvlink_state,omitempty" yaml:"nvlink_state,omitempty"`
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
		logrus.Errorf("Error converting struct to JSON: %v", err)
		return ""
	}
	return string(jsonData)
}

func (nvlinkStates *NVLinkStates) Get(dev nvml.Device, uuid string) error {
	for link := 0; link < int(nvml.NVLINK_MAX_LINKS); link++ {
		var nvlinkState NVLinkState
		ret := nvlinkState.Get(dev, uuid, link)
		switch ret {
		case nvml.SUCCESS:
			if nvlinkState.NVlinkSupported {
				nvlinkStates.NVLinkStates = append(nvlinkStates.NVLinkStates, nvlinkState)
			}
		case nvml.ERROR_INVALID_ARGUMENT, nvml.ERROR_NOT_SUPPORTED:
			// No more nvlink links to query or no nvlink support
			break
		default:
			return fmt.Errorf("get nvlink state failed: %v", ret)
		}
	}
	// If we reach here, it means we have queried all links or encountered an error
	nvlinkStates.NvlinkNum = len(nvlinkStates.NVLinkStates)
	nvlinkStates.NVlinkSupported = nvlinkStates.NvlinkNum > 0
	nvlinkStates.AllFeatureEnabled = nvlinkStates.getAllFeatureEnabled()
	// logrus.Infof("Collector GPU %s NVLink states: %s", uuid, nvlinkStates.ToString())
	return nil
}

func (nvlinkStates *NVLinkStates) getAllFeatureEnabled() bool {
	for _, nvlinkState := range nvlinkStates.NVLinkStates {
		if !nvlinkState.FeatureEnabled {
			logrus.Warnf("NVLink link %d feature is supported but not enabled", nvlinkState.LinkNo)
			return false
		}
	}
	return true
}

func (nvlinkState *NVLinkState) Get(device nvml.Device, uuid string, linkNo int) nvml.Return {
	nvlinkState.LinkNo = linkNo
	// Get NVLink feature enabled status, like nvidia-smi nvlink --status
	enableState, ret := device.GetNvLinkState(linkNo)
	switch ret {
	case nvml.SUCCESS:
		nvlinkState.NVlinkSupported = true
		nvlinkState.FeatureEnabled = enableState == nvml.FEATURE_ENABLED
	case nvml.ERROR_NOT_SUPPORTED:
		nvlinkState.NVlinkSupported = false
		nvlinkState.FeatureEnabled = false
	case nvml.ERROR_INVALID_ARGUMENT:
		// Ignore invalid argument error for non-existing links
		logrus.Infof("GPU %s NVLink link %d is invalid or isActive is NULL ", uuid, linkNo)
	default:
		logrus.Errorf("error getting NVLink state for GPU %s link %d: %s", uuid, linkNo, ret.String())
	}
	return nvml.SUCCESS
}
