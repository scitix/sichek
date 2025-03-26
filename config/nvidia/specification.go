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
package nvidia

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/yaml"
)

type NvidiaSpec struct {
	nvidiaSpecMap map[int32]*NvidiaSpecItem
	// Other fileds like `infiniband` can be added here if needed
}

type NvidiaSpecItem struct {
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

func (s *NvidiaSpec) GetSpec(specFile string) *NvidiaSpecItem {
	for gpu_id, nvidiaSpec := range s.nvidiaSpecMap {
		logrus.WithField("component", "NVIDIA").Infof("parsed spec for gpu_id: 0x%x", gpu_id)
		gpu_id_hex := fmt.Sprintf("0x%x", gpu_id)
		nvidiaSpecsMap[gpu_id_hex] = nvidiaSpec
	}
	// specs, err := LoadSpecFromYaml(specFile)
	// if err != nil {
	// 	panic(fmt.Errorf("failed to load spec file: %v", err))
	// }
	deviceID := getDeviceID()
	if _, ok := specs[deviceID]; !ok {
		panic("failed to find spec file for deviceID: " + deviceID)
	}
	return specs[deviceID]
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

// LoadSpecFromYaml loads nvidia specifications from a single yaml file or multiple yaml files in default directory.
func LoadSpecFromYaml(specFile string) (map[string]*NvidiaSpecItem, error) {
	nvidiaSpecsMap := make(map[string]*NvidiaSpecItem)
	if specFile != "" {
		// Parse from the specified file
		if err := parseSpecYamlFile(specFile, nvidiaSpecsMap); err != nil {
			return nil, err
		}
	} else {
		// Parse from the default directory
		specDirs := []string{
			"/var/sichek/nvidia", // directory for production environment
			getLocalSpecDir(),    // Local directory relative to the source code for development
		}

		foundFile := false
		for _, dir := range specDirs {
			files, err := os.ReadDir(dir)
			if err != nil {
				continue // Skip if the directory does not exist
			}
			foundFile = true
			for _, file := range files {
				if strings.HasSuffix(file.Name(), "spec.yaml") {
					specFile := filepath.Join(dir, file.Name())
					if err := parseSpecYamlFile(specFile, nvidiaSpecsMap); err != nil {
						return nil, err
					}
				}
			}
		}

		if !foundFile {
			return nil, fmt.Errorf("failed to find spec file in %v", specDirs)
		}
	}
	return nvidiaSpecsMap, nil
}

// getLocalSpecDir returns the directory of the spec yaml files.
func getLocalSpecDir() string {
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("unable to determine src dirctory path, return current directory")
		return "."
	}
	return filepath.Dir(curFile)
}

// parseSpecYamlFile read and parses a single YAML file into a map of NvidiaSpec.
func parseSpecYamlFile(specFile string, nvidiaSpecsMap map[string]*NvidiaSpecItem) error {
	data, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", specFile, err)
	}
	var spec NvidiaSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to unmarshal file %s: %w", specFile, err)
	}
	// Merge the parsed spec into the main nvidiaSpec map
	for gpu_id, nvidiaSpec := range spec.nvidiaSpecMap {
		logrus.WithField("component", "NVIDIA").Infof("parsed spec for gpu_id: 0x%x", gpu_id)
		gpu_id_hex := fmt.Sprintf("0x%x", gpu_id)
		nvidiaSpecsMap[gpu_id_hex] = nvidiaSpec
	}
	return nil
}
