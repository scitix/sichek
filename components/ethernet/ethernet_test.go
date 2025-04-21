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
package ethernet

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create temporary files for testing
	configFile, err := os.CreateTemp("", "cfg_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp config file: %v", err)
		}
	}(configFile.Name())

	// Write config data to the temporary files
	configData := `
ethernet:
  query_interval: 30s
  cache_size: 5
  enable_metrics: false

memory:
  query_interval: 30s
  cache_size: 5
  enable_metrics: false
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
ethernet:
  hw_spec:
  - model: "Connectx_5"
    type: "MT4117"
    specifications:
      mode:
      - "infiniband"
      phy_state: "up"
      port_speed: "200Gbps"
      fw:
      - "28.39.2048"
      - "28.39.2049"
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	component, err := NewEthernetComponent(configFile.Name(), specFile.Name())
	if err != nil {
		t.Fatalf("failed to create Ethernet component: %v", err)
	}
	if err != nil {
		t.Fatalf("failed to create Ethernet component: %v", err)
	}
	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)
	if err != nil {
		t.Fatalf("failed to Ethernet HealthCheck: %v", err)
		return
	}
	output := common.ToString(result)
	t.Logf("Ethernet Analysis Result: \n%s", output)
}
