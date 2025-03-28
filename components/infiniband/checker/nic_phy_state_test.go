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
	"github.com/scitix/sichek/config/infiniband"	
	"github.com/scitix/sichek/consts"
)

func TestIBPhyStateChecker_Check(t *testing.T) {
	// 模拟 Spec 配置
	// 模拟 Spec 配置
	spec := &infiniband.InfinibandSpecItem{
		HCAs: map[string]*collector.IBHardWareInfo{
			"ib0": {
				IBDev:     "ib0",
				PhyState: "LinkUp",
			},
			"ib1": {
				IBDev:     "ib1",
				PhyState: "LinkUp",
			},
		},
	}


	// 创建 Checker 实例
	checker, err := NewIBPhyStateChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBPhyStateChecker: %v", err)
	}

	ibChecker := checker.(*IBPhyStateChecker)

	// 测试用例
	tests := []struct {
		name               string
		data               *collector.InfinibandInfo
		expectedStatus     string
		expectedLevel      string
		expectedDetail     string
		expectedSuggestion string
		expectedError      bool
	}{
		{
			name: "All devices are LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PhyState: "LinkUp"},
					{HCAType: "cx5", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus:     consts.StatusNormal,
			expectedLevel:      consts.LevelInfo,
			expectedDetail:     "all ib phy link status is up",
			expectedSuggestion: "",
			expectedError:      false,
		},
		{
			name: "No Infiniband devices found",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      infiniband.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     infiniband.NOIBFOUND,
			expectedSuggestion: "",
			expectedError:      true,
		},
		{
			name: "One device is not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "cx5", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      infiniband.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     "ib0 status is not LinkUp, curr:Down,LinkUp",
			expectedSuggestion: "check nic to up ib0 link status",
			expectedError:      false,
		},
		{
			name: "Multiple devices are not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "cx5", IBDev: "ib1", PhyState: "Init"},
				},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedLevel:      infiniband.InfinibandCheckItems[ibChecker.name].Level,
			expectedDetail:     "ib0,ib1 status is not LinkUp, curr:Down,Init",
			expectedSuggestion: "check nic to up ib0,ib1 link status",
			expectedError:      false,
		},
	}

	// 遍历测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行 Check 方法
			ctx := context.Background()
			result, err := ibChecker.Check(ctx, tt.data)

			// 验证错误状态
			if (err != nil) != tt.expectedError {
				t.Errorf("unexpected error state, got error=%v, expectedError=%v", err, tt.expectedError)
			}

			// 验证返回结果
			if result != nil {
				if result.Status != tt.expectedStatus {
					t.Errorf("unexpected status, got=%s, want=%s", result.Status, tt.expectedStatus)
				}

				if result.Level != tt.expectedLevel {
					t.Errorf("unexpected level, got=%s, want=%s", result.Level, tt.expectedLevel)
				}

				if result.Detail != tt.expectedDetail {
					t.Errorf("unexpected detail, got=%s, want=%s", result.Detail, tt.expectedDetail)
				}

				if result.Suggestion != tt.expectedSuggestion {
					t.Errorf("unexpected suggestion, got=%s, want=%s", result.Suggestion, tt.expectedSuggestion)
				}
			}
		})
	}
}
