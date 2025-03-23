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
package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/config/cpu"
)

func TestCollector_Collect(t *testing.T) {
	// Mock context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock configuration
	cfg := &cpu.CPUConfig{
		EventCheckers: map[string]*cpu.CPUEventConfig{
			"testChecker": {
				Name:        "testChecker",
				Description: "Test checker",
				LogFile:     "/var/log/test.log",
				Regexp:      "test",
				Level:       "warning",
				Suggestion:  "test suggestion",
			},
		},
	}

	// Create a temporary log file for testing in a goroutine
	var logFile *os.File
	var err error
	logFile, err = os.CreateTemp("", "test.log")
	if err != nil {
		t.Fatalf("Failed to create temp log file: %v", err)
	}
	t.Logf("Log file: %s", logFile.Name())
	// Write some test data to the log file
	_, err = logFile.WriteString("test log data\n")
	if err != nil {
		t.Fatalf("Failed to write to temp log file: %+v", err)
	}
	logFile.Close()

	// Start a goroutine to write to the log file continuously
	go func() {
		for {
			select {
			case <-ctx.Done():
				os.Remove(logFile.Name())
				return
			default:
				file, err := os.OpenFile(logFile.Name(), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Errorf("Failed to open log file: %v", err)
					return
				}
				_, err = file.WriteString("additional test log data\n")
				if err != nil {
					t.Errorf("Failed to write to log file: %v", err)
					file.Close()
					return
				}
				file.Close()
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Update the log file path in the configuration
	cfg.CPU.EventCheckers["testChecker"].LogFile = logFile.Name()
	t.Logf("Event checkers: %+v", cfg.CPU.EventCheckers["testChecker"])
	// Create a new collector instance
	collector, err := NewCpuCollector(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}
	// Read the current content of the log file
	content, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file0: %v", err)
	}
	t.Logf("Current log file content: %s", string(content))

	collector.Collect(ctx)
	time.Sleep(2 * time.Second)
	// Read the current content of the log file
	content, err = os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file1: %v", err)
	}
	t.Logf("Current log file content: %s", string(content))
	// Call the Collect method
	info, err := collector.Collect(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the results
	cpuOutput, ok := info.(*CPUOutput)
	if !ok {
		t.Fatalf("Expected CPUOutput, got %T", info)
	}

	if cpuOutput.Time.IsZero() {
		t.Errorf("Expected non-zero time, got zero")
	}

	if len(cpuOutput.CPUArchInfo.NumaNodeInfo) == 0 {
		t.Errorf("Expected non-empty CPUArchInfo, got empty")
	}

	if cpuOutput.HostInfo == (HostInfo{}) {
		t.Errorf("Expected non-empty HostInfo, got empty")
	}

	if cpuOutput.Uptime == "" {
		t.Errorf("Expected non-empty uptime, got empty")
	}

	if len(cpuOutput.EventResults) == 0 {
		t.Errorf("Expected non-empty event results, got empty")
	}

	t.Logf("CPUOutput: %+v", cpuOutput)
}

func TestGetUptime(t *testing.T) {
	// Call GetUptime and verify the result
	uptime, err := GetUptime()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	t.Logf("Uptime: %s", uptime)
}
