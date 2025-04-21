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
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestLoadSpecFromYaml(t *testing.T) {
	// Create temporary files for testing
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp spec file: %v", err)
		}
	}(specFile.Name())

	// Write sample data to the temporary files
	specData := `
nvidia:
  nvidia_spec:
    0x233010de:
      name: NVIDIA H100 80GB HBM3
      gpu_nums: 8
      gpu_memory: 80
      pcie:
        pci_gen: 5
        pci_width: 16
      software:
        driver_version: "535.129.03"
        cuda_version: "12.0"
        vbios_version: "96.00.89.00.01"
        nvidiafabric_manager: "535.129.03"
      dependence:
        pcie-acs: disable
        iommu: disable
        nv-peermem: enable
        nv_fabricmanager: active
        cpu_performance: enable
      MaxClock:
        Graphics: 1410 # MHz
        Memory: 1593 # MHz
        SM: 1410 # MHz
      nvlink:
        nvlink_supported: true
        active_nvlink_num: 12
        total_replay_errors: 0
        total_recovery_errors: 0
        total_crc_errors: 0
      state:
        persistence: enable
        pstate: 0
      memory_errors_threshold:
        remapped_uncorrectable_errors: 512
        sram_volatile_uncorrectable_errors: 0
        sram_aggregate_uncorrectable_errors: 4
        sram_volatile_correctable_errors: 10000000
        sram_aggregate_correctable_errors: 10000000
      temperature_threshold:
        gpu: 75
        memory: 95
    0x233010f7:
      name: NVIDIA H100 80GB HBM3
      gpu_nums: 8
      gpu_memory: 80
      pcie:
        pci_gen: 5
        pci_width: 16
      software:
        driver_version: "535.129.03"
        cuda_version: "12.2"
        vbios_version: 96.00.89.00.01
        nvidiafabric_manager: "535.129.03"
      dependence:
        pcie-acs: disable
        iommu: disable
        nv-peermem: enable
        nv_fabricmanager: active
        cpu_performance: enable
      MaxClock:
        Graphics: 1410 # MHz
        Memory: 1593 # MHz
        SM: 1410 # MHz
      nvlink:
        nvlink_supported: true
        active_nvlink_num: 12
        total_replay_errors: 0
        total_recovery_errors: 0
        total_crc_errors: 0
      state:
        persistence: enable
        pstate: 0
      memory_errors_threshold:
        remapped_uncorrectable_errors: 512
        sram_volatile_uncorrectable_errors: 0
        sram_aggregate_uncorrectable_errors: 4
        sram_volatile_correctable_errors: 10000000
        sram_aggregate_correctable_errors: 10000000
      temperature_threshold:
        gpu: 75
        memory: 95
infiniband:
  tbd: tbd
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	spec := &NvidiaSpecConfig{}
	// Test the LoadSpecFromYaml function
	err = spec.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
	}
	formatedNvidiaSpecsMap := make(map[string]*NvidiaSpecItem)
	for gpuId, nvidiaSpec := range spec.NvidiaSpec.NvidiaSpecMap {
		gpuIdHex := fmt.Sprintf("0x%x", gpuId)
		formatedNvidiaSpecsMap[gpuIdHex] = nvidiaSpec
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(spec.NvidiaSpec.NvidiaSpecMap) < 2 {
		t.Fatalf("Expected spec at least have 2 entry, got %d", len(spec.NvidiaSpec.NvidiaSpecMap))
	}
	if _, ok := formatedNvidiaSpecsMap["0x233010de"]; !ok {
		t.Fatalf("Expected spec to have key '0x233010de', it doesn't exist")
	}
	if formatedNvidiaSpecsMap["0x233010de"].Name != "NVIDIA H100 80GB HBM3" {
		t.Fatalf("Expected Spec.Name to be 'NVIDIA H100 80GB HBM3', got '%s'", formatedNvidiaSpecsMap["0x233010de"].Name)
	}
	if formatedNvidiaSpecsMap["0x233010de"].Software.CUDAVersion != "12.0" {
		t.Fatalf("Expected Software.CUDAVersion to be '12.0', got '%s'", formatedNvidiaSpecsMap["0x233010de"].Software.CUDAVersion)
	}
}

func TestLoadSpecFromDefaultYaml(t *testing.T) {
	// Test the LoadSpecFromYaml function
	spec := &NvidiaSpecConfig{}
	err := spec.LoadSpecConfigFromYaml("")
	if err != nil {
		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
	}
	formatedNvidiaSpecsMap := make(map[string]*NvidiaSpecItem)
	for gpuId, nvidiaSpec := range spec.NvidiaSpec.NvidiaSpecMap {
		gpuIdHex := fmt.Sprintf("0x%x", gpuId)
		formatedNvidiaSpecsMap[gpuIdHex] = nvidiaSpec
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(spec.NvidiaSpec.NvidiaSpecMap) < 1 {
		t.Fatalf("Expected spec at least have 1 entry, got %d", len(spec.NvidiaSpec.NvidiaSpecMap))
	}
	if _, ok := formatedNvidiaSpecsMap["0x233010de"]; !ok {
		t.Fatalf("Expected spec to have key '0x233010de', it doesn't exist")
	}
}

func TestNvidiaConfig(t *testing.T) {
	// Create temporary files for testing
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp spec file: %v", err)
		}
	}(specFile.Name())

	userConfigFile, err := os.CreateTemp("", "user_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp user config file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp user config file: %v", err)
		}
	}(userConfigFile.Name())

	// Write sample data to the temporary files
	specData := `
