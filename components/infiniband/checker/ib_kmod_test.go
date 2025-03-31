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

func TestIBKmodChecker_Check(t *testing.T) {
	// 模拟配置文件 Spec
	spec := &config.InfinibandSpecItem{
		IBSoftWareInfo: &collector.IBSoftWareInfo{
			KernelModule: []string{"mlx5_core", "ib_uverbs", "rdma_ucm"},
		},
	}

	// 创建 Checker 实例
	checker, err := NewIBKmodChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBKmodChecker: %v", err)
	}

	ibChecker := checker.(*IBKmodChecker)

	// 定义测试用例
	tests := []struct {
		name               string
		data               *collector.InfinibandInfo
		expectedStatus     string
		expectedDetail     string
		expectedSuggestion string
		expectError        bool
	}{
		{
			name: "Normal case with all necessary kernel modules installed",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0"},
				},
				IBSoftWareInfo: collector.IBSoftWareInfo{
					KernelModule: []string{"mlx5_core", "ib_uverbs", "rdma_ucm"},
				},
			},
			expectedStatus:     consts.StatusNormal,
			expectedDetail:     config.InfinibandCheckItems[ibChecker.name].Detail,
			expectedSuggestion: "",
			expectError:        false,
		},
		{
			name: config.NOIBFOUND,
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedDetail:     config.NOIBFOUND,
			expectedSuggestion: "need check the kernel module is all installed",
			expectError:        true,
		},
		{
			name: "Missing required kernel modules",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0"},
				},
				IBSoftWareInfo: collector.IBSoftWareInfo{
					KernelModule: []string{"ib_uverbs"},
				},
			},
			expectedStatus:     consts.StatusAbnormal,
			expectedDetail:     "need to install kmod:mlx5_core,rdma_ucm",
			expectedSuggestion: "use modprobe to install kmod:mlx5_core,rdma_ucm",
			expectError:        false,
		},
	}

	// 遍历所有的测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 执行 Check 方法
			ctx := context.Background()
			result, err := ibChecker.Check(ctx, tt.data)

			// 检查错误
			if (err != nil) != tt.expectError {
				t.Errorf("unexpected error status: got %v, want error=%v", err, tt.expectError)
			}

			// 检查结果
			if result.Status != tt.expectedStatus {
				t.Errorf("unexpected status: got %s, want %s", result.Status, tt.expectedStatus)
			}
			if result.Detail != tt.expectedDetail {
				t.Errorf("unexpected detail: got %s, want %s", result.Detail, tt.expectedDetail)
			}
			if result.Suggestion != tt.expectedSuggestion {
				t.Errorf("unexpected suggestion: got %s, want %s", result.Suggestion, tt.expectedSuggestion)
			}
		})
	}
}
