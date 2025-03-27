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
	NVlinkSupported      bool `json:"nvlink_supported"`
	FeatureEnabled       bool `json:"feature_enabled"`
	LinkNo               int  `json:"link_no"`
	ThroughputRawRxBytes int  `json:"throughput_raw_rx_bytes"`
	ThroughputRawTxBytes int  `json:"throughput_raw_tx_bytes"`
	// ThroughputDataRxBytes int    `json:"throughput_data_rx_bytes"`
	// ThroughputDataTxBytes int    `json:"throughput_data_tx_bytes"`
	ReplayErrors   uint64 `json:"replay_errors"`
	RecoveryErrors uint64 `json:"recovery_errors"`
	CRCErrors      uint64 `json:"crc_errors"`
}

type NVLinkStates struct {
	NVlinkSupported     bool   `json:"nvlink_supported"`
	AllFeatureEnabled   bool   `json:"all_feature_enabled,omitempty"`
	NvlinkNum           int    `json:"active_nvlink_num"`
	TotalReplayErrors   uint64 `json:"total_replay_errors"`
	TotalRecoveryErrors uint64 `json:"total_recovery_errors"`
	TotalCRCErrors      uint64 `json:"total_crc_errors"`

	NVLinkStates []NVLinkState `json:"nvlink_state,omitempty"`
}

func (nvlinkState *NVLinkState) JSON() ([]byte, error) {
	return common.JSON(nvlinkState)
}

// Convert struct to JSON (pretty-printed)
func (nvlinkState *NVLinkState) ToString() string {
	return common.ToString(nvlinkState)
}

func (nvlinkStates *NVLinkStates) JSON() ([]byte, error) {
	return json.Marshal(nvlinkStates)
}

// convert nvlinkStates to a pretty-printed JSON string
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
		if err != nvml.SUCCESS {
			if err == nvml.ERROR_INVALID_ARGUMENT {
				// No more nvlink links to query
				break
			}
			if err == nvml.ERROR_NOT_SUPPORTED {
				// No nvlink support or No more nvlink links to query
				break
			}
			return fmt.Errorf("Error getting nvlink state: %v", err)
		} else {
			if nvlinkState.NVlinkSupported {
				nvlinkStates.NVLinkStates = append(nvlinkStates.NVLinkStates, nvlinkState)
			}
		}
	}
	nvlinkStates.NvlinkNum = len(nvlinkStates.NVLinkStates)
	nvlinkStates.NVlinkSupported = nvlinkStates.NvlinkNum > 0
	nvlinkStates.AllFeatureEnabled = nvlinkStates.getAllFeatureEnabled()
	nvlinkStates.TotalReplayErrors = nvlinkStates.getTotalRelayErrors()
	nvlinkStates.TotalRecoveryErrors = nvlinkStates.getTotalRecoveryErrors()
	nvlinkStates.TotalCRCErrors = nvlinkStates.getTotalCRCErrors()
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

func (nvlinkStates *NVLinkStates) getTotalRelayErrors() uint64 {
	var total uint64
	for _, nvlinkState := range nvlinkStates.NVLinkStates {
		total += nvlinkState.ReplayErrors
	}
	return total
}

func (nvlinkStates *NVLinkStates) getTotalRecoveryErrors() uint64 {
	var total uint64
	for _, nvlinkState := range nvlinkStates.NVLinkStates {
		total += nvlinkState.RecoveryErrors
	}
	return total
}

func (nvlinkStates *NVLinkStates) getTotalCRCErrors() uint64 {
	var total uint64
	for _, nvlinkState := range nvlinkStates.NVLinkStates {
		total += nvlinkState.CRCErrors
	}
	return total
}

func (nvlinkState *NVLinkState) Get(device nvml.Device, uuid string, linkNo int) nvml.Return {
	nvlinkState.LinkNo = linkNo
	// Get NVLink feature enabled status, like nvidia-smi nvlink --status
	state, err := device.GetNvLinkState(linkNo)
	if err == nvml.ERROR_NOT_SUPPORTED {
		nvlinkState.NVlinkSupported = false
		return err
	} else if err != nvml.SUCCESS {
		// Ignore invalid argument error for non-existing links
		if err != nvml.ERROR_INVALID_ARGUMENT {
			logrus.Errorf("failed to get NVLink state for GPU %v link %d: %v", uuid, linkNo, err.String())
		}
		return err
	}
	nvlinkState.NVlinkSupported = true
	nvlinkState.FeatureEnabled = state == nvml.FEATURE_ENABLED

	if nvlinkState.FeatureEnabled {
		// Get NVLink replay errors
		// replayErrors, err := device.GetNvLinkErrorCounter(linkNo, nvml.NVLINK_ERROR_DL_REPLAY)
		// if err != nvml.SUCCESS {
		// 	logrus.Errorf("failed to get NVLink replay errors for GPU %v link %d: %v", uuid, linkNo, err.String())
		// 	return err
		// }
		// nvlinkState.ReplayErrors = replayErrors

		// // Get NVLink recovery errors
		// recoveryErrors, err := device.GetNvLinkErrorCounter(linkNo, nvml.NVLINK_ERROR_DL_RECOVERY)
		// if err != nvml.SUCCESS {
		// 	logrus.Errorf("failed to get NVLink recovery errors for GPU %v link %d: %v", uuid, linkNo, err.String())
		// 	return err
		// }
		// nvlinkState.RecoveryErrors = recoveryErrors

		// // Get NVLink CRC errors
		// crcErrors, err := device.GetNvLinkErrorCounter(linkNo, nvml.NVLINK_ERROR_DL_CRC_FLIT)
		// if err != nvml.SUCCESS {
		// 	logrus.Errorf("failed to get NVLink CRC errors for GPU %v link %d: %v", uuid, linkNo, err.String())
		// 	return err
		// }
		// nvlinkState.CRCErrors = crcErrors

		// Get NVLink throughput raw TX bytes
		// ref. https://docs.nvidia.com/deploy/nvml-api/group__NvLink.html#group__NvLink_1gd623d8eaf212205fd282abbeb8f8c395
		// ref. https://github.com/NVIDIA/go-nvml/blob/main/pkg/nvml/nvml.h#L1929
		// values := []nvml.FieldValue{
		// 	{FieldId: nvml.FI_DEV_NVLINK_THROUGHPUT_RAW_TX},
		// 	{FieldId: nvml.FI_DEV_NVLINK_THROUGHPUT_RAW_RX},
		// }
		// err = device.GetFieldValues(values)
		// if err == nvml.SUCCESS {
		// 	nvlinkState.ThroughputRawTxBytes = int(binary.LittleEndian.Uint64(values[0].Value[:]))
		// 	nvlinkState.ThroughputRawRxBytes = int(binary.LittleEndian.Uint64(values[1].Value[:]))
		// } else {
		// 	nvlinkState.ThroughputRawTxBytes = 0
		// 	nvlinkState.ThroughputRawRxBytes = 0
		// 	logrus.Errorf("failed to get NVLink throughput raw TX/RX bytes for GPU %v link %d: %v", uuid, linkNo, err.String())
		// 	return err
		// }
	}
	return nvml.SUCCESS
}
