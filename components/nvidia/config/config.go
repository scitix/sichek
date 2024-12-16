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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"

	"sigs.k8s.io/yaml"
)

type NvidiaConfig struct {
	Spec            NvidiaSpec
	ComponentConfig ComponentConfig
}

func (c *NvidiaConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *NvidiaConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *NvidiaConfig) LoadFromYaml(userConfig string, specConfig string) error {
	spec, err := LoadSpecConfig(specConfig)
	if err != nil {
		return err
	}
	c.Spec = *spec
	err = c.ComponentConfig.LoadFromYaml(userConfig)
	if err != nil {
		return err
	}
	return nil
}

func NewNvidiaConfig(userConfig string, specFile string) (*NvidiaConfig, error) {
	spec, err := LoadSpecConfig(specFile)
	if err != nil {
		return nil, err
	}

	componentConfig := &ComponentConfig{}
	err = componentConfig.LoadFromYaml(userConfig)
	if err != nil {
		return nil, err
	}

	return &NvidiaConfig{
		Spec:            *spec,
		ComponentConfig: *componentConfig,
	}, nil
}

type NvidiaSpec struct {
	Name                 string                 `json:"name"`
	GpuNums              int                    `json:"gpu_nums"`
	GpuMemory            int                    `json:"gpu_memory"`
	GpuMemoryBandwidth   int                    `json:"gpu_memory_bandwidth,omitempty"`
	PCIe                 collector.PCIeInfo     `json:"pcie,omitempty"`
	Dependence           Dependence             `json:"dependence"`
	Software             collector.SoftwareInfo `json:"software"`
	Nvlink               collector.NVLinkStates `json:"nvlink"`
	State                collector.StatesInfo   `json:"state"`
	MemoryErrorThreshold MemoryErrorThreshold   `json:"memory_errors_threshold"`
	TemperatureThreshold TemperatureThreshold   `json:"temperature_threshold"`
	CriticalXidEvents    map[int]string         `json:"critical_xid_events"`
}

func (s NvidiaSpec) JSON() (string, error) {
	data, err := json.Marshal(s)
	return string(data), err
}

func (s NvidiaSpec) Yaml() (string, error) {
	data, err := yaml.Marshal(s)
	return string(data), err
}

func (s NvidiaSpec) LoadFromYaml(file string) error {
	_, err := LoadSpecConfig("")
	if err != nil {
		return err
	}
	return nil
}

type Dependence struct {
	PcieAcs        string `json:"pcie-acs"`
	Iommu          string `json:"iommu"`
	NvidiaPeermem  string `json:"nv-peermem"`
	FabricManager  string `json:"nv_fabricmanager"`
	CpuPerformance string `json:"cpu_performance"`
}

type MemoryErrorThreshold struct {
	RemappedUncorrectableErrors      uint64 `json:"remapped_uncorrectable_errors"`
	SRAMVolatileUncorrectableErrors  uint64 `json:"sram_volatile_uncorrectable_errors"`
	SRAMAggregateUncorrectableErrors uint64 `json:"sram_aggregate_uncorrectable_errors"`
	SRAMVolatileCorrectableErrors    uint64 `json:"sram_volatile_correctable_errors"`
	SRAMAggregateCorrectableErrors   uint64 `json:"sram_aggregate_correctable_errors"`
}

type TemperatureThreshold struct {
	Gpu    int `json:"gpu"`
	Memory int `json:"memory"`
}

func LoadSpecConfig(specFile string) (*NvidiaSpec, error) {
	if specFile == "" {
		_, err := os.Stat("/var/sichek/nvidia/default_spec.yaml")
		if err == nil {
			// run in pod use /var/sichek/nvidia/default_spec.yaml
			specFile = "/var/sichek/nvidia/default_spec.yaml"
		} else {
			// run on host use local config
			_, curFile, _, ok := runtime.Caller(0)
			if !ok {
				return nil, fmt.Errorf("get curr file path failed")
			}

			specFile = filepath.Dir(curFile) + "/default_spec.yaml"
		}
	}
	data, err := os.ReadFile(specFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config NvidiaSpec
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &config, nil
}

type ComponentConfig struct {
	Name            string        `json:"name"`
	QueryInterval   time.Duration `json:"query_interval"`
	CacheSize       int64         `json:"cache_size"`
	IgnoredCheckers []string      `json:"ignored_checkers,omitempty"`
}

func (c *ComponentConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	// for _, name := range c.IgnoredCheckers {
	// 	commonCfgMap[name] = {}
	// }
	return commonCfgMap
}

func (c *ComponentConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}

func (c *ComponentConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *ComponentConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *ComponentConfig) LoadFromYaml(filename string) error {
	if filename == "" {
		_, err := os.Stat("/var/sichek/nvidia/default_user_config.yaml")
		if err == nil {
			// run in pod use /var/sichek/nvidia/default_user_config.yaml
			filename = "/var/sichek/nvidia/default_user_config.yaml"
		} else {
			// run on host use local config
			_, curFile, _, ok := runtime.Caller(0)
			if !ok {
				return fmt.Errorf("get curr file path failed")
			}

			filename = filepath.Dir(curFile) + "/default_user_config.yaml"
		}
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}

// Map of NVIDIA Device IDs to Product Names
var NvidiaDeviceNames = map[string]string{
	"0x233010de": "NVIDIA H100 80GB HBM3",
	"0x20b510de": "NVIDIA A100 80GB PCIe",
	"0x20f310de": "NVIDIA A800-SXM4-80GB",
	"0x26b510de": "NVIDIA L40",
	"0x1df610de": "Tesla V100S-PCIE-32GB",
}

// Map of NVIDIA Device IDs to Product Names
var SupportedNvidiaDeviceSpecYaml = map[string]string{
	"0x233010de": "default_h100_spec.yaml",
	"0x20b510de": "default_a100_spec.yaml",
	"0x20f310de": "default_a800_spec.yaml",
	"0x26b510de": "default_l40_spec.yaml",
	"0x1df610de": "default_v100s_spec.yaml",
}

func GetSpec(deviceID string) (string, error) {
	specName := ""
	specFile := ""
	for device, spec := range SupportedNvidiaDeviceSpecYaml {
		if deviceID == device {
			specName = spec
			break
		}
	}
	if specName == "" {
		panic(fmt.Sprintf("unsupported NVIDIA product name, right now only support %v\n", SupportedNvidiaDeviceSpecYaml))
	}

	specFile = "/var/sichek/nvidia/" + specName
	_, err := os.Stat(specFile)
	if err == nil {
		// run in pod use /var/sichek/nvidia/${specName}
		return specFile, nil
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", fmt.Errorf("get curr file path failed")
		}
		specFile = filepath.Dir(curFile) + "/config/" + specName
		_, err := os.Stat(specFile)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}

	}

	return specFile, nil
}