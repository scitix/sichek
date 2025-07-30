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
	"errors"
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

type NvidiaSpec struct {
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
	CriticalXidEvents    map[int]string         `json:"critical_xid_events,omitempty" yaml:"critical_xid_events,omitempty"`
	Perf                 PerfMetrics            `json:"perf,omitempty" yaml:"perf,omitempty"`
}

type NvidiaSpecs struct {
	Specs map[string]*NvidiaSpec `json:"nvidia" yaml:"nvidia"`
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
	Memory int `json:"memory" yaml:"memory"`
}

type PerfMetrics struct {
	NcclAllReduceBw float64 `json:"nccl-all-reduce-bw" yaml:"nccl-all-reduce-bw"`
}

func LoadSpec(file string) (*NvidiaSpec, error) {
	s := &NvidiaSpecs{}
	// 1. Load spec from provided file
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err == nil && s.Specs != nil {
			return FilterSpec(s)
		} else {
			logrus.WithField("component", "nvidia").Warnf("%v", err)
		}
	}
	// 2. try to Load default spec from production env if no file specified
	// e.g., /var/sichek/config/default_spec.yaml
	err := s.tryLoadFromDefault()
	if err == nil && s.Specs != nil {
		spec, err := FilterSpec(s)
		if err == nil {
			return spec, nil
		}
		logrus.WithField("component", "nvidia").Warnf("failed to filter specs from default production top spec")
	} else {
		logrus.WithField("component", "nvidia").Warnf("%v", err)
	}

	// 3. try to load default spec from default config directory
	// for production env, it checks the default config path (e.g., /var/sichek/config/xx-component).
	// for development env, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
	err = s.tryLoadFromDevConfig()
	if err == nil && s.Specs != nil {
		return FilterSpec(s)
	} else {
		if err != nil {
			logrus.WithField("component", "nvidia").Warnf("failed to load from default dev directory: %v", err)
		} else {
			logrus.WithField("component", "nvidia").Warnf("default dev spec loaded but contains no nvidia section")
		}
	}
	return nil, fmt.Errorf("failed to load NVIDIA spec from any source, please check the configuration")
}

func (s *NvidiaSpecs) tryLoadFromFile(file string) error {
	if file == "" {
		return fmt.Errorf("file path is empty")
	}
	err := utils.LoadFromYaml(file, s)
	if err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}

	if s.Specs == nil {
		return fmt.Errorf("YAML file %s loaded but contains no nvidia section", file)
	}
	logrus.WithField("component", "nvidia").Infof("loaded default spec")
	return nil
}

func (s *NvidiaSpecs) tryLoadFromDefault() error {
	specs := &NvidiaSpecs{}
	err := common.LoadSpecFromProductionPath(specs)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if specs.Specs == nil {
		return fmt.Errorf("default production top spec loaded but contains no nvidia section")
	}

	if s.Specs == nil {
		s.Specs = make(map[string]*NvidiaSpec)
	}

	for id, spec := range specs.Specs {
		if _, ok := s.Specs[id]; !ok {
			s.Specs[id] = spec
		}
	}
	logrus.WithField("component", "nvidia").Infof("loaded default production top spec")
	return nil
}

func (s *NvidiaSpecs) tryLoadFromDevConfig() error {
	defaultDevCfgDirPath, files, err := common.GetDevDefaultConfigFiles(consts.ComponentNameNvidia)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), consts.DefaultSpecSuffix) {
				specs := &NvidiaSpecs{}
				filePath := filepath.Join(defaultDevCfgDirPath, file.Name())
				err := utils.LoadFromYaml(filePath, specs)
				if err != nil || specs.Specs == nil {
					// If the file is not found or does not contain HCA specs, log the error
					// and continue to the next file.
					logrus.WithField("component", "nvidia").Warnf("failed to load nvidia spec from YAML file %s: %v", filePath, err)
					continue
				}
				if s.Specs == nil {
					s.Specs = make(map[string]*NvidiaSpec)
				}
				for id, spec := range specs.Specs {
					if _, ok := s.Specs[id]; !ok {
						s.Specs[id] = spec
					}
				}
			}
		}
	}
	return err
}

// FilterSpec retrieves the NVIDIA spec for the local GPU device ID.
func FilterSpec(s *NvidiaSpecs) (*NvidiaSpec, error) {
	localDeviceID, err := GetDeviceID()
	if err != nil {
		return nil, err
	}
	var nvSpec *NvidiaSpec
	if spec, ok := s.Specs[localDeviceID]; ok {
		nvSpec = spec
	} else {
		nvidiaSpec := &NvidiaSpecs{}
		url := fmt.Sprintf("%s/%s/%s.yaml", consts.DefaultOssCfgPath, consts.ComponentNameNvidia, localDeviceID)
		logrus.WithField("component", "nvidia").Infof("Loading spec from OSS for gpu %s: %s", localDeviceID, url)
		err := common.LoadSpecFromOss(url, nvidiaSpec)
		if err == nil && nvidiaSpec.Specs != nil {
			if spec, ok := nvidiaSpec.Specs[localDeviceID]; ok {
				nvSpec = spec
			} else {
				return nil, fmt.Errorf("failed to find NVIDIA spec for local GPU %s in OSS", localDeviceID)
			}
		} else {
			return nil, fmt.Errorf("failed to load NVIDIA spec from OSS: %v", err)
		}
	}
	return nvSpec, nil
}

func GetDeviceID() (string, error) {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); !errors.Is(ret, nvml.SUCCESS) {
		return "", fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))

	}
	defer nvmlInst.Shutdown()

	// In case of GPU error, iterate through all GPUs to find the first valid one
	deviceCount, err := nvmlInst.DeviceGetCount()
	if !errors.Is(err, nvml.SUCCESS) {
		return "", fmt.Errorf("failed to get device count: %s", nvml.ErrorString(err))
	}
	var deviceID string
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Errorf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		pciInfo, err := device.GetPciInfo()
		if !errors.Is(err, nvml.SUCCESS) {
			logrus.WithField("component", "nvidia").Errorf("failed to get PCIe Info  for NVIDIA GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		deviceID = fmt.Sprintf("0x%x", pciInfo.PciDeviceId)
		return deviceID, nil
	}
	return "", fmt.Errorf("failed to get product name for NVIDIA GPU")
}