nvidia:
  nvidia_spec:
    0x1df610de:
      name: Tesla V100S-PCIE-32GB
      gpu_nums: 8
      gpu_memory: 80
      pcie:
        pci_gen: 5
        pci_width: 16
      software:
        driver_version: "535.129.03"
        cuda_version: "12.2"
        vbios_version: 96.00.89.00.01
        nvidiafabric_manager: "535.129.03"
      dependence:
        pcie-acs: disable
        iommu: disable
        nv-peermem: enable
        nv_fabricmanager: active
        cpu_performance: enable
      MaxClock:
        Graphics: 1410 # MHz
        Memory: 1593 # MHz
        SM: 1410 # MHz
      nvlink:
        nvlink_supported: true
        active_nvlink_num: 12
        total_replay_errors: 0
        total_recovery_errors: 0
        total_crc_errors: 0
      state:
        persistence: enable
        pstate: 0
      memory_errors_threshold:
        remapped_uncorrectable_errors: 512
        sram_volatile_uncorrectable_errors: 0
        sram_aggregate_uncorrectable_errors: 4
        sram_volatile_correctable_errors: 10000000
        sram_aggregate_correctable_errors: 10000000
      temperature_threshold:
        gpu: 75
        memory: 95
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	userConfigData := `
nvidia:
  name: "nvidia"
  query_interval: 30s
  cache_size: 5
  ignored_checkers: ["cpu_performance"]
`
	if _, err := userConfigFile.Write([]byte(userConfigData)); err != nil {
		t.Fatalf("Failed to write to temp user config file: %v", err)
	}

	// Test the NvidiaConfig function
	cfg := &NvidiaUserConfig{}
	err = common.LoadComponentUserConfig(userConfigFile.Name(), cfg)
	if err != nil || cfg.Nvidia == nil {
		t.Fatalf("Failed to load user config: %v", err)
	}
	spec := &NvidiaSpecConfig{}
	err = spec.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
	}
	specItem := spec.GetSpec("")
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("Config JSON:\n%s\n", string(jsonData))

	// Validate the returned NvidiaConfig
	if specItem.Name != "NVIDIA H100 80GB HBM3" {
		t.Errorf("Expected Spec.Name to be 'NVIDIA H100 80GB HBM3', got '%s'", specItem.Name)
	}

	if cfg.Nvidia.QueryInterval.Duration != 30*time.Second {
		t.Errorf("Expected ComponentConfig.Nvidia.UpdateInterval to be 1, got %d", cfg.Nvidia.QueryInterval)
	}
	if cfg.Nvidia.CacheSize != 5 {
		t.Errorf("Expected ComponentConfig.Nvidia.CacheSize to be 10, got %d", cfg.Nvidia.CacheSize)
	}
	if len(cfg.Nvidia.IgnoredCheckers) != 1 {
		t.Errorf("Expected 1 ignored checkers, got %d", len(cfg.Nvidia.IgnoredCheckers))
	}
}

