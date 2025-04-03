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

func TestIBPhyStateChecker_Check(t *testing.T) {
	// 模拟 Spec 配置
	// 模拟 Spec 配置
	spec := &config.InfinibandSpecItem{
		HCAs: map[string]*collector.IBHardWareInfo{
			"MT_0000000970": {
				IBDev:    "MT_0000000970",
				BoardID:  "MT_0000000970",
				PhyState: "LinkUp",
			},
			"MT_0000001119": {
				IBDev:    "MT_0000001119",
				BoardID:  "MT_0000001119",
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
		name           string
		data           *collector.InfinibandInfo
		expectedStatus string
		expectedLevel  string
		expectedError  bool
	}{
		{
			name: "All devices are LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "LinkUp"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus: consts.StatusNormal,
			expectedLevel:  consts.LevelCritical,
			expectedError:  false,
		},
		{
			name: "No Infiniband devices found",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  true,
		},
		{
			name: "One device is not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "LinkUp"},
				},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  false,
		},
		{
			name: "Multiple devices are not LinkUp",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT_0000001119", BoardID: "MT_0000001119", IBDev: "ib0", PhyState: "Down"},
					{HCAType: "MT_0000000970", BoardID: "MT_0000000970", IBDev: "ib1", PhyState: "Init"},
				},
			},
			expectedStatus: consts.StatusAbnormal,
			expectedLevel:  config.InfinibandCheckItems[ibChecker.name].Level,
			expectedError:  false,
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
			}
		})
	}
}
