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

type ClockInfo struct {
	CurGraphicsClk uint32 `json:"cur_graphics_clk" yaml:"cur_graphics_clk"`
	AppGraphicsClk uint32 `json:"app_graphics_clk" yaml:"app_graphics_clk"`
	MaxGraphicsClk uint32 `json:"max_graphics_clk" yaml:"max_graphics_clk"`
	CurMemoryClk   uint32 `json:"cur_memory_clk" yaml:"cur_memory_clk"`
	AppMemoryClk   uint32 `json:"app_memory_clk" yaml:"app_memory_clk"`
	MaxMemoryClk   uint32 `json:"max_memory_clk" yaml:"max_memory_clk"`
	CurSMClk       uint32 `json:"cur_sm_clk" yaml:"cur_sm_clk"`
	AppSMClk       uint32 `json:"app_sm_clk" yaml:"app_sm_clk"`
	MaxSMClk       uint32 `json:"max_sm_clk" yaml:"max_sm_clk"`
}

func (clk *ClockInfo) JSON() ([]byte, error) {
	return common.JSON(clk)
}

// ToString Convert struct to JSON (pretty-printed)
func (clk *ClockInfo) ToString() string {
	return common.ToString(clk)
}

func (clk *ClockInfo) Get(device nvml.Device, uuid string) error {
	err := clk.getGraphicsClocks(device, uuid)
	if err != nil {
		return err
	}
	err = clk.getMemoryClocks(device, uuid)
	if err != nil {
		return err
	}
	err = clk.getSMClocks(device, uuid)
	if err != nil {
		return err
	}
	return nil
}

func (clk *ClockInfo) getGraphicsClocks(device nvml.Device, uuid string) error {
	var err nvml.Return
	clk.CurGraphicsClk, err = device.GetClockInfo(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current graphics clock for GPU %v: %v", uuid, err)
	}

	clk.AppGraphicsClk, err = device.GetApplicationsClock(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application graphics clock setting for GPU %v: %v", uuid, err)
	}

	clk.MaxGraphicsClk, err = device.GetMaxClockInfo(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max graphics clock for GPU %v: %v", uuid, err)
	}

	return nil
}

func (clk *ClockInfo) getMemoryClocks(device nvml.Device, uuid string) error {
	var err nvml.Return
	clk.CurMemoryClk, err = device.GetClockInfo(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current memory clock for GPU %v: %v", uuid, err)
	}

	clk.AppMemoryClk, err = device.GetApplicationsClock(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application memory clock setting for GPU %v: %v", uuid, err)
	}

	clk.MaxMemoryClk, err = device.GetMaxClockInfo(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max memory clock for GPU %v: %v", uuid, err)
	}
	return nil
}

func (clk *ClockInfo) getSMClocks(device nvml.Device, uuid string) error {
	var err nvml.Return
	clk.CurSMClk, err = device.GetClockInfo(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current SM clock for GPU %v: %v", uuid, err)
	}

	clk.AppSMClk, err = device.GetApplicationsClock(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application SM clock setting for GPU %v: %v", uuid, err)
	}

	clk.MaxSMClk, err = device.GetMaxClockInfo(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max SM clock for GPU %v: %v", uuid, err)
	}
	return nil
}