func TestLoadComponentUserConfig_WithValidFile(t *testing.T) {
	// Create a temporary user config file
	userConfigFile, err := os.CreateTemp("", "user_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp user config file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp user config file: %v", err)
		}
	}(userConfigFile.Name())

	// Write sample data to the temporary file
	userConfigData := `
nvidia:
  query_interval: 1m
  cache_size: 5
  enable_metrics: true
  ignored_checkers: ["cpu_performance"]

memory:
  query_interval: 30s
  cache_size: 5
  enable_metrics: false
`
	if _, err := userConfigFile.Write([]byte(userConfigData)); err != nil {
		t.Fatalf("Failed to write to temp user config file: %v", err)
	}

	// Test the LoadComponentUserConfig function
	cfg := &NvidiaUserConfig{}
	err = common.LoadComponentUserConfig(userConfigFile.Name(), cfg)
	if err != nil {
		t.Fatalf("LoadComponentUserConfig() returned an error: %v", err)
	}

	// Validate the loaded configuration
	if cfg.Nvidia == nil {
		t.Fatalf("Expected Nvidia config to be non-nil")
	}
	if cfg.Nvidia.QueryInterval.Duration != 60*time.Second {
		t.Errorf("Expected QueryInterval to be 1m, got %s", cfg.Nvidia.QueryInterval.Duration)
	}
	if cfg.Nvidia.CacheSize != 5 {
		t.Errorf("Expected CacheSize to be 5, got %d", cfg.Nvidia.CacheSize)
	}
	if len(cfg.Nvidia.IgnoredCheckers) != 1 || cfg.Nvidia.IgnoredCheckers[0] != "cpu_performance" {
		t.Errorf("Expected IgnoredCheckers to contain 'cpu_performance', got %v", cfg.Nvidia.IgnoredCheckers)
	}
}

func TestLoadComponentUserConfig_WithInvalidFile(t *testing.T) {
	// Test with an invalid file path
	cfg := &NvidiaUserConfig{}
	err := common.LoadComponentUserConfig("invalid_file_path.yaml", cfg)
	// Validate the loaded configuration with default config values)
	if err != nil {
		t.Fatalf("Expected LoadComponentUserConfig() with default config, got: %v", err)
	}
	if cfg.Nvidia == nil {
		t.Fatalf("Expected Nvidia config to be non-nil")
	}
	if cfg.Nvidia.QueryInterval.Duration != 30*time.Second {
		t.Errorf("Expected QueryInterval to be 30s, got %v", cfg.Nvidia.QueryInterval.Duration)
	}
	if cfg.Nvidia.CacheSize != 5 {
		t.Errorf("Expected CacheSize to be 5, got %d", cfg.Nvidia.CacheSize)
	}
	if len(cfg.Nvidia.IgnoredCheckers) != 0{
		t.Errorf("Expected 0 IgnoredCheckers, got %d", len(cfg.Nvidia.IgnoredCheckers))
	}
}

func TestLoadComponentUserConfig_WithDefaultConfig(t *testing.T) {
	// Simulate the absence of a user-provided file to test loading the default config
	cfg := &NvidiaUserConfig{}
	err := common.LoadComponentUserConfig("", cfg)
	if err != nil {
		t.Fatalf("LoadComponentUserConfig() returned an error: %v", err)
	}

	// Validate the loaded configuration with default config values)
	if cfg.Nvidia == nil {
		t.Fatalf("Expected Nvidia config to be non-nil")
	}
	// Add additional assertions here based on the expected default configuration
	if cfg.Nvidia.QueryInterval.Duration != 30*time.Second {
		t.Errorf("Expected QueryInterval to be 30s, got %v", cfg.Nvidia.QueryInterval.Duration)
	}
	if cfg.Nvidia.CacheSize != 5 {
		t.Errorf("Expected CacheSize to be 5, got %d", cfg.Nvidia.CacheSize)
	}
	if len(cfg.Nvidia.IgnoredCheckers) != 0{
		t.Errorf("Expected 0 IgnoredCheckers, got %d", len(cfg.Nvidia.IgnoredCheckers))
	}
}

