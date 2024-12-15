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
package checker

import (
	"context"
	"testing"
	"time"

	commonCfg "github.com/scitix/sichek/config"
)

func TestCPUPerfChecker_Check(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// disable perfomance mode for testing
	// echo performance > /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
	err := setCPUMode("powersave")
	if err != nil {
		t.Fatalf("failed to set CPU governor to powersave: %v", err)
	}
	cpu_performance_enable, _ := checkCPUPerformance()
	if cpu_performance_enable {
		t.Fatalf("unexpected cpu_performance_enable")
	}

	// Create a new CPUPerfChecker
	checker, err := NewCPUPerfChecker()
	if err != nil {
		t.Fatalf("failed to create CPUPerfChecker: %v", err)
	}

	result, _ := checker.Check(ctx, nil)
	if result.Status != commonCfg.StatusNormal {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}