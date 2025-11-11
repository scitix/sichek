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

	"github.com/scitix/sichek/components/common"
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

func NewNvidiaCollector(ctx context.Context, nvmlInstPtr *nvml.Interface, expectedDeviceCount int) (*NvidiaCollector, error) {
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
		collector.ExpectedDeviceCount = expectedDeviceCount
		collector.DeviceUUIDs = make(map[int]string, expectedDeviceCount)
		collector.getUUID()
	} else {
		return nil, fmt.Errorf("failed to NewNvidiaCollector: %v", err)
	}
	return collector, nil
}

func (collector *NvidiaCollector) getUUID() {
	collector.UUIDAllValidFlag = true
	for i := 0; i < collector.ExpectedDeviceCount; i++ {
		device, err := (*collector.nvmlInst).DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			collector.UUIDAllValidFlag = false
			logrus.WithField("component", "NVIDIA-Collector-getUUID").Errorf("failed to get Nvidia GPU device %d: %v", i, err)
			return
		}
		uuid, err := device.GetUUID()
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "NVIDIA-Collector-getUUID").Errorf("failed to get UUID for GPU %d: %v", i, nvml.ErrorString(err))
			collector.UUIDAllValidFlag = false
		}
		collector.DeviceUUIDs[i] = uuid
	}
}

func (collector *NvidiaCollector) Name() string {
	return "NvidiaCollector"
}

func (collector *NvidiaCollector) GetCfg() common.ComponentUserConfig {
	return nil
}

func (collector *NvidiaCollector) Collect(ctx context.Context) (*NvidiaInfo, error) {
	if !collector.UUIDAllValidFlag {
		collector.getUUID()
	}

	nvidia := &NvidiaInfo{
		Time:                time.Now(),
		SoftwareInfo:        collector.softwareInfo,
		ValiddeviceUUIDFlag: collector.UUIDAllValidFlag,
		DeviceUUIDs:         collector.DeviceUUIDs,
	}

	// Get the number of devices
	numDevices, err := (*collector.nvmlInst).DeviceGetCount()
	if !errors.Is(err, nvml.SUCCESS) {
		return nil, fmt.Errorf("failed to get Nvidia GPU device count: %v", err)
	}
	nvidia.DeviceCount = numDevices

	// Get the device info
	nvidia.DevicesInfo = make([]DeviceInfo, numDevices)
	nvidia.DeviceUsedCount = 0
	for i := 0; i < numDevices; i++ {
		device, err := (*collector.nvmlInst).DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "NVIDIA-Collector-Collect").Errorf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		err2 := nvidia.DevicesInfo[i].Get(device, i, collector.softwareInfo.DriverVersion)
		if err2 != nil {
			logrus.WithField("component", "NVIDIA-Collector-Collect").Errorf("failed to get Nvidia GPU deviceInfo %d: %v", i, err2)
			continue
		}
		if nvidia.DevicesInfo[i].NProcess > 0 {
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
