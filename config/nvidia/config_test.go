// /*
// Copyright 2024 The Scitix Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package nvidia

// import (
// 	"encoding/json"
// 	"fmt"
// 	"os"
// 	"testing"
// )

// func TestLoadSpecFromYaml(t *testing.T) {
// 	// Create temporary files for testing
// 	specFile, err := os.CreateTemp("", "spec_*.yaml")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp spec file: %v", err)
// 	}
// 	defer os.Remove(specFile.Name())

// 	// Write sample data to the temporary files
// 	specData := `
// nvidia:
//   0x233010de:
//     name: NVIDIA H100 80GB HBM3
//     gpu_nums: 8
//     gpu_memory: 80
//     pcie:
//       pci_gen: 5
//       pci_width: 16
//     software:
//       driver_version: "535.129.03"
//       cuda_version: "12.0"
//       vbios_version: "96.00.89.00.01"
//       nvidiafabric_manager: "535.129.03"
//     dependence:
//       pcie-acs: disable
//       iommu: disable
//       nv-peermem: enable
//       nv_fabricmanager: active
//       cpu_performance: enable
//     MaxClock:
//       Graphics: 1410 # MHz
//       Memory: 1593 # MHz
//       SM: 1410 # MHz
//     nvlink:
//       nvlink_supported: true
//       active_nvlink_num: 12
//       total_replay_errors: 0
//       total_recovery_errors: 0
//       total_crc_errors: 0
//     state:
//       persistence: enable
//       pstate: 0
//     memory_errors_threshold:
//       remapped_uncorrectable_errors: 512
//       sram_volatile_uncorrectable_errors: 0
//       sram_aggregate_uncorrectable_errors: 4
//       sram_volatile_correctable_errors: 10000000
//       sram_aggregate_correctable_errors: 10000000
//     temperature_threshold:
//       gpu: 75
//       memory: 95
//   0x233010f7:
//     name: NVIDIA H100 80GB HBM3
//     gpu_nums: 8
//     gpu_memory: 80
//     pcie:
//       pci_gen: 5
//       pci_width: 16
//     software:
//       driver_version: "535.129.03"
//       cuda_version: "12.2"
//       vbios_version: 96.00.89.00.01
//       nvidiafabric_manager: "535.129.03"
//     dependence:
//       pcie-acs: disable
//       iommu: disable
//       nv-peermem: enable
//       nv_fabricmanager: active
//       cpu_performance: enable
//     MaxClock:
//       Graphics: 1410 # MHz
//       Memory: 1593 # MHz
//       SM: 1410 # MHz
//     nvlink:
//       nvlink_supported: true
//       active_nvlink_num: 12
//       total_replay_errors: 0
//       total_recovery_errors: 0
//       total_crc_errors: 0
//     state:
//       persistence: enable
//       pstate: 0
//     memory_errors_threshold:
//       remapped_uncorrectable_errors: 512
//       sram_volatile_uncorrectable_errors: 0
//       sram_aggregate_uncorrectable_errors: 4
//       sram_volatile_correctable_errors: 10000000
//       sram_aggregate_correctable_errors: 10000000
//     temperature_threshold:
//       gpu: 75
//       memory: 95
// infiniband:
//   tbd: tbd
// `
// 	if _, err := specFile.Write([]byte(specData)); err != nil {
// 		t.Fatalf("Failed to write to temp spec file: %v", err)
// 	}

// 	// Test the LoadSpecFromYaml function
// 	spec, err := LoadSpecFromYaml(specFile.Name())
// 	if err != nil {
// 		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
// 	}

// 	// Convert the config struct to a pretty-printed JSON string and print it
// 	jsonData, err := json.MarshalIndent(spec, "", "  ")
// 	if err != nil {
// 		t.Fatalf("Failed to marshal config to JSON: %v", err)
// 	}
// 	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

// 	// Validate the returned spec
// 	if len(spec) != 2 {
// 		t.Fatalf("Expected spec to have 2 entry, got %d", len(spec))
// 	}
// 	if _, ok := spec["0x233010de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x233010de', it doesn't exist")
// 	}
// 	if spec["0x233010de"].Name != "NVIDIA H100 80GB HBM3" {
// 		t.Fatalf("Expected Spec.Name to be 'NVIDIA H100 80GB HBM3', got '%s'", spec["0x233010de"].Name)
// 	}
// 	if spec["0x233010de"].Software.CUDAVersion != "12.0" {
// 		t.Fatalf("Expected Software.CUDAVersion to be '12.0', got '%s'", spec["0x233010de"].Software.CUDAVersion)
// 	}
// }

// func TestLoadSpecFromDefaultYaml(t *testing.T) {
// 	// Test the LoadSpecFromYaml function
// 	spec, err := LoadSpecFromYaml("")
// 	if err != nil {
// 		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
// 	}

// 	// Convert the config struct to a pretty-printed JSON string and print it
// 	jsonData, err := json.MarshalIndent(spec, "", "  ")
// 	if err != nil {
// 		t.Fatalf("Failed to marshal config to JSON: %v", err)
// 	}
// 	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

