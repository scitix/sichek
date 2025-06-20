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
	"testing"
)

func TestCollector_Collect(t *testing.T) {
	// Mock context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new collector instance
	collector, err := NewCpuCollector(ctx)
	if err != nil {
		t.Fatalf("Failed to create collector: %v", err)
	}
	// Call the Collect method
	info, err := collector.Collect(ctx)
	if err != nil {
		t.Fatalf("Failed to collect info: %v", err)
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
