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
    pwr:
      threshold: 100
      compare: low
    gclk:
      threshold: 1400
      compare: high
    smclk:
      threshold: 1400
      compare: high
    sm:
      threshold: 95
      compare: high
    mem:
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
    - model: "0x233010dex"
      override:
        pwr:
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
	// Test the LoadSpecFromYaml function
	gpuHangRule, err := LoadEventRules(specFile.Name())
	if err != nil {
		t.Fatalf("LoadSpecFromYaml() returned an error: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(gpuHangRule, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if gpuHangRule.Name != "GPUHang" {
		t.Fatalf("Expected spec name to be 'GPUHang', got '%s'", gpuHangRule.Name)
	}
	if len(gpuHangRule.Indicators) != 8 {
		t.Fatalf("Expected spec to have 8 Indicators, got %d", len(gpuHangRule.Indicators))
	}
	if len(gpuHangRule.IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuHangRule.IndicatorsByModel))
	}
	if gpuHangRule.Description != "GPU Hang" {
		t.Fatalf("Expected spec description to be 'GPU Hang', got '%s'", gpuHangRule.Description)
	}
	if gpuHangRule.DurationThreshold.Duration != 150*time.Second {
		// Note: The duration is 2m30s, which is 150 seconds
		t.Fatalf("Expected spec duration threshold to be 150s, got %v", gpuHangRule.DurationThreshold.Duration)
	}
	if gpuHangRule.Level != "warn" {
		t.Fatalf("Expected spec level to be 'warn', got '%s'", gpuHangRule.Level)
	}
	if gpuHangRule.Indicators["pwr"].Threshold != 100 {
		t.Fatalf("Expected spec pwr threshold to be 100, got %d", gpuHangRule.Indicators["pwr"].Threshold)
	}
	if gpuHangRule.Indicators["pwr"].CompareType != "low" {
		t.Fatalf("Expected spec pwr compare type to be 'low', got '%s'", gpuHangRule.Indicators["pwr"].CompareType)
	}
	if gpuHangRule.Indicators["gclk"].Threshold != 1400 {
		t.Fatalf("Expected spec gclk threshold to be 1400, got %d", gpuHangRule.Indicators["gclk"].Threshold)
	}
	if gpuHangRule.Indicators["gclk"].CompareType != "high" {
		t.Fatalf("Expected spec gclk compare type to be 'high', got '%s'", gpuHangRule.Indicators["gclk"].CompareType)
	}
	if gpuHangRule.Indicators["smclk"].Threshold != 1400 {
		t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuHangRule.Indicators["smclk"].Threshold)
	}
	if gpuHangRule.Indicators["smclk"].CompareType != "high" {
		t.Fatalf("Expected spec smclk compare type to be 'high', got '%s'", gpuHangRule.Indicators["smclk"].CompareType)
	}
	if gpuHangRule.Indicators["sm"].Threshold != 95 {
		t.Fatalf("Expected spec sm threshold to be 95, got %d", gpuHangRule.Indicators["sm"].Threshold)
	}
	if gpuHangRule.Indicators["sm"].CompareType != "high" {
		t.Fatalf("Expected spec sm compare type to be 'high', got '%s'", gpuHangRule.Indicators["sm"].CompareType)
	}
	if gpuHangRule.Indicators["mem"].Threshold != 5 {
		t.Fatalf("Expected spec mem threshold to be 5, got %d", gpuHangRule.Indicators["mem"].Threshold)
	}
	if gpuHangRule.Indicators["mem"].CompareType != "low" {
		t.Fatalf("Expected spec mem compare type to be 'low', got '%s'", gpuHangRule.Indicators["mem"].CompareType)
	}
	if gpuHangRule.Indicators["pviol"].Threshold != 5 {
		t.Fatalf("Expected spec pviol threshold to be 5, got %d", gpuHangRule.Indicators["pviol"].Threshold)
	}
	if gpuHangRule.Indicators["pviol"].CompareType != "low" {
		t.Fatalf("Expected spec pviol compare type to be 'low', got '%s'", gpuHangRule.Indicators["pviol"].CompareType)
	}
	if gpuHangRule.Indicators["rxpci"].Threshold != 20 {
		t.Fatalf("Expected spec rxpci threshold to be 20, got %d", gpuHangRule.Indicators["rxpci"].Threshold)
	}
	if gpuHangRule.Indicators["rxpci"].CompareType != "low" {
		t.Fatalf("Expected spec rxpci compare type to be 'low', got '%s'", gpuHangRule.Indicators["rxpci"].CompareType)
	}
	if gpuHangRule.Indicators["txpci"].Threshold != 20 {
		t.Fatalf("Expected spec txpci threshold to be 20, got %d", gpuHangRule.Indicators["txpci"].Threshold)
	}
	if gpuHangRule.Indicators["txpci"].CompareType != "low" {
		t.Fatalf("Expected spec txpci compare type to be 'low', got '%s'", gpuHangRule.Indicators["txpci"].CompareType)
	}
	if gpuHangRule.IndicatorsByModel[0].Model != "0x233010dex" {
		t.Fatalf("Expected first model to be '0x233010dex', got '%s'", gpuHangRule.IndicatorsByModel[0].Model)
	}
	if len(gpuHangRule.IndicatorsByModel[0].Override) != 3 {
		t.Fatalf("Expected first model to have 3 overrides, got %d", len(gpuHangRule.IndicatorsByModel[0].Override))
	}
	if gpuHangRule.IndicatorsByModel[0].Override["pwr"].Threshold != 150 {
		t.Fatalf("Expected spec pwr override threshold to be 150, got %d", gpuHangRule.IndicatorsByModel[0].Override["pwr"].Threshold)
	}
	if gpuHangRule.IndicatorsByModel[0].Override["gclk"].Threshold != 1900 {
		t.Fatalf("Expected spec gclk override threshold to be 1900, got %d", gpuHangRule.IndicatorsByModel[0].Override["gclk"].Threshold)
	}
	if gpuHangRule.IndicatorsByModel[0].Override["smclk"].Threshold != 1900 {
		t.Fatalf("Expected spec smclk override threshold to be 1900, got %d", gpuHangRule.IndicatorsByModel[0].Override["smclk"].Threshold)
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
    pwr:
      threshold: 100
      compare: low
    gclk:
      threshold: 1400
      compare: high
    smclk:
      threshold: 1400
      compare: high
    sm:
      threshold: 95
      compare: high
    mem:
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
        pwr:
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
	spec := &HangEventRules{}
	// Test the GetSpec function
	gpuHangRule, err := LoadEventRules(specFile.Name())
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
	device, err := config.GetDeviceID()
	if err != nil {
		t.Fatalf("Failed to GetDeviceID: %v", err)
	}
	if gpuHangRule.Name != "GPUHang" {
		t.Fatalf("Expected spec name to be 'GPUHang', got '%s'", gpuHangRule.Name)
	}
	if len(gpuHangRule.Indicators) != 8 {
		t.Fatalf("Expected spec to have 8 Indicators, got %d", len(gpuHangRule.Indicators))
	}
	if len(gpuHangRule.IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuHangRule.IndicatorsByModel))
	}
	if gpuHangRule.Description != "GPU Hang" {
		t.Fatalf("Expected spec description to be 'GPU Hang', got '%s'", gpuHangRule.Description)
	}
	if gpuHangRule.DurationThreshold.Duration != 150*time.Second {
		// Note: The duration is 2m30s, which is 150 seconds
		t.Fatalf("Expected spec duration threshold to be 150s, got %v", gpuHangRule.DurationThreshold.Duration)
	}
	if gpuHangRule.Level != "warn" {
		t.Fatalf("Expected spec level to be 'warn', got '%s'", gpuHangRule.Level)
	}
	if device == "0x233010de" {
		if gpuHangRule.Indicators["pwr"].Threshold != 150 {
			t.Fatalf("Expected spec pwr threshold to be 150, got %d", gpuHangRule.Indicators["pwr"].Threshold)
		}
	} else {
		if gpuHangRule.Indicators["pwr"].Threshold != 100 {
			t.Fatalf("Expected spec pwr threshold to be 100, got %d", gpuHangRule.Indicators["pwr"].Threshold)
		}
	}
	if gpuHangRule.Indicators["pwr"].CompareType != "low" {
		t.Fatalf("Expected spec pwr compare type to be 'low', got '%s'", gpuHangRule.Indicators["pwr"].CompareType)
	}
	if device == "0x233010de" {
		if gpuHangRule.Indicators["gclk"].Threshold != 1900 {
			t.Fatalf("Expected spec gclk threshold to be 1900, got %d", gpuHangRule.Indicators["gclk"].Threshold)
		}
	} else {
		if gpuHangRule.Indicators["gclk"].Threshold != 1400 {
			t.Fatalf("Expected spec gclk threshold to be 1400, got %d", gpuHangRule.Indicators["gclk"].Threshold)
		}
	}
	if gpuHangRule.Indicators["gclk"].CompareType != "high" {
		t.Fatalf("Expected spec gclk compare type to be 'high', got '%s'", gpuHangRule.Indicators["gclk"].CompareType)
	}
	if device == "0x233010de" {
		if gpuHangRule.Indicators["smclk"].Threshold != 1900 {
			t.Fatalf("Expected spec smclk threshold to be 1900, got %d", gpuHangRule.Indicators["smclk"].Threshold)
		}
	} else {
		if gpuHangRule.Indicators["smclk"].Threshold != 1400 {
			t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuHangRule.Indicators["smclk"].Threshold)
		}
	}
	if gpuHangRule.Indicators["smclk"].CompareType != "high" {
		t.Fatalf("Expected spec smclk compare type to be 'high', got '%s'", gpuHangRule.Indicators["smclk"].CompareType)
	}
	if gpuHangRule.Indicators["sm"].Threshold != 95 {
		t.Fatalf("Expected spec sm threshold to be 95, got %d", gpuHangRule.Indicators["sm"].Threshold)
	}
	if gpuHangRule.Indicators["sm"].CompareType != "high" {
		t.Fatalf("Expected spec sm compare type to be 'high', got '%s'", gpuHangRule.Indicators["sm"].CompareType)
	}
	if gpuHangRule.Indicators["mem"].Threshold != 5 {
		t.Fatalf("Expected spec mem threshold to be 5, got %d", gpuHangRule.Indicators["mem"].Threshold)
	}
	if gpuHangRule.Indicators["mem"].CompareType != "low" {
		t.Fatalf("Expected spec mem compare type to be 'low', got '%s'", gpuHangRule.Indicators["mem"].CompareType)
	}
	if gpuHangRule.Indicators["pviol"].Threshold != 5 {
		t.Fatalf("Expected spec pviol threshold to be 5, got %d", gpuHangRule.Indicators["pviol"].Threshold)
	}
	if gpuHangRule.Indicators["pviol"].CompareType != "low" {
		t.Fatalf("Expected spec pviol compare type to be 'low', got '%s'", gpuHangRule.Indicators["pviol"].CompareType)
	}
	if gpuHangRule.Indicators["rxpci"].Threshold != 20 {
		t.Fatalf("Expected spec rxpci threshold to be 20, got %d", gpuHangRule.Indicators["rxpci"].Threshold)
	}
	if gpuHangRule.Indicators["rxpci"].CompareType != "low" {
		t.Fatalf("Expected spec rxpci compare type to be 'low', got '%s'", gpuHangRule.Indicators["rxpci"].CompareType)
	}
	if gpuHangRule.Indicators["txpci"].Threshold != 20 {
		t.Fatalf("Expected spec txpci threshold to be 20, got %d", gpuHangRule.Indicators["txpci"].Threshold)
	}
	if gpuHangRule.Indicators["txpci"].CompareType != "low" {
		t.Fatalf("Expected spec txpci compare type to be 'low', got '%s'", gpuHangRule.Indicators["txpci"].CompareType)
	}
	if gpuHangRule.IndicatorsByModel[0].Model != "0x233010de" {
		t.Fatalf("Expected first model to be '0x233010de', got '%s'", gpuHangRule.IndicatorsByModel[0].Model)
	}
	if len(gpuHangRule.IndicatorsByModel[0].Override) != 3 {
		t.Fatalf("Expected first model to have 3 overrides, got %d", len(gpuHangRule.IndicatorsByModel[0].Override))
	}
	if gpuHangRule.IndicatorsByModel[0].Override["pwr"].Threshold != 150 {
		t.Fatalf("Expected spec pwr override threshold to be 150, got %d", gpuHangRule.IndicatorsByModel[0].Override["pwr"].Threshold)
	}
	if gpuHangRule.IndicatorsByModel[0].Override["gclk"].Threshold != 1900 {
		t.Fatalf("Expected spec gclk override threshold to be 1900, got %d", gpuHangRule.IndicatorsByModel[0].Override["gclk"].Threshold)
	}
	if gpuHangRule.IndicatorsByModel[0].Override["smclk"].Threshold != 1900 {
		t.Fatalf("Expected spec smclk override threshold to be 1900, got %d", gpuHangRule.IndicatorsByModel[0].Override["smclk"].Threshold)
	}
}