// 	// Validate the returned spec
// 	if len(spec) != 6 {
// 		t.Fatalf("Expected spec to have 6 entry, got %d", len(spec))
// 	}
// 	if _, ok := spec["0x233010de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x233010de', it doesn't exist")
// 	}
// 	if _, ok := spec["0x20b510de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x20b510de', it doesn't exist")
// 	}
// 	if _, ok := spec["0x20b210de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x20b210de', it doesn't exist")
// 	}
// 	if _, ok := spec["0x20f310de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x20f310de', it doesn't exist")
// 	}
// 	if _, ok := spec["0x26b510de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x26b510de', it doesn't exist")
// 	}
// 	if _, ok := spec["0x1df610de"]; !ok {
// 		t.Fatalf("Expected spec to have key '0x1df610de', it doesn't exist")
// 	}
// }

// func TestNvidiaConfig(t *testing.T) {
// 	// Create temporary files for testing
// 	specFile, err := os.CreateTemp("", "spec_*.yaml")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp spec file: %v", err)
// 	}
// 	defer os.Remove(specFile.Name())

// 	userConfigFile, err := os.CreateTemp("", "user_config_*.yaml")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp user config file: %v", err)
// 	}
// 	defer os.Remove(userConfigFile.Name())

// 	// Write sample data to the temporary files
// 	specData := `
// nvidia:
//   0x1df610de:
//     name: Tesla V100S-PCIE-32GB
//     gpu_nums: 8
//     gpu_memory: 80
//     pcie:
//       pci_gen: 5
//       pci_width: 16
//     software:
//       driver_version: "535.129.03"
//       cuda_version: "12.2"
//       vbios_version: 96.00.89.00.01
//       nvidiafabric_manager: "535.129.03"
//     dependence:
//       pcie-acs: disable
//       iommu: disable
//       nv-peermem: enable
//       nv_fabricmanager: active
//       cpu_performance: enable
//     MaxClock:
//       Graphics: 1410 # MHz
//       Memory: 1593 # MHz
//       SM: 1410 # MHz
//     nvlink:
//       nvlink_supported: true
//       active_nvlink_num: 12
//       total_replay_errors: 0
//       total_recovery_errors: 0
//       total_crc_errors: 0
//     state:
//       persistence: enable
//       pstate: 0
//     memory_errors_threshold:
//       remapped_uncorrectable_errors: 512
//       sram_volatile_uncorrectable_errors: 0
//       sram_aggregate_uncorrectable_errors: 4
//       sram_volatile_correctable_errors: 10000000
//       sram_aggregate_correctable_errors: 10000000
//     temperature_threshold:
//       gpu: 75
//       memory: 95
// `
// 	if _, err := specFile.Write([]byte(specData)); err != nil {
// 		t.Fatalf("Failed to write to temp spec file: %v", err)
// 	}

// 	userConfigData := `
// nvidia:
//   name: "nvidia"
//   query_interval: 10
//   cache_size: 10
//   ignored_checkers: ["cpu_performance"]
// `
// 	if _, err := userConfigFile.Write([]byte(userConfigData)); err != nil {
// 		t.Fatalf("Failed to write to temp user config file: %v", err)
// 	}

// 	// Test the NvidiaConfig function
// 	config := &NvidiaConfig{}
// 	config.LoadFromYaml(userConfigFile.Name(), specFile.Name())

// 	// Convert the config struct to a pretty-printed JSON string and print it
// 	jsonData, err := json.MarshalIndent(config, "", "  ")
// 	if err != nil {
// 		t.Fatalf("Failed to marshal config to JSON: %v", err)
// 	}
// 	fmt.Printf("Config JSON:\n%s\n", string(jsonData))

// 	// config, err := NewNvidiaConfig("", "")

// 	if err != nil {
// 		t.Fatalf("New() returned an error: %v", err)
// 	}

// 	// Validate the returned NvidiaConfig
// 	if config.Spec.Name != "Tesla V100S-PCIE-32GB" {
// 		t.Errorf("Expected Spec.Name to be 'Tesla V100S-PCIE-32GB', got '%s'", config.Spec.Name)
// 	}

// 	if config.ComponentConfig.Nvidia.Name != "nvidia" {
// 		t.Errorf("Expected ComponentConfig.Nvidia.Name to be 'nvidia', got '%s'", config.ComponentConfig.Nvidia.Name)
// 	}
// 	if config.ComponentConfig.Nvidia.QueryInterval != 1 {
// 		t.Errorf("Expected ComponentConfig.Nvidia.UpdateInterval to be 1, got %d", config.ComponentConfig.Nvidia.QueryInterval)
// 	}
// 	if config.ComponentConfig.Nvidia.CacheSize != 10 {
// 		t.Errorf("Expected ComponentConfig.Nvidia.CacheSize to be 10, got %d", config.ComponentConfig.Nvidia.CacheSize)
// 	}
// 	if len(config.ComponentConfig.Nvidia.IgnoredCheckers) != 1 {
// 		t.Errorf("Expected 1 ignored checkers, got %d", len(config.ComponentConfig.Nvidia.IgnoredCheckers))
// 	}
// }
