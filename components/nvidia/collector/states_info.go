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

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type StatesInfo struct {
	GpuPersistenceM string `json:"persistence" yaml:"persistence"`
	// TODO
	// GpuRstState     string `json:"gpu_reset_required"`
	GpuPstate uint32 `json:"pstate" yaml:"pstate"`
}

func (s *StatesInfo) JSON() ([]byte, error) {
	return common.JSON(s)
}

// ToString Convert struct to JSON (pretty-printed)
func (s *StatesInfo) ToString() string {
	return common.ToString(s)
}

func (s *StatesInfo) Get(device nvml.Device, uuid string) error {
	// Get GPU Persistence Mode
	persistenceMode, err := device.GetPersistenceMode()
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get GPU persistence mode for GPU %v: %v", uuid, err)
	}
	if persistenceMode == nvml.FEATURE_ENABLED {
		s.GpuPersistenceM = "enable"
	} else {
		s.GpuPersistenceM = "disable"
	}

	// Get GPU Reset State
	//     resetState, err := device.ResetRequired()
	//       if err != nvml.SUCCESS {
	//           return nil, fmt.Errorf("failed to get GPU reset state: %v", err)
	//       }
	//       if resetState == nvml.FEATURE_ENABLED {
	//           statesInfo.GpuRstState = "required"
	//       } else {
	//           statesInfo.GpuRstState = "not required"
	//       }

	// Get GPU Performance State (P-State)
	pstate, err := device.GetPerformanceState()
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get GPU performance state for GPU %v: %v", uuid, err)
	}
	s.GpuPstate = uint32(pstate)

	return nil
}
