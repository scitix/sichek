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
	commonCfg "github.com/scitix/sichek/config"
)

// TestIBFirmwareChecker_Check 测试 IBFirmwareChecker 的 Check 方法
func TestIBFirmwareChecker_Check(t *testing.T) {
	// 模拟 Checker 的 Spec 配置
	spec := &config.InfinibandHCASpec{
		HWSpec: []config.HWSpec{
			{
				Type: "MT4129", // 设备类型
				Specifications: config.Specifications{
					FwVersion: "14.27.4002", // 符合的固件版本
				},
			},
			{
				Type: "MT4130",
				Specifications: config.Specifications{
					FwVersion: "16.42.0000",
				},
			},
		},
	}

	// 创建 Checker 实例
	checker, err := NewFirmwareChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBFirmwareChecker: %v", err)
	}

	ibChecker := checker.(*IBFirmwareChecker)

	// 定义测试用例
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
			name: "Normal case with correct firmware versions",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT4129", IBDev: "ib0", FWVer: "14.27.4002"},
					{HCAType: "MT4130", IBDev: "ib1", FWVer: "16.42.0000"},
				},
			},
			expectedStatus:     commonCfg.StatusNormal,
			expectedLevel:      commonCfg.LevelInfo,
			expectedDetail:     "all ib use the same fw version include in spec",
			expectedSuggestion: "",
			expectError:        false,
		},
		{
			name: "Error case - No IB devices found",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      commonCfg.LevelWarning,
			expectedDetail:     config.NOIBFOUND,
			expectedSuggestion: config.InfinibandCheckItems[ibChecker.name].Suggestion,
			expectError:        true,
		},
		{
			name: "Error case - Incorrect firmware version for some devices",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "MT4129", IBDev: "ib0", FWVer: "14.27.3000"},
					{HCAType: "MT4130", IBDev: "ib1", FWVer: "16.42.0000"},
				},
			},
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      commonCfg.LevelWarning,
			expectedDetail:     "ib0 fw is not in the spec, curr:14.27.3000,16.42.0000, spec:14.27.4002",
			expectedSuggestion: "use flint tool to burn ib0 fw ",
			expectError:        false,
		},
	}

	// 遍历所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建上下文
			ctx := context.Background()

			// 执行目标方法
			result, err := ibChecker.Check(ctx, tt.data)

			// 检查错误状态
			if (err != nil) != tt.expectError {
				t.Errorf("unexpected error status, got: %v, want error: %v", err, tt.expectError)
			}

			// 检查结果
			if result == nil {
				if !tt.expectError {
					t.Errorf("unexpected nil result")
				}
				return
			}

			// 检查返回的状态
			if result.Status != tt.expectedStatus {
				t.Errorf("unexpected status, got: %s, want: %s", result.Status, tt.expectedStatus)
			}

			// 检查级别
			if result.Level != tt.expectedLevel {
				t.Errorf("unexpected level, got: %s, want: %s", result.Level, tt.expectedLevel)
			}

			// 检查详细信息
			if result.Detail != tt.expectedDetail {
				t.Errorf("unexpected detail, got: %s, want: %s", result.Detail, tt.expectedDetail)
			}

			// 检查建议
			if result.Suggestion != tt.expectedSuggestion {
				t.Errorf("unexpected suggestion, got: %s, want: %s", result.Suggestion, tt.expectedSuggestion)
			}
		})
	}
}
