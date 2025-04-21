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

	"github.com/scitix/sichek/components/nvidia/config"
)

func TestLoadSpecConfigFromYaml(t *testing.T) {
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
hang:
  name: "GPUHang"
  description: "GPU Hang"
  duration_threshold: 2m30s
  level: warn
  check_items:
    power:
      threshold: 100
      compare: low
    gclk:
      threshold: 1400
      compare: high
    smclk:
      threshold: 1400
      compare: high
    sm_util:
      threshold: 95
      compare: high
    mem_util:
      threshold: 5
      compare: low
    pviol:
      threshold: 5
      compare: low
    rxpci:
      threshold: 20 # MB/s
      compare: low
    txpci:
      threshold: 20 # MB/s
      compare: low
  check_items_by_model:
    - model: "0x233010de"
      override:
        power:
          threshold: 150
          compare: low
        gclk:
          threshold: 1900
          compare: high
        smclk:
          threshold: 1900
          compare: high
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	spec := &HangSpecConfig{}
	// Test the LoadSpecFromYaml function
	err = spec.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	gpuHangSpec := spec.HangSpec
	if gpuHangSpec.Name != "GPUHang" {
		t.Fatalf("Expected spec name to be 'GPUHang', got '%s'", gpuHangSpec.Name)
	}
	if len(gpuHangSpec.Indicators) != 8 {
		t.Fatalf("Expected spec to have 8 Indicators, got %d", len(gpuHangSpec.Indicators))
	}
	if len(gpuHangSpec.IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuHangSpec.IndicatorsByModel))
	}
	if gpuHangSpec.Description != "GPU Hang" {
		t.Fatalf("Expected spec description to be 'GPU Hang', got '%s'", gpuHangSpec.Description)
	}
	if gpuHangSpec.DurationThreshold.Duration != 150*time.Second {
		// Note: The duration is 2m30s, which is 150 seconds
		t.Fatalf("Expected spec duration threshold to be 150s, got %v", gpuHangSpec.DurationThreshold.Duration)
	}
	if gpuHangSpec.Level != "warn" {
		t.Fatalf("Expected spec level to be 'warn', got '%s'", gpuHangSpec.Level)
	}
	if gpuHangSpec.Indicators["power"].Threshold != 100 {
		t.Fatalf("Expected spec power threshold to be 100, got %d", gpuHangSpec.Indicators["power"].Threshold)
	}
	if gpuHangSpec.Indicators["power"].CompareType != "low" {
		t.Fatalf("Expected spec power compare type to be 'low', got '%s'", gpuHangSpec.Indicators["power"].CompareType)
	}
	if gpuHangSpec.Indicators["gclk"].Threshold != 1400 {
		t.Fatalf("Expected spec gclk threshold to be 1400, got %d", gpuHangSpec.Indicators["gclk"].Threshold)
	}
	if gpuHangSpec.Indicators["gclk"].CompareType != "high" {
		t.Fatalf("Expected spec gclk compare type to be 'high', got '%s'", gpuHangSpec.Indicators["gclk"].CompareType)
	}
	if gpuHangSpec.Indicators["smclk"].Threshold != 1400 {
		t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuHangSpec.Indicators["smclk"].Threshold)
	}
	if gpuHangSpec.Indicators["smclk"].CompareType != "high" {
		t.Fatalf("Expected spec smclk compare type to be 'high', got '%s'", gpuHangSpec.Indicators["smclk"].CompareType)
	}
	if gpuHangSpec.Indicators["sm_util"].Threshold != 95 {
		t.Fatalf("Expected spec sm_util threshold to be 95, got %d", gpuHangSpec.Indicators["sm_util"].Threshold)
	}
	if gpuHangSpec.Indicators["sm_util"].CompareType != "high" {
		t.Fatalf("Expected spec sm_util compare type to be 'high', got '%s'", gpuHangSpec.Indicators["sm_util"].CompareType)
	}
	if gpuHangSpec.Indicators["mem_util"].Threshold != 5 {
		t.Fatalf("Expected spec mem_util threshold to be 5, got %d", gpuHangSpec.Indicators["mem_util"].Threshold)
	}
	if gpuHangSpec.Indicators["mem_util"].CompareType != "low" {
		t.Fatalf("Expected spec mem_util compare type to be 'low', got '%s'", gpuHangSpec.Indicators["mem_util"].CompareType)
	}
	if gpuHangSpec.Indicators["pviol"].Threshold != 5 {
		t.Fatalf("Expected spec pviol threshold to be 5, got %d", gpuHangSpec.Indicators["pviol"].Threshold)
	}
	if gpuHangSpec.Indicators["pviol"].CompareType != "low" {
		t.Fatalf("Expected spec pviol compare type to be 'low', got '%s'", gpuHangSpec.Indicators["pviol"].CompareType)
	}
	if gpuHangSpec.Indicators["rxpci"].Threshold != 20 {
		t.Fatalf("Expected spec rxpci threshold to be 20, got %d", gpuHangSpec.Indicators["rxpci"].Threshold)
	}
	if gpuHangSpec.Indicators["rxpci"].CompareType != "low" {
		t.Fatalf("Expected spec rxpci compare type to be 'low', got '%s'", gpuHangSpec.Indicators["rxpci"].CompareType)
	}
	if gpuHangSpec.Indicators["txpci"].Threshold != 20 {
		t.Fatalf("Expected spec txpci threshold to be 20, got %d", gpuHangSpec.Indicators["txpci"].Threshold)
	}
	if gpuHangSpec.Indicators["txpci"].CompareType != "low" {
		t.Fatalf("Expected spec txpci compare type to be 'low', got '%s'", gpuHangSpec.Indicators["txpci"].CompareType)
	}
	if gpuHangSpec.IndicatorsByModel[0].Model != "0x233010de" {
		t.Fatalf("Expected first model to be '0x233010de', got '%s'", gpuHangSpec.IndicatorsByModel[0].Model)
	}
	if len(gpuHangSpec.IndicatorsByModel[0].Override) != 3 {
		t.Fatalf("Expected first model to have 3 overrides, got %d", len(gpuHangSpec.IndicatorsByModel[0].Override))
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["power"].Threshold != 150 {
		t.Fatalf("Expected spec power override threshold to be 150, got %d", gpuHangSpec.IndicatorsByModel[0].Override["power"].Threshold)
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["gclk"].Threshold != 1900 {
		t.Fatalf("Expected spec gclk override threshold to be 1900, got %d", gpuHangSpec.IndicatorsByModel[0].Override["gclk"].Threshold)
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["smclk"].Threshold != 1900 {
		t.Fatalf("Expected spec smclk override threshold to be 1900, got %d", gpuHangSpec.IndicatorsByModel[0].Override["smclk"].Threshold)
	}
}

func TestGetSpec(t *testing.T) {
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
hang:
  name: "GPUHang"
  description: "GPU Hang"
  duration_threshold: 2m30s
  level: warn
  check_items:
    power:
      threshold: 100
      compare: low
    gclk:
      threshold: 1400
      compare: high
    smclk:
      threshold: 1400
      compare: high
    sm_util:
      threshold: 95
      compare: high
    mem_util:
      threshold: 5
      compare: low
    pviol:
      threshold: 5
      compare: low
    rxpci:
      threshold: 20 # MB/s
      compare: low
    txpci:
      threshold: 20 # MB/s
      compare: low
  check_items_by_model:
    - model: "0x233010de"
      override:
        power:
          threshold: 150
          compare: low
        gclk:
          threshold: 1900
          compare: high
        smclk:
          threshold: 1900
          compare: high
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	spec := &HangSpecConfig{}
	// Test the GetSpec function
	err = spec.GetSpec(specFile.Name())
	if err != nil {
		t.Fatalf("GetSpec() returned an error: %v", err)
	}

	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	device := config.GetDeviceID()
	gpuHangSpec := spec.HangSpec
	if gpuHangSpec.Name != "GPUHang" {
		t.Fatalf("Expected spec name to be 'GPUHang', got '%s'", gpuHangSpec.Name)
	}
	if len(gpuHangSpec.Indicators) != 8 {
		t.Fatalf("Expected spec to have 8 Indicators, got %d", len(gpuHangSpec.Indicators))
	}
	if len(gpuHangSpec.IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuHangSpec.IndicatorsByModel))
	}
	if gpuHangSpec.Description != "GPU Hang" {
		t.Fatalf("Expected spec description to be 'GPU Hang', got '%s'", gpuHangSpec.Description)
	}
	if gpuHangSpec.DurationThreshold.Duration != 150*time.Second {
		// Note: The duration is 2m30s, which is 150 seconds
		t.Fatalf("Expected spec duration threshold to be 150s, got %v", gpuHangSpec.DurationThreshold.Duration)
	}
	if gpuHangSpec.Level != "warn" {
		t.Fatalf("Expected spec level to be 'warn', got '%s'", gpuHangSpec.Level)
	}
	if device == "0x233010de" {
		if gpuHangSpec.Indicators["power"].Threshold != 150 {
			t.Fatalf("Expected spec power threshold to be 150, got %d", gpuHangSpec.Indicators["power"].Threshold)
		}
	} else {
		if gpuHangSpec.Indicators["power"].Threshold != 100 {
			t.Fatalf("Expected spec power threshold to be 100, got %d", gpuHangSpec.Indicators["power"].Threshold)
		}
	}
	if gpuHangSpec.Indicators["power"].CompareType != "low" {
		t.Fatalf("Expected spec power compare type to be 'low', got '%s'", gpuHangSpec.Indicators["power"].CompareType)
	}
	if device == "0x233010de" {
		if gpuHangSpec.Indicators["gclk"].Threshold != 1900 {
			t.Fatalf("Expected spec gclk threshold to be 1900, got %d", gpuHangSpec.Indicators["gclk"].Threshold)
		}
	} else {
		if gpuHangSpec.Indicators["gclk"].Threshold != 1400 {
			t.Fatalf("Expected spec gclk threshold to be 1400, got %d", gpuHangSpec.Indicators["gclk"].Threshold)
		}
	}
	if gpuHangSpec.Indicators["gclk"].CompareType != "high" {
		t.Fatalf("Expected spec gclk compare type to be 'high', got '%s'", gpuHangSpec.Indicators["gclk"].CompareType)
	}
	if device == "0x233010de" {
		if gpuHangSpec.Indicators["smclk"].Threshold != 1900 {
			t.Fatalf("Expected spec smclk threshold to be 1900, got %d", gpuHangSpec.Indicators["smclk"].Threshold)
		}
	} else {
		if gpuHangSpec.Indicators["smclk"].Threshold != 1400 {
			t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuHangSpec.Indicators["smclk"].Threshold)
		}
	}
	if gpuHangSpec.Indicators["smclk"].CompareType != "high" {
		t.Fatalf("Expected spec smclk compare type to be 'high', got '%s'", gpuHangSpec.Indicators["smclk"].CompareType)
	}
	if gpuHangSpec.Indicators["sm_util"].Threshold != 95 {
		t.Fatalf("Expected spec sm_util threshold to be 95, got %d", gpuHangSpec.Indicators["sm_util"].Threshold)
	}
	if gpuHangSpec.Indicators["sm_util"].CompareType != "high" {
		t.Fatalf("Expected spec sm_util compare type to be 'high', got '%s'", gpuHangSpec.Indicators["sm_util"].CompareType)
	}
	if gpuHangSpec.Indicators["mem_util"].Threshold != 5 {
		t.Fatalf("Expected spec mem_util threshold to be 5, got %d", gpuHangSpec.Indicators["mem_util"].Threshold)
	}
	if gpuHangSpec.Indicators["mem_util"].CompareType != "low" {
		t.Fatalf("Expected spec mem_util compare type to be 'low', got '%s'", gpuHangSpec.Indicators["mem_util"].CompareType)
	}
	if gpuHangSpec.Indicators["pviol"].Threshold != 5 {
		t.Fatalf("Expected spec pviol threshold to be 5, got %d", gpuHangSpec.Indicators["pviol"].Threshold)
	}
	if gpuHangSpec.Indicators["pviol"].CompareType != "low" {
		t.Fatalf("Expected spec pviol compare type to be 'low', got '%s'", gpuHangSpec.Indicators["pviol"].CompareType)
	}
	if gpuHangSpec.Indicators["rxpci"].Threshold != 20 {
		t.Fatalf("Expected spec rxpci threshold to be 20, got %d", gpuHangSpec.Indicators["rxpci"].Threshold)
	}
	if gpuHangSpec.Indicators["rxpci"].CompareType != "low" {
		t.Fatalf("Expected spec rxpci compare type to be 'low', got '%s'", gpuHangSpec.Indicators["rxpci"].CompareType)
	}
	if gpuHangSpec.Indicators["txpci"].Threshold != 20 {
		t.Fatalf("Expected spec txpci threshold to be 20, got %d", gpuHangSpec.Indicators["txpci"].Threshold)
	}
	if gpuHangSpec.Indicators["txpci"].CompareType != "low" {
		t.Fatalf("Expected spec txpci compare type to be 'low', got '%s'", gpuHangSpec.Indicators["txpci"].CompareType)
	}
	if gpuHangSpec.IndicatorsByModel[0].Model != "0x233010de" {
		t.Fatalf("Expected first model to be '0x233010de', got '%s'", gpuHangSpec.IndicatorsByModel[0].Model)
	}
	if len(gpuHangSpec.IndicatorsByModel[0].Override) != 3 {
		t.Fatalf("Expected first model to have 3 overrides, got %d", len(gpuHangSpec.IndicatorsByModel[0].Override))
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["power"].Threshold != 150 {
		t.Fatalf("Expected spec power override threshold to be 150, got %d", gpuHangSpec.IndicatorsByModel[0].Override["power"].Threshold)
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["gclk"].Threshold != 1900 {
		t.Fatalf("Expected spec gclk override threshold to be 1900, got %d", gpuHangSpec.IndicatorsByModel[0].Override["gclk"].Threshold)
	}
	if gpuHangSpec.IndicatorsByModel[0].Override["smclk"].Threshold != 1900 {
		t.Fatalf("Expected spec smclk override threshold to be 1900, got %d", gpuHangSpec.IndicatorsByModel[0].Override["smclk"].Threshold)
	}
}
