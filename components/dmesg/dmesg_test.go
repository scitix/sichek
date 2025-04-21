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
package dmesg

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
)

func TestDmesg(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
dmesg:
  query_interval: 30
  cache_size: 5
  enable_metrics: false

memory:
  query_interval: 30
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
dmesg: 
  file_name: ["/var/log/dmesg"]
  # cmd:
  #   - ["dmesg"]
  event_checkers:
    SysOOM:
      name: "sys_oom"
      description: "oom error in dmesg"
      regexp: 'Out of memory:'
      level: error
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

	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)
	if err != nil {
		t.Log(err)
	}
	js, errr := result.JSON()
	if errr != nil {
		t.Log(errr)
	}
	t.Logf("test dmesg analysis result: %s", js)
	t.Logf("Running time: %ds", time.Since(start))
}
