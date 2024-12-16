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
	"os"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"
)

func TestIBOFEDChecker_Check(t *testing.T) {
	// 模拟 Spec 配置
	spec := &config.InfinibandHCASpec{
		SoftwareDependencies: config.SoftwareDependencies{
			OFED: []config.OFEDSpec{
				{NodeName: "css2", OFEDVer: "5.8-1.0.1.1"},
				{NodeName: "node2", OFEDVer: "5.3-3.0.0.0"},
			},
		},
	}

	// 创建 Checker 实例
	checker, err := NewIBOFEDChecker(spec)
	if err != nil {
		t.Fatalf("failed to create IBOFEDChecker: %v", err)
	}

	ofedChecker := checker.(*IBOFEDChecker)

	// Mock 主机名
	originalHostname, _ := os.Hostname()
	defer func() {
		_ = os.Setenv("HOSTNAME", originalHostname)
	}()

	tests := []struct {
		name               string
		mockHostname       string
		data               *collector.InfinibandInfo
		expectedStatus     string
		expectedLevel      string
		expectedDetail     string
		expectedSuggestion string
		expectedError      bool
	}{
		{
			name:         "OFED version matches spec",
			mockHostname: "test-node",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0"},
				},
				IBSoftWareInfo: collector.IBSoftWareInfo{
					OFEDVer: "5.8-1.0.1.1",
				},
			},
			expectedStatus:     commonCfg.StatusNormal,
			expectedLevel:      commonCfg.LevelInfo,
			expectedDetail:     "the ofed is right version",
			expectedSuggestion: "",
			expectedError:      false,
		},
		{
			name:         "OFED version doesn't match spec",
			mockHostname: "test-node",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0"},
				},
				IBSoftWareInfo: collector.IBSoftWareInfo{
					OFEDVer: "5.3-3.0.0.0",
				},
			},
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ofedChecker.name].Level,
			expectedDetail:     "OFED version mismatch, expected:5.8-1.0.1.1, current:5.3-3.0.0.0",
			expectedSuggestion: "update the OFED version",
			expectedError:      false,
		},
		{
			name:         "Host name does not match spec",
			mockHostname: "unknown-host",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{
					{HCAType: "mlx5", IBDev: "ib0"},
				},
				IBSoftWareInfo: collector.IBSoftWareInfo{
					OFEDVer: "5.3-3.0.0.0",
				},
			},
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ofedChecker.name].Level,
			expectedDetail:     "host name does not match any spec, curr:unknown-host",
			expectedSuggestion: "check the default configuration",
			expectedError:      false,
		},
		{
			name:         "No hardware found",
			mockHostname: "test-node",
			data: &collector.InfinibandInfo{
				IBHardWareInfo: []collector.IBHardWareInfo{},
			},
			expectedStatus:     commonCfg.StatusAbnormal,
			expectedLevel:      config.InfinibandCheckItems[ofedChecker.name].Level,
			expectedDetail:     config.NOIBFOUND,
			expectedSuggestion: "",
			expectedError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 Mock 主机名
			_ = os.Setenv("HOSTNAME", tt.mockHostname)

			// 执行检测逻辑
			ctx := context.Background()
			result, err := ofedChecker.Check(ctx, tt.data)

			// 验证错误状态
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error state: %v; got: %v", tt.expectedError, err)
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
