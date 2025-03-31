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
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type NvidiaSpecConfig struct {
	NvidiaSpec *NvidiaSpec `json:"nvidia" yaml:"nvidia"`
}

type NvidiaSpec struct {
	NvidiaSpecMap map[int32]*NvidiaSpecItem `json:"nvidia_spec" yaml:"nvidia_spec"`
	// Other fileds like `infiniband` can be added here if needed
}

type NvidiaSpecItem struct {
	Name                 string                 `json:"name" yaml:"name"`
	GpuNums              int                    `json:"gpu_nums" yaml:"gpu_nums"`
	GpuMemory            int                    `json:"gpu_memory" yaml:"gpu_memory"`
	GpuMemoryBandwidth   int                    `json:"gpu_memory_bandwidth,omitempty" yaml:"gpu_memory_bandwidth,omitempty"`
	PCIe                 collector.PCIeInfo     `json:"pcie,omitempty" yaml:"pcie,omitempty"`
	Dependence           Dependence             `json:"dependence" yaml:"dependence"`
	Software             collector.SoftwareInfo `json:"software" yaml:"software"`
	Nvlink               collector.NVLinkStates `json:"nvlink" yaml:"nvlink"`
	State                collector.StatesInfo   `json:"state" yaml:"state"`
	MemoryErrorThreshold MemoryErrorThreshold   `json:"memory_errors_threshold" yaml:"memory_errors_threshold"`
	TemperatureThreshold TemperatureThreshold   `json:"temperature_threshold" yaml:"temperature_threshold"`
	CriticalXidEvents    map[int]string         `json:"critical_xid_events" yaml:"critical_xid_events"`
}

type Dependence struct {
	PcieAcs        string `json:"pcie-acs" yaml:"pcie-acs"`
	Iommu          string `json:"iommu" yaml:"iommu"`
	NvidiaPeermem  string `json:"nv-peermem" yaml:"nv-peermem"`
	FabricManager  string `json:"nv_fabricmanager" yaml:"nv_fabricmanager"`
	CpuPerformance string `json:"cpu_performance" yaml:"cpu_performance"`
}

type MemoryErrorThreshold struct {
	RemappedUncorrectableErrors      uint64 `json:"remapped_uncorrectable_errors" yaml:"remapped_uncorrectable_errors"`
	SRAMVolatileUncorrectableErrors  uint64 `json:"sram_volatile_uncorrectable_errors" yaml:"sram_volatile_uncorrectable_errors"`
	SRAMAggregateUncorrectableErrors uint64 `json:"sram_aggregate_uncorrectable_errors" yaml:"sram_aggregate_uncorrectable_errors"`
	SRAMVolatileCorrectableErrors    uint64 `json:"sram_volatile_correctable_errors" yaml:"sram_volatile_correctable_errors"`
	SRAMAggregateCorrectableErrors   uint64 `json:"sram_aggregate_correctable_errors" yaml:"sram_aggregate_correctable_errors"`
}

type TemperatureThreshold struct {
	Gpu    int `json:"gpu" yaml:"gpu"`
	Memory int `json:"memory" yaml:"memory"	`
}

func (s *NvidiaSpecConfig) GetSpec() *NvidiaSpecItem {
	formatedNvidiaSpecsMap := make(map[string]*NvidiaSpecItem)
	for gpu_id, nvidiaSpec := range s.NvidiaSpec.NvidiaSpecMap {
		logrus.WithField("component", "NVIDIA").Infof("parsed spec for gpu_id: 0x%x", gpu_id)
		gpu_id_hex := fmt.Sprintf("0x%x", gpu_id)
		formatedNvidiaSpecsMap[gpu_id_hex] = nvidiaSpec
	}
	deviceID := getDeviceID()
	if _, ok := formatedNvidiaSpecsMap[deviceID]; !ok {
		panic("failed to find spec file for deviceID: " + deviceID)
	}
	return formatedNvidiaSpecsMap[deviceID]
}

func (s *NvidiaSpecConfig) LoadSpecConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, s)
		if err != nil {
			return fmt.Errorf("failed to load nvidia spec from YAML file %s: %v", file, err)
		}
	}
	err := s.LoadDefaultSpec()
	if err != nil || s.NvidiaSpec == nil {
		return fmt.Errorf("failed to load default nvidia spec: %v", err)
	}
	return nil
}

func (s *NvidiaSpecConfig) LoadDefaultSpec() error {
	if s.NvidiaSpec == nil {
		s.NvidiaSpec = &NvidiaSpec{
			NvidiaSpecMap: make(map[int32]*NvidiaSpecItem),
		}
	}
	defaultCfgDirPath, files, err := common.GetDefaultConfigFiles(consts.ComponentNameNvidia)
	if err != nil {
		return fmt.Errorf("failed to get default nvidia config files: %v", err)
	}
	// 遍历文件并加载符合条件的 YAML 文件
	for _, file := range files {
		if strings.HasSuffix(file.Name(), consts.DefaultSpecCfgSuffix) {
			nvidiaSpec := &NvidiaSpecConfig{}
			filePath := filepath.Join(defaultCfgDirPath, file.Name())
			err := utils.LoadFromYaml(filePath, nvidiaSpec)
			if err != nil || nvidiaSpec.NvidiaSpec == nil {
				return fmt.Errorf("failed to load nvidia spec from YAML file %s: %v", filePath, err)
			}
			for gpu_id, nvidiaSpec := range nvidiaSpec.NvidiaSpec.NvidiaSpecMap {
				if _, ok := s.NvidiaSpec.NvidiaSpecMap[gpu_id]; !ok {
					s.NvidiaSpec.NvidiaSpecMap[gpu_id] = nvidiaSpec
				}
			}

		}
	}
	return nil
}

func getDeviceID() string {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); ret != nvml.SUCCESS {
		panic(fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret)))

	}
	defer nvmlInst.Shutdown()

	// In case of GPU error, iterate through all GPUs to find the first valid one
	deviceCount, err := nvmlInst.DeviceGetCount()
	if err != nvml.SUCCESS {
		panic(fmt.Errorf("Failed to get device count: %s\n", nvml.ErrorString(err)))
	}
	var deviceID string
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		if err != nvml.SUCCESS {
			logrus.WithField("component", "NVIDIA").Errorf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		pciInfo, err := device.GetPciInfo()
		if err != nvml.SUCCESS {
			logrus.WithField("component", "NVIDIA").Errorf("failed to get PCIe Info  for NVIDIA GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		deviceID = fmt.Sprintf("0x%x", pciInfo.PciDeviceId)
		return deviceID
	}
	panic(fmt.Errorf("failed to get product name for NVIDIA GPU"))
}
