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
	CurGraphicsClk string `json:"cur_graphics_clk"`
	AppGraphicsClk string `json:"app_graphics_clk"`
	MaxGraphicsClk string `json:"max_graphics_clk"`
	CurMemoryClk   string `json:"cur_memory_clk"`
	AppMemoryClk   string `json:"app_memory_clk"`
	MaxMemoryClk   string `json:"max_memory_clk"`
	CurSMClk       string `json:"cur_sm_clk"`
	AppSMClk       string `json:"app_sm_clk"`
	MaxSMClk       string `json:"max_sm_clk"`
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
	curGraphicsClk, err := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current graphics clock for GPU %v: %v", uuid, err)
	}
	clk.CurGraphicsClk = fmt.Sprintf("%d MHz", curGraphicsClk)

	graphicsClkSet, err := device.GetApplicationsClock(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application graphics clock setting for GPU %v: %v", uuid, err)
	}
	clk.AppGraphicsClk = fmt.Sprintf("%d MHz", graphicsClkSet)

	maxGraphicsClk, err := device.GetMaxClockInfo(nvml.CLOCK_GRAPHICS)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max graphics clock for GPU %v: %v", uuid, err)
	}
	clk.MaxGraphicsClk = fmt.Sprintf("%d MHz", maxGraphicsClk)

	return nil
}

func (clk *ClockInfo) getMemoryClocks(device nvml.Device, uuid string) error {
	curMemClock, err := device.GetClockInfo(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current memory clock for GPU %v: %v", uuid, err)
	}
	clk.CurMemoryClk = fmt.Sprintf("%d MHz", curMemClock)

	memClockSet, err := device.GetApplicationsClock(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application memory clock setting for GPU %v: %v", uuid, err)
	}
	clk.AppMemoryClk = fmt.Sprintf("%d MHz", memClockSet)

	maxMemClock, err := device.GetMaxClockInfo(nvml.CLOCK_MEM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max memory clock for GPU %v: %v", uuid, err)
	}
	clk.MaxMemoryClk = fmt.Sprintf("%d MHz", maxMemClock)
	return nil
}

func (clk *ClockInfo) getSMClocks(device nvml.Device, uuid string) error {
	curSMClock, err := device.GetClockInfo(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current SM clock for GPU %v: %v", uuid, err)
	}
	clk.CurSMClk = fmt.Sprintf("%d MHz", curSMClock)

	smClockSet, err := device.GetApplicationsClock(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get application SM clock setting for GPU %v: %v", uuid, err)
	}
	clk.AppSMClk = fmt.Sprintf("%d MHz", smClockSet)

	maxSMClock, err := device.GetMaxClockInfo(nvml.CLOCK_SM)
	if !errors.Is(err, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max SM clock for GPU %v: %v", uuid, err)
	}
	clk.MaxSMClk = fmt.Sprintf("%d MHz", maxSMClock)
	return nil
}
