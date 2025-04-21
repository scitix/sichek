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
package cpu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"

	"github.com/sirupsen/logrus"
)

func TestHealthCheck(t *testing.T) {
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
cpu:
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
cpu:  
  event_checkers:
    kernel_panic:
      name: "kernel_panic"
      description: "kernel panic are alerted"
      log_file: "/var/log/syslog"
      regexp: "kernel panic"
      level: critical
      suggestion: "restart node"
    cpu_overheating:
      name: "cpu_overheating"
      description: "CPU Core temperature is above threshold, cpu clock is throttled"
      log_file: "/var/log/syslog"
      regexp: "temperature above threshold"
      level: warning
      suggestion: ""
    cpu_lockup:
      name: "cpu_lockup"
      description: "CPU lockup occurs indicating the CPU cannot execute scheduled tasks due to software or hardware issues"
      log_file: "/var/log/syslog"
      regexp: "(soft lockup)|(hard LOCKUP)"
      level: warning
      suggestion: ""
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	component, err := NewComponent(configFile.Name(), specFile.Name())
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("create cpu component failed: %v", err)
		return
	}

	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("analyze cpu failed: %v", err)
		return
	}

	logrus.WithField("component", "cpu").Infof("cpu analysis result: \n%s", common.ToString(result))
}
