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

	"github.com/scitix/sichek/components/nvidia/collector"
	nvutils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
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
	PcieAcs        string `json:"pcie_acs" yaml:"pcie_acs"`
	Iommu          string `json:"iommu" yaml:"iommu"`
	NvidiaPeermem  string `json:"nv_peermem" yaml:"nv_peermem"`
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

// LoadSpec loads NVIDIA spec from the given file path.
// The file path is expected to be already resolved by the command layer (e.g. via spec.EnsureSpecFile).
func LoadSpec(file string) (*NvidiaSpec, error) {
	if file == "" {
		return nil, fmt.Errorf("nvidia spec file path is empty")
	}
	s := &NvidiaSpecs{}
	if err := utils.LoadFromYaml(file, s); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}
	// Note: No need to check if s.Specs is nil, it will be checked in FilterSpec
	return FilterSpec(s)
}

// FilterSpec retrieves the NVIDIA spec for the local GPU device ID.
func FilterSpec(s *NvidiaSpecs) (*NvidiaSpec, error) {
	localDeviceID, err := nvutils.GetDeviceID()
	logrus.WithField("component", "nvidia").Infof("local GPU device ID: %s", localDeviceID)
	if err != nil {
		return nil, err
	}
	var nvSpec *NvidiaSpec
	if spec, ok := s.Specs[localDeviceID]; ok {
		nvSpec = spec
		logrus.WithField("component", "nvidia").Infof("NVIDIA spec for local GPU %s found in provided specs: %s", localDeviceID, nvSpec.Name)
	} else {
		logrus.WithField("component", "nvidia").Infof("NVIDIA spec for local GPU %s not found in provided specs, current gpu spec:", localDeviceID)
		for gpu, spec := range s.Specs {
			logrus.WithField("component", "nvidia").Infof("    %s: %s", gpu, spec.Name)
		}
		specURL := httpclient.GetSichekSpecURL()
		if specURL == "" {
			return nil, fmt.Errorf("NVIDIA spec for local GPU %s not found in provided specs and SICHEK_SPEC_URL environment variable is not set", localDeviceID)
		}
		nvidiaSpec := &NvidiaSpecs{}
		url := fmt.Sprintf("%s/%s/%s.yaml", specURL, consts.ComponentNameNvidia, localDeviceID)
		logrus.WithField("component", "nvidia").Infof("Loading spec for gpu %s from %s", localDeviceID, url)
		err := httpclient.LoadSpecFromURL(url, nvidiaSpec)
		if err == nil && nvidiaSpec.Specs != nil {
			if spec, ok := nvidiaSpec.Specs[localDeviceID]; ok {
				nvSpec = spec
			} else {
				return nil, fmt.Errorf("failed to find NVIDIA spec for local GPU %s from remote URL %s", localDeviceID, url)
			}
		} else {
			return nil, fmt.Errorf("failed to load NVIDIA spec from remote URL %s: %v", url, err)
		}
	}
	return nvSpec, nil
}
