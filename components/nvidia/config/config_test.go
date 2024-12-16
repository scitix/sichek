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
	"testing"
)

func TestNew(t *testing.T) {
	// Create temporary files for testing
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer os.Remove(specFile.Name())

	userConfigFile, err := os.CreateTemp("", "user_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp user config file: %v", err)
	}
	defer os.Remove(userConfigFile.Name())

	// Write sample data to the temporary files
	specData := `
name: NVIDIA H100 80GB HBM3
gpu_nums: 8
gpu_memory: 80
pcie:
  pci_gen: 5
  pci_width: 16
software:
  driver_version: 535.129.03
  cuda_version: 12.2
  vbios_version: 96.00.89.00.01
  nvidiafabric_manager: 535.129.03
dependence:
  pcie-acs: disable
  iommu: disable
  nv-peermem: enable
  nv_fabricmanager: active
  cpu_performance: enable
memory_errors_threshold:
  correctable_errors: 512
  uncorrectable_errors: 0
temperature_threshold:
  gpu: 75
  memory: 75
critical_xid_events:
  48: "DBE (Double Bit Error) ECC Error"
  63: "ECC Page Retirement or Row Remapping"
  64: "ECC Page Retirement or Row Remapping"
  92: "High single-bit ECC error rate"
  74: "NVLink Error"
  79: "GPU has fallen off the bus"
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	userConfigData := `
name: "nvidia"
update_interval: 1
cache_size: 10
ignored_checkers: ["cpu_performance"]
`
	if _, err := userConfigFile.Write([]byte(userConfigData)); err != nil {
		t.Fatalf("Failed to write to temp user config file: %v", err)
	}

	// Test the New function
	// config, _ := NewNvidiaConfig(userConfigFile.Name(), specFile.Name())
	config := &NvidiaConfig{}
	err = config.LoadFromYaml(userConfigFile.Name(), "")

	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("Config JSON:\n%s\n", string(jsonData))

	// config, err := NewNvidiaConfig("", "")

	if err != nil {
		t.Fatalf("New() returned an error: %v", err)
	}

	// Validate the returned NvidiaConfig
	if config.Spec.Name != "NVIDIA H100 80GB HBM3" {
		t.Errorf("Expected Spec.Name to be 'NVIDIA H100 80GB HBM3', got '%s'", config.Spec.Name)
	}

	if config.ComponentConfig.Name != "nvidia" {
		t.Errorf("Expected ComponentConfig.Name to be 'nvidia', got '%s'", config.ComponentConfig.Name)
	}
	if config.ComponentConfig.QueryInterval != 1 {
		t.Errorf("Expected ComponentConfig.UpdateInterval to be 10, got %d", config.ComponentConfig.QueryInterval)
	}
	if config.ComponentConfig.CacheSize != 10 {
		t.Errorf("Expected ComponentConfig.CacheSize to be 100, got %d", config.ComponentConfig.CacheSize)
	}
	if len(config.ComponentConfig.IgnoredCheckers) != 1 {
		t.Errorf("Expected 2 ignored checkers, got %d", len(config.ComponentConfig.IgnoredCheckers))
	}
}

func TestGetSpec(t *testing.T) {
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to get current file")
	}
	// set the current file as the root path
	defaultConfigPath := filepath.Dir(curFile) + "/config/"

	tests := []struct {
		productName string
		expected    string
		expectError bool
	}{
		{"0x233010de", defaultConfigPath + "default_h100_spec.yaml", false},
		{"0x20b510de", defaultConfigPath + "default_a100_pcie_spec.yaml", false},
		{"0x20f310de", defaultConfigPath + "default_a800_spec.yaml", false},
		{"0x26b510de", defaultConfigPath + "default_l40_spec.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.productName, func(t *testing.T) {
			specFile, err := GetSpec(tt.productName)
			if (err != nil) != tt.expectError {
				t.Errorf("getSpec() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if specFile != tt.expected {
				t.Errorf("getSpec() = %v, expected %v", specFile, tt.expected)
			}
		})
	}
}
