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

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"	
	"github.com/scitix/sichek/consts"
)

func TestIBStateChecker_Check(t *testing.T) {
	// 模拟 Spec 配置
	spec := &config.InfinibandSpecItem{
		HCAs: map[string]*collector.IBHardWareInfo{
			"mlx5": {
				IBDev:     "mlx5",
				PortState: "ACTIVE",
			},
			"cx6dx": {
				IBDev:     "cx6dx",
				PortState: "ACTIVE",
			},
		},
	}

	// 创建 Checker 实例
	checker, err := NewIBStateChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBStateChecker: %v", err)
	}

	ibChecker := checker.(*IBStateChecker)

	// 测试用例
	tests := []struct {
		name               string
		data               *collector.InfinibandInfo
		expectedStatus     string
		expectedLevel      string
		expectedDetail     string
		expectedSuggestion string
		expectError        bool
	}{
		{
			name: "Normal case with all ports ACTIVE",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PortState: "ACTIVE"},
					{HCAType: "cx6dx", IBDev: "ib1", PortState: "ACTIVE"},
				},
			},
			expectedStatus:     consts.StatusNormal,
			expectedLevel:      consts.LevelInfo,
			expectedDetail:     "all ib state is active",
			expectedSuggestion: "",
			expectError:        false,
		},
		{
			name: "Error case - No IB devices found",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     config.NOIBFOUND,
			expectedSuggestion: "",
			expectError:        true,
		},
		{
			name: "Error case - One device with non-ACTIVE port state",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PortState: "DOWN"},
					{HCAType: "cx6dx", IBDev: "ib1", PortState: "ACTIVE"},
				},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     "ib0 status is not ACTIVE, curr status:DOWN,ACTIVE",
			expectedSuggestion: "check opensm to active ib0 status",
			expectError:        false,
		},
		{
			name: "Error case - Multiple devices with non-ACTIVE port states",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PortState: "DOWN"},
					{HCAType: "cx6dx", IBDev: "ib1", PortState: "INIT"},
				},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     "ib0,ib1 status is not ACTIVE, curr status:DOWN,INIT",
			expectedSuggestion: "check opensm to active ib0,ib1 status",
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx := context.Background()
			result, err := ibChecker.Check(ctx, tt.data)

			if (err != nil) != tt.expectError {
				t.Errorf("unexpected error status, got error=%v, want error=%v", err, tt.expectError)
			}

			if result != nil {
				if result.Status != tt.expectedStatus {
					t.Errorf("unexpected status, got %s, want %s", result.Status, tt.expectedStatus)
				}
				if result.Level != tt.expectedLevel {
					t.Errorf("unexpected level, got %s, want %s", result.Level, tt.expectedLevel)
				}
				if result.Detail != tt.expectedDetail {
					t.Errorf("unexpected detail, got %s, want %s", result.Detail, tt.expectedDetail)
				}
				if result.Suggestion != tt.expectedSuggestion {
					t.Errorf("unexpected suggestion, got %s, want %s", result.Suggestion, tt.expectedSuggestion)
				}
			}
		})
	}
}
