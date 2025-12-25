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

	"github.com/scitix/sichek/components/nvidia/utils"
)

func TestLoadEventRules(t *testing.T) {
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
gpuevents:
  GPUHang:
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
  SmClkStuckLow:
    name: "SmClkStuckLow"
    description: "SM clock too low for long time"
    duration_threshold: 15m
    level: fatal
    check_items:
      smclk:
        threshold: 800
        compare: low
    check_items_by_model:
      - model: "0x233010de"
        override:
          smclk:
            threshold: 1000
            compare: low
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	// Test the LoadEventRules function
	gpuCostomEventRules, err := LoadEventRules(specFile.Name())
	if err != nil {
		t.Fatalf("LoadEventRules() returned an error: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(gpuCostomEventRules, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	device, err := utils.GetDeviceID()
	if err != nil {
		t.Fatalf("Failed to GetDeviceID: %v", err)
	}
	if gpuCostomEventRules["GPUHang"].Name != "GPUHang" {
		t.Fatalf("Expected spec name to be 'GPUHang', got '%s'", gpuCostomEventRules["GPUHang"].Name)
	}
	if len(gpuCostomEventRules["GPUHang"].Indicators) != 8 {
		t.Fatalf("Expected spec to have 8 Indicators, got %d", len(gpuCostomEventRules["GPUHang"].Indicators))
	}
	if len(gpuCostomEventRules["GPUHang"].IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuCostomEventRules["GPUHang"].IndicatorsByModel))
	}
	if gpuCostomEventRules["GPUHang"].Description != "GPU Hang" {
		t.Fatalf("Expected spec description to be 'GPU Hang', got '%s'", gpuCostomEventRules["GPUHang"].Description)
	}
	if gpuCostomEventRules["GPUHang"].DurationThreshold.Duration != 150*time.Second {
		// Note: The duration is 2m30s, which is 150 seconds
		t.Fatalf("Expected spec duration threshold to be 150s, got %v", gpuCostomEventRules["GPUHang"].DurationThreshold.Duration)
	}
	if gpuCostomEventRules["GPUHang"].Level != "warn" {
		t.Fatalf("Expected spec level to be 'warn', got '%s'", gpuCostomEventRules["GPUHang"].Level)
	}
	if device == "0x233010de" {
		if gpuCostomEventRules["GPUHang"].Indicators["pwr"].Threshold != 150 {
			t.Fatalf("Expected spec pwr threshold to be 150, got %d", gpuCostomEventRules["GPUHang"].Indicators["pwr"].Threshold)
		}
	} else {
		if gpuCostomEventRules["GPUHang"].Indicators["pwr"].Threshold != 100 {
			t.Fatalf("Expected spec pwr threshold to be 100, got %d", gpuCostomEventRules["GPUHang"].Indicators["pwr"].Threshold)
		}
	}
	if gpuCostomEventRules["GPUHang"].Indicators["pwr"].CompareType != "low" {
		t.Fatalf("Expected spec pwr compare type to be 'low', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["pwr"].CompareType)
	}
	if device == "0x233010de" {
		if gpuCostomEventRules["GPUHang"].Indicators["gclk"].Threshold != 1900 {
			t.Fatalf("Expected spec gclk threshold to be 1900, got %d", gpuCostomEventRules["GPUHang"].Indicators["gclk"].Threshold)
		}
	} else {
		if gpuCostomEventRules["GPUHang"].Indicators["gclk"].Threshold != 1400 {
			t.Fatalf("Expected spec gclk threshold to be 1400, got %d", gpuCostomEventRules["GPUHang"].Indicators["gclk"].Threshold)
		}
	}
	if gpuCostomEventRules["GPUHang"].Indicators["gclk"].CompareType != "high" {
		t.Fatalf("Expected spec gclk compare type to be 'high', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["gclk"].CompareType)
	}
	if device == "0x233010de" {
		if gpuCostomEventRules["GPUHang"].Indicators["smclk"].Threshold != 1900 {
			t.Fatalf("Expected spec smclk threshold to be 1900, got %d", gpuCostomEventRules["GPUHang"].Indicators["smclk"].Threshold)
		}
	} else {
		if gpuCostomEventRules["GPUHang"].Indicators["smclk"].Threshold != 1400 {
			t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuCostomEventRules["GPUHang"].Indicators["smclk"].Threshold)
		}
	}
	if gpuCostomEventRules["GPUHang"].Indicators["smclk"].CompareType != "high" {
		t.Fatalf("Expected spec smclk compare type to be 'high', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["smclk"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["sm"].Threshold != 95 {
		t.Fatalf("Expected spec sm threshold to be 95, got %d", gpuCostomEventRules["GPUHang"].Indicators["sm"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["sm"].CompareType != "high" {
		t.Fatalf("Expected spec sm compare type to be 'high', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["sm"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["mem"].Threshold != 5 {
		t.Fatalf("Expected spec mem threshold to be 5, got %d", gpuCostomEventRules["GPUHang"].Indicators["mem"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["mem"].CompareType != "low" {
		t.Fatalf("Expected spec mem compare type to be 'low', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["mem"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["pviol"].Threshold != 5 {
		t.Fatalf("Expected spec pviol threshold to be 5, got %d", gpuCostomEventRules["GPUHang"].Indicators["pviol"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["pviol"].CompareType != "low" {
		t.Fatalf("Expected spec pviol compare type to be 'low', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["pviol"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["rxpci"].Threshold != 20 {
		t.Fatalf("Expected spec rxpci threshold to be 20, got %d", gpuCostomEventRules["GPUHang"].Indicators["rxpci"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["rxpci"].CompareType != "low" {
		t.Fatalf("Expected spec rxpci compare type to be 'low', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["rxpci"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["txpci"].Threshold != 20 {
		t.Fatalf("Expected spec txpci threshold to be 20, got %d", gpuCostomEventRules["GPUHang"].Indicators["txpci"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].Indicators["txpci"].CompareType != "low" {
		t.Fatalf("Expected spec txpci compare type to be 'low', got '%s'", gpuCostomEventRules["GPUHang"].Indicators["txpci"].CompareType)
	}
	if gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Model != "0x233010de" {
		t.Fatalf("Expected first model to be '0x233010de', got '%s'", gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Model)
	}
	if len(gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override) != 3 {
		t.Fatalf("Expected first model to have 3 overrides, got %d", len(gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override))
	}
	if gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["pwr"].Threshold != 150 {
		t.Fatalf("Expected spec pwr override threshold to be 150, got %d", gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["pwr"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["gclk"].Threshold != 1900 {
		t.Fatalf("Expected spec gclk override threshold to be 1900, got %d", gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["gclk"].Threshold)
	}
	if gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["smclk"].Threshold != 1900 {
		t.Fatalf("Expected spec smclk override threshold to be 1900, got %d", gpuCostomEventRules["GPUHang"].IndicatorsByModel[0].Override["smclk"].Threshold)
	}
	if gpuCostomEventRules["SmClkStuckLow"].Name != "SmClkStuckLow" {
		t.Fatalf("Expected spec name to be 'SmClkStuckLow', got '%s'", gpuCostomEventRules["SmClkStuckLow"].Name)
	}
	if gpuCostomEventRules["SmClkStuckLow"].Description != "SM clock too low for long time" {
		t.Fatalf("Expected spec description to be 'SM clock too low for long time', got '%s'", gpuCostomEventRules["SmClkStuckLow"].Description)
	}
	if gpuCostomEventRules["SmClkStuckLow"].DurationThreshold.Duration != 15*time.Minute {
		t.Fatalf("Expected spec duration threshold to be 15 minutes, got %v", gpuCostomEventRules["SmClkStuckLow"].DurationThreshold.Duration)
	}
	if gpuCostomEventRules["SmClkStuckLow"].Level != "fatal" {
		t.Fatalf("Expected spec level to be 'fatal', got '%s'", gpuCostomEventRules["SmClkStuckLow"].Level)
	}
	if len(gpuCostomEventRules["SmClkStuckLow"].Indicators) != 1 {
		t.Fatalf("Expected spec to have 1 Indicator, got %d", len(gpuCostomEventRules["SmClkStuckLow"].Indicators))
	}
	if device == "0x233010de" {
		if gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].Threshold != 1000 {
			t.Fatalf("Expected spec smclk threshold to be 1900, got %d", gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].Threshold)
		}
	} else {
		if gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].Threshold != 800 {
			t.Fatalf("Expected spec smclk threshold to be 1400, got %d", gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].Threshold)
		}
	}
	if gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].CompareType != "low" {
		t.Fatalf("Expected spec smclk compare type to be 'low', got '%s'", gpuCostomEventRules["SmClkStuckLow"].Indicators["smclk"].CompareType)
	}
	if len(gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel) != 1 {
		t.Fatalf("Expected spec to have 1 IndicatorsByModel, got %d", len(gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel))
	}
	if gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Model != "0x233010de" {
		t.Fatalf("Expected first model to be '0x233010de', got '%s'", gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Model)
	}
	if len(gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Override) != 1 {
		t.Fatalf("Expected first model to have 1 overrides, got %d", len(gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Override))
	}
	if gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Override["smclk"].Threshold != 1000 {
		t.Fatalf("Expected spec smclk override threshold to be 1000, got %d", gpuCostomEventRules["SmClkStuckLow"].IndicatorsByModel[0].Override["smclk"].Threshold)
	}
}
