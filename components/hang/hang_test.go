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
package hang

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestHang(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create temporary files for testing
	configFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp spec file: %v", err)
		}
	}(configFile.Name())

	// Write config data to the temporary files
	configData := `
hang:
  query_interval: 30
  cache_size: 5
  nvsmi: false
  mock: true
  enable_metrics: true
`
	if _, err := configFile.Write([]byte(configData)); err != nil {
		t.Fatalf("Failed to write to temp config file: %v", err)
	}
		
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

	// Write spec data to the temporary files
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
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	start := time.Now()
	component, err := NewComponent(configFile.Name(), specFile.Name())
	if err != nil {
		t.Log(err)
		return
	}

	for i := 0; i < 100; i++ {
		_, err := component.HealthCheck(ctx)
		if err != nil {
			t.Error(err)
		}
	}
	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)

	if err != nil {
		t.Log(err)
	}
	js, err := result.JSON()
	if err != nil {
		t.Log(err)
	}
	t.Logf("test hang analysis result: %s", js)
	t.Logf("Running time: %ds", time.Since(start))
}
