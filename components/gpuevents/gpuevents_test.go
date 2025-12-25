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
package gpuevents

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

func TestGPUEvents(t *testing.T) {
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
gpuevents:
  query_interval: 30s
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

	// start := time.Now()
	component, err := NewComponent(configFile.Name(), specFile.Name())
	if err != nil {
		t.Log(err)
		return
	}
	// 32 times mock_nv 30s to ensure the hang and SmClkStuckLow is detected
	for i := 0; i < 32; i++ {
		t.Logf("Running health check for %s, attempt %d", component.Name(), i+1)
		result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)

		if err != nil {
			t.Log(err)
		}
		js, err := result.JSON()
		if err != nil {
			t.Log(err)
		}
		t.Logf("test gpuevents analysis result: %s", js)
		if i > 13 {
			if result.Status != consts.StatusAbnormal {
				t.Errorf("Expected StatusAbnormal, got %s", result.Status)
			}
			for _, checker := range result.Checkers {
				if checker.Name == "GPUHang" {
					if checker.Status != consts.StatusAbnormal {
						t.Errorf("Expected GPUHang StatusAbnormal, got %s", checker.Status)
					}
					break
				}
			}
		}
		if i > 30 {
			if result.Status != consts.StatusAbnormal {
				t.Errorf("Expected StatusAbnormal, got %s", result.Status)
			}
			for _, checker := range result.Checkers {
				if checker.Name == "SmClkStuckLow" {
					if checker.Status != consts.StatusAbnormal {
						t.Errorf("Expected SmClkStuckLow StatusAbnormal, got %s", checker.Status)
					}
					break
				}
			}
		}
	}
}
