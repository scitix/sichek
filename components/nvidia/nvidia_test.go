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
package nvidia

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/consts"
)

func TestCheckInitError_Classification(t *testing.T) {
	tests := []struct {
		name          string
		initError     error
		wantReported  bool
		wantErrorName string
	}{
		{
			name:         "no init error",
			initError:    nil,
			wantReported: false,
		},
		{
			name:          "nvidia-smi probe timeout maps to NvidiaSMITimeout",
			initError:     fmt.Errorf("failed to create nvidia collector: %w", fmt.Errorf("wrap: %w", collector.ErrSWInfoProbeTimeout)),
			wantReported:  true,
			wantErrorName: consts.NvidiaSMITimeoutErrName,
		},
		{
			name:          "generic init failure stays InitError",
			initError:     fmt.Errorf("spec loading failed: boom"),
			wantReported:  true,
			wantErrorName: "InitError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &component{componentName: consts.ComponentNameNvidia, initError: tt.initError}
			result, ok := c.checkInitError()
			if ok != tt.wantReported {
				t.Fatalf("checkInitError reported=%v, want %v", ok, tt.wantReported)
			}
			if !tt.wantReported {
				return
			}
			if len(result.Checkers) != 1 {
				t.Fatalf("expected 1 checker result, got %d", len(result.Checkers))
			}
			cr := result.Checkers[0]
			if cr.ErrorName != tt.wantErrorName || cr.Name != tt.wantErrorName {
				t.Fatalf("ErrorName/Name = %q/%q, want %q", cr.ErrorName, cr.Name, tt.wantErrorName)
			}
			if cr.Level != consts.LevelCritical {
				t.Fatalf("Level = %q, want %q", cr.Level, consts.LevelCritical)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	component, err := NewComponent("", "", nil)
	if err != nil {
		t.Fatalf("failed to create Nvidia component: %v", err)
	}
	result, err := common.RunHealthCheckWithTimeout(ctx, component.GetTimeout(), component.Name(), component.HealthCheck)
	if err != nil {
		t.Fatalf("failed to Nvidia HealthCheck: %v", err)
	}
	output := common.ToString(result)
	t.Logf("Nvidia Analysis Result: \n%s", output)
}

func TestSplit(t *testing.T) {
	input := ":gpu"
	result := strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])

	input = "device:"
	result = strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])

	input = "device:gpu"
	result = strings.Split(input, ":")
	if len(result) != 2 {
		t.Fatalf("failed to split string")
	}
	t.Logf("the first: %s, the second: %s", result[0], result[1])
}
